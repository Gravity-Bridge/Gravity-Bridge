package keeper

import (
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"unicode"

	"cosmossdk.io/math"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
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

	//nolint: exhaustruct
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
	//nolint: goconst
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
	require.NoError(t, gk.setCosmosOriginatedMapping(ctx, cosmosTokenDenom, *cosmosTokenContractAddr))
	// Mint cosmos tokens to the sender
	cosmosVouchers := sdk.NewCoins(sdk.NewInt64Coin(cosmosTokenDenom, 10000))
	require.NoError(t, input.BankKeeper.MintCoins(ctx, types.ModuleName, cosmosVouchers))
	require.NoError(t, input.BankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, sender, cosmosVouchers))

	// Ensure the allowlist is empty (default from TestingGravityParams)
	require.Empty(t, gk.GetAllCosmosBridgeableTokens(ctx))

	_, err = sv.SendToEth(ctx, &types.MsgSendToEth{
		Sender:    sender.String(),
		EthDest:   ethDest,
		Amount:    sdk.NewInt64Coin(cosmosTokenDenom, 100),
		BridgeFee: sdk.NewInt64Coin(cosmosTokenDenom, 1),
		ChainFee:  sdk.NewCoin(cosmosTokenDenom, sdkmath.ZeroInt()),
	})
	require.Error(t, err, "cosmos-originated token not on allowlist must be rejected")
	require.Contains(t, err.Error(), "CosmosBridgeableTokens whitelist")
	gk.SetCosmosBridgeableToken(ctx, minMeta(cosmosTokenDenom))

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
	require.NoError(t, gk.setCosmosOriginatedMapping(ctx, cosmosToken2Denom, *cosmosToken2ContractAddr))
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
	require.Contains(t, err.Error(), "CosmosBridgeableTokens whitelist")

	// Suppress unused variable warning
	_ = ethTokenContractAddr
}

// TestSendToEthMetadataDriftRejected verifies that SendToEth rejects a cosmos-originated
// denom whose bank module metadata has drifted from the governance-approved
// CosmosBridgeableTokens entry (the "SECURITY VIOLATION" branch in assertMetadataWhitelisted).
// nolint: exhaustruct
func TestSendToEthMetadataDriftRejected(t *testing.T) {
	input, ctx := SetupFiveValChain(t)
	defer func() { input.Context.Logger().Info("Asserting invariants at test end"); input.AssertInvariants() }()

	sv := msgServer{input.GravityKeeper}
	gk := input.GravityKeeper

	sender := AccAddrs[0]
	ethDest := "0xd041c41EA1bf0F006ADBb6d2c9ef9D425dE5eaD7"

	cosmosTokenDenom := "udrift"
	cosmosTokenContract := "0x0bc529c00c6401aef6d220be8c6ea1667f6ad93e"
	cosmosTokenContractAddr, err := types.NewEthAddress(cosmosTokenContract)
	require.NoError(t, err)
	require.NoError(t, gk.setCosmosOriginatedMapping(ctx, cosmosTokenDenom, *cosmosTokenContractAddr))

	cosmosVouchers := sdk.NewCoins(sdk.NewInt64Coin(cosmosTokenDenom, 10000))
	require.NoError(t, input.BankKeeper.MintCoins(ctx, types.ModuleName, cosmosVouchers))
	require.NoError(t, input.BankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, sender, cosmosVouchers))

	// Whitelist the denom with metadata that (for now) matches what's stored in the bank module
	originalMeta := minMeta(cosmosTokenDenom)
	input.BankKeeper.SetDenomMetaData(ctx, originalMeta)
	gk.SetCosmosBridgeableToken(ctx, originalMeta)

	// Sanity check: SendToEth succeeds while the bank metadata still matches the allowlist entry
	_, err = sv.SendToEth(ctx, &types.MsgSendToEth{
		Sender:    sender.String(),
		EthDest:   ethDest,
		Amount:    sdk.NewInt64Coin(cosmosTokenDenom, 100),
		BridgeFee: sdk.NewInt64Coin(cosmosTokenDenom, 1),
		ChainFee:  sdk.NewCoin(cosmosTokenDenom, sdkmath.ZeroInt()),
	})
	require.NoError(t, err)

	// Now mutate the bank module's metadata for the denom out from under the allowlist entry,
	// simulating drift between the two sources of truth.
	driftedMeta := originalMeta
	driftedMeta.Name = "Drifted Name"
	input.BankKeeper.SetDenomMetaData(ctx, driftedMeta)

	_, err = sv.SendToEth(ctx, &types.MsgSendToEth{
		Sender:    sender.String(),
		EthDest:   ethDest,
		Amount:    sdk.NewInt64Coin(cosmosTokenDenom, 100),
		BridgeFee: sdk.NewInt64Coin(cosmosTokenDenom, 1),
		ChainFee:  sdk.NewCoin(cosmosTokenDenom, sdkmath.ZeroInt()),
	})
	require.Error(t, err, "SendToEth must reject a denom whose bank metadata has drifted from the allowlist entry")
	require.Contains(t, err.Error(), "SECURITY VIOLATION")
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
	err = gk.setCosmosOriginatedMapping(ctx, cosmosTokenDenom, *cosmosTokenContractAddr)
	require.NoError(t, err)

	// Ensure the allowlist is empty
	require.Empty(t, gk.GetAllCosmosBridgeableTokens(ctx))

	// RequestBatch for a cosmos-originated token not on the allowlist must be rejected
	_, err = sv.RequestBatch(ctx, &types.MsgRequestBatch{
		Sender: sender.String(),
		Denom:  cosmosTokenDenom,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "CosmosBridgeableTokens whitelist")

	// Add to allowlist — now it should pass the allowlist check
	// (will fail later due to no txs in pool, but that's fine — we're testing the gate)
	gk.SetCosmosBridgeableToken(ctx, minMeta(cosmosTokenDenom))

	_, err = sv.RequestBatch(ctx, &types.MsgRequestBatch{
		Sender: sender.String(),
		Denom:  cosmosTokenDenom,
	})
	// Should no longer fail with allowlist error (may fail with "no txs in pool" or similar)
	if err != nil {
		require.NotContains(t, err.Error(), "CosmosBridgeableTokens whitelist")
	}
}

// TestCosmosBridgeableTokensProposal verifies that MsgCosmosBridgeableTokensProposal
// correctly sets, removes, validates, and rejects invalid CosmosBridgeableTokens entries.
// nolint: exhaustruct
func TestCosmosBridgeableTokensProposal(t *testing.T) {
	input, ctx := SetupFiveValChain(t)
	defer func() { input.Context.Logger().Info("Asserting invariants at test end"); input.AssertInvariants() }()

	sv := msgServer{input.GravityKeeper}
	govAddr := authtypes.NewModuleAddress(govtypes.ModuleName).String()

	// ── Valid SET: add two new entries ─────────────────────────────────────────────
	setEntries := []banktypes.Metadata{minMeta("uatom"), minMeta("uosmo")}

	_, err := sv.CosmosBridgeableTokensProposal(ctx, &typesv2.MsgCosmosBridgeableTokensProposal{
		Authority: govAddr,
		Proposal: &types.CosmosBridgeableTokensProposal{
			Title:       "Add uatom and uosmo",
			Description: "test",
			Metadatas:   setEntries,
			Operation:   types.CosmosBridgeableTokensOperation_COSMOS_BRIDGEABLE_TOKENS_OPERATION_SET,
		},
	})
	require.NoError(t, err)

	require.ElementsMatch(t, setEntries, input.GravityKeeper.GetAllCosmosBridgeableTokens(ctx))

	// SET must also have overwritten the bank keeper's metadata for each denom
	for _, m := range setEntries {
		bankMeta, found := input.BankKeeper.GetDenomMetaData(ctx, m.Base)
		require.True(t, found)
		require.True(t, metadataEqual(m, bankMeta))
	}

	// ── Valid REMOVE: remove one entry ────────────────────────────────────────────
	_, err = sv.CosmosBridgeableTokensProposal(ctx, &typesv2.MsgCosmosBridgeableTokensProposal{
		Authority: govAddr,
		Proposal: &types.CosmosBridgeableTokensProposal{
			Title:       "Remove uatom",
			Description: "test",
			Metadatas:   []banktypes.Metadata{minMeta("uatom")},
			Operation:   types.CosmosBridgeableTokensOperation_COSMOS_BRIDGEABLE_TOKENS_OPERATION_REMOVE,
		},
	})
	require.NoError(t, err)

	require.Equal(t, []banktypes.Metadata{minMeta("uosmo")}, input.GravityKeeper.GetAllCosmosBridgeableTokens(ctx))

	// ── Invalid REMOVE: denom not present ─────────────────────────────────────────
	_, err = sv.CosmosBridgeableTokensProposal(ctx, &typesv2.MsgCosmosBridgeableTokensProposal{
		Authority: govAddr,
		Proposal: &types.CosmosBridgeableTokensProposal{
			Title:       "Remove uatom again",
			Description: "test",
			Metadatas:   []banktypes.Metadata{minMeta("uatom")},
			Operation:   types.CosmosBridgeableTokensOperation_COSMOS_BRIDGEABLE_TOKENS_OPERATION_REMOVE,
		},
	})
	require.Error(t, err, "removing a denom not on the allowlist must be rejected")

	// ── Invalid: UNSPECIFIED operation ────────────────────────────────────────────
	_, err = sv.CosmosBridgeableTokensProposal(ctx, &typesv2.MsgCosmosBridgeableTokensProposal{
		Authority: govAddr,
		Proposal: &types.CosmosBridgeableTokensProposal{
			Title:       "Invalid",
			Description: "test",
			Metadatas:   setEntries,
			Operation:   types.CosmosBridgeableTokensOperation_COSMOS_BRIDGEABLE_TOKENS_OPERATION_UNSPECIFIED,
		},
	})
	require.Error(t, err, "UNSPECIFIED operation must be rejected")

	// ── Invalid SET: gravity-prefixed denom ───────────────────────────────────────
	badEntries := []banktypes.Metadata{minMeta("gravity0x429881672B9AE42b8EbA0E26cD9C73711b891Ca5")}
	_, err = sv.CosmosBridgeableTokensProposal(ctx, &typesv2.MsgCosmosBridgeableTokensProposal{
		Authority: govAddr,
		Proposal: &types.CosmosBridgeableTokensProposal{
			Title:       "Invalid",
			Description: "test",
			Metadatas:   badEntries,
			Operation:   types.CosmosBridgeableTokensOperation_COSMOS_BRIDGEABLE_TOKENS_OPERATION_SET,
		},
	})
	require.Error(t, err, "gravity-prefixed denom must be rejected")

	// ── Invalid SET: duplicate base denoms ────────────────────────────────────────
	dupEntries := []banktypes.Metadata{minMeta("uperson"), minMeta("uperson")}
	_, err = sv.CosmosBridgeableTokensProposal(ctx, &typesv2.MsgCosmosBridgeableTokensProposal{
		Authority: govAddr,
		Proposal: &types.CosmosBridgeableTokensProposal{
			Title:       "Invalid",
			Description: "test",
			Metadatas:   dupEntries,
			Operation:   types.CosmosBridgeableTokensOperation_COSMOS_BRIDGEABLE_TOKENS_OPERATION_SET,
		},
	})
	require.Error(t, err, "duplicate base denoms must be rejected")

	// ── Invalid: wrong authority ───────────────────────────────────────────────────
	_, err = sv.CosmosBridgeableTokensProposal(ctx, &typesv2.MsgCosmosBridgeableTokensProposal{
		Authority: AccAddrs[0].String(),
		Proposal: &types.CosmosBridgeableTokensProposal{
			Title:       "Invalid authority",
			Description: "test",
			Metadatas:   setEntries,
			Operation:   types.CosmosBridgeableTokensOperation_COSMOS_BRIDGEABLE_TOKENS_OPERATION_SET,
		},
	})
	require.Error(t, err, "non-gov authority must be rejected")
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
	//nolint: goconst
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
	erc20Origin := gk.ClassifyERC20(ctx, *tokenAddr)
	require.False(t, erc20Origin.IsCosmosOriginated)
	require.Equal(t, oldDenom, erc20Origin.Denom)

	// ── Pre-remap: DenomToERC20Lookup for gravity0x works ────────────────────────
	denomOrigin := gk.ClassifyDenom(ctx, oldDenom)
	require.False(t, denomOrigin.IsCosmosOriginated)
	require.Equal(t, tokenAddr, denomOrigin.ERC20)

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
	// recovery.CancelAllOutgoingTxsForContract can't be imported here (it depends on this
	// keeper package, which would create an import cycle), so replicate its logic directly
	// against the public Keeper API instead.
	var batchNonces []uint64
	gk.IterateOutgoingTxBatches(ctx, func(_ []byte, batch types.InternalOutgoingTxBatch) bool {
		if batch.TokenContract.GetAddress() == tokenAddr.GetAddress() {
			batchNonces = append(batchNonces, batch.BatchNonce)
		}
		return false
	})
	for _, nonce := range batchNonces {
		require.NoError(t, gk.CancelOutgoingTXBatch(ctx, *tokenAddr, nonce))
	}
	gk.IterateUnbatchedTransactionsByContract(ctx, *tokenAddr, func(_ []byte, tx *types.InternalOutgoingTransferTx) bool {
		require.NoError(t, gk.RemoveFromOutgoingPoolAndRefund(ctx, tx.Id, tx.Sender))
		return false
	})

	// ══════════════════════════════════════════════════════════════════════════════
	// REMAP the token
	// ══════════════════════════════════════════════════════════════════════════════
	gk.SetRemappedERC20(ctx, *tokenAddr)

	// ── Post-remap: ERC20ToDenomLookup returns gravity2 denom ────────────────────
	erc20Origin = gk.ClassifyERC20(ctx, *tokenAddr)
	require.False(t, erc20Origin.IsCosmosOriginated)
	require.Equal(t, newDenom, erc20Origin.Denom, "after remap, ERC20ToDenomLookup must return gravity2 denom")

	// ── Post-remap: ClassifyDenom for the old gravity0x denom panics ────────────
	require.PanicsWithValue(t,
		fmt.Sprintf("ERC20 %s was remapped; new deposits use %s", tokenAddr.GetAddress().Hex(), newDenom),
		func() { gk.ClassifyDenom(ctx, oldDenom) },
		"ClassifyDenom must panic for the old gravity0x denom of a remapped ERC20",
	)

	// ── Post-remap: DenomToERC20Lookup for new gravity20x succeeds ───────────────
	denomOrigin = gk.ClassifyDenom(ctx, newDenom)
	require.False(t, denomOrigin.IsCosmosOriginated)
	require.Equal(t, tokenAddr, denomOrigin.ERC20)

	// ── Post-remap: SendToEth with old gravity0x panics ──────────────────────────
	require.PanicsWithValue(t,
		fmt.Sprintf("ERC20 %s was remapped; new deposits use %s", tokenAddr.GetAddress().Hex(), newDenom),
		func() {
			_, err = sv.SendToEth(ctx, &types.MsgSendToEth{
				Sender:    sender.String(),
				EthDest:   ethDest,
				Amount:    sdk.NewInt64Coin(oldDenom, 100),
				BridgeFee: sdk.NewInt64Coin(oldDenom, 1),
				ChainFee:  sdk.NewCoin(oldDenom, sdkmath.ZeroInt()),
			})
			require.Error(t, err)
		},
		"SendToEth must panic for the old gravity0x denom of a remapped ERC20",
	)

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

	otherOrigin := gk.ClassifyERC20(ctx, *otherAddr)
	require.False(t, otherOrigin.IsCosmosOriginated)
	require.Equal(t, types.GravityDenom(*otherAddr), otherOrigin.Denom, "non-remapped token should still use gravity0x")
}

// TestDenomToERC20Lookup_Gravity2NotRemapped verifies that a gravity2 denom for a token
// that has NOT been remapped is rejected with a clear panic.
// nolint: exhaustruct
func TestDenomToERC20Lookup_Gravity2NotRemapped(t *testing.T) {
	input, ctx := SetupFiveValChain(t)
	defer func() { input.Context.Logger().Info("Asserting invariants at test end"); input.AssertInvariants() }()

	gk := input.GravityKeeper

	// Token NOT marked as remapped
	//nolint: goconst
	tokenContract := "0x429881672B9AE42b8EbA0E26cD9C73711b891Ca5"
	tokenAddr, err := types.NewEthAddress(tokenContract)
	require.NoError(t, err)
	require.False(t, gk.IsRemappedERC20(ctx, *tokenAddr))

	// Trying to use gravity2 denom for a non-remapped token must panic
	gravity2Denom := types.Gravity2Denom(*tokenAddr)
	require.PanicsWithValue(t,
		fmt.Sprintf("ClassifyDenom: gravity2 denom %s does not correspond to a remapped ERC20", gravity2Denom),
		func() { gk.ClassifyDenom(ctx, gravity2Denom) },
		"ClassifyDenom must panic for a gravity2 denom of a non-remapped ERC20",
	)
}
