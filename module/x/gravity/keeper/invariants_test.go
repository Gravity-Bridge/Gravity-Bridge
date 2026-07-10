package keeper

import (
	"fmt"
	"strings"
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"
	codecTypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	gogoproto "github.com/cosmos/gogoproto/proto"
	"github.com/stretchr/testify/require"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
)

// Tests that the gravity module's balance is accounted for with unbatched txs, including tx cancellation
func TestModuleBalanceUnbatchedTxs(t *testing.T) {
	////////////////// SETUP //////////////////
	input := CreateTestEnv(t)
	defer func() { input.Context.Logger().Info("Asserting invariants at test end"); input.AssertInvariants() }()

	ctx := input.Context
	var (
		mySender, e1        = sdk.AccAddressFromBech32("gravity1ahx7f8wyertuus9r20284ej0asrs085ceqtfnm")
		myReceiver          = "0xd041c41EA1bf0F006ADBb6d2c9ef9D425dE5eaD7"
		myTokenContractAddr = "0x429881672B9AE42b8EbA0E26cD9C73711b891Ca5"
	)
	require.NoError(t, e1)
	receiver, err := types.NewEthAddress(myReceiver)
	require.NoError(t, err)
	// mint some voucher first
	allVouchersToken, err := types.NewInternalERC20Token(sdkmath.NewInt(99999), myTokenContractAddr)
	require.NoError(t, err)
	allVouchers := sdk.Coins{NewGravityCoin(ctx, input.GravityKeeper, *allVouchersToken)}
	err = input.BankKeeper.MintCoins(ctx, types.ModuleName, allVouchers)
	require.NoError(t, err)
	// set senders balance
	input.AccountKeeper.NewAccountWithAddress(ctx, mySender)
	err = input.BankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, mySender, allVouchers)
	require.NoError(t, err)

	////////////////// EXECUTE //////////////////
	// Check the invariant without any transactions
	checkInvariant(t, ctx, input.GravityKeeper, true)

	// Create some unbatched transactions
	for i, v := range []uint64{2, 3, 2, 1} {
		amountToken, err := types.NewInternalERC20Token(sdkmath.NewInt(int64(i+100)), myTokenContractAddr)
		require.NoError(t, err)
		amount := NewGravityCoin(ctx, input.GravityKeeper, *amountToken)
		feeToken, err := types.NewInternalERC20Token(sdkmath.NewIntFromUint64(v), myTokenContractAddr)
		require.NoError(t, err)
		fee := NewGravityCoin(ctx, input.GravityKeeper, *feeToken)

		r, err := input.GravityKeeper.AddToOutgoingPool(ctx, mySender, *receiver, amount, fee)
		require.NotZero(t, r)
		require.NoError(t, err)
		// Should create:
		// 1: amount 100, fee 2
		// 2: amount 101, fee 3
		// 3: amount 102, fee 2
		// 4: amount 103, fee 1
	}
	checkInvariant(t, ctx, input.GravityKeeper, true)

	// Remove one of the transactions
	err = input.GravityKeeper.RemoveFromOutgoingPoolAndRefund(ctx, 1, mySender)
	require.NoError(t, err)
	checkInvariant(t, ctx, input.GravityKeeper, true)

	// Ensure an error is returned for a mismatched balance
	oneVoucher, err := types.NewInternalERC20Token(sdkmath.NewInt(1), myTokenContractAddr)
	require.NoError(t, err)

	checkImbalancedModule(t, ctx, input.GravityKeeper, input.BankKeeper, mySender, sdk.NewCoins(NewGravityCoin(ctx, input.GravityKeeper, *oneVoucher)))
}

// Tests that the gravity module's balance is accounted for with batches of txs, including unbatched txs and tx cancellation
func TestModuleBalanceBatchedTxs(t *testing.T) {
	////////////////// SETUP //////////////////
	input := CreateTestEnv(t)
	defer func() { input.Context.Logger().Info("Asserting invariants at test end"); input.AssertInvariants() }()

	ctx := input.Context
	var (
		now                      = time.Now().UTC()
		mySender, e1             = sdk.AccAddressFromBech32("gravity1ahx7f8wyertuus9r20284ej0asrs085ceqtfnm")
		myReceiver, e2           = types.NewEthAddress("0xd041c41EA1bf0F006ADBb6d2c9ef9D425dE5eaD7")
		myTokenContractAddr1, e3 = types.NewEthAddress("0x429881672B9AE42b8EbA0E26cD9C73711b891Ca5")
		myTokenContractAddr2, e4 = types.NewEthAddress("0xF815240800ddf3E0be80e0d848B13ecaa504BF37")
	)
	require.NoError(t, e1)
	require.NoError(t, e2)
	require.NoError(t, e3)
	require.NoError(t, e4)
	tokens := make([]*types.InternalERC20Token, 2)
	var err error
	tokens[0], err = types.NewInternalERC20Token(sdkmath.NewInt(150000000000000), myTokenContractAddr1.GetAddress().Hex())
	require.NoError(t, err)
	tokens[1], err = types.NewInternalERC20Token(sdkmath.NewInt(150000000000000), myTokenContractAddr2.GetAddress().Hex())
	require.NoError(t, err)
	voucher1, err := types.NewInternalERC20Token(sdkmath.NewInt(1), myTokenContractAddr1.GetAddress().Hex())
	require.NoError(t, err)
	voucher2, err := types.NewInternalERC20Token(sdkmath.NewInt(1), myTokenContractAddr2.GetAddress().Hex())
	require.NoError(t, err)
	voucherCoins := []sdk.Coins{
		sdk.NewCoins(NewGravityCoin(ctx, input.GravityKeeper, *voucher1)),
		sdk.NewCoins(NewGravityCoin(ctx, input.GravityKeeper, *voucher2)),
	}
	allVouchers := []sdk.Coins{
		sdk.NewCoins(NewGravityCoin(ctx, input.GravityKeeper, *tokens[0])),
		sdk.NewCoins(NewGravityCoin(ctx, input.GravityKeeper, *tokens[1])),
	}

	// mint some voucher first
	for _, v := range allVouchers {
		require.NoError(t, input.BankKeeper.MintCoins(ctx, types.ModuleName, v))
		// set senders balance
		input.AccountKeeper.NewAccountWithAddress(ctx, mySender)
		require.NoError(t, input.BankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, mySender, v))
	}
	input.GravityKeeper.SetLastObservedEthereumBlockHeight(ctx, 1234567)

	////////////////// EXECUTE //////////////////
	// Check the invariant without any transactions
	checkInvariant(t, ctx, input.GravityKeeper, true)

	for _, tok := range tokens {
		// add some TX to the pool
		for i, v := range []uint64{2, 3, 2, 1, 2, 4, 5, 1} {
			amountToken, err := types.NewInternalERC20Token(sdkmath.NewInt(int64(i+100)), tok.Contract.GetAddress().Hex())
			require.NoError(t, err)
			amount := NewGravityCoin(ctx, input.GravityKeeper, *amountToken)
			feeToken, err := types.NewInternalERC20Token(sdkmath.NewIntFromUint64(v), tok.Contract.GetAddress().Hex())
			require.NoError(t, err)
			fee := NewGravityCoin(ctx, input.GravityKeeper, *feeToken)

			r, err := input.GravityKeeper.AddToOutgoingPool(ctx, mySender, *myReceiver, amount, fee)
			require.NoError(t, err)
			ctx.Logger().Info(fmt.Sprintf("Created transaction %v with amount %v and fee %v", r, amount, fee))
			// Should create:
			// 1: tx amount is 100, fee is 2, id is 1
			// 2: tx amount is 101, fee is 3, id is 2
			// 3: tx amount is 102, fee is 2, id is 3
			// 4: tx amount is 103, fee is 1, id is 4
		}
	}
	// The module should be balanced with these unbatched txs
	checkInvariant(t, ctx, input.GravityKeeper, true)

	batches := []*types.InternalOutgoingTxBatch{nil, nil}
	// Create a batch for each token, perform some checks
	for i, tok := range tokens {
		// when
		ctx = ctx.WithBlockTime(now)
		// tx batch size is 3, so that some of them stay behind
		batch, err := input.GravityKeeper.BuildOutgoingTXBatch(ctx, tok.Contract, 3)
		require.NoError(t, err)
		// then check the batch persists
		gotBatch := input.GravityKeeper.GetOutgoingTXBatch(ctx, batch.TokenContract, batch.BatchNonce)
		require.NotNil(t, gotBatch)
		batches[i] = gotBatch
		// The module should be balanced with the new unobserved batch + leftover unbatched txs
		checkInvariant(t, ctx, input.GravityKeeper, true)
		checkImbalancedModule(t, ctx, input.GravityKeeper, input.BankKeeper, mySender, voucherCoins[i])
	}
	// Remove a tx from the pool for each contract (both of these have fee = 1 and won't be batched
	require.NoError(t, input.GravityKeeper.RemoveFromOutgoingPoolAndRefund(ctx, 4, mySender))
	require.NoError(t, input.GravityKeeper.RemoveFromOutgoingPoolAndRefund(ctx, 8, mySender))

	// Here we execute the most recently created batch to test the module's balance is correct after deletion of the first batch
	// All of the batch's transactions need to end up back in the unbatched tx pool and should be counted there for us

	// The module should be balanced with the unobserved batch + one leftover unbatched tx
	checkInvariant(t, ctx, input.GravityKeeper, true)
	checkImbalancedModule(t, ctx, input.GravityKeeper, input.BankKeeper, mySender, voucherCoins[1])

	// Simulate one batch being relayed and observed
	fakeBlock := batches[1].CosmosBlockCreated // A fake ethereum block used for the test only
	//nolint: exhaustruct
	msg := types.MsgBatchSendToEthClaim{
		EventNonce:     0,
		EthBlockHeight: fakeBlock,
		BatchNonce:     batches[1].BatchNonce,
		TokenContract:  "",
		Orchestrator:   "",
	}
	input.GravityKeeper.OutgoingTxBatchExecuted(ctx, batches[1].TokenContract, msg)
	// The module should be balanced with the batch now being observed + one leftover unbatched tx still in the pool
	checkInvariant(t, ctx, input.GravityKeeper, true)
	checkImbalancedModule(t, ctx, input.GravityKeeper, input.BankKeeper, mySender, voucherCoins[0])
	checkImbalancedModule(t, ctx, input.GravityKeeper, input.BankKeeper, mySender, voucherCoins[1])
}

func checkInvariant(t *testing.T, ctx sdk.Context, k Keeper, succeed bool) {
	res, ok := ModuleBalanceInvariant(k)(ctx)
	if succeed {
		require.False(t, ok, "Invariant should have returned false")
		require.Empty(t, res, "Invariant should have returned no message")
	} else {
		require.True(t, ok, "Invariant should have returned true")
		require.NotEmpty(t, res, "Invariant should have returned a message")
	}
}

func checkImbalancedModule(t *testing.T, ctx sdk.Context, gravityKeeper Keeper, bankKeeper bankkeeper.BaseKeeper, sender sdk.AccAddress, coins sdk.Coins) {
	// Imbalance the module
	require.NoError(t, bankKeeper.SendCoinsFromAccountToModule(ctx, sender, types.ModuleName, coins))
	checkInvariant(t, ctx, gravityKeeper, false)
	// Rebalance the module
	require.NoError(t, bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, sender, coins))
}

// TestValidateCrossListIntegrity_Valid verifies that a clean cosmos-originated index, whether
// empty or containing one or more properly registered mappings, produces no errors.
func TestValidateCrossListIntegrity_Valid(t *testing.T) {
	input := CreateTestEnv(t)
	ctx := input.Context
	k := input.GravityKeeper

	erc20A, err := types.NewEthAddress("0x429881672B9AE42b8EbA0E26cD9C73711b891Ca5")
	require.NoError(t, err)
	erc20B, err := types.NewEthAddress("0xF815240800ddf3E0be80e0d848B13ecaa504BF37")
	require.NoError(t, err)

	// Empty state → passes
	require.NoError(t, ValidateCrossListIntegrity(ctx, k))

	// Single valid mapping → passes
	require.NoError(t, k.setCosmosOriginatedMapping(ctx, "footoken", *erc20A))
	require.NoError(t, ValidateCrossListIntegrity(ctx, k))

	// Two independent valid mappings → passes
	require.NoError(t, k.setCosmosOriginatedMapping(ctx, "bartoken", *erc20B))
	require.NoError(t, ValidateCrossListIntegrity(ctx, k))
}

// TestValidateCrossListIntegrity_OrphanedEntries verifies that entries present on only one
// side of the DenomToERC20 / ERC20ToDenom double-index are detected in both directions.
func TestValidateCrossListIntegrity_OrphanedEntries(t *testing.T) {
	erc20A, err := types.NewEthAddress("0x429881672B9AE42b8EbA0E26cD9C73711b891Ca5")
	require.NoError(t, err)
	//nolint: goconst
	denomA := "footoken"

	t.Run("forward entry with no reverse entry is detected", func(t *testing.T) {
		input := CreateTestEnv(t)
		ctx := input.Context
		k := input.GravityKeeper

		require.NoError(t, k.setCosmosOriginatedMapping(ctx, denomA, *erc20A))
		store := ctx.KVStore(k.storeKey)
		store.Delete(types.GetERC20ToDenomKey(*erc20A))

		err := ValidateCrossListIntegrity(ctx, k)
		require.Error(t, err)
		require.Contains(t, err.Error(), "not bidirectionally consistent")
	})

	t.Run("reverse entry with no forward entry is detected", func(t *testing.T) {
		input := CreateTestEnv(t)
		ctx := input.Context
		k := input.GravityKeeper

		require.NoError(t, k.setCosmosOriginatedMapping(ctx, denomA, *erc20A))
		store := ctx.KVStore(k.storeKey)
		store.Delete(types.GetDenomToERC20Key(denomA))

		err := ValidateCrossListIntegrity(ctx, k)
		require.Error(t, err)
		require.Contains(t, err.Error(), "no forward DenomToERC20 entry")
	})

	t.Run("reverse entry pointing at a mismatched forward denom is detected", func(t *testing.T) {
		input := CreateTestEnv(t)
		ctx := input.Context
		k := input.GravityKeeper
		erc20B, err := types.NewEthAddress("0xF815240800ddf3E0be80e0d848B13ecaa504BF37")
		require.NoError(t, err)
		denomB := "bartoken"

		require.NoError(t, k.setCosmosOriginatedMapping(ctx, denomA, *erc20A))
		require.NoError(t, k.setCosmosOriginatedMapping(ctx, denomB, *erc20B))

		// Corrupt just the reverse pointer for erc20A so it disagrees with the forward index
		store := ctx.KVStore(k.storeKey)
		store.Set(types.GetERC20ToDenomKey(*erc20A), []byte(denomB))

		err = ValidateCrossListIntegrity(ctx, k)
		require.Error(t, err)
		require.Contains(t, err.Error(), "mismatched forward DenomToERC20")
	})
}

// TestValidateCrossListIntegrity_DuplicateERC20 verifies that two denoms mapping to the same
// ERC20 address in the forward index are detected as a duplicate.
func TestValidateCrossListIntegrity_DuplicateERC20(t *testing.T) {
	input := CreateTestEnv(t)
	ctx := input.Context
	k := input.GravityKeeper

	erc20A, err := types.NewEthAddress("0x429881672B9AE42b8EbA0E26cD9C73711b891Ca5")
	require.NoError(t, err)
	//nolint: goconst
	denomA := "footoken"
	denomB := "bartoken"

	require.NoError(t, k.setCosmosOriginatedMapping(ctx, denomA, *erc20A))

	// Directly register a second denom forward-pointing at the same ERC20; setCosmosOriginatedMapping
	// would reject this, so the corrupt state is written straight to the store.
	store := ctx.KVStore(k.storeKey)
	store.Set(types.GetDenomToERC20Key(denomB), erc20A.GetAddress().Bytes())

	err = ValidateCrossListIntegrity(ctx, k)
	require.Error(t, err)
	require.Contains(t, err.Error(), "duplicate cosmos-originated ERC20")
}

// TestValidateCrossListIntegrity_EmbeddedEthAddress verifies that a cosmos-originated denom
// containing an embedded Ethereum address is detected.
func TestValidateCrossListIntegrity_EmbeddedEthAddress(t *testing.T) {
	input := CreateTestEnv(t)
	ctx := input.Context
	k := input.GravityKeeper

	erc20A, err := types.NewEthAddress("0x429881672B9AE42b8EbA0E26cD9C73711b891Ca5")
	require.NoError(t, err)
	// denom itself embeds a valid ethereum address, which setCosmosOriginatedMapping would
	// reject, so write both directions of the corrupt state straight to the store.
	denom := fmt.Sprintf("footoken%s", erc20A.GetAddress().Hex())

	store := ctx.KVStore(k.storeKey)
	store.Set(types.GetDenomToERC20Key(denom), erc20A.GetAddress().Bytes())
	store.Set(types.GetERC20ToDenomKey(*erc20A), []byte(denom))

	err = ValidateCrossListIntegrity(ctx, k)
	require.Error(t, err)
	require.Contains(t, err.Error(), "contains an embedded Ethereum address")
}

// TestValidateCrossListIntegrity_GravityPrefixCollision verifies that a cosmos-originated denom
// colliding with either the gravity or gravity2 eth-originated denom formats is detected.
func TestValidateCrossListIntegrity_GravityPrefixCollision(t *testing.T) {
	erc20A, err := types.NewEthAddress("0x429881672B9AE42b8EbA0E26cD9C73711b891Ca5")
	require.NoError(t, err)

	t.Run("gravity-prefixed denom is detected", func(t *testing.T) {
		input := CreateTestEnv(t)
		ctx := input.Context
		k := input.GravityKeeper

		gravityDenom := types.GravityDenom(*erc20A) // e.g. "gravity0x4298..."
		store := ctx.KVStore(k.storeKey)
		store.Set(types.GetDenomToERC20Key(gravityDenom), erc20A.GetAddress().Bytes())
		store.Set(types.GetERC20ToDenomKey(*erc20A), []byte(gravityDenom))

		err := ValidateCrossListIntegrity(ctx, k)
		require.Error(t, err)
		require.Contains(t, err.Error(), "collides with an eth-originated gravity denom")
	})

	t.Run("gravity2-prefixed denom is detected", func(t *testing.T) {
		input := CreateTestEnv(t)
		ctx := input.Context
		k := input.GravityKeeper

		gravity2Denom := types.Gravity2Denom(*erc20A) // e.g. "gravity20x4298..."
		store := ctx.KVStore(k.storeKey)
		store.Set(types.GetDenomToERC20Key(gravity2Denom), erc20A.GetAddress().Bytes())
		store.Set(types.GetERC20ToDenomKey(*erc20A), []byte(gravity2Denom))

		err := ValidateCrossListIntegrity(ctx, k)
		require.Error(t, err)
		require.Contains(t, err.Error(), "collides with an eth-originated gravity2 denom")
	})
}

// TestValidateCrossListIntegrity_RemappedERC20 verifies that a cosmos-originated ERC20 which
// is also present in the remapped eth-originated set is detected.
func TestValidateCrossListIntegrity_RemappedERC20(t *testing.T) {
	input := CreateTestEnv(t)
	ctx := input.Context
	k := input.GravityKeeper

	erc20A, err := types.NewEthAddress("0x429881672B9AE42b8EbA0E26cD9C73711b891Ca5")
	require.NoError(t, err)

	// Register the mapping first (setCosmosOriginatedMapping would reject it once remapped),
	// then mark the ERC20 as remapped to corrupt the state.
	require.NoError(t, k.setCosmosOriginatedMapping(ctx, "footoken", *erc20A))
	k.SetRemappedERC20(ctx, *erc20A)

	err = ValidateCrossListIntegrity(ctx, k)
	require.Error(t, err)
	require.Contains(t, err.Error(), "is also in the remapped eth-originated set")
}

// TestStoreValidityInvariant verifies that the public StoreValidityInvariant (which wraps
// ValidateStore, including ValidateCrossListIntegrity) passes on clean state and fires when
// the cosmos-originated index is corrupted.
func TestStoreValidityInvariant(t *testing.T) {
	input := CreateTestEnv(t)
	ctx := input.Context
	k := input.GravityKeeper

	erc20A, err := types.NewEthAddress("0x429881672B9AE42b8EbA0E26cD9C73711b891Ca5")
	require.NoError(t, err)
	//nolint: goconst
	denomA := "footoken"

	// Empty state → passes
	res, broken := StoreValidityInvariant(k)(ctx)
	require.False(t, broken, res)

	// Valid mapping → passes
	require.NoError(t, k.setCosmosOriginatedMapping(ctx, denomA, *erc20A))
	res, broken = StoreValidityInvariant(k)(ctx)
	require.False(t, broken, res)

	// Corrupt the reverse index directly → invariant surfaces the cross-list violation
	store := ctx.KVStore(k.storeKey)
	store.Delete(types.GetERC20ToDenomKey(*erc20A))
	res, broken = StoreValidityInvariant(k)(ctx)
	require.True(t, broken)
	require.Contains(t, res, "cross-list integrity violations")
}

// TestCosmosBridgeableTokensInvariant verifies that the CosmosBridgeableTokens invariant
// passes for clean state and fails for corrupted allowlist entries.
func TestCosmosBridgeableTokensInvariant(t *testing.T) {
	input := CreateTestEnv(t)
	ctx := input.Context
	k := input.GravityKeeper

	// Empty state
	res, broken := CosmosBridgeableTokensInvariant(k)(ctx)
	require.False(t, broken, res)

	// Valid entry with matching bank metadata
	validMeta := minMeta("uatom")
	k.SetCosmosBridgeableToken(ctx, validMeta)
	input.BankKeeper.SetDenomMetaData(ctx, validMeta)
	res, broken = CosmosBridgeableTokensInvariant(k)(ctx)
	require.False(t, broken, res)

	store := ctx.KVStore(k.storeKey)

	// Duplicate base denom (fails)
	dupKey := append([]byte{}, types.CosmosBridgeableTokensKey...)
	dupKey = append(dupKey, []byte("uatom-corrupt")...)
	store.Set(dupKey, k.cdc.MustMarshal(&validMeta))
	res, broken = CosmosBridgeableTokensInvariant(k)(ctx)
	require.True(t, broken)
	require.Contains(t, res, "duplicate base denom")
	require.Contains(t, res, "uatom")
	store.Delete(dupKey)

	// Remove the original uatom entry and continue with corrupted entries
	store.Delete(types.GetCosmosBridgeableTokenKey("uatom"))

	// Invalid metadata (fails)
	// nolint: exhaustruct
	invalidMeta := banktypes.Metadata{Base: ""}
	k.SetCosmosBridgeableToken(ctx, invalidMeta)
	res, broken = CosmosBridgeableTokensInvariant(k)(ctx)
	require.True(t, broken)
	require.Contains(t, res, "invalid metadata")
	store.Delete(types.GetCosmosBridgeableTokenKey(""))

	// Invalid base denom (fails)
	slashMeta := minMeta("bad/denom")
	k.SetCosmosBridgeableToken(ctx, slashMeta)
	input.BankKeeper.SetDenomMetaData(ctx, slashMeta)
	res, broken = CosmosBridgeableTokensInvariant(k)(ctx)
	require.True(t, broken)
	require.Contains(t, res, "invalid base denom")
	store.Delete(types.GetCosmosBridgeableTokenKey("bad/denom"))

	// Base denom containing an Ethereum address (fails)
	ethAddrMeta := minMeta("gravity0x429881672B9AE42b8EbA0E26cD9C73711b891Ca5")
	k.SetCosmosBridgeableToken(ctx, ethAddrMeta)
	input.BankKeeper.SetDenomMetaData(ctx, ethAddrMeta)
	res, broken = CosmosBridgeableTokensInvariant(k)(ctx)
	require.True(t, broken)
	require.Contains(t, res, "contains an embedded Ethereum address")
	store.Delete(types.GetCosmosBridgeableTokenKey("gravity0x429881672B9AE42b8EbA0E26cD9C73711b891Ca5"))

	// Missing bank metadata (fails)
	missingBankMeta := minMeta("uosmo")
	k.SetCosmosBridgeableToken(ctx, missingBankMeta)
	res, broken = CosmosBridgeableTokensInvariant(k)(ctx)
	require.True(t, broken)
	require.Contains(t, res, "no bank metadata found")
	input.BankKeeper.SetDenomMetaData(ctx, missingBankMeta)

	// Mismatched bank metadata (fails)
	mismatchedBankMeta := minMeta("uosmo")
	mismatchedBankMeta.Symbol = "MISMATCH"
	input.BankKeeper.SetDenomMetaData(ctx, mismatchedBankMeta)
	res, broken = CosmosBridgeableTokensInvariant(k)(ctx)
	require.True(t, broken)
	require.Contains(t, res, "does not match bank metadata")
}

// allAttestationClaimTypes lists every claim type with a ClaimHashComponents variant, used to
// exercise the AttestationHashIntegrityInvariant tests across the full set of claim shapes.
var allAttestationClaimTypes = []types.ClaimType{
	types.CLAIM_TYPE_SEND_TO_COSMOS,
	types.CLAIM_TYPE_BATCH_SEND_TO_ETH,
	types.CLAIM_TYPE_ERC20_DEPLOYED,
	types.CLAIM_TYPE_LOGIC_CALL_EXECUTED,
	types.CLAIM_TYPE_VALSET_UPDATED,
}

// newTestClaimOfType builds a valid, fully populated EthereumClaim of the given type, using
// eventNonce to keep otherwise-identical claims distinguishable from each other.
// nolint: exhaustruct
func newTestClaimOfType(claimType types.ClaimType, eventNonce uint64) types.EthereumClaim {
	switch claimType {
	case types.CLAIM_TYPE_SEND_TO_COSMOS:
		return &types.MsgSendToCosmosClaim{
			EventNonce:     eventNonce,
			EthBlockHeight: eventNonce + 1000,
			TokenContract:  TokenContractAddrs[0],
			Amount:         sdkmath.NewInt(int64(100 + eventNonce)),
			EthereumSender: EthAddrs[0].String(),
			CosmosReceiver: AccAddrs[0].String(),
			Orchestrator:   AccAddrs[0].String(),
		}
	case types.CLAIM_TYPE_BATCH_SEND_TO_ETH:
		return &types.MsgBatchSendToEthClaim{
			EventNonce:     eventNonce,
			EthBlockHeight: eventNonce + 1000,
			BatchNonce:     eventNonce,
			TokenContract:  TokenContractAddrs[1],
			Orchestrator:   AccAddrs[0].String(),
		}
	case types.CLAIM_TYPE_ERC20_DEPLOYED:
		return &types.MsgERC20DeployedClaim{
			EventNonce:     eventNonce,
			EthBlockHeight: eventNonce + 1000,
			CosmosDenom:    "footoken",
			TokenContract:  TokenContractAddrs[2],
			Name:           "Foo Token",
			Symbol:         "FOO",
			Decimals:       18,
			Orchestrator:   AccAddrs[0].String(),
		}
	case types.CLAIM_TYPE_LOGIC_CALL_EXECUTED:
		return &types.MsgLogicCallExecutedClaim{
			EventNonce:        eventNonce,
			EthBlockHeight:    eventNonce + 1000,
			InvalidationId:    AccAddrs[0].Bytes(),
			InvalidationNonce: eventNonce,
			Orchestrator:      AccAddrs[0].String(),
		}
	case types.CLAIM_TYPE_VALSET_UPDATED:
		return &types.MsgValsetUpdatedClaim{
			EventNonce:     eventNonce,
			ValsetNonce:    eventNonce,
			EthBlockHeight: eventNonce + 1000,
			Members: []types.BridgeValidator{
				{Power: 100, EthereumAddress: EthAddrs[0].String()},
				{Power: 200, EthereumAddress: EthAddrs[1].String()},
			},
			RewardAmount: sdkmath.NewInt(int64(100 + eventNonce)),
			RewardToken:  EthAddrs[2].String(),
			Orchestrator: AccAddrs[0].String(),
		}
	default:
		panic(fmt.Sprintf("newTestClaimOfType: unsupported claim type %v", claimType))
	}
}

// newTestAttestation builds a self-consistent Attestation (and its claim hash) of the given
// claim type using the real ExtractClaimHashComponents logic, mirroring how the keeper itself
// populates ClaimComponents in Attest.
// nolint: exhaustruct
func newTestAttestation(t *testing.T, claimType types.ClaimType, eventNonce uint64) (*types.Attestation, []byte) {
	t.Helper()
	claim := newTestClaimOfType(claimType, eventNonce)

	protoClaim, ok := claim.(gogoproto.Message)
	require.True(t, ok, "claim of type %v does not implement proto.Message", claimType)
	anyClaim, err := codecTypes.NewAnyWithValue(protoClaim)
	require.NoError(t, err)
	components, err := types.ExtractClaimHashComponents(claim)
	require.NoError(t, err)
	hash, err := claim.ClaimHash()
	require.NoError(t, err)

	att := &types.Attestation{
		Claim:           anyClaim,
		Observed:        true,
		Votes:           []string{ValAddrs[0].String()},
		ClaimType:       claimType,
		ClaimComponents: components,
	}
	return att, hash
}

// tamperClaimComponents mutates a single field of the attestation's stored ClaimComponents,
// selecting the field based on the attestation's claim type. This desynchronizes the stored
// components from the stored Any claim without touching the claim itself, exactly the kind of
// tampering AttestationHashIntegrityInvariant is meant to catch.
func tamperClaimComponents(t *testing.T, att *types.Attestation) {
	t.Helper()
	switch att.ClaimType {
	case types.CLAIM_TYPE_SEND_TO_COSMOS:
		comp := att.ClaimComponents.GetSendToCosmos()
		require.NotNil(t, comp)
		comp.Amount = sdkmath.NewInt(999999).String()
	case types.CLAIM_TYPE_BATCH_SEND_TO_ETH:
		comp := att.ClaimComponents.GetBatchSendToEth()
		require.NotNil(t, comp)
		comp.BatchNonce++
	case types.CLAIM_TYPE_ERC20_DEPLOYED:
		comp := att.ClaimComponents.GetErc20Deployed()
		require.NotNil(t, comp)
		comp.Symbol = "TAMPERED"
	case types.CLAIM_TYPE_LOGIC_CALL_EXECUTED:
		comp := att.ClaimComponents.GetLogicCallExecuted()
		require.NotNil(t, comp)
		comp.InvalidationNonce++
	case types.CLAIM_TYPE_VALSET_UPDATED:
		comp := att.ClaimComponents.GetValsetUpdated()
		require.NotNil(t, comp)
		comp.RewardAmount = sdkmath.NewInt(1).String()
	default:
		t.Fatalf("tamperClaimComponents: unsupported claim type %v", att.ClaimType)
	}
}

// TestAttestationHashIntegrityInvariant_Valid verifies that the invariant passes on an empty
// store and continues to pass as multiple consistent attestations of every claim type are added.
func TestAttestationHashIntegrityInvariant_Valid(t *testing.T) {
	input := CreateTestEnv(t)
	ctx := input.Context
	k := input.GravityKeeper

	// Empty state → passes
	res, broken := AttestationHashIntegrityInvariant(k)(ctx)
	require.False(t, broken, res)

	// Populate the store with several consistent attestations of every claim type, checking the
	// invariant after each individual addition as well as at the very end with everything present.
	var nonce uint64
	for _, claimType := range allAttestationClaimTypes {
		for i := 0; i < 3; i++ {
			nonce++
			att, hash := newTestAttestation(t, claimType, nonce)
			k.SetAttestation(ctx, nonce, hash, att)

			res, broken := AttestationHashIntegrityInvariant(k)(ctx)
			require.False(t, broken, res)
		}
	}

	res, broken = AttestationHashIntegrityInvariant(k)(ctx)
	require.False(t, broken, res)
}

// TestAttestationHashIntegrityInvariant_Tampered verifies that the invariant fires when any
// single claim type's stored components have been tampered with, and that it continues to
// detect every tampered attestation when multiple types are corrupted simultaneously alongside
// a larger set of otherwise-valid attestations.
func TestAttestationHashIntegrityInvariant_Tampered(t *testing.T) {
	// Individual check per claim type: a lone tampered attestation of that type is caught.
	for i, claimType := range allAttestationClaimTypes {
		claimType := claimType
		eventNonce := uint64(i + 1)
		t.Run(claimType.String(), func(t *testing.T) {
			input := CreateTestEnv(t)
			ctx := input.Context
			k := input.GravityKeeper

			atts := make(map[uint64]*types.Attestation, len(allAttestationClaimTypes)*2)
			hashes := make(map[uint64][]byte, len(allAttestationClaimTypes)*2)
			var nonce uint64
			for _, claimType := range allAttestationClaimTypes {
				for i := 0; i < 2; i++ {
					nonce++
					att, hash := newTestAttestation(t, claimType, nonce)
					k.SetAttestation(ctx, nonce, hash, att)
					atts[nonce] = att
					hashes[nonce] = hash
				}
			}

			att, hash := newTestAttestation(t, claimType, eventNonce)
			k.SetAttestation(ctx, eventNonce, hash, att)

			// Sanity check: passes before tampering
			res, broken := AttestationHashIntegrityInvariant(k)(ctx)
			require.False(t, broken, res)

			// Tamper with the stored components without touching the stored Any claim, then
			// overwrite the same key so the attestation is no longer internally consistent.
			tamperClaimComponents(t, att)
			k.SetAttestation(ctx, eventNonce, hash, att)

			res, broken = AttestationHashIntegrityInvariant(k)(ctx)
			require.True(t, broken)
			require.Contains(t, res, "broken attestations")
		})
	}

	// Combined check: populate the store with multiple valid attestations of every claim type,
	// then tamper a subset spanning several different types, and verify the invariant still
	// finds exactly those broken attestations amongst all the untouched, still-valid ones.
	t.Run("multiple tampered attestations across types", func(t *testing.T) {
		input := CreateTestEnv(t)
		ctx := input.Context
		k := input.GravityKeeper

		atts := make(map[uint64]*types.Attestation, len(allAttestationClaimTypes)*2)
		hashes := make(map[uint64][]byte, len(allAttestationClaimTypes)*2)
		var nonce uint64
		for _, claimType := range allAttestationClaimTypes {
			for i := 0; i < 2; i++ {
				nonce++
				att, hash := newTestAttestation(t, claimType, nonce)
				k.SetAttestation(ctx, nonce, hash, att)
				atts[nonce] = att
				hashes[nonce] = hash
			}
		}

		// Sanity check: the fully-populated, untampered store passes
		res, broken := AttestationHashIntegrityInvariant(k)(ctx)
		require.False(t, broken, res)

		// Tamper with one attestation from three different claim types:
		// nonce 1 -> CLAIM_TYPE_SEND_TO_COSMOS, nonce 4 -> CLAIM_TYPE_BATCH_SEND_TO_ETH,
		// nonce 9 -> CLAIM_TYPE_VALSET_UPDATED (2 attestations per type, in the order of
		// allAttestationClaimTypes).
		tamperedNonces := []uint64{1, 4, 9}
		for _, n := range tamperedNonces {
			att := atts[n]
			tamperClaimComponents(t, att)
			k.SetAttestation(ctx, n, hashes[n], att)
		}

		res, broken = AttestationHashIntegrityInvariant(k)(ctx)
		require.True(t, broken)
		require.Contains(t, res, "broken attestations")
		// Every tampered attestation, and only those, should be reported as broken; the
		// remaining untouched attestations of every type must not show up as failures.
		require.Equal(t, len(tamperedNonces),
			strings.Count(res, "claim hash from components does not match hash from stored claim"))
	})
}
