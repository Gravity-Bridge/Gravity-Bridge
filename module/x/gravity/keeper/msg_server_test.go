package keeper

import (
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"math/rand"
	"strings"
	"testing"
	"unicode"

	"cosmossdk.io/math"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
	typesv2 "github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types/v2"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestSendToEth_BadDenom(t *testing.T) {
	input, ctx := SetupFiveValChain(t)
	defer func() { input.Context.Logger().Info("Asserting invariants at test end"); input.AssertInvariants() }()

	msgSrv := NewMsgServerImpl(input.GravityKeeper)

	msg := &types.MsgSendToEth{
		Sender:    sdk.AccAddress([]byte{1, 2, 3}).String(),
		EthDest:   "0xb462864E395d88d6bc7C5dd5F3F5eb4cc2599255",
		Amount:    sdk.NewCoin("ibc/gravity0xbad", math.NewInt(100)),
		BridgeFee: sdk.NewCoin("ibc/gravity0xbad", math.NewInt(1)),
		ChainFee:  sdk.NewCoin("ibc/gravity0xbad", math.NewInt(1)),
	}

	_, err := msgSrv.SendToEth(ctx, msg)
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrInvalidDenom)
}

func TestRequestBatch_BadDenom(t *testing.T) {
	input, ctx := SetupFiveValChain(t)
	defer func() { input.Context.Logger().Info("Asserting invariants at test end"); input.AssertInvariants() }()

	msgSrv := NewMsgServerImpl(input.GravityKeeper)

	msg := &types.MsgRequestBatch{
		Sender: sdk.AccAddress([]byte{1, 2, 3}).String(),
		Denom:  "ibc/gravity0xbad",
	}

	_, err := msgSrv.RequestBatch(ctx, msg)
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrInvalidDenom)
}

func TestERC20DeployedClaim_BadDenom(t *testing.T) {
	input, ctx := SetupFiveValChain(t)
	defer func() { input.Context.Logger().Info("Asserting invariants at test end"); input.AssertInvariants() }()

	msgSrv := NewMsgServerImpl(input.GravityKeeper)

	msg := &types.MsgERC20DeployedClaim{
		Orchestrator:  sdk.AccAddress([]byte{1, 2, 3}).String(),
		CosmosDenom:   "ibc/gravity0xbad",
		TokenContract: "0xb462864E395d88d6bc7C5dd5F3F5eb4cc2599255",
		EventNonce:    1,
	}

	_, err := msgSrv.ERC20DeployedClaim(ctx, msg)
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrInvalidDenom)
}

// TestSendToEthCosmosBridgeableTokensAllowlist verifies that SendToEth enforces the
// CosmosBridgeableTokens allowlist for Cosmos-originated assets, while Ethereum-originated
// (gravity-prefixed) assets are always permitted.
// nolint: exhaustruct
func TestSendToEthCosmosBridgeableTokensAllowlist(t *testing.T) {
	input, ctx := SetupFiveValChain(t)
	defer func() { input.Context.Logger().Info("Asserting invariants at test end"); input.AssertInvariants() }()

	sv := msgServer{input.GravityKeeper}
	gk := input.GravityKeeper

	sender := AccAddrs[0]
	ethDest := "0xd041c41EA1bf0F006ADBb6d2c9ef9D425dE5eaD7"

	// ── Ethereum-originated token ────────────────────────────────────────────────
	// gravity0x... denoms are always permitted regardless of the allowlist.
	ethTokenContract := "0x429881672B9AE42b8EbA0E26cD9C73711b891Ca5"
	ethTokenContractAddr, err := types.NewEthAddress(ethTokenContract)
	require.NoError(t, err)
	ethVoucherToken, err := types.NewInternalERC20Token(sdkmath.NewInt(10000), ethTokenContract)
	require.NoError(t, err)
	ethVouchers := sdk.NewCoins(ethVoucherToken.GravityCoin())
	require.NoError(t, input.BankKeeper.MintCoins(ctx, types.ModuleName, ethVouchers))
	input.AccountKeeper.NewAccountWithAddress(ctx, sender)
	require.NoError(t, input.BankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, sender, ethVouchers))

	// Allowlist is empty but Ethereum-originated token should still be accepted
	_, err = sv.SendToEth(ctx, &types.MsgSendToEth{
		Sender:    sender.String(),
		EthDest:   ethDest,
		Amount:    sdk.NewCoin(ethVoucherToken.GravityCoin().Denom, sdkmath.NewInt(100)),
		BridgeFee: sdk.NewCoin(ethVoucherToken.GravityCoin().Denom, sdkmath.NewInt(1)),
		ChainFee:  sdk.NewCoin(ethVoucherToken.GravityCoin().Denom, sdkmath.ZeroInt()),
	})
	require.NoError(t, err, "ethereum-originated token must be allowed even when allowlist is empty")

	// ── Cosmos-originated token, NOT on allowlist ────────────────────────────────
	cosmosTokenDenom := "uatom"
	cosmosTokenContract := "0x0bc529c00c6401aef6d220be8c6ea1667f6ad93e"
	cosmosTokenContractAddr, err := types.NewEthAddress(cosmosTokenContract)
	require.NoError(t, err)
	// Register the cosmos-originated denom <-> ERC20 mapping
	gk.setCosmosOriginatedDenomToERC20(ctx, cosmosTokenDenom, *cosmosTokenContractAddr)
	// Mint cosmos tokens to the sender
	cosmosVouchers := sdk.NewCoins(sdk.NewInt64Coin(cosmosTokenDenom, 10000))
	require.NoError(t, input.BankKeeper.MintCoins(ctx, types.ModuleName, cosmosVouchers))
	require.NoError(t, input.BankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, sender, cosmosVouchers))

	// Ensure the allowlist is empty (default from TestingGravityParams)
	params, err := gk.GetParams(ctx)
	require.NoError(t, err)
	require.Empty(t, params.CosmosBridgeableTokens)

	_, err = sv.SendToEth(ctx, &types.MsgSendToEth{
		Sender:    sender.String(),
		EthDest:   ethDest,
		Amount:    sdk.NewInt64Coin(cosmosTokenDenom, 100),
		BridgeFee: sdk.NewInt64Coin(cosmosTokenDenom, 1),
		ChainFee:  sdk.NewCoin(cosmosTokenDenom, sdkmath.ZeroInt()),
	})
	require.Error(t, err, "cosmos-originated token not on allowlist must be rejected")
	require.Contains(t, err.Error(), "CosmosBridgeableTokens allowlist")

	// ── Cosmos-originated token, ON allowlist ────────────────────────────────────
	params.CosmosBridgeableTokens = []string{cosmosTokenDenom}
	require.NoError(t, gk.SetParams(ctx, params))

	_, err = sv.SendToEth(ctx, &types.MsgSendToEth{
		Sender:    sender.String(),
		EthDest:   ethDest,
		Amount:    sdk.NewInt64Coin(cosmosTokenDenom, 100),
		BridgeFee: sdk.NewInt64Coin(cosmosTokenDenom, 1),
		ChainFee:  sdk.NewCoin(cosmosTokenDenom, sdkmath.ZeroInt()),
	})
	require.NoError(t, err, "cosmos-originated token on allowlist must be permitted")

	// ── Second cosmos token, allowlist has first but not second ──────────────────
	cosmosToken2Denom := "uosmo"
	cosmosToken2Contract := "0x1f9840a85d5af5bf1d1762f925bdaddc4201f984"
	cosmosToken2ContractAddr, err := types.NewEthAddress(cosmosToken2Contract)
	require.NoError(t, err)
	gk.setCosmosOriginatedDenomToERC20(ctx, cosmosToken2Denom, *cosmosToken2ContractAddr)
	cosmos2Vouchers := sdk.NewCoins(sdk.NewInt64Coin(cosmosToken2Denom, 10000))
	require.NoError(t, input.BankKeeper.MintCoins(ctx, types.ModuleName, cosmos2Vouchers))
	require.NoError(t, input.BankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, sender, cosmos2Vouchers))

	_, err = sv.SendToEth(ctx, &types.MsgSendToEth{
		Sender:    sender.String(),
		EthDest:   ethDest,
		Amount:    sdk.NewInt64Coin(cosmosToken2Denom, 100),
		BridgeFee: sdk.NewInt64Coin(cosmosToken2Denom, 1),
		ChainFee:  sdk.NewCoin(cosmosToken2Denom, sdkmath.ZeroInt()),
	})
	require.Error(t, err, "cosmos-originated token not on allowlist must be rejected even when other tokens are allowed")
	require.Contains(t, err.Error(), "CosmosBridgeableTokens allowlist")

	// Suppress unused variable warning
	_ = ethTokenContractAddr
}

// TestUpdateParamsProposalCosmosBridgeableTokens verifies that MsgUpdateParamsProposal
// correctly updates, validates, and rejects invalid values for CosmosBridgeableTokens.
// nolint: exhaustruct
func TestUpdateParamsProposalCosmosBridgeableTokens(t *testing.T) {
	input, ctx := SetupFiveValChain(t)
	defer func() { input.Context.Logger().Info("Asserting invariants at test end"); input.AssertInvariants() }()

	sv := msgServer{input.GravityKeeper}
	govAddr := authtypes.NewModuleAddress(govtypes.ModuleName).String()

	// ── Valid update: set a non-empty allowlist ───────────────────────────────────
	denoms := []string{"uatom", "uosmo"}
	denomsJSON, err := json.Marshal(denoms)
	require.NoError(t, err)

	_, err = sv.UpdateParamsProposal(ctx, &typesv2.MsgUpdateParamsProposal{
		Authority: govAddr,
		ParamUpdates: []*typesv2.Param{
			{Key: types.ParamCosmosBridgeableTokens, Value: string(denomsJSON)},
		},
	})
	require.NoError(t, err)

	params, err := input.GravityKeeper.GetParams(ctx)
	require.NoError(t, err)
	require.Equal(t, denoms, params.CosmosBridgeableTokens)

	// ── Valid update: clear the allowlist ────────────────────────────────────────
	emptyJSON, err := json.Marshal([]string{})
	require.NoError(t, err)

	_, err = sv.UpdateParamsProposal(ctx, &typesv2.MsgUpdateParamsProposal{
		Authority: govAddr,
		ParamUpdates: []*typesv2.Param{
			{Key: types.ParamCosmosBridgeableTokens, Value: string(emptyJSON)},
		},
	})
	require.NoError(t, err)

	params, err = input.GravityKeeper.GetParams(ctx)
	require.NoError(t, err)
	require.Empty(t, params.CosmosBridgeableTokens)

	// ── Invalid update: malformed JSON ────────────────────────────────────────────
	_, err = sv.UpdateParamsProposal(ctx, &typesv2.MsgUpdateParamsProposal{
		Authority: govAddr,
		ParamUpdates: []*typesv2.Param{
			{Key: types.ParamCosmosBridgeableTokens, Value: "not-valid-json"},
		},
	})
	require.Error(t, err)

	// ── Invalid update: gravity-prefixed denom (rejected by ValidateBasic) ────────
	badDenoms := []string{"gravity0x429881672B9AE42b8EbA0E26cD9C73711b891Ca5"}
	badDenomsJSON, err := json.Marshal(badDenoms)
	require.NoError(t, err)

	_, err = sv.UpdateParamsProposal(ctx, &typesv2.MsgUpdateParamsProposal{
		Authority: govAddr,
		ParamUpdates: []*typesv2.Param{
			{Key: types.ParamCosmosBridgeableTokens, Value: string(badDenomsJSON)},
		},
	})
	require.Error(t, err, "gravity-prefixed denom must be rejected by ValidateBasic")

	// ── Invalid update: duplicate denoms (rejected by ValidateBasic) ──────────────
	dupDenoms := []string{"uatom", "uatom"}
	dupDenomsJSON, err := json.Marshal(dupDenoms)
	require.NoError(t, err)

	_, err = sv.UpdateParamsProposal(ctx, &typesv2.MsgUpdateParamsProposal{
		Authority: govAddr,
		ParamUpdates: []*typesv2.Param{
			{Key: types.ParamCosmosBridgeableTokens, Value: string(dupDenomsJSON)},
		},
	})
	require.Error(t, err, "duplicate denoms must be rejected by ValidateBasic")
}
