package keeper

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/exp/slices"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
)

// Tests IterateBridgeBalanceSnapshots, CollectBridgeBalanceSnapshots
func TestGetBridgeBalanceSnapshots(t *testing.T) {
	input, ctx := SetupFiveValChain(t)
	defer func() { input.Context.Logger().Info("Asserting invariants at test end"); input.AssertInvariants() }()

	pk := input.GravityKeeper
	tokens := pk.MonitoredERC20Tokens(ctx)

	var balances []*types.ERC20Token
	for _, t := range tokens {
		bal := types.ERC20Token{Contract: t.GetAddress().String(), Amount: sdk.OneInt()}
		balances = append(balances, &bal)
	}
	// The balances which
	slices.SortFunc(balances, func(a, b *types.ERC20Token) bool {
		if a == nil || b == nil {
			panic("nil balance when trying to sort snapshot balances")
		}
		return a.Contract < b.Contract
	})

	// Create test snapshots
	numSnaps := 3000
	snapshots := make([]types.BridgeBalanceSnapshot, numSnaps)
	for i := 0; i < numSnaps; i++ {
		snap := types.BridgeBalanceSnapshot{
			CosmosBlockHeight:   uint64(ctx.BlockHeight()),
			EthereumBlockHeight: uint64(1234567 + i),
			Balances:            balances,
			EventNonce:          uint64(i + 1),
		}
		pk.storeBridgeBalanceSnapshot(ctx, snap)
		snapshots[i] = snap
	}

	// Iterate in ascending order
	pk.IterateBridgeBalanceSnapshots(ctx, false,
		func(key []byte, snapshot types.BridgeBalanceSnapshot) (stop bool) {
			n, err := types.ExtractNonceFromBridgeBalanceSnapshotKey(key)
			if err != nil || n != snapshot.EventNonce {
				panic(fmt.Sprintf("bad key (%v) snap (%v) nonce (%v): err %v", key, snapshot, snapshot.EventNonce, err))
			}
			expectedSnap := snapshots[snapshot.EventNonce-1]
			require.Equal(t, expectedSnap, snapshot)
			return false
		},
	)

	collectedSnaps := pk.CollectBridgeBalanceSnapshots(ctx, false, uint64(numSnaps))
	require.Equalf(t, len(collectedSnaps), len(snapshots),
		"bad number of snaps returned (%v) compared to those stored (%v)",
		len(collectedSnaps), len(snapshots),
	)
	for i := 0; i < len(collectedSnaps); i++ {
		require.Equal(t, snapshots[i].CosmosBlockHeight, collectedSnaps[i].CosmosBlockHeight)
		require.Equal(t, snapshots[i].EthereumBlockHeight, collectedSnaps[i].EthereumBlockHeight)
		expectedBalances := snapshots[i].Balances
		collectedBalances := collectedSnaps[i].Balances
		for j := 0; j < len(expectedBalances); j++ {
			require.Equal(t, expectedBalances[j], collectedBalances[j])
		}
	}
}

func Test_fetchAndStoreBridgeBalanceSnapshot(t *testing.T) {
	input, ctx := SetupFiveValChain(t)
	defer func() { input.Context.Logger().Info("Asserting invariants at test end"); input.AssertInvariants() }()

	pk := input.GravityKeeper
	tokens := pk.MonitoredERC20Tokens(ctx)
	require.Greater(t, len(tokens), 1, "Need at least 2 monitored ERC20 tokens for this test")
	var desiredSupplies sdk.Coins
	for i, t := range tokens {
		coinDenom := types.GravityDenom(t)
		amount := sdk.NewInt(int64(1_000000 * (i + 1)))
		coin := sdk.NewCoin(coinDenom, amount)
		desiredSupplies = append(desiredSupplies, coin)
	}

	desiredSupplies = desiredSupplies.Sort()
	require.NoError(t, desiredSupplies.Validate())
	bk := pk.bankKeeper
	err := bk.MintCoins(ctx, "gravity", desiredSupplies)
	require.NoError(t, err)
	err = bk.SendCoinsFromModuleToModule(ctx, "gravity", "bank", desiredSupplies)
	require.NoError(t, err)

	claim := types.MsgBatchSendToEthClaim{
		EventNonce:     1,
		EthBlockHeight: 12345,
		BatchNonce:     1,
		TokenContract:  tokens[0].GetAddress().String(),
		Orchestrator:   OrchAddrs[0].String(),
	}

	snapshot := pk.FetchBridgeBalanceSnapshot(ctx, &claim)
	pk.storeBridgeBalanceSnapshot(ctx, snapshot)

	require.NoError(t, err)

	// Confirm that the newly stored snapshot is as expected
	snaps := pk.CollectBridgeBalanceSnapshots(ctx, false, 1)
	require.Equal(t, len(snaps), 1)
	snap := snaps[0]
	require.Equal(t, snap.CosmosBlockHeight, uint64(ctx.BlockHeight()))
	require.Equal(t, snap.EthereumBlockHeight, uint64(12345))
	for _, token := range snap.Balances {
		ctr, err := types.NewEthAddress(token.Contract)
		require.NoError(t, err)
		denom := types.GravityDenom(*ctr)

		supply := pk.bankKeeper.GetSupply(ctx, denom)
		require.Equal(t, token.Amount, supply.Amount)
	}
}

// Checks that the UnaccountedGravityModuleBalances function can correctly detect unbatched transactions, batched transactions,
// and multiple tokens
func Test_UnaccountedGravityModuleBalances(t *testing.T) {
	input := CreateTestEnv(t)
	defer func() { input.Context.Logger().Info("Asserting invariants at test end"); input.AssertInvariants() }()

	ctx := input.Context
	var (
		sender, e1     = sdk.AccAddressFromBech32("gravity1ahx7f8wyertuus9r20284ej0asrs085ceqtfnm")
		myReceiver     = "0xd041c41EA1bf0F006ADBb6d2c9ef9D425dE5eaD7"
		tokenContract1 = "0x429881672B9AE42b8EbA0E26cD9C73711b891Ca5"
		tokenContract2 = "0xd041c41EA1bf0F006ADBb6d2c9ef9D425dE5eaD7"
	)
	require.NoError(t, e1)
	receiver, err := types.NewEthAddress(myReceiver)
	require.NoError(t, err)
	// mint some voucher first
	token1Vouchers, err := types.NewInternalERC20Token(sdk.NewInt(99999), tokenContract1)
	require.NoError(t, err)
	token2Vouchers, err := types.NewInternalERC20Token(sdk.NewInt(99999), tokenContract2)
	require.NoError(t, err)
	allVouchers := sdk.Coins{token1Vouchers.GravityCoin(), token2Vouchers.GravityCoin()}
	err = input.BankKeeper.MintCoins(ctx, types.ModuleName, allVouchers)
	require.NoError(t, err)
	// set senders balance
	input.AccountKeeper.NewAccountWithAddress(ctx, sender)
	err = input.BankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, sender, allVouchers)
	require.NoError(t, err)

	////////////////// EXECUTE //////////////////
	amountToken, err := types.NewInternalERC20Token(sdk.NewInt(100), tokenContract1)
	require.NoError(t, err)
	amount := amountToken.GravityCoin()
	feeToken, err := types.NewInternalERC20Token(sdk.NewInt(1), tokenContract1)
	require.NoError(t, err)
	fee := feeToken.GravityCoin()
	// Create some unbatched transactions
	for i := 0; i < 4; i++ {
		r, err := input.GravityKeeper.AddToOutgoingPool(ctx, sender, *receiver, amount, fee)
		require.NotZero(t, r)
		require.NoError(t, err)
		// Should create 4 transactions, all amount 100 and fee 1
	}
	expGravBal := int64(404) // 4 * 100 (amounts) + 4 * 1 (fees)
	bals := input.GravityKeeper.UnaccountedGravityModuleBalances(ctx)
	require.Equal(t, 1, len(bals))                                         // only 1 token held
	require.True(t, strings.HasSuffix(bals[0].GetDenom(), tokenContract1)) // Must be the gravity denom of the erc20
	require.Equal(t, expGravBal, bals[0].Amount.Int64())                   // Must be the amount in all the transactions

	// Remove one of the transactions
	err = input.GravityKeeper.RemoveFromOutgoingPoolAndRefund(ctx, 1, sender)
	require.NoError(t, err)

	expGravBal -= 101 // remove the cancelled amount and fee
	bals = input.GravityKeeper.UnaccountedGravityModuleBalances(ctx)
	require.Equal(t, 1, len(bals))
	require.True(t, strings.HasSuffix(bals[0].GetDenom(), tokenContract1))
	require.Equal(t, expGravBal, bals[0].Amount.Int64())

	// Create a batch using the remaining transactions
	batch, err := input.GravityKeeper.BuildOutgoingTXBatch(ctx, amountToken.Contract, 3)
	batchTV := batch.TotalValue()
	require.NoError(t, err)
	require.Equal(t, expGravBal, batchTV.Amount.Int64())
	require.True(t, strings.HasSuffix(batchTV.GetDenom(), tokenContract1))

	// Add another transaction after the new batch
	r, err := input.GravityKeeper.AddToOutgoingPool(ctx, sender, *receiver, amount, fee)
	require.NotZero(t, r)
	require.NoError(t, err)

	expGravBal += 101 // Existing batch + new tx amount and fees
	bals = input.GravityKeeper.UnaccountedGravityModuleBalances(ctx)
	require.Equal(t, 1, len(bals))
	require.True(t, strings.HasSuffix(bals[0].GetDenom(), tokenContract1))
	require.Equal(t, expGravBal, bals[0].Amount.Int64())

	// Add a tx for a different contract
	newAmount, err := types.NewInternalERC20Token(sdk.NewInt(100), tokenContract2)
	require.NoError(t, err)
	newFee, err := types.NewInternalERC20Token(sdk.NewInt(1), tokenContract2)
	require.NoError(t, err)
	r, err = input.GravityKeeper.AddToOutgoingPool(ctx, sender, *receiver, newAmount.GravityCoin(), newFee.GravityCoin())
	require.NotZero(t, r)
	require.NoError(t, err)

	bals = input.GravityKeeper.UnaccountedGravityModuleBalances(ctx)
	require.Equal(t, 2, len(bals)) // Existing batch, unbatched tx for contract 1 | new unbatched tx for contract 2
	for _, b := range bals {
		if strings.HasSuffix(b.GetDenom(), tokenContract1) {
			require.Equal(t, expGravBal, b.Amount.Int64()) // Existing balance should not change
		} else {
			require.True(t, strings.HasSuffix(b.GetDenom(), tokenContract2))
			require.Equal(t, int64(101), b.Amount.Int64()) // 100 (newAmount) + 1 (newFee)
		}
	}
}
