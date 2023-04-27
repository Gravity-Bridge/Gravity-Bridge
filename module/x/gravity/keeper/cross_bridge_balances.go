// This file deals with the MonitoredERC20Tokens list and their associated BridgeBalanceSnapshots,
// which are store entries containing the Cosmos Height, claim Ethereum Height, and monitored erc20 bank supply for
// each applied Attestation the moment after its changes take effect
// These BridgeBalanceSnapshots are used both by the gravity module to
package keeper

import (
	"encoding/hex"
	"fmt"
	"math"

	"golang.org/x/exp/slices"

	"github.com/cosmos/cosmos-sdk/store/prefix"
	sdk "github.com/cosmos/cosmos-sdk/types"
	db "github.com/tendermint/tm-db"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
)

// MonitoredERC20Tokens fetches the current list of ERC20 tokens which the Orchestrators should monitor
func (k Keeper) MonitoredERC20Tokens(ctx sdk.Context) []types.EthAddress {
	store := ctx.KVStore(k.storeKey)
	key := types.MonitoredERC20TokensKey
	var addresses []types.EthAddress
	if !store.Has(key) {
		return addresses
	}

	tokenBz := store.Get(key)
	var monitoredErc20s types.MonitoredERC20Addresses
	k.cdc.MustUnmarshal(tokenBz, &monitoredErc20s)

	return types.FromMonitoredERC20Addresses(monitoredErc20s)
}

// setMonitoredERC20Tokens will update the list of ERC20 tokens which the Orchestrators should monitor,
// Note that this list should ONLY be updated via governance or as part of an upgrade which includes consensus on the token list!
func (k Keeper) setMonitoredERC20Tokens(ctx sdk.Context, erc20s types.EthAddresses) {
	store := ctx.KVStore(k.storeKey)
	key := types.MonitoredERC20TokensKey

	storeVal := erc20s.ToMonitoredERC20Addresses()
	bytes := k.cdc.MustMarshal(&storeVal)

	store.Set(key, bytes)
}

// MonitoredERC20TokenDenoms fetches the MonitoredERC20Tokens, gets their denom equivalent from the store, and separates
// the values into two slices, the cosmosOriginated denoms and the ethOriginated denoms (returned in that order)
func (k Keeper) MonitoredERC20TokenDenoms(ctx sdk.Context) (cosmosOriginated []string, ethOriginated []string) {
	monitoredTokens := k.MonitoredERC20Tokens(ctx)
	for _, token := range monitoredTokens {
		isCosmosOriginated, denom := k.ERC20ToDenomLookup(ctx, token)
		if isCosmosOriginated {
			cosmosOriginated = append(cosmosOriginated, denom)
		} else {
			ethOriginated = append(ethOriginated, denom)
		}
	}

	return
}

// IterateBridgeBalanceSnapshots will call `cb` on every discovered BridgeBalanceSnapshot in the store,
// returning early if `cb` returns true, additionally exposing the event nonce encoded in `key`
// The snapshots are iterated in order of ascending event nonce (oldest first) if `reverse` is false,
// ascending (newest first) otherwise
func (k Keeper) IterateBridgeBalanceSnapshots(
	ctx sdk.Context,
	reverse bool,
	cb func(key []byte, snapshot types.BridgeBalanceSnapshot) (stop bool),
) {
	store := ctx.KVStore(k.storeKey)
	pref := types.BridgeBalanceSnapshotsKey
	prefixStore := prefix.NewStore(store, pref)
	var iter db.Iterator
	var lastNonce uint64
	if !reverse { // Ascending (oldest first)
		iter = prefixStore.Iterator(nil, nil)
		lastNonce = 0
	} else { // Descending (newest first)
		iter = prefixStore.ReverseIterator(nil, nil)
		lastNonce = math.MaxUint64
	}

	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		key := iter.Key()
		keyWithPrefix := types.AppendBytes(pref, key)

		// nolint: exhaustruct
		snap := types.BridgeBalanceSnapshot{}
		k.cdc.MustUnmarshal(iter.Value(), &snap)

		if !reverse && lastNonce >= snap.EventNonce { // Expect incrementing nonces
			panic(fmt.Sprintf("Non-incrementing snapshot event nonce discovered: last nonce %v current snapshot nonce %v", lastNonce, snap.EventNonce))
		}
		if reverse && lastNonce <= snap.EventNonce { // Expect decrementing nonces
			panic(fmt.Sprintf("Non-decrementing snapshot event nonce discovered: last nonce %v current snapshot nonce %v", lastNonce, snap.EventNonce))
		}
		lastNonce = snap.EventNonce
		// cb returns true to stop early
		if cb(keyWithPrefix, snap) {
			break
		}
	}
}

// CollectBridgeBalanceSnapshots will iterate through the snapshots in the store, collecting them into a slice
// If `limit` is positive, only `limit` results will be returned
// The snapshots are returned in order of ascending event nonce (oldest first) if `reverse` is false,
// ascending (newest first) if true
func (k Keeper) CollectBridgeBalanceSnapshots(ctx sdk.Context, reverse bool, limit uint64) []*types.BridgeBalanceSnapshot {
	var snapshots []*types.BridgeBalanceSnapshot
	k.IterateBridgeBalanceSnapshots(ctx, reverse,
		func(_ []byte, snapshot types.BridgeBalanceSnapshot) (stop bool) {
			snapshots = append(snapshots, &snapshot)
			return limit == uint64(len(snapshots)) // Halt now if the limit has been collected
		},
	)
	return snapshots
}

// updateBridgeBalanceSnapshots will store a new snapshot with the current supply of each MonitoredERC20Token()
// and perform a sanity check to ensure that the current state and previous state match what `att` should suggest
// This performs one half of the checking for Cross-Bridge Balances, the other half is performed in the Orchestrator
// where a mismatch on the Ethereum side will halt Orchestrator operation
func (k Keeper) updateBridgeBalanceSnapshots(ctx sdk.Context, claim types.EthereumClaim, expectedSupplyChange sdk.Coins) error {
	snapshot := k.FetchBridgeBalanceSnapshot(ctx, claim)
	if snapshot.IsEmpty() {
		// There are no elligible balances to store, return early
		return nil
	}
	k.storeBridgeBalanceSnapshot(ctx, snapshot)
	return k.AssertBridgeBalanceSanity(ctx, claim, expectedSupplyChange)
}

// FetchBridgeBalanceSnapshot creates a BridgeBalanceSnapshot for the given claim under
// the BridgeBalanceSnapshotsKey + `claim`'s event nonce, populated with the supply from x/bank
func (k Keeper) FetchBridgeBalanceSnapshot(ctx sdk.Context, claim types.EthereumClaim) types.BridgeBalanceSnapshot {
	snapshotBalances := k.FetchBridgedTokenBalances(ctx)
	return types.NewBridgeBalanceSnapshot(uint64(ctx.BlockHeight()), claim.GetEthBlockHeight(), snapshotBalances, claim.GetEventNonce())
}

// FetchMonitoredERC20Supply collects the supply of each Monitored ERC20 Token's associated denom
// For Ethereum originated assets, total supply is collected. For Cosmos originated assets,
// just the gravity module's accounted-for balances are collected. The balances are returned with the ERC20 address,
// not the cosmos denom (e.g. gravity0x... or ugraviton)
func (k Keeper) FetchBridgedTokenBalances(ctx sdk.Context) types.InternalERC20Tokens {
	var balances types.InternalERC20Tokens
	gravityModuleAcc := k.accountKeeper.GetModuleAccount(ctx, types.ModuleName).GetAddress()
	var internalToken *types.InternalERC20Token // Declaration for use in iterator
	var err error                               // Declaration for use in iterator
	var isCosmosOriginated bool                 // Declaration for use in iterator
	var erc20 *types.EthAddress                 // Declaration for use in iterator
	k.bankKeeper.IterateTotalSupply(ctx, func(c sdk.Coin) bool {
		isCosmosOriginated, erc20, err = k.DenomToERC20Lookup(ctx, c.Denom)
		if err != nil {
			// Cosmos originated denom has not been deployed on ethereum, do nothing
			return false
		}

		if isCosmosOriginated { // Cosmos originated asset, collect gravity module balance only
			gravityBalance := k.bankKeeper.GetBalance(ctx, gravityModuleAcc, c.Denom)
			token := types.ERC20Token{Contract: erc20.GetAddress().String(), Amount: gravityBalance.Amount}
			internalToken, err = token.ToInternal()
		} else { // Ethereum originated asset, collect the whole bank supply
			token := types.ERC20Token{Contract: erc20.GetAddress().String(), Amount: c.Amount}
			internalToken, err = token.ToInternal()
		}
		if err != nil {
			errMsg := fmt.Sprintf("Invalid ERC20 returned from DenomToERC20Lookup: %v", err)
			k.logger(ctx).Error(errMsg)
			panic(errMsg)
		}
		balances = append(balances, internalToken)

		return false
	})

	// Gravity holds some assets which are not accounted-for yet, like ibc auto forwards and pending batch txs
	// These may be refunded at any time so they are not included in the snapshot
	unaccountedBalances := k.CoinsToInternalERC20Tokens(ctx, k.UnaccountedGravityModuleBalances(ctx))

	balances.Sort()
	unaccountedBalances.Sort()
	balances = balances.SubSorted(unaccountedBalances)

	return balances
}

func (k Keeper) CoinsToInternalERC20Tokens(ctx sdk.Context, coins sdk.Coins) types.InternalERC20Tokens {
	gravityModuleAcc := k.accountKeeper.GetModuleAccount(ctx, types.ModuleName).GetAddress()
	var tokens types.InternalERC20Tokens

	var internalToken *types.InternalERC20Token // Declaration for use in loop
	var err error                               // Declaration for use in loop
	for _, c := range coins {
		var isCosmosOriginated bool
		var erc20 *types.EthAddress
		isCosmosOriginated, erc20, err = k.DenomToERC20Lookup(ctx, c.Denom)
		if err != nil {
			// Cosmos originated denom has not been deployed on ethereum, do nothing
			continue
		}

		if isCosmosOriginated { // Cosmos originated asset, collect gravity module balance only
			gravityBalance := k.bankKeeper.GetBalance(ctx, gravityModuleAcc, c.Denom)
			token := types.ERC20Token{Contract: erc20.GetAddress().String(), Amount: gravityBalance.Amount}
			internalToken, err = token.ToInternal()
		} else { // Ethereum originated asset, collect the whole bank supply
			token := types.ERC20Token{Contract: erc20.GetAddress().String(), Amount: c.Amount}
			internalToken, err = token.ToInternal()
		}
		if err != nil {
			errMsg := fmt.Sprintf("Invalid ERC20 returned from DenomToERC20Lookup: %v", err)
			k.logger(ctx).Error(errMsg)
			panic(errMsg)
		}
		tokens = append(tokens, internalToken)
	}

	return tokens
}

// storeBridgeBalanceSnapshot stores the given snapshot at its appropriate key
func (k Keeper) storeBridgeBalanceSnapshot(ctx sdk.Context, snapshot types.BridgeBalanceSnapshot) {
	store := ctx.KVStore(k.storeKey)
	key := types.GetBridgeBalanceSnapshotKey(snapshot.EventNonce)

	// Sort the balances by contract address for consistency
	slices.SortFunc(snapshot.Balances, func(a, b *types.ERC20Token) bool {
		if a == nil || b == nil {
			panic("nil balance when trying to sort snapshot balances")
		}
		return a.Contract < b.Contract
	})

	store.Set(key, k.cdc.MustMarshal(&snapshot))
}

// deleteBridgeBalanceSnapshot deletes the snapshot with the given eventNonce, returning an error if no such entry exists
func (k Keeper) DeleteBridgeBalanceSnapshot(ctx sdk.Context, eventNonce uint64) error {
	store := ctx.KVStore(k.storeKey)
	key := types.GetBridgeBalanceSnapshotKey(eventNonce)

	if !store.Has(key) {
		return fmt.Errorf("snapshot with key %v does not exist in store", hex.EncodeToString(key))
	}

	store.Delete(key)
	if store.Has(key) {
		panic(fmt.Sprintf("Unable to delete store entry with key %x", key))
	}
	return nil
}

// AssertBridgeBalanceSanity compares the current (ultimate) and previous (penultimate) BridgeBalanceSnapshots against the
// given Attestation to make sure that the actual token balances reflect what should have happened
func (k Keeper) AssertBridgeBalanceSanity(ctx sdk.Context, claim types.EthereumClaim, expectedSupplyChange sdk.Coins) error {
	snaps := k.CollectBridgeBalanceSnapshots(ctx, true, uint64(2))
	if len(snaps) != 2 {
		k.logger(ctx).Info("Too few snapshots stored to make assertions - skipping for now! There should only be at most 2 of these warnings.")
		return nil
	}
	// The most recent (including att's state changes) and previous (not including att's state changes) snapshots
	ultimate, penultimate := snaps[0], snaps[1]

	ultBals, err := types.ERC20Tokens(ultimate.Balances).ToInternal()
	if err != nil {
		return fmt.Errorf("unable to convert latest bridge balances (%v) to internal type: %v", ultimate.Balances, err)
	}
	penultBals, err := types.ERC20Tokens(penultimate.Balances).ToInternal()
	if err != nil {
		return fmt.Errorf("unable to convert previous bridge balances (%v) to internal type: %v", penultimate.Balances, err)
	}

	// ultimate and penultimate were stored with sorted balances, so we can safely use SubSorted
	actualDiff := ultBals.SubSorted(penultBals)
	if len(actualDiff) > 1 {
		return fmt.Errorf(
			"unexpected actual monitored token supply difference - too many tokens modified: expected (%v) != actual (%v)",
			expectedSupplyChange, actualDiff,
		)
	}

	actualTokens := actualDiff.ToCoins()
	if actualTokens.IsAnyNegative() {
		for i, t := range actualTokens {
			amount := t.Amount.Abs()
			actualTokens[i] = sdk.Coin{Amount: amount, Denom: t.Denom}
		}
	}

	unexpectedDiff, _ := actualTokens.SafeSub(expectedSupplyChange)

	if !unexpectedDiff.IsZero() {
		return fmt.Errorf(
			"!!! unexpected difference (%v) between actual supply change (%v) and expected supply change (%v) for claim %v !!!",
			unexpectedDiff.String(), actualTokens, expectedSupplyChange, claim,
		)
	}

	return nil
}
