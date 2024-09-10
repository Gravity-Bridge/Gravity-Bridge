package ante

import (
	"errors"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/crypto"

	sdkclient "github.com/cosmos/cosmos-sdk/client"
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
	ah, err := NewAnteHandler(options, &gk, &ak, &bk, nil, &ibck, input.Marshaler, gravityconfig.GravityEvmChainID, gravityconfig.BridgeForeignChainIDs)
	require.NoError(t, err)
	require.NotNil(t, ah)
	return *ah
}

// Tests that a regular MsgSend and an EIP712-signed MsgSend are accepted by the ante handler
func TestSendAnteHandlerHappy(t *testing.T) {
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

// Tests that a regular-signed Msg[Cancel]SendToEth and an EIP712-signed Msg[Cancel]SendToEth are accepted by the ante handler
func TestBridgeAnteHandlerHappy(t *testing.T) {
	input, ctx := keeper.SetupFiveValChain(t)
	ah := MakeAnteHandler(t, input)
	gravityEvmChainId, err := strconv.ParseUint(gravityconfig.GravityEvmChainID, 10, 64)
	require.NoError(t, err)
	foreignChain0, err := strconv.ParseUint(gravityconfig.BridgeForeignChainIDs[0], 10, 64)
	require.NoError(t, err)
	foreignChain1, err := strconv.ParseUint(gravityconfig.BridgeForeignChainIDs[1], 10, 64)
	require.NoError(t, err)

	// Create an eth_secp256k1 account and fund it
	ethPrivkey, err := ethsecp256k1.GenerateKey()
	require.NoError(t, err)
	ethBalance := sdk.NewCoins(sdk.NewInt64Coin("stake", 100))
	require.NoError(t, input.BankKeeper.MintCoins(input.Context, types.ModuleName, ethBalance))
	address := sdk.AccAddress(ethPrivkey.PubKey().Address())
	require.NoError(t, input.BankKeeper.SendCoinsFromModuleToAccount(input.Context, types.ModuleName, address, ethBalance))

	// Normal bridge transactions
	var noExtensionTx sdk.Tx = BuildRegularBridgeTx(t, ctx, input.EncodingConfig.TxConfig, input.AccountKeeper, false)
	_, err = ah(ctx, noExtensionTx, false)
	require.NoError(t, err)
	var noExtensionCancelTx sdk.Tx = BuildRegularBridgeTx(t, ctx, input.EncodingConfig.TxConfig, input.AccountKeeper, true)
	_, err = ah(ctx, noExtensionCancelTx, false)
	require.NoError(t, err)

	// EIP712 signed transactions (local chain ID)
	oneExtensionTxBuilder, _ := BuildBridgeWeb3ExtensionTx(t, ctx, input.EncodingConfig.TxConfig, input.AccountKeeper, ethPrivkey, gravityEvmChainId, false)
	var oneExtensionTx sdk.Tx = oneExtensionTxBuilder.GetTx()
	_, err = ah(ctx, oneExtensionTx, false)
	require.NoError(t, err)

	oneExtensionCancelTxBuilder, _ := BuildBridgeWeb3ExtensionTx(t, ctx, input.EncodingConfig.TxConfig, input.AccountKeeper, ethPrivkey, gravityEvmChainId, true)
	var oneExtensionCancelTx sdk.Tx = oneExtensionCancelTxBuilder.GetTx()
	_, err = ah(ctx, oneExtensionCancelTx, false)
	require.NoError(t, err)

	// EIP712 signed transactions (foreign chain IDs)
	foreignSendTxBuilder, _ := BuildBridgeWeb3ExtensionTx(t, ctx, input.EncodingConfig.TxConfig, input.AccountKeeper, ethPrivkey, foreignChain0, false)
	var foreignSendTx sdk.Tx = foreignSendTxBuilder.GetTx()
	_, err = ah(ctx, foreignSendTx, false)
	require.NoError(t, err)

	foreignCancelTxBuilder, _ := BuildBridgeWeb3ExtensionTx(t, ctx, input.EncodingConfig.TxConfig, input.AccountKeeper, ethPrivkey, foreignChain1, true)
	var foreignCancelTx sdk.Tx = foreignCancelTxBuilder.GetTx()
	_, err = ah(ctx, foreignCancelTx, false)
	require.NoError(t, err)
}

func TestSendAnteHandlerStrangeCases(t *testing.T) {
	input, ctx := keeper.SetupFiveValChain(t)
	badIdCtx := ctx.WithChainID("gruvity-brij-2")
	ah := MakeAnteHandler(t, input)
	// gravityEvmChainId, err := strconv.ParseUint(gravityconfig.GravityEvmChainID, 10, 64)
	// require.NoError(t, err)

	// Create an eth_secp256k1 account and fund it
	ethPrivkey, err := ethsecp256k1.GenerateKey()
	require.NoError(t, err)
	ethBalance := sdk.NewCoins(sdk.NewInt64Coin("stake", 100))
	require.NoError(t, input.BankKeeper.MintCoins(input.Context, types.ModuleName, ethBalance))
	address := sdk.AccAddress(ethPrivkey.PubKey().Address())
	require.NoError(t, input.BankKeeper.SendCoinsFromModuleToAccount(input.Context, types.ModuleName, address, ethBalance))

	// Build a normal tx but with the wrong chain id
	var noExtensionTx sdk.Tx = BuildRegularTx(t, badIdCtx, input.EncodingConfig.TxConfig, input.AccountKeeper)
	_, err = ah(ctx, noExtensionTx, false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "please verify account number")
	require.Contains(t, err.Error(), "and chain-id")

	// Build an EIP712 signed cosmos transaction but with the wrong cosmos chain id
	oneExtensionTxBuilder, _ := BuildWeb3ExtensionTx(t, badIdCtx, input.EncodingConfig.TxConfig, input.AccountKeeper, ethPrivkey)
	var oneExtensionTx sdk.Tx = oneExtensionTxBuilder.GetTx()
	_, err = ah(ctx, oneExtensionTx, false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "is different from transaction pubkey")
}

func TestBridgeAnteHandlerStrangeCases(t *testing.T) {
	input, ctx := keeper.SetupFiveValChain(t)
	ah := MakeAnteHandler(t, input)

	// Create an eth_secp256k1 account and fund it
	ethPrivkey, err := ethsecp256k1.GenerateKey()
	require.NoError(t, err)
	ethBalance := sdk.NewCoins(sdk.NewInt64Coin("stake", 100))
	require.NoError(t, input.BankKeeper.MintCoins(input.Context, types.ModuleName, ethBalance))
	address := sdk.AccAddress(ethPrivkey.PubKey().Address())
	require.NoError(t, input.BankKeeper.SendCoinsFromModuleToAccount(input.Context, types.ModuleName, address, ethBalance))

	// EIP712 signed cosmos transactions (wrong foreign chain IDs)
	foreignSendTxBuilder, _ := BuildBridgeWeb3ExtensionTx(t, ctx, input.EncodingConfig.TxConfig, input.AccountKeeper, ethPrivkey, uint64(78910), false)
	var foreignSendTx sdk.Tx = foreignSendTxBuilder.GetTx()
	_, err = ah(ctx, foreignSendTx, false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid chain-id")

	foreignCancelTxBuilder, _ := BuildBridgeWeb3ExtensionTx(t, ctx, input.EncodingConfig.TxConfig, input.AccountKeeper, ethPrivkey, uint64(101112), true)
	var foreignCancelTx sdk.Tx = foreignCancelTxBuilder.GetTx()
	_, err = ah(ctx, foreignCancelTx, false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid chain-id")
}

// Creates invalid EIP712 transactions which contain the EIP712 signature twice in the extension options
func TestTwoWeb3Extensions(t *testing.T) {
	input, ctx := keeper.SetupFiveValChain(t)
	badIdCtx := ctx.WithChainID("gruvity-brij-2")
	ah := MakeAnteHandler(t, input)
	gravityEvmChainId, err := strconv.ParseUint(gravityconfig.GravityEvmChainID, 10, 64)
	require.NoError(t, err)
	foreignChain0, err := strconv.ParseUint(gravityconfig.BridgeForeignChainIDs[0], 10, 64)
	require.NoError(t, err)
	foreignChain1, err := strconv.ParseUint(gravityconfig.BridgeForeignChainIDs[1], 10, 64)
	require.NoError(t, err)

	// Create an eth_secp256k1 account and fund it
	ethPrivkey, err := ethsecp256k1.GenerateKey()
	require.NoError(t, err)
	ethBalance := sdk.NewCoins(sdk.NewInt64Coin("stake", 100))
	require.NoError(t, input.BankKeeper.MintCoins(input.Context, types.ModuleName, ethBalance))
	address := sdk.AccAddress(ethPrivkey.PubKey().Address())
	require.NoError(t, input.BankKeeper.SendCoinsFromModuleToAccount(input.Context, types.ModuleName, address, ethBalance))

	// Create an invalid EIP712-signed Send transaction with the signature added twice
	var twoExtensionTx sdk.Tx = BuildInvalidWeb3ExtensionTx(t, ctx, input.EncodingConfig.TxConfig, input.AccountKeeper, ethPrivkey)

	_, err = ah(ctx, twoExtensionTx, false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid transaction extension options")

	// Create an invalid EIP712-signed Send transaction with the wrong chain id and signature added twice
	var twoExtensionBadIdTx sdk.Tx = BuildInvalidWeb3ExtensionTx(t, badIdCtx, input.EncodingConfig.TxConfig, input.AccountKeeper, ethPrivkey)

	_, err = ah(ctx, twoExtensionBadIdTx, false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid transaction extension options")

	// Create invalid bridge transactions with the local chain ID and signature added twice
	var twoExtensionBridgeTx sdk.Tx = BuildInvalidBridgeWeb3ExtensionTx(t, ctx, input.EncodingConfig.TxConfig, input.AccountKeeper, ethPrivkey, gravityEvmChainId, false)
	_, err = ah(ctx, twoExtensionBridgeTx, false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid transaction extension options")

	var twoExtensionCancelBridgeTx sdk.Tx = BuildInvalidBridgeWeb3ExtensionTx(t, ctx, input.EncodingConfig.TxConfig, input.AccountKeeper, ethPrivkey, gravityEvmChainId, true)
	_, err = ah(ctx, twoExtensionCancelBridgeTx, false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid transaction extension options")

	var twoExtensionForeignBridgeTx sdk.Tx = BuildInvalidBridgeWeb3ExtensionTx(t, ctx, input.EncodingConfig.TxConfig, input.AccountKeeper, ethPrivkey, foreignChain0, false)
	_, err = ah(ctx, twoExtensionForeignBridgeTx, false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid transaction extension options")

	var twoExtensionForeignCancelBridgeTx sdk.Tx = BuildInvalidBridgeWeb3ExtensionTx(t, ctx, input.EncodingConfig.TxConfig, input.AccountKeeper, ethPrivkey, foreignChain1, true)
	_, err = ah(ctx, twoExtensionForeignCancelBridgeTx, false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid transaction extension options")
}

// Creates a franken-transaction which has been signed with the regular cosmos style and adds an EIP712 extension to it
func TestBothSignMethods(t *testing.T) {
	input, ctx := keeper.SetupFiveValChain(t)
	ah := MakeAnteHandler(t, input)

	// Create an eth_secp256k1 account and fund it
	ethPrivkey, err := ethsecp256k1.GenerateKey()
	require.NoError(t, err)
	ethBalance := sdk.NewCoins(sdk.NewInt64Coin("stake", 100))
	require.NoError(t, input.BankKeeper.MintCoins(input.Context, types.ModuleName, ethBalance))
	address := sdk.AccAddress(ethPrivkey.PubKey().Address())
	require.NoError(t, input.BankKeeper.SendCoinsFromModuleToAccount(input.Context, types.ModuleName, address, ethBalance))

	// Create a cosmos-signed MsgSend and add the EIP712 extension to it
	var bothSendBuilder sdkclient.TxBuilder = RegularTxBuilder(t, ctx, input.EncodingConfig.TxConfig, input.AccountKeeper)
	var _, eip712Option = BuildWeb3ExtensionTx(t, ctx, input.EncodingConfig.TxConfig, input.AccountKeeper, ethPrivkey)
	bothSendBuilder.(authtx.ExtensionOptionsTxBuilder).SetExtensionOptions(eip712Option)
	bothSendTx := bothSendBuilder.GetTx()
	_, err = ah(ctx, bothSendTx, false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unexpected SignatureData")
}

// Creates transactions of varying styles with multiple messages to test foreign EIP712 chain ids only work with single msg transactions
func TestMultiMsgTxs(t *testing.T) {
	input, ctx := keeper.SetupFiveValChain(t)
	ah := MakeAnteHandler(t, input)
	gravityEvmChainId, err := strconv.ParseUint(gravityconfig.GravityEvmChainID, 10, 64)
	require.NoError(t, err)
	foreignChain0, err := strconv.ParseUint(gravityconfig.BridgeForeignChainIDs[0], 10, 64)
	require.NoError(t, err)
	foreignChain1, err := strconv.ParseUint(gravityconfig.BridgeForeignChainIDs[1], 10, 64)
	require.NoError(t, err)

	// Create an eth_secp256k1 account and fund it
	ethPrivkey, err := ethsecp256k1.GenerateKey()
	require.NoError(t, err)
	ethBalance := sdk.NewCoins(sdk.NewInt64Coin("stake", 100))
	require.NoError(t, input.BankKeeper.MintCoins(input.Context, types.ModuleName, ethBalance))
	address := sdk.AccAddress(ethPrivkey.PubKey().Address())
	require.NoError(t, input.BankKeeper.SendCoinsFromModuleToAccount(input.Context, types.ModuleName, address, ethBalance))

	// Regular Cosmos Tx with two MsgSend
	regularSendMultiTx := BuildRegularMultiMsgTx(t, ctx, input.EncodingConfig.TxConfig, input.AccountKeeper)
	_, err = ah(ctx, regularSendMultiTx, false)
	require.NoError(t, err)

	// EIP712 Tx with two MsgSend
	eipSendMultiTxBuilder, _ := BuildWeb3ExtensionMultiMsgTx(t, ctx, input.EncodingConfig.TxConfig, input.AccountKeeper, ethPrivkey)
	eipSendMultiTx := eipSendMultiTxBuilder.GetTx()
	_, err = ah(ctx, eipSendMultiTx, false)
	require.NoError(t, err)

	// Regular Cosmos Tx with two bridge msgs
	regularBridgeMultiTx := BuildRegularMultiMsgBridgeTx(t, ctx, input.EncodingConfig.TxConfig, input.AccountKeeper)
	_, err = ah(ctx, regularBridgeMultiTx, false)
	require.NoError(t, err)

	// EIP712 Tx with two bridge msgs, local chain ID
	eipBridgeMultiBuilder, _ := BuildBridgeWeb3ExtensionMultiMsgTx(t, ctx, input.EncodingConfig.TxConfig, input.AccountKeeper, ethPrivkey, gravityEvmChainId, false)
	eipBridgeMultiTx := eipBridgeMultiBuilder.GetTx()
	_, err = ah(ctx, eipBridgeMultiTx, false)
	require.NoError(t, err)
	eipBridgeMultiBuilder, _ = BuildBridgeWeb3ExtensionMultiMsgTx(t, ctx, input.EncodingConfig.TxConfig, input.AccountKeeper, ethPrivkey, gravityEvmChainId, true)
	eipBridgeMultiTx = eipBridgeMultiBuilder.GetTx()
	_, err = ah(ctx, eipBridgeMultiTx, false)
	require.NoError(t, err)

	// Invalid EIP712 foreign chain id tx with two bridge msgs
	eipForeignBridgeMultiBuilder, _ := BuildBridgeWeb3ExtensionMultiMsgTx(t, ctx, input.EncodingConfig.TxConfig, input.AccountKeeper, ethPrivkey, foreignChain0, false)
	eipForeignBridgeMultiTx := eipForeignBridgeMultiBuilder.GetTx()
	_, err = ah(ctx, eipForeignBridgeMultiTx, false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "eip-712 domain chainID")

	eipForeignBridgeMultiBuilder, _ = BuildBridgeWeb3ExtensionMultiMsgTx(t, ctx, input.EncodingConfig.TxConfig, input.AccountKeeper, ethPrivkey, foreignChain1, true)
	eipForeignBridgeMultiTx = eipForeignBridgeMultiBuilder.GetTx()
	_, err = ah(ctx, eipForeignBridgeMultiTx, false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "eip-712 domain chainID")

	// Invalid foreign chain id with two bridge msgs
	eipForeignInvalidBridgeMultiBuilder, _ := BuildBridgeWeb3ExtensionMultiMsgTx(t, ctx, input.EncodingConfig.TxConfig, input.AccountKeeper, ethPrivkey, uint64(101112), false)
	eipForeignInvalidBridgeMultiTx := eipForeignInvalidBridgeMultiBuilder.GetTx()
	_, err = ah(ctx, eipForeignInvalidBridgeMultiTx, false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "eip-712 domain chainID")
}

// Util function to build a standard-signed MsgSend tx
func BuildRegularTx(t *testing.T, ctx sdk.Context, txConfig sdkclient.TxConfig, ak authkeeper.AccountKeeper) sdk.Tx {
	return RegularTxBuilder(t, ctx, txConfig, ak).GetTx()
}

func RegularTxBuilder(t *testing.T, ctx sdk.Context, txConfig sdkclient.TxConfig, ak authkeeper.AccountKeeper) sdkclient.TxBuilder {
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

	return tx
}

// Util function to build a standard-signed MsgSendToEth or MsgCancelSendToEth (if cancel == true) tx
func BuildRegularBridgeTx(t *testing.T, ctx sdk.Context, txConfig sdkclient.TxConfig, ak authkeeper.AccountKeeper, cancel bool) sdk.Tx {
	return RegularBridgeTxBuilder(t, ctx, txConfig, ak, cancel).GetTx()
}

func BuildRegularMultiMsgTx(t *testing.T, ctx sdk.Context, txConfig sdkclient.TxConfig, ak authkeeper.AccountKeeper) sdk.Tx {
	tx := txConfig.NewTxBuilder()
	var msgs []sdk.Msg
	msgs = append(msgs, banktypes.NewMsgSend(keeper.AccAddrs[0], keeper.AccAddrs[1], sdk.NewCoins(sdk.NewInt64Coin("stake", 100))))
	msgs = append(msgs, banktypes.NewMsgSend(keeper.AccAddrs[0], keeper.AccAddrs[1], sdk.NewCoins(sdk.NewInt64Coin("stake", 100))))

	if err := tx.SetMsgs(msgs...); err != nil {
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

func RegularBridgeTxBuilder(t *testing.T, ctx sdk.Context, txConfig sdkclient.TxConfig, ak authkeeper.AccountKeeper, cancel bool) sdkclient.TxBuilder {
	tx := txConfig.NewTxBuilder()
	zeroEthAddress, _ := types.NewEthAddress(types.ZeroAddressString)
	var msg sdk.Msg
	if cancel {
		msg = types.NewMsgCancelSendToEth(keeper.AccAddrs[0], 1)
	} else {
		msg = types.NewMsgSendToEth(keeper.AccAddrs[0], *zeroEthAddress, sdk.NewInt64Coin("stake", 100), sdk.NewInt64Coin("stake", 100), sdk.NewInt64Coin("stake", 100))
	}

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

	return tx
}

func BuildRegularMultiMsgBridgeTx(t *testing.T, ctx sdk.Context, txConfig sdkclient.TxConfig, ak authkeeper.AccountKeeper) sdk.Tx {
	tx := txConfig.NewTxBuilder()
	zeroEthAddress, _ := types.NewEthAddress(types.ZeroAddressString)
	var msgs []sdk.Msg
	msgs = append(msgs, types.NewMsgCancelSendToEth(keeper.AccAddrs[0], 1))
	msgs = append(msgs, types.NewMsgSendToEth(keeper.AccAddrs[0], *zeroEthAddress, sdk.NewInt64Coin("stake", 100), sdk.NewInt64Coin("stake", 100), sdk.NewInt64Coin("stake", 100)))

	if err := tx.SetMsgs(msgs...); err != nil {
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
func SignRegular(t *testing.T, txBuilder sdkclient.TxBuilder, signMode signing.SignMode, txConfig sdkclient.TxConfig, chainID string, accNum uint64, seq uint64) error {
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
func BuildWeb3ExtensionTx(t *testing.T, ctx sdk.Context, txConfig sdkclient.TxConfig, ak authkeeper.AccountKeeper, priv cryptotypes.PrivKey) (sdkclient.TxBuilder, *codectypes.Any) {
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

// Builds a tx with two MsgSends signed via EIP712. Returns the TxBuilder and the Web3ExtensionOption containing the signature
func BuildWeb3ExtensionMultiMsgTx(t *testing.T, ctx sdk.Context, txConfig sdkclient.TxConfig, ak authkeeper.AccountKeeper, priv cryptotypes.PrivKey) (sdkclient.TxBuilder, *codectypes.Any) {
	tx := txConfig.NewTxBuilder()

	address := sdk.AccAddress(priv.PubKey().Address())
	var msgs []sdk.Msg
	msgs = append(msgs, banktypes.NewMsgSend(address, keeper.AccAddrs[1], sdk.NewCoins(sdk.NewInt64Coin("stake", 100))))
	msgs = append(msgs, banktypes.NewMsgSend(address, keeper.AccAddrs[1], sdk.NewCoins(sdk.NewInt64Coin("stake", 100))))

	if err := tx.SetMsgs(msgs...); err != nil {
		panic(err)
	}

	fees := sdk.NewCoins()
	gasAmount := uint64(2000000)
	chainId, err := strconv.ParseUint(gravityconfig.GravityEvmChainID, 10, 64)
	require.NoError(t, err)
	tx.SetFeeAmount(fees)
	tx.SetGasLimit(gasAmount)
	tx.SetTimeoutHeight(uint64(ctx.BlockHeight() + 100))

	txBuilder, option, err := SignEip712(t, ak, ctx, priv, chainId, gasAmount, fees, msgs, txConfig)
	require.NoError(t, err)

	return txBuilder, option
}

// Builds a MsgSendToEth or MsgCancelSendToEth (when cancel == true) tx signed via EIP712. Returns the TxBuilder and the Web3ExtensionOption containing the signature
func BuildBridgeWeb3ExtensionTx(
	t *testing.T,
	ctx sdk.Context,
	txConfig sdkclient.TxConfig,
	ak authkeeper.AccountKeeper,
	priv cryptotypes.PrivKey,
	evmChainId uint64,
	cancel bool,
) (sdkclient.TxBuilder, *codectypes.Any) {
	tx := txConfig.NewTxBuilder()

	address := sdk.AccAddress(priv.PubKey().Address().Bytes())
	zeroEthAddress, _ := types.NewEthAddress(types.ZeroAddressString)
	var msg sdk.Msg
	if cancel {
		msg = types.NewMsgCancelSendToEth(address, 1)
	} else {
		msg = types.NewMsgSendToEth(address, *zeroEthAddress, sdk.NewInt64Coin("stake", 100), sdk.NewInt64Coin("stake", 1), sdk.NewInt64Coin("stake", 1))
	}

	if err := tx.SetMsgs(msg); err != nil {
		panic(err)
	}

	fees := sdk.NewCoins()
	gasAmount := uint64(2000000)
	tx.SetFeeAmount(fees)
	tx.SetGasLimit(gasAmount)
	tx.SetTimeoutHeight(uint64(ctx.BlockHeight() + 100))

	txBuilder, option, err := SignEip712(t, ak, ctx, priv, evmChainId, gasAmount, fees, []sdk.Msg{msg}, txConfig)
	require.NoError(t, err)

	return txBuilder, option
}

// Builds a tx signed via EIP712 with both a MsgSendToEth and MsgCancelSendToEth. Returns the TxBuilder and the Web3ExtensionOption containing the signature
func BuildBridgeWeb3ExtensionMultiMsgTx(
	t *testing.T,
	ctx sdk.Context,
	txConfig sdkclient.TxConfig,
	ak authkeeper.AccountKeeper,
	priv cryptotypes.PrivKey,
	evmChainId uint64,
	cancel bool,
) (sdkclient.TxBuilder, *codectypes.Any) {
	tx := txConfig.NewTxBuilder()

	address := sdk.AccAddress(priv.PubKey().Address().Bytes())
	zeroEthAddress, _ := types.NewEthAddress(types.ZeroAddressString)
	var msgs []sdk.Msg
	// We can only have one type of message in this legacy EIP712 implementation, but many of that same type
	if cancel {
		msgs = append(msgs, types.NewMsgCancelSendToEth(address, 1))
		msgs = append(msgs, types.NewMsgCancelSendToEth(address, 1))
	} else {
		msgs = append(msgs, types.NewMsgSendToEth(address, *zeroEthAddress, sdk.NewInt64Coin("stake", 100), sdk.NewInt64Coin("stake", 1), sdk.NewInt64Coin("stake", 1)))
		msgs = append(msgs, types.NewMsgSendToEth(address, *zeroEthAddress, sdk.NewInt64Coin("stake", 100), sdk.NewInt64Coin("stake", 1), sdk.NewInt64Coin("stake", 1)))
	}

	if err := tx.SetMsgs(msgs...); err != nil {
		panic(err)
	}

	fees := sdk.NewCoins()
	gasAmount := uint64(2000000)
	tx.SetFeeAmount(fees)
	tx.SetGasLimit(gasAmount)
	tx.SetTimeoutHeight(uint64(ctx.BlockHeight() + 100))

	txBuilder, option, err := SignEip712(t, ak, ctx, priv, evmChainId, gasAmount, fees, msgs, txConfig)
	require.NoError(t, err)

	return txBuilder, option
}

// Takes a valid EIP712-signed Tx and adds another Web3ExtensionOption to it so that the Tx is invalid
func BuildInvalidWeb3ExtensionTx(t *testing.T, ctx sdk.Context, txConfig sdkclient.TxConfig, ak authkeeper.AccountKeeper, priv cryptotypes.PrivKey) sdk.Tx {
	txBuilder, option := BuildWeb3ExtensionTx(t, ctx, txConfig, ak, priv)
	extensionBuilder := txBuilder.(authtx.ExtensionOptionsTxBuilder)
	extensionBuilder.SetExtensionOptions(option, option)

	return extensionBuilder.GetTx()
}

// Takes a valid EIP712-signed bridge Tx and adds another Web3ExtensionOption to it so that the Tx is invalid
func BuildInvalidBridgeWeb3ExtensionTx(t *testing.T, ctx sdk.Context, txConfig sdkclient.TxConfig, ak authkeeper.AccountKeeper, priv cryptotypes.PrivKey, evmChainId uint64, cancel bool) sdk.Tx {
	txBuilder, option := BuildBridgeWeb3ExtensionTx(t, ctx, txConfig, ak, priv, evmChainId, cancel)
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
	txCfg sdkclient.TxConfig,
) (sdkclient.TxBuilder, *codectypes.Any, error) {
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
