package keeper

import (
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"unicode"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	eip712 "github.com/evmos/ethermint/ethereum/eip712"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/auth/legacy/legacytx"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
)

type testInitStruct struct {
	privKey    *ecdsa.PrivateKey
	ethAddress string
}

func TestConfirmHandlerCommon(t *testing.T) {
	input, ctx := SetupFiveValChain(t)
	defer func() { input.Context.Logger().Info("Asserting invariants at test end"); input.AssertInvariants() }()

	privKey, err := crypto.GenerateKey()
	require.NoError(t, err)

	ethAddress, err := types.NewEthAddress(crypto.PubkeyToAddress(privKey.PublicKey).String())
	require.NoError(t, err)

	input.GravityKeeper.SetEthAddressForValidator(ctx, ValAddrs[0], *ethAddress)
	input.GravityKeeper.SetOrchestratorValidator(ctx, ValAddrs[0], AccAddrs[0])

	batch := types.OutgoingTxBatch{
		BatchNonce:         0,
		BatchTimeout:       420,
		Transactions:       []types.OutgoingTransferTx{},
		TokenContract:      "0xd041c41EA1bf0F006ADBb6d2c9ef9D425dE5eaD7",
		CosmosBlockCreated: 0,
	}

	checkpoint := batch.GetCheckpoint(input.GravityKeeper.GetGravityID(ctx))

	ethSignature, err := types.NewEthereumSignature(checkpoint, privKey)
	require.NoError(t, err)

	sv := msgServer{input.GravityKeeper}
	err = sv.confirmHandlerCommon(input.Context, ethAddress.GetAddress().Hex(), AccAddrs[0], hex.EncodeToString(ethSignature), checkpoint)
	assert.Nil(t, err)
}
func confirmHandlerCommonWithAddress(t *testing.T, address string, testVar testInitStruct) error {
	input, ctx := SetupFiveValChain(t)
	defer func() { input.Context.Logger().Info("Asserting invariants at test end"); input.AssertInvariants() }()

	privKey := testVar.privKey

	ethAddress, err := types.NewEthAddress(testVar.ethAddress)
	require.NoError(t, err)

	input.GravityKeeper.SetEthAddressForValidator(ctx, ValAddrs[0], *ethAddress)
	input.GravityKeeper.SetOrchestratorValidator(ctx, ValAddrs[0], AccAddrs[0])

	batch := types.OutgoingTxBatch{
		BatchNonce:         0,
		BatchTimeout:       420,
		Transactions:       []types.OutgoingTransferTx{},
		TokenContract:      "0xd041c41EA1bf0F006ADBb6d2c9ef9D425dE5eaD7",
		CosmosBlockCreated: 0,
	}

	checkpoint := batch.GetCheckpoint(input.GravityKeeper.GetGravityID(ctx))

	ethSignature, err := types.NewEthereumSignature(checkpoint, privKey)
	require.NoError(t, err)

	sv := msgServer{input.GravityKeeper}

	err = sv.confirmHandlerCommon(input.Context, address, AccAddrs[0], hex.EncodeToString(ethSignature), checkpoint)

	return err
}
func TestConfirmHandlerCommonWithLowercaseAddress(t *testing.T) {
	privKey, err := crypto.GenerateKey()
	require.NoError(t, err)

	ethAddress := crypto.PubkeyToAddress(privKey.PublicKey).String()
	require.NoError(t, err)

	initVar := testInitStruct{privKey: privKey, ethAddress: ethAddress}

	ret_err := confirmHandlerCommonWithAddress(t, strings.ToLower(ethAddress), initVar)
	assert.Nil(t, ret_err)

}
func TestConfirmHandlerCommonWithUppercaseAddress(t *testing.T) {
	privKey, err := crypto.GenerateKey()
	require.NoError(t, err)

	ethAddress := crypto.PubkeyToAddress(privKey.PublicKey).String()

	initVar := testInitStruct{privKey: privKey, ethAddress: ethAddress}

	ret_err := confirmHandlerCommonWithAddress(t, strings.ToUpper(ethAddress), initVar)
	assert.Nil(t, ret_err)
}
func TestConfirmHandlerCommonWithMixedCaseAddress(t *testing.T) {
	privKey, err := crypto.GenerateKey()
	require.NoError(t, err)

	ethString := crypto.PubkeyToAddress(privKey.PublicKey).String()

	initVar := testInitStruct{privKey: privKey, ethAddress: ethString}

	ethAddress, err := types.NewEthAddress(ethString)
	require.NoError(t, err)

	mixedCase := []rune(ethAddress.GetAddress().Hex())
	for i := range mixedCase {
		if rand.Float64() > 0.5 {
			mixedCase[i] = unicode.ToLower(mixedCase[i])
		} else {
			mixedCase[i] = unicode.ToUpper(mixedCase[i])
		}
	}

	ret_err := confirmHandlerCommonWithAddress(t, string(mixedCase), initVar)
	assert.Nil(t, ret_err)
}

// This test helps in generating Gravity Msg EIP712 types. It doesn't really test much but it is
// the simplest way to understand the expected Types (and therefore the expected hash) of an EIP712 signed message.
func TestGenerateMsgEIP712Types(t *testing.T) {
	input, ctx := SetupFiveValChain(t)
	cdc := input.Marshaler

	sender := sdk.MustAccAddressFromBech32("gravity1hanqss6jsq66tfyjz56wz44z0ejtyv0724h32c")
	dest, err := types.NewEthAddress("0xBf660843528035a5A4921534E156a27e64B231fE")
	require.NoError(t, err)
	send := sdk.NewCoin("ugraviton", sdk.NewInt(1))
	bridgeFee := sdk.NewCoin("ugraviton", sdk.NewInt(1))
	chainFee := sdk.NewCoin("ugraviton", sdk.NewInt(1))
	msg := types.NewMsgSendToEth(sender, *dest, send, bridgeFee, chainFee)
	require.NotNil(t, msg)
	fee := legacytx.NewStdFee(2000000, sdk.NewCoins(sdk.NewCoin("ugraviton", sdk.ZeroInt()))) //nolint: staticcheck
	data := legacytx.StdSignBytes(ctx.ChainID(), 13, 0, 0, fee, []sdk.Msg{msg}, "")

	typedData, err := eip712.WrapTxToTypedData(cdc, 999999, msg, data, nil)
	require.NoError(t, err)

	fmt.Println("Got typed data:", typedData)
	// panic("Panic for logs")
}
