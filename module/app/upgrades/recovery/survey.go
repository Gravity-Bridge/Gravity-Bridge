package recovery

import (
	"fmt"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	gravitykeeper "github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/keeper"
	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
)

// logResumeStateSnapshot is a read-only diagnostic helper used by the recovery upgrade.
//
// It enumerates and logs gravity module state that is relevant to safely
// resuming consensus. It is intentionally side-effect free (it only
// reads state and writes logs) so it can be called multiple times during the upgrade — e.g.
// once BEFORE the remap/cancel logic runs ("pre-remap") to capture the raw production state
// the chain is upgrading from, and once at the very END ("resume") to capture the exact
// state consensus will resume with.
//
//	These logs can then be reviewed to check for consistency and migration bugs.
func logResumeStateSnapshot(ctx sdk.Context, k *gravitykeeper.Keeper, stage string) {
	ctx.Logger().Info("Recovery upgrade: ===== RESUME-STATE SNAPSHOT BEGIN =====", "stage", stage)

	logPendingBatches(ctx, k, stage)
	logUnbatchedPool(ctx, k, stage)
	logBatchConfirms(ctx, k, stage)
	logLogicCalls(ctx, k, stage)
	logValsetsAndConfirms(ctx, k, stage)
	logPendingIbcAutoForwards(ctx, k, stage)

	// BridgeActive is the master switch controlling whether the bridge is paused. It should be
	// true in the "pre-remap" snapshot (chain was live) and false in the "resume" snapshot.
	if params, err := k.GetParams(ctx); err != nil {
		ctx.Logger().Error("Recovery upgrade: snapshot could not read params", "stage", stage, "error", err)
	} else {
		ctx.Logger().Info("Recovery upgrade: snapshot BridgeActive", "stage", stage, "bridgeActive", params.GetBridgeActive())
	}

	ctx.Logger().Info("Recovery upgrade: ===== RESUME-STATE SNAPSHOT END =====", "stage", stage)
}

// logPendingBatches enumerates every unconfirmed outgoing tx batch, grouped by ERC20 contract,
// reporting the number of batches, their nonces, the number of transactions, and the total
// escrowed amount (send + bridge fee) per token. This answers "exact state of any pending
// batches the bridge is resuming with".
func logPendingBatches(ctx sdk.Context, k *gravitykeeper.Keeper, stage string) {
	type agg struct {
		batchCount int
		txCount    int
		nonces     []uint64
		escrow     sdkmath.Int
	}
	byToken := map[string]*agg{}
	totalBatches := 0

	k.IterateOutgoingTxBatches(ctx, func(_ []byte, batch types.InternalOutgoingTxBatch) bool {
		token := batch.TokenContract.GetAddress().Hex()
		a := byToken[token]
		if a == nil {
			a = &agg{escrow: sdkmath.ZeroInt()}
			byToken[token] = a
		}
		a.batchCount++
		totalBatches++
		a.nonces = append(a.nonces, batch.BatchNonce)
		for _, tx := range batch.Transactions {
			a.txCount++
			a.escrow = a.escrow.Add(tx.Erc20Token.Amount).Add(tx.Erc20Fee.Amount)
		}
		return false
	})

	ctx.Logger().Info("Recovery upgrade: pending outgoing batches", "stage", stage, "totalBatches", totalBatches, "distinctTokens", len(byToken))
	for token, a := range byToken {
		ctx.Logger().Info("Recovery upgrade: pending batches for token",
			"stage", stage,
			"erc20", token,
			"batchCount", a.batchCount,
			"nonces", fmt.Sprintf("%v", a.nonces),
			"txCount", a.txCount,
			"escrow", a.escrow.String(),
		)
	}
}

// logUnbatchedPool enumerates every unbatched outgoing transfer in the pool, grouped by ERC20
// contract, reporting the count and total escrowed amount (send + bridge fee) per token.
func logUnbatchedPool(ctx sdk.Context, k *gravitykeeper.Keeper, stage string) {
	type agg struct {
		count  int
		escrow sdkmath.Int
	}
	byToken := map[string]*agg{}
	total := 0

	k.IterateUnbatchedTransactions(ctx, func(_ []byte, tx *types.InternalOutgoingTransferTx) bool {
		token := tx.Erc20Token.Contract.GetAddress().Hex()
		a := byToken[token]
		if a == nil {
			a = &agg{escrow: sdkmath.ZeroInt()}
			byToken[token] = a
		}
		a.count++
		total++
		a.escrow = a.escrow.Add(tx.Erc20Token.Amount).Add(tx.Erc20Fee.Amount)
		return false
	})

	ctx.Logger().Info("Recovery upgrade: unbatched pool transactions", "stage", stage, "totalTxs", total, "distinctTokens", len(byToken))
	for token, a := range byToken {
		ctx.Logger().Info("Recovery upgrade: unbatched pool for token",
			"stage", stage, "erc20", token, "txCount", a.count, "escrow", a.escrow.String(),
		)
	}
}

// logBatchConfirms enumerates the pending batch confirmations (validator signatures), grouped by
// ERC20 contract. These are the pending signatures the bridge resumes with, and are still allowed
// to be submitted while paused (to avoid unfair slashing).
func logBatchConfirms(ctx sdk.Context, k *gravitykeeper.Keeper, stage string) {
	byToken := map[string]int{}
	total := 0
	k.IterateBatchConfirms(ctx, func(_ []byte, confirm types.MsgConfirmBatch) bool {
		byToken[confirm.TokenContract]++
		total++
		return false
	})
	ctx.Logger().Info("Recovery upgrade: pending batch confirms (signatures)", "stage", stage, "totalConfirms", total, "distinctTokens", len(byToken))
	for token, count := range byToken {
		ctx.Logger().Info("Recovery upgrade: batch confirms for token", "stage", stage, "erc20", token, "confirmCount", count)
	}
}

// logLogicCalls enumerates every pending outgoing logic call and its confirmations (signatures).
// Logic calls are relevant to resume because MsgConfirmLogicCall is BLOCKED while the bridge is
// paused, so any logic call still inside its SignedLogicCallsWindow at resume is a potential
// unfair-slashing surface. A total of zero here means there is no logic-call slashing exposure.
func logLogicCalls(ctx sdk.Context, k *gravitykeeper.Keeper, stage string) {
	calls := k.GetOutgoingLogicCalls(ctx)
	ctx.Logger().Info("Recovery upgrade: pending outgoing logic calls", "stage", stage, "logicCallCount", len(calls))
	for _, call := range calls {
		ctx.Logger().Info("Recovery upgrade: pending logic call",
			"stage", stage,
			"invalidationId", fmt.Sprintf("%x", call.InvalidationId),
			"invalidationNonce", call.InvalidationNonce,
			"logicContract", call.LogicContractAddress,
			"timeout", call.Timeout,
			"cosmosBlockCreated", call.CosmosBlockCreated,
			"transfers", len(call.Transfers),
			"fees", len(call.Fees),
		)
	}

	confirmsByCall := map[string]int{}
	totalConfirms := 0
	k.IterateLogicConfirms(ctx, func(_ []byte, confirm *types.MsgConfirmLogicCall) bool {
		key := fmt.Sprintf("%s/%d", confirm.InvalidationId, confirm.InvalidationNonce)
		confirmsByCall[key]++
		totalConfirms++
		return false
	})
	ctx.Logger().Info("Recovery upgrade: logic call confirms (signatures)",
		"stage", stage, "totalConfirms", totalConfirms, "distinctLogicCalls", len(confirmsByCall))
	for key, count := range confirmsByCall {
		ctx.Logger().Info("Recovery upgrade: logic call confirms for call", "stage", stage, "invalidationIdNonce", key, "confirmCount", count)
	}
}

// logValsetsAndConfirms enumerates the stored valsets and their confirmations. On resume the
// chain will require confirms for any un-confirmed valset (and slashing continues while paused),
// so it is important to know exactly which valset nonces exist and which have confirms.
func logValsetsAndConfirms(ctx sdk.Context, k *gravitykeeper.Keeper, stage string) {
	valsets := k.GetValsets(ctx)
	var nonces []uint64
	for _, v := range valsets {
		nonces = append(nonces, v.Nonce)
	}
	ctx.Logger().Info("Recovery upgrade: stored valsets", "stage", stage, "valsetCount", len(valsets), "nonces", fmt.Sprintf("%v", nonces))

	if latest := k.GetLatestValset(ctx); latest != nil {
		ctx.Logger().Info("Recovery upgrade: latest valset", "stage", stage, "nonce", latest.Nonce, "height", latest.Height, "members", len(latest.Members))
	} else {
		ctx.Logger().Info("Recovery upgrade: no latest valset exists", "stage", stage)
	}

	totalNonces := 0
	totalConfirms := 0
	k.IterateValsetConfirms(ctx, func(_ []byte, confirms []types.MsgValsetConfirm, nonce uint64) bool {
		totalNonces++
		totalConfirms += len(confirms)
		// Per-nonce detail is emitted at Debug level to keep the default upgrade log concise.
		ctx.Logger().Debug("Recovery upgrade: valset confirms for nonce", "stage", stage, "nonce", nonce, "confirmCount", len(confirms))
		return false
	})
	ctx.Logger().Info("Recovery upgrade: valset confirms (signatures)", "stage", stage, "noncesWithConfirms", totalNonces, "totalConfirms", totalConfirms)
}

// logPendingIbcAutoForwards enumerates EVERY pending IBC Auto Forward (not just those for remapped
// tokens), classifying each as remapped-gravity2 / remapped-gravity1 / legitimate-gravity /
// cosmos-or-unknown, and logging the denom, amount, target IBC channel and foreign receiver.
//
// This is the key signal for the "stale IBC channels" concern: pending auto-forwards can be fired
// on resume via MsgExecuteIbcAutoForwards (which is NOT gated by the bridge-paused param) and would
// attempt to relay over channels that may be expired/frozen after the downtime. A total count of
// zero here means we are "home free" on that concern.
func logPendingIbcAutoForwards(ctx sdk.Context, k *gravitykeeper.Keeper, stage string) {
	forwards := k.PendingIbcAutoForwards(ctx, 0)

	var remapped, legitimate, cosmosOrUnknown int
	for _, forward := range forwards {
		denom := forward.Token.Denom
		category := "cosmos-or-unknown"

		if tc, err := types.Gravity2DenomToERC20(denom); err == nil {
			remapped++
			category = "remapped-gravity2"
			ctx.Logger().Error("Recovery upgrade: pending IBC Auto Forward for remapped gravity2 denom (UNEXPECTED)",
				"stage", stage, "erc20", tc.GetAddress().Hex(), "denom", denom,
				"amount", forward.Token.Amount.String(), "ibcChannel", forward.IbcChannel,
				"foreignReceiver", forward.ForeignReceiver, "eventNonce", forward.EventNonce)
			continue
		}

		if tc, err := types.GravityDenomToERC20(denom); err == nil {
			if k.IsRemappedERC20(ctx, *tc) {
				remapped++
				category = "remapped-gravity1"
				ctx.Logger().Error("Recovery upgrade: pending IBC Auto Forward for a remapped ERC20's old denom (UNEXPECTED)",
					"stage", stage, "erc20", tc.GetAddress().Hex(), "denom", denom,
					"amount", forward.Token.Amount.String(), "ibcChannel", forward.IbcChannel,
					"foreignReceiver", forward.ForeignReceiver, "eventNonce", forward.EventNonce)
				continue
			}
			legitimate++
			category = "legitimate-gravity"
		} else {
			cosmosOrUnknown++
		}

		ctx.Logger().Info("Recovery upgrade: pending IBC Auto Forward",
			"stage", stage, "category", category, "denom", denom,
			"amount", forward.Token.Amount.String(), "ibcChannel", forward.IbcChannel,
			"foreignReceiver", forward.ForeignReceiver, "eventNonce", forward.EventNonce)
	}

	ctx.Logger().Info("Recovery upgrade: pending IBC Auto Forward summary",
		"stage", stage, "total", len(forwards),
		"remapped", remapped, "legitimate", legitimate, "cosmosOrUnknown", cosmosOrUnknown)

	if len(forwards) > 0 {
		ctx.Logger().Warn("Recovery upgrade: pending IBC Auto Forwards exist; these can be relayed on resume via MsgExecuteIbcAutoForwards over possibly-stale channels",
			"stage", stage, "total", len(forwards))
	}
}
