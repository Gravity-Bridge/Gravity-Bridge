package ante

import (
	"errors"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/crypto"

	client "github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	sdkante "github.com/cosmos/cosmos-sdk/x/auth/ante"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	"github.com/cosmos/cosmos-sdk/x/auth/migrations/legacytx"
	authsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
	authtx "github.com/cosmos/cosmos-sdk/x/auth/tx"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"

	eip712signer "github.com/ethereum/go-ethereum/signer/core/apitypes"
	ethante "github.com/evmos/ethermint/app/ante"
	"github.com/evmos/ethermint/crypto/ethsecp256k1"
	"github.com/evmos/ethermint/ethereum/eip712"
	testtx "github.com/evmos/ethermint/tests"
	etherminttypes "github.com/evmos/ethermint/types"

	gravityconfig "github.com/Gravity-Bridge/Gravity-Bridge/module/config"
	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/keeper"
	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
)

func MakeAnteHandler(t *testing.T, input keeper.TestInput) sdk.AnteHandler {
	gk := input.GravityKeeper
	ak := input.AccountKeeper
	bk := input.BankKeeper
	ibck := input.IbcKeeper
	encodingConfig := input.EncodingConfig

	options := sdkante.HandlerOptions{
		AccountKeeper:          ak,
		BankKeeper:             bk,
		FeegrantKeeper:         nil,
		SignModeHandler:        encodingConfig.TxConfig.SignModeHandler(),
		SigGasConsumer:         ethante.DefaultSigVerificationGasConsumer,
		ExtensionOptionChecker: nil,
		TxFeeChecker:           nil,
	}
	ah, err := NewAnteHandler(options, &gk, &ak, &bk, nil, &ibck, input.Marshaler, gravityconfig.GravityEvmChainID)
	require.NoError(t, err)
	require.NotNil(t, ah)
	return *ah
}

// Tests that a regular MsgSend and an EIP712-signed MsgSend are accepted by the ante handler
func TestAnteHandlerHappy(t *testing.T) {
	input, ctx := keeper.SetupFiveValChain(t)
	ah := MakeAnteHandler(t, input)

	// Create an eth_secp256k1 account and fund it
	ethPrivkey, err := ethsecp256k1.GenerateKey()
	require.NoError(t, err)
	ethBalance := sdk.NewCoins(sdk.NewInt64Coin("stake", 100))
	require.NoError(t, input.BankKeeper.MintCoins(input.Context, types.ModuleName, ethBalance))
	address := sdk.AccAddress(ethPrivkey.PubKey().Address())
	require.NoError(t, input.BankKeeper.SendCoinsFromModuleToAccount(input.Context, types.ModuleName, address, ethBalance))

	// Build a normal cosmos transaction
	var noExtensionTx sdk.Tx = BuildRegularTx(t, ctx, input.EncodingConfig.TxConfig, input.AccountKeeper)

	// Build an EIP712 signed cosmos transaction
	oneExtensionTxBuilder, _ := BuildWeb3ExtensionTx(t, ctx, input.EncodingConfig.TxConfig, input.AccountKeeper, ethPrivkey)
	var oneExtensionTx sdk.Tx = oneExtensionTxBuilder.GetTx()

	_, err = ah(ctx, noExtensionTx, false)
	require.NoError(t, err)

	_, err = ah(ctx, oneExtensionTx, false)
	require.NoError(t, err)
}

// Creates an invalid EIP712 transaction which contains the EIP712 signature twice in the extension options
func TestTwoWeb3Extensions(t *testing.T) {
	input, ctx := keeper.SetupFiveValChain(t)
	ah := MakeAnteHandler(t, input)

	// Create an eth_secp256k1 account and fund it
	ethPrivkey, err := ethsecp256k1.GenerateKey()
	require.NoError(t, err)
	ethBalance := sdk.NewCoins(sdk.NewInt64Coin("stake", 100))
	require.NoError(t, input.BankKeeper.MintCoins(input.Context, types.ModuleName, ethBalance))
	address := sdk.AccAddress(ethPrivkey.PubKey().Address())
	require.NoError(t, input.BankKeeper.SendCoinsFromModuleToAccount(input.Context, types.ModuleName, address, ethBalance))

	var twoExtensionTx sdk.Tx = BuildInvalidWeb3ExtensionTx(t, ctx, input.EncodingConfig.TxConfig, input.AccountKeeper, ethPrivkey)

	_, err = ah(ctx, twoExtensionTx, false)
	require.Error(t, err)
}

// Util function to build a standard-signed MsgSend tx
func BuildRegularTx(t *testing.T, ctx sdk.Context, txConfig client.TxConfig, ak authkeeper.AccountKeeper) sdk.Tx {
	tx := txConfig.NewTxBuilder()
	msg := banktypes.NewMsgSend(keeper.AccAddrs[0], keeper.AccAddrs[1], sdk.NewCoins(sdk.NewInt64Coin("stake", 100)))

	if err := tx.SetMsgs(msg); err != nil {
		panic(err)
	}

	tx.SetFeeAmount(sdk.NewCoins())
	tx.SetGasLimit(2000000)
	tx.SetTimeoutHeight(0)

	account := ak.GetAccount(ctx, keeper.AccAddrs[0])
	an := account.GetAccountNumber()
	s := account.GetSequence()
	err := SignRegular(t, tx, signing.SignMode_SIGN_MODE_DIRECT, txConfig, ctx.ChainID(), an, s)
	require.NoError(t, err)

	return tx.GetTx()
}

// Signs the WIP Tx in txBuilder using the standard cosmos signing style
func SignRegular(t *testing.T, txBuilder client.TxBuilder, signMode signing.SignMode, txConfig client.TxConfig, chainID string, accNum uint64, seq uint64) error {
	if signMode == signing.SignMode_SIGN_MODE_UNSPECIFIED {
		// use the SignModeHandler's default mode if unspecified
		signMode = txConfig.SignModeHandler().DefaultMode()
	}
	pubKey := keeper.AccPubKeys[0]
	// nolint: exhaustruct
	signerData := authsigning.SignerData{
		ChainID:       chainID,
		AccountNumber: accNum,
		Sequence:      seq,
	}

	sigData := signing.SingleSignatureData{
		SignMode:  signMode,
		Signature: nil,
	}
	sig := signing.SignatureV2{
		PubKey:   pubKey,
		Data:     &sigData,
		Sequence: seq,
	}
	if err := txBuilder.SetSignatures(sig); err != nil {
		return err
	}

	// Generate the bytes to be signed.
	bytesToSign, err := txConfig.SignModeHandler().GetSignBytes(signMode, signerData, txBuilder.GetTx())
	if err != nil {
		return err
	}

	privKey := keeper.AccPrivKeys[0]
	// Sign those bytes
	sigBytes, err := privKey.Sign(bytesToSign)
	if err != nil {
		return err
	}

	// Construct the SignatureV2 struct
	sigData = signing.SingleSignatureData{
		SignMode:  signMode,
		Signature: sigBytes,
	}
	sig = signing.SignatureV2{
		PubKey:   pubKey,
		Data:     &sigData,
		Sequence: seq,
	}

	return txBuilder.SetSignatures(sig)
}

// Builds a MsgSend tx signed via EIP712. Returns the TxBuilder and the Web3ExtensionOption containing the signature
func BuildWeb3ExtensionTx(t *testing.T, ctx sdk.Context, txConfig client.TxConfig, ak authkeeper.AccountKeeper, priv cryptotypes.PrivKey) (client.TxBuilder, *codectypes.Any) {
	tx := txConfig.NewTxBuilder()

	address := sdk.AccAddress(priv.PubKey().Address())
	msg := banktypes.NewMsgSend(address, keeper.AccAddrs[1], sdk.NewCoins(sdk.NewInt64Coin("stake", 100)))

	if err := tx.SetMsgs(msg); err != nil {
		panic(err)
	}

	fees := sdk.NewCoins()
	gasAmount := uint64(2000000)
	chainId, err := strconv.ParseUint(gravityconfig.GravityEvmChainID, 10, 64)
	require.NoError(t, err)
	tx.SetFeeAmount(fees)
	tx.SetGasLimit(gasAmount)
	tx.SetTimeoutHeight(uint64(ctx.BlockHeight() + 100))

	txBuilder, option, err := SignEip712(t, ak, ctx, priv, chainId, gasAmount, fees, []sdk.Msg{msg}, txConfig)
	require.NoError(t, err)

	return txBuilder, option
}

// Takes a valid EIP712-signed Tx and adds another Web3ExtensionOption to it so that the Tx is invalid
func BuildInvalidWeb3ExtensionTx(t *testing.T, ctx sdk.Context, txConfig client.TxConfig, ak authkeeper.AccountKeeper, priv cryptotypes.PrivKey) sdk.Tx {
	txBuilder, option := BuildWeb3ExtensionTx(t, ctx, txConfig, ak, priv)
	extensionBuilder := txBuilder.(authtx.ExtensionOptionsTxBuilder)
	extensionBuilder.SetExtensionOptions(option, option)

	return extensionBuilder.GetTx()
}

// Creates a TxBuilder containing a Tx that holds the given `msgs` with an EIP712 signature
// Returns the WIP Tx as a TxBuilder and the Web3ExtensionOption containing the signature
func SignEip712(
	t *testing.T,
	ak authkeeper.AccountKeeper,
	ctx sdk.Context,
	privKey cryptotypes.PrivKey,
	chainId uint64,
	gas uint64,
	fees sdk.Coins,
	msgs []sdk.Msg,
	txCfg client.TxConfig,
) (client.TxBuilder, *codectypes.Any, error) {
	from := sdk.AccAddress(privKey.PubKey().Address())
	acc := ak.GetAccount(ctx, from)

	accNumber := acc.GetAccountNumber()

	nonce, err := ak.GetSequence(ctx, from)
	if err != nil {
		return nil, nil, err
	}

	fee := legacytx.NewStdFee(gas, fees) //nolint: staticcheck

	data := legacytx.StdSignBytes(ctx.ChainID(), accNumber, nonce, 0, fee, msgs, "", nil)

	typedData, err := CreateTypedData(chainId, msgs[0], data, from)
	if err != nil {
		return nil, nil, err
	}

	txBuilder := txCfg.NewTxBuilder()
	builder, ok := txBuilder.(authtx.ExtensionOptionsTxBuilder)
	if !ok {
		return nil, nil, errors.New("txBuilder could not be casted to authtx.ExtensionOptionsTxBuilder type")
	}

	builder.SetFeeAmount(fee.Amount)
	builder.SetGasLimit(gas)

	err = builder.SetMsgs(msgs...)
	if err != nil {
		return nil, nil, err
	}

	sigHash, _, err := eip712signer.TypedDataAndHash(typedData)
	if err != nil {
		return nil, nil, err
	}

	keyringSigner := testtx.NewSigner(privKey)
	signature, pubKey, err := keyringSigner.SignByAddress(from, sigHash)
	if err != nil {
		return nil, nil, err
	}
	signature[crypto.RecoveryIDOffset] += 27 // Transform V from 0/1 to 27/28 according to the yellow paper

	// Add ExtensionOptionsWeb3Tx extension
	var option *codectypes.Any
	option, err = codectypes.NewAnyWithValue(&etherminttypes.ExtensionOptionsWeb3Tx{
		FeePayer:         from.String(),
		TypedDataChainID: chainId,
		FeePayerSig:      signature,
	})
	require.NoError(t, err)

	builder.SetExtensionOptions(option)
	builder.SetFeeAmount(fees)
	builder.SetGasLimit(gas)

	// nolint: exhaustruct
	sigsV2 := signing.SignatureV2{
		PubKey: pubKey,
		Data: &signing.SingleSignatureData{
			SignMode: signing.SignMode_SIGN_MODE_LEGACY_AMINO_JSON,
		},
		Sequence: nonce,
	}

	err = builder.SetSignatures(sigsV2)
	require.NoError(t, err)

	err = builder.SetMsgs(msgs...)
	require.NoError(t, err)

	return builder, option, nil
}

// This is a helper function used to convert a Tx (as signData) to TypedData derived from the msg type (needed for EIP712 signing)
func CreateTypedData(chainID uint64, msg sdk.Msg, signData []byte, from sdk.AccAddress) (eip712signer.TypedData, error) {
	registry := codectypes.NewInterfaceRegistry()
	etherminttypes.RegisterInterfaces(registry)
	cryptocodec.RegisterInterfaces(registry)
	ethermintCodec := codec.NewProtoCodec(registry)

	return eip712.LegacyWrapTxToTypedData(
		ethermintCodec,
		chainID,
		msg,
		signData,
		&eip712.FeeDelegationOptions{
			FeePayer: from,
		},
	)
}
