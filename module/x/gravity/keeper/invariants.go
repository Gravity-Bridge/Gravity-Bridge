package keeper

import (
	"fmt"

	"github.com/althea-net/cosmos-gravity-bridge/module/x/gravity/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// TODO: Add any future invariants here
// TODO: (see the sdk docs for more info https://docs.cosmos.network/master/building-modules/invariants.html)
func AllInvariants(k Keeper) sdk.Invariant {
	return func(ctx sdk.Context) (string, bool) {
		return ModuleBalanceInvariant(k)(ctx)

		// Example additional invariants
		//  res, stop := FutureInvariant(k)(ctx)
		//	if stop {
		//		return res, stop
		//	}
		//
		//	return AnotherFutureInvariant(k)(ctx)
	}
}

// Checks that the module account's balance is equal to the balance of unbatched transactions and unobserved batches
// Note that the returned bool should be true if there is an error, e.g. an unexpected module balance
func ModuleBalanceInvariant(k Keeper) sdk.Invariant {
	return func(ctx sdk.Context) (string, bool) {
		modAcc := k.accountKeeper.GetModuleAddress(types.ModuleName)
		actualBals := k.bankKeeper.GetAllBalances(ctx, modAcc)
		expectedBals := make(map[string]*sdk.Int, len(actualBals)) // Collect balances by contract
		for _, v := range actualBals {
			newInt := sdk.NewInt(0)
			expectedBals[v.Denom] = &newInt
		}

		// The module is given the balance of all unobserved batches
		k.IterateOutgoingTXBatches(ctx, func(_ []byte, batch types.InternalOutgoingTxBatch) bool {
			batchTotal := sdk.NewInt(0)
			// Collect the send amount + fee amount for each tx
			for _, tx := range batch.Transactions {
				newTotal := batchTotal.Add(tx.Erc20Token.Amount.Add(tx.Erc20Fee.Amount))
				batchTotal = newTotal
			}
			contract := batch.TokenContract
			_, denom := k.ERC20ToDenomLookup(ctx, contract)
			// Add the batch total to the contract counter
			denomTotal := expectedBals[denom].Add(batchTotal)
			expectedBals[denom] = &denomTotal

			return false // continue iterating
		})
		// It is also given the balance of all unbatched txs in the pool
		k.IterateUnbatchedTransactions(ctx, []byte(types.OutgoingTXPoolKey), func(_ []byte, tx *types.InternalOutgoingTransferTx) bool {
			contract := tx.Erc20Token.Contract
			_, denom := k.ERC20ToDenomLookup(ctx, contract)

			// Collect the send amount + fee amount for each tx
			txTotal := tx.Erc20Token.Amount.Add(tx.Erc20Fee.Amount)
			*expectedBals[denom] = expectedBals[denom].Add(txTotal)

			return false // continue iterating
		})

		for _, actual := range actualBals {
			if expected, ok := expectedBals[actual.GetDenom()]; !ok {
				return fmt.Sprint("Could not find contract matching module balance of ", actual), true
			} else if !expected.Equal(actual.Amount) {
				return fmt.Sprint("Mismatched balance of ", actual.Denom, " actual balance ", actual.Amount, " expected balance ", expected), true
			}
		}
		return "", false
	}
}
