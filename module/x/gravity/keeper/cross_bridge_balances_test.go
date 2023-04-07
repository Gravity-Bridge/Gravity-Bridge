package keeper

import (
	"fmt"
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
	bk.SendCoinsFromModuleToModule(ctx, "gravity", "bank", desiredSupplies)

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
