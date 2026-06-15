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

// TestRequestBatchCosmosBridgeableTokensAllowlist verifies that RequestBatch enforces the
// CosmosBridgeableTokens allowlist for Cosmos-originated assets.
// nolint: exhaustruct
func TestRequestBatchCosmosBridgeableTokensAllowlist(t *testing.T) {
	input, ctx := SetupFiveValChain(t)
	defer func() { input.Context.Logger().Info("Asserting invariants at test end"); input.AssertInvariants() }()

	sv := msgServer{input.GravityKeeper}
	gk := input.GravityKeeper
	sender := AccAddrs[0]

	// Register a cosmos-originated denom <-> ERC20 mapping
	cosmosTokenDenom := "uatom"
	cosmosTokenContract := "0x0bc529c00c6401aef6d220be8c6ea1667f6ad93e"
	cosmosTokenContractAddr, err := types.NewEthAddress(cosmosTokenContract)
	require.NoError(t, err)
	gk.setCosmosOriginatedDenomToERC20(ctx, cosmosTokenDenom, *cosmosTokenContractAddr)

	// Ensure the allowlist is empty
	params, err := gk.GetParams(ctx)
	require.NoError(t, err)
	params.CosmosBridgeableTokens = []string{}
	require.NoError(t, gk.SetParams(ctx, params))

	// RequestBatch for a cosmos-originated token not on the allowlist must be rejected
	_, err = sv.RequestBatch(ctx, &types.MsgRequestBatch{
		Sender: sender.String(),
		Denom:  cosmosTokenDenom,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "CosmosBridgeableTokens allowlist")

	// Add to allowlist — now it should pass the allowlist check
	// (will fail later due to no txs in pool, but that's fine — we're testing the gate)
	params.CosmosBridgeableTokens = []string{cosmosTokenDenom}
	require.NoError(t, gk.SetParams(ctx, params))

	_, err = sv.RequestBatch(ctx, &types.MsgRequestBatch{
		Sender: sender.String(),
		Denom:  cosmosTokenDenom,
	})
	// Should no longer fail with allowlist error (may fail with "no txs in pool" or similar)
	if err != nil {
		require.NotContains(t, err.Error(), "CosmosBridgeableTokens allowlist")
	}
}

// TestEnsureCosmosBridgeable tests Keeper.EnsureCosmosBridgeable
//  1. Ethereum-originated denoms (not in cosmos-originated index) always pass.
//  2. Cosmos-originated denoms NOT on the allowlist are rejected.
//  3. Cosmos-originated denoms ON the allowlist are permitted.
//
// nolint: exhaustruct
func TestEnsureCosmosBridgeable(t *testing.T) {
	input, ctx := SetupFiveValChain(t)
	defer func() { input.Context.Logger().Info("Asserting invariants at test end"); input.AssertInvariants() }()

	gk := input.GravityKeeper

	// Register a cosmos-originated denom
	cosmosDenom := "uatom"
	cosmosContract, err := types.NewEthAddress("0x0bc529c00c6401aef6d220be8c6ea1667f6ad93e")
	require.NoError(t, err)
	gk.setCosmosOriginatedDenomToERC20(ctx, cosmosDenom, *cosmosContract)

	// Start with an empty allowlist
	params, err := gk.GetParams(ctx)
	require.NoError(t, err)
	params.CosmosBridgeableTokens = []string{}
	require.NoError(t, gk.SetParams(ctx, params))

	// ── Case 1: Ethereum-originated denom (not in cosmos index) always passes ────
	ethDenom := "gravity0x429881672B9AE42b8EbA0E26cD9C73711b891Ca5"
	err = gk.EnsureCosmosBridgeable(ctx, ethDenom)
	require.NoError(t, err, "ethereum-originated denom must pass regardless of allowlist")

	// ── Case 2: Unknown denom (neither gravity-prefixed nor registered) passes ───
	unknownDenom := "ufoo"
	err = gk.EnsureCosmosBridgeable(ctx, unknownDenom)
	require.NoError(t, err, "unregistered denom is not cosmos-originated, must pass")

	// ── Case 3: Cosmos-originated denom NOT on allowlist is rejected ─────────────
	err = gk.EnsureCosmosBridgeable(ctx, cosmosDenom)
	require.Error(t, err)
	require.Contains(t, err.Error(), "CosmosBridgeableTokens allowlist")
	require.Contains(t, err.Error(), cosmosDenom)

	// ── Case 4: Cosmos-originated denom ON allowlist is permitted ─────────────────
	params.CosmosBridgeableTokens = []string{cosmosDenom}
	require.NoError(t, gk.SetParams(ctx, params))

	err = gk.EnsureCosmosBridgeable(ctx, cosmosDenom)
	require.NoError(t, err, "cosmos-originated denom on allowlist must pass")

	// ── Case 5: Second cosmos-originated denom not on partial allowlist ───────────
	cosmosDenom2 := "uosmo"
	cosmosContract2, err := types.NewEthAddress("0x1f9840a85d5af5bf1d1762f925bdaddc4201f984")
	require.NoError(t, err)
	gk.setCosmosOriginatedDenomToERC20(ctx, cosmosDenom2, *cosmosContract2)

	err = gk.EnsureCosmosBridgeable(ctx, cosmosDenom2)
	require.Error(t, err, "second cosmos denom not on allowlist must be rejected")
	require.Contains(t, err.Error(), cosmosDenom2)

	// ── Case 6: Both on allowlist, both pass ─────────────────────────────────────
	params.CosmosBridgeableTokens = []string{cosmosDenom, cosmosDenom2}
	require.NoError(t, gk.SetParams(ctx, params))

	require.NoError(t, gk.EnsureCosmosBridgeable(ctx, cosmosDenom))
	require.NoError(t, gk.EnsureCosmosBridgeable(ctx, cosmosDenom2))

	// ── Case 7: Remapped (gravity2) denom passes — it is Ethereum-originated ────
	remappedDenom := "gravity20x429881672B9AE42b8EbA0E26cD9C73711b891Ca5"
	err = gk.EnsureCosmosBridgeable(ctx, remappedDenom)
	require.NoError(t, err, "gravity2 (remapped) denom is ethereum-originated, must pass")

	// ── Case 8: Deleted cosmos-originated mapping → no longer subject to allowlist
	gk.DeleteCosmosOriginatedDenomToERC20(ctx, *cosmosContract2, cosmosDenom2)
	// Remove cosmosDenom2 from allowlist so it would fail if still cosmos-originated
	params.CosmosBridgeableTokens = []string{cosmosDenom}
	require.NoError(t, gk.SetParams(ctx, params))

	err = gk.EnsureCosmosBridgeable(ctx, cosmosDenom2)
	require.NoError(t, err, "deleted cosmos mapping means denom is no longer cosmos-originated, must pass")
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

// TestRemappedTokenRoundTrip verifies the behavior of a remapped ERC20:
//
//  1. An Ethereum-originated token's gravity0x denom is usable for SendToEth
//
//  2. After SetRemappedERC20, ERC20ToDenomLookup returns the gravity2 denom.
//
//  3. DenomToERC20Lookup for the old gravity0x denom returns an error.
//
//  4. DenomToERC20Lookup for the new gravity20x denom succeeds.
//
//  5. EnsureCosmosBridgeable passes for both gravity0x and gravity20x (both are Ethereum-originated).
//
//  6. SendToEth with the old gravity0x denom fails (remapped error).
//
//  7. SendToEth with the new gravity20x denom succeeds.
//
//     This is primarily useful for upgrade logic testing since that is theo nly time tokens should be remapped
//     but it also touches the behavior for remapped tokens and verifies it.
//
// nolint: exhaustruct
func TestRemappedTokenRoundTrip(t *testing.T) {
	input, ctx := SetupFiveValChain(t)
	// We skip the deferred invariant assertion because the remap intentionally creates a state
	// where old gravity0x tokens remain in user wallets but are no longer bridgeable — the module
	// balance invariant would flag the discrepancy between denom lookup (now gravity2) and what's
	// physically held (gravity0x). The real recovery handler calls CancelAllOutgoingTxsForContract
	// before remapping; here we test the lookup/send logic in isolation.

	sv := msgServer{input.GravityKeeper}
	gk := input.GravityKeeper
	sender := AccAddrs[0]
	ethDest := "0xd041c41EA1bf0F006ADBb6d2c9ef9D425dE5eaD7"

	// The ERC20 contract on Ethereum we'll remap
	tokenContract := "0x429881672B9AE42b8EbA0E26cD9C73711b891Ca5"
	tokenAddr, err := types.NewEthAddress(tokenContract)
	require.NoError(t, err)

	oldDenom := types.GravityDenom(*tokenAddr)  // gravity0x429881672B9AE42b8EbA0E26cD9C73711b891Ca5
	newDenom := types.Gravity2Denom(*tokenAddr) // gravity20x429881672B9AE42b8EbA0E26cD9C73711b891Ca5

	// ── Pre-remap: mint and fund user with old-style gravity0x voucher ──────────
	oldVouchers := sdk.NewCoins(sdk.NewInt64Coin(oldDenom, 10000))
	require.NoError(t, input.BankKeeper.MintCoins(ctx, types.ModuleName, oldVouchers))
	input.AccountKeeper.NewAccountWithAddress(ctx, sender)
	require.NoError(t, input.BankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, sender, oldVouchers))

	// ── Pre-remap: ERC20ToDenomLookup returns gravity0x ──────────────────────────
	isCosmosOriginated, denom := gk.ERC20ToDenomLookup(ctx, *tokenAddr)
	require.False(t, isCosmosOriginated)
	require.Equal(t, oldDenom, denom)

	// ── Pre-remap: DenomToERC20Lookup for gravity0x works ────────────────────────
	isCosmos, erc20, err := gk.DenomToERC20Lookup(ctx, oldDenom)
	require.NoError(t, err)
	require.False(t, isCosmos)
	require.Equal(t, tokenAddr.GetAddress(), erc20.GetAddress())

	// ── Pre-remap: SendToEth with gravity0x succeeds ─────────────────────────────
	_, err = sv.SendToEth(ctx, &types.MsgSendToEth{
		Sender:    sender.String(),
		EthDest:   ethDest,
		Amount:    sdk.NewInt64Coin(oldDenom, 100),
		BridgeFee: sdk.NewInt64Coin(oldDenom, 1),
		ChainFee:  sdk.NewCoin(oldDenom, sdkmath.ZeroInt()),
	})
	require.NoError(t, err, "pre-remap SendToEth with gravity0x must succeed")

	// ── Cancel the outgoing tx before remapping (like recovery handler) ──────────
	require.NoError(t, gk.CancelAllOutgoingTxsForContract(ctx, *tokenAddr))

	// ══════════════════════════════════════════════════════════════════════════════
	// REMAP the token
	// ══════════════════════════════════════════════════════════════════════════════
	gk.SetRemappedERC20(ctx, *tokenAddr)

	// ── Post-remap: ERC20ToDenomLookup returns gravity2 denom ────────────────────
	isCosmosOriginated, denom = gk.ERC20ToDenomLookup(ctx, *tokenAddr)
	require.False(t, isCosmosOriginated)
	require.Equal(t, newDenom, denom, "after remap, ERC20ToDenomLookup must return gravity2 denom")

	// ── Post-remap: DenomToERC20Lookup for old gravity0x fails ───────────────────
	_, _, err = gk.DenomToERC20Lookup(ctx, oldDenom)
	require.Error(t, err, "after remap, old gravity0x denom must be rejected")
	require.Contains(t, err.Error(), "was remapped")

	// ── Post-remap: DenomToERC20Lookup for new gravity20x succeeds ───────────────
	isCosmos, erc20, err = gk.DenomToERC20Lookup(ctx, newDenom)
	require.NoError(t, err, "after remap, gravity20x denom must resolve")
	require.False(t, isCosmos)
	require.Equal(t, tokenAddr.GetAddress(), erc20.GetAddress())

	// ── Post-remap: EnsureCosmosBridgeable passes for both (neither is cosmos-originated)
	require.NoError(t, gk.EnsureCosmosBridgeable(ctx, oldDenom))
	require.NoError(t, gk.EnsureCosmosBridgeable(ctx, newDenom))

	// ── Post-remap: SendToEth with old gravity0x fails ───────────────────────────
	_, err = sv.SendToEth(ctx, &types.MsgSendToEth{
		Sender:    sender.String(),
		EthDest:   ethDest,
		Amount:    sdk.NewInt64Coin(oldDenom, 100),
		BridgeFee: sdk.NewInt64Coin(oldDenom, 1),
		ChainFee:  sdk.NewCoin(oldDenom, sdkmath.ZeroInt()),
	})
	require.Error(t, err, "post-remap SendToEth with old gravity0x must fail")
	require.Contains(t, err.Error(), "remapped")

	// ── Post-remap: SendToEth with new gravity20x succeeds ───────────────────────
	// Fund the user with gravity2 vouchers (simulating a new deposit after remap)
	newVouchers := sdk.NewCoins(sdk.NewInt64Coin(newDenom, 10000))
	require.NoError(t, input.BankKeeper.MintCoins(ctx, types.ModuleName, newVouchers))
	require.NoError(t, input.BankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, sender, newVouchers))

	_, err = sv.SendToEth(ctx, &types.MsgSendToEth{
		Sender:    sender.String(),
		EthDest:   ethDest,
		Amount:    sdk.NewInt64Coin(newDenom, 100),
		BridgeFee: sdk.NewInt64Coin(newDenom, 1),
		ChainFee:  sdk.NewCoin(newDenom, sdkmath.ZeroInt()),
	})
	require.NoError(t, err, "post-remap SendToEth with new gravity20x must succeed")

	// ── Post-remap: IsRemappedERC20 is queryable ─────────────────────────────────
	require.True(t, gk.IsRemappedERC20(ctx, *tokenAddr))

	// ── A different non-remapped token is unaffected ─────────────────────────────
	otherContract := "0xdAC17F958D2ee523a2206206994597C13D831ec7"
	otherAddr, err := types.NewEthAddress(otherContract)
	require.NoError(t, err)
	require.False(t, gk.IsRemappedERC20(ctx, *otherAddr))

	isCosmosOriginated, otherDenom := gk.ERC20ToDenomLookup(ctx, *otherAddr)
	require.False(t, isCosmosOriginated)
	require.Equal(t, types.GravityDenom(*otherAddr), otherDenom, "non-remapped token should still use gravity0x")
}

// TestDenomToERC20Lookup_Gravity2NotRemapped verifies that a gravity2 denom for a token
// that has NOT been remapped is rejected with a clear error.
// nolint: exhaustruct
func TestDenomToERC20Lookup_Gravity2NotRemapped(t *testing.T) {
	input, ctx := SetupFiveValChain(t)
	defer func() { input.Context.Logger().Info("Asserting invariants at test end"); input.AssertInvariants() }()

	gk := input.GravityKeeper

	// Token NOT marked as remapped
	tokenContract := "0x429881672B9AE42b8EbA0E26cD9C73711b891Ca5"
	tokenAddr, err := types.NewEthAddress(tokenContract)
	require.NoError(t, err)
	require.False(t, gk.IsRemappedERC20(ctx, *tokenAddr))

	// Trying to use gravity2 denom for a non-remapped token must fail
	gravity2Denom := types.Gravity2Denom(*tokenAddr)
	_, _, err = gk.DenomToERC20Lookup(ctx, gravity2Denom)
	require.Error(t, err)
	require.Contains(t, err.Error(), "does not correspond to a remapped ERC20")
}
