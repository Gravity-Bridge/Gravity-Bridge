// This file is a modified version of the original file from the Evmos Ethermint repo.
// the changes to the upstream file have been marked with special comments describing the
// deviations made for Gravity
package ante

import (
	"fmt"
	"slices"

	errorsmod "cosmossdk.io/errors"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	errortypes "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	authante "github.com/cosmos/cosmos-sdk/x/auth/ante"
	"github.com/cosmos/cosmos-sdk/x/auth/migrations/legacytx"
	authsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"

	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/crypto/secp256k1"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"

	"github.com/evmos/ethermint/crypto/ethsecp256k1"
	"github.com/evmos/ethermint/ethereum/eip712"
	ethermint "github.com/evmos/ethermint/types"
	evmtypes "github.com/evmos/ethermint/x/evm/types"

	gravitykeeper "github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/keeper"
	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
)

var RELAXED_CHAIN_ID_MESSAGE_TYPES = []string{sdk.MsgTypeURL(&types.MsgSendToEth{}), sdk.MsgTypeURL(&types.MsgCancelSendToEth{})}

var cdc codec.ProtoCodecMarshaler

func init() {
	registry := codectypes.NewInterfaceRegistry()
	ethermint.RegisterInterfaces(registry)
	cdc = codec.NewProtoCodec(registry)
}

// GRAVITY CHANGES: Gravity uses the legacy EIP712 signature verification for Cosmos transactions
// and notably accepts bridging transactions from foreign chains. The evmChainId parameter is used
// for local EIP712 signature verification, while the bridgeForeignChainIDs parameter is used to
// accept bridging transactions from foreign chain IDs.
//
// LegacyEip712SigVerificationDecorator Verify all signatures for a tx and return an error if any are invalid. Note,
// the LegacyEip712SigVerificationDecorator decorator will not get executed on ReCheck.
// NOTE: As of v0.20.0, EIP-712 signature verification is handled by the ethsecp256k1 public key (see ethsecp256k1.go)
//
// CONTRACT: Pubkeys are set in context for all signers before this decorator runs
// CONTRACT: Tx must implement SigVerifiableTx interface
type LegacyEip712SigVerificationDecorator struct {
	ak              evmtypes.AccountKeeper
	gravityKeeper   gravitykeeper.Keeper
	signModeHandler authsigning.SignModeHandler
	evmChainId      uint64
}

// GRAVITY CHANGES: The bridgeForeignChainIDs parameter is used to accept bridging transactions from foreign chain IDs.
//
// Deprecated: NewLegacyEip712SigVerificationDecorator creates a new LegacyEip712SigVerificationDecorator
// Allows an optional EVM Chain ID (e.g. 1 for Ethereum Mainnet). The upstream Evmos repo requires the Cosmos Chain ID
// to be parseable by a regex but evmChainId removes that restriction by allowing an override.
func NewLegacyEip712SigVerificationDecorator(
	ak evmtypes.AccountKeeper,
	gk gravitykeeper.Keeper,
	signModeHandler authsigning.SignModeHandler,
	evmChainId uint64,
) LegacyEip712SigVerificationDecorator {
	return LegacyEip712SigVerificationDecorator{
		ak:              ak,
		gravityKeeper:   gk,
		signModeHandler: signModeHandler,
		evmChainId:      evmChainId,
	}
}

// GRAVITY CHANGES: This antehandler performs the legacy EIP712 signature verification for Cosmos transactions but
// also accepts transactions containing a singular Msg with one of the hard-coded Gravity bridging msg types for which
// the the chain ID requirement is relaxed to include the bridgeForeignChainIDs.
//
// is used to accept bridging transactions from foreign chain IDs.
//
// AnteHandle handles validation of EIP712 signed cosmos txs.
// it is not run on RecheckTx
// Note that this decorator differs from the upstream repo by removing the chain ID format restriction
func (svd LegacyEip712SigVerificationDecorator) AnteHandle(ctx sdk.Context,
	tx sdk.Tx,
	simulate bool,
	next sdk.AnteHandler,
) (newCtx sdk.Context, err error) {
	// no need to verify signatures on recheck tx
	if ctx.IsReCheckTx() {
		return next(ctx, tx, simulate)
	}

	sigTx, ok := tx.(authsigning.SigVerifiableTx)
	if !ok {
		return ctx, errorsmod.Wrapf(errortypes.ErrInvalidType, "tx %T doesn't implement authsigning.SigVerifiableTx", tx)
	}

	authSignTx, ok := tx.(authsigning.Tx)
	if !ok {
		return ctx, errorsmod.Wrapf(errortypes.ErrInvalidType, "tx %T doesn't implement the authsigning.Tx interface", tx)
	}

	// stdSigs contains the sequence number, account number, and signatures.
	// When simulating, this would just be a 0-length slice.
	sigs, err := sigTx.GetSignaturesV2()
	if err != nil {
		return ctx, err
	}

	signerAddrs := sigTx.GetSigners()

	// EIP712 allows just one signature
	if len(sigs) != 1 {
		return ctx, errorsmod.Wrapf(
			errortypes.ErrTooManySignatures,
			"invalid number of signers (%d);  EIP712 signatures allows just one signature",
			len(sigs),
		)
	}

	// check that signer length and signature length are the same
	if len(sigs) != len(signerAddrs) {
		return ctx, errorsmod.Wrapf(errortypes.ErrorInvalidSigner, "invalid number of signers;  expected: %d, got %d", len(signerAddrs), len(sigs))
	}

	// EIP712 has just one signature, avoid looping here and only read index 0
	i := 0
	sig := sigs[i]

	acc, err := authante.GetSignerAcc(ctx, svd.ak, signerAddrs[i])
	if err != nil {
		return ctx, err
	}

	// retrieve pubkey
	pubKey := acc.GetPubKey()
	if !simulate && pubKey == nil {
		return ctx, errorsmod.Wrap(errortypes.ErrInvalidPubKey, "pubkey on account is not set")
	}

	// Check account sequence number.
	if sig.Sequence != acc.GetSequence() {
		return ctx, errorsmod.Wrapf(
			errortypes.ErrWrongSequence,
			"account sequence mismatch, expected %d, got %d", acc.GetSequence(), sig.Sequence,
		)
	}

	// retrieve signer data
	genesis := ctx.BlockHeight() == 0
	chainID := ctx.ChainID()

	var accNum uint64
	if !genesis {
		accNum = acc.GetAccountNumber()
	}

	// There are two ChainIDs to verify, the Cosmos string and the EVM number. First try the EVM override value,
	// then try to parse if no override provided
	var evmChainID uint64 = svd.evmChainId
	if evmChainID == 0 {
		cId, err := ethermint.ParseChainID(chainID)

		if err != nil {
			return ctx, errorsmod.Wrapf(err, "failed to parse chainID: %s", chainID)
		}
		evmChainID = cId.Uint64()
	}

	signerData := authsigning.SignerData{
		ChainID:       chainID,
		AccountNumber: accNum,
		Sequence:      acc.GetSequence(),
	}

	if simulate {
		return next(ctx, tx, simulate)
	}

	params, err := svd.gravityKeeper.GetParamsIfSet(ctx)
	if err != nil {
		return ctx, errorsmod.Wrapf(err, "failed to get Gravity params")
	}
	if err := VerifySignature(pubKey, signerData, sig.Data, svd.signModeHandler, authSignTx, evmChainID, params.Eip712BridgeForeignChainIds); err != nil {
		errMsg := fmt.Errorf("signature verification failed; please verify account number (%d) and chain-id (%s) and evm-chain-id (%d): %w", accNum, chainID, evmChainID, err)
		return ctx, errorsmod.Wrap(errortypes.ErrUnauthorized, errMsg.Error())
	}

	return next(ctx, tx, simulate)
}

// VerifySignature verifies a transaction signature contained in SignatureData abstracting over different signing modes
// and single vs multi-signatures.
// Note that this decorator differs from the upstream repo by removing the chain ID format restriction, so signerData.ChainID is not parsed.
// The evmChainID parameter is used instead of Cosmos ChainID parsing
func VerifySignature(
	pubKey cryptotypes.PubKey,
	signerData authsigning.SignerData,
	sigData signing.SignatureData,
	_ authsigning.SignModeHandler,
	tx authsigning.Tx,
	evmChainID uint64,
	bridgeForeignChainIDs []uint64,
) error {
	switch data := sigData.(type) {
	case *signing.SingleSignatureData:
		if data.SignMode != signing.SignMode_SIGN_MODE_LEGACY_AMINO_JSON {
			return errorsmod.Wrapf(errortypes.ErrNotSupported, "unexpected SignatureData %T: wrong SignMode", sigData)
		}

		// Note: this prevents the user from sending trash data in the signature field
		if len(data.Signature) != 0 {
			return errorsmod.Wrap(errortypes.ErrTooManySignatures, "invalid signature value; EIP712 must have the cosmos transaction signature empty")
		}

		// @contract: this code is reached only when Msg has Web3Tx extension (so this custom Ante handler flow),
		// and the signature is SIGN_MODE_LEGACY_AMINO_JSON which is supported for EIP712 for now

		msgs := tx.GetMsgs()
		if len(msgs) == 0 {
			return errorsmod.Wrap(errortypes.ErrNoSignatures, "tx doesn't contain any msgs to verify signature")
		}

		txBytes := legacytx.StdSignBytes(
			signerData.ChainID,
			signerData.AccountNumber,
			signerData.Sequence,
			tx.GetTimeoutHeight(),
			legacytx.StdFee{
				Amount: tx.GetFee(),
				Gas:    tx.GetGas(),
			},
			msgs, tx.GetMemo(), tx.GetTip(),
		)

		txWithExtensions, ok := tx.(authante.HasExtensionOptionsTx)
		if !ok {
			return errorsmod.Wrap(errortypes.ErrUnknownExtensionOptions, "tx doesnt contain any extensions")
		}
		opts := txWithExtensions.GetExtensionOptions()
		if len(opts) != 1 {
			return errorsmod.Wrap(errortypes.ErrUnknownExtensionOptions, "tx doesnt contain expected amount of extension options")
		}

		extOpt, ok := opts[0].GetCachedValue().(*ethermint.ExtensionOptionsWeb3Tx)
		if !ok {
			return errorsmod.Wrap(errortypes.ErrUnknownExtensionOptions, "unknown extension option")
		}

		typedDataChainID := extOpt.TypedDataChainID
		acceptableChainId := false
		foreignChainIdTransaction := false

		// Gravity allows single-msg bridging transactions from whitelisted foreign chain IDs
		foreignChainIDsAllowed := len(msgs) == 1 && slices.Contains(RELAXED_CHAIN_ID_MESSAGE_TYPES, sdk.MsgTypeURL(msgs[0]))

		if foreignChainIDsAllowed && slices.Contains(bridgeForeignChainIDs, typedDataChainID) {
			acceptableChainId = true
			foreignChainIdTransaction = true
		} else if typedDataChainID == evmChainID {
			acceptableChainId = true
		}

		if !acceptableChainId {
			if foreignChainIdTransaction {
				return errorsmod.Wrapf(errortypes.ErrInvalidChainID, "eip-712 foreign chainID: (%d) unacceptable", typedDataChainID)
			} else {
				return errorsmod.Wrapf(errortypes.ErrInvalidChainID, "eip-712 domain chainID: (%d) != (%d)", typedDataChainID, evmChainID)
			}
		}

		if len(extOpt.FeePayer) == 0 {
			return errorsmod.Wrap(errortypes.ErrUnknownExtensionOptions, "no feePayer on ExtensionOptionsWeb3Tx")
		}
		feePayer, err := sdk.AccAddressFromBech32(extOpt.FeePayer)
		if err != nil {
			return errorsmod.Wrap(err, "failed to parse feePayer from ExtensionOptionsWeb3Tx")
		}

		feeDelegation := &eip712.FeeDelegationOptions{
			FeePayer: feePayer,
		}

		typedData, err := eip712.LegacyWrapTxToTypedData(cdc, extOpt.TypedDataChainID, msgs[0], txBytes, feeDelegation)
		if err != nil {
			return errorsmod.Wrap(err, "failed to create EIP-712 typed data from tx")
		}

		sigHash, _, err := apitypes.TypedDataAndHash(typedData)
		if err != nil {
			return err
		}

		feePayerSig := extOpt.FeePayerSig
		if len(feePayerSig) != ethcrypto.SignatureLength {
			return errorsmod.Wrap(errortypes.ErrorInvalidSigner, "signature length doesn't match typical [R||S||V] signature 65 bytes")
		}

		// Remove the recovery offset if needed (ie. Metamask eip712 signature)
		if feePayerSig[ethcrypto.RecoveryIDOffset] == 27 || feePayerSig[ethcrypto.RecoveryIDOffset] == 28 {
			feePayerSig[ethcrypto.RecoveryIDOffset] -= 27
		}

		feePayerPubkey, err := secp256k1.RecoverPubkey(sigHash, feePayerSig)
		if err != nil {
			return errorsmod.Wrap(err, "failed to recover delegated fee payer from sig")
		}

		ecPubKey, err := ethcrypto.UnmarshalPubkey(feePayerPubkey)
		if err != nil {
			return errorsmod.Wrap(err, "failed to unmarshal recovered fee payer pubkey")
		}

		pk := &ethsecp256k1.PubKey{
			Key: ethcrypto.CompressPubkey(ecPubKey),
		}

		if !pubKey.Equals(pk) {
			return errorsmod.Wrapf(errortypes.ErrInvalidPubKey, "feePayer pubkey %s is different from transaction pubkey %s", pubKey, pk)
		}

		recoveredFeePayerAcc := sdk.AccAddress(pk.Address().Bytes())

		if !recoveredFeePayerAcc.Equals(feePayer) {
			return errorsmod.Wrapf(errortypes.ErrorInvalidSigner, "failed to verify delegated fee payer %s signature", recoveredFeePayerAcc)
		}

		// VerifySignature of ethsecp256k1 accepts 64 byte signature [R||S]
		// WARNING! Under NO CIRCUMSTANCES try to use pubKey.VerifySignature there
		if !secp256k1.VerifySignature(pubKey.Bytes(), sigHash, feePayerSig[:len(feePayerSig)-1]) {
			return errorsmod.Wrap(errortypes.ErrorInvalidSigner, "unable to verify signer signature of EIP712 typed data")
		}

		return nil
	default:
		return errorsmod.Wrapf(errortypes.ErrTooManySignatures, "unexpected SignatureData %T", sigData)
	}
}
