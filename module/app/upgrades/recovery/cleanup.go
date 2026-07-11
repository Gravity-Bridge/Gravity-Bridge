package recovery

import (
	sdk "github.com/cosmos/cosmos-sdk/types"

	gravitykeeper "github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/keeper"
	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
)

// cleanupOrphanedValsetConfirms deletes valset confirmations (validator signatures) whose
// corresponding valset no longer exists in state. These orphaned confirms accumulate over time
// because confirm pruning can lag valset pruning; they are inert (valset slashing only iterates
// valsets that still exist), but leaving them behind bloats state and muddies the resume snapshot.
//
// It is safe to run while the bridge is paused: no new valset confirms reference a pruned valset,
// and removing a confirm for a non-existent valset cannot affect slashing or Ethereum submission.
func cleanupOrphanedValsetConfirms(ctx sdk.Context, k *gravitykeeper.Keeper) {
	// Collect first, delete after, so we never mutate the store mid-iteration.
	type orphan struct {
		nonce        uint64
		confirmCount int
	}
	var orphans []orphan

	k.IterateValsetConfirms(ctx, func(_ []byte, confirms []types.MsgValsetConfirm, nonce uint64) bool {
		if k.GetValset(ctx, nonce) == nil {
			orphans = append(orphans, orphan{nonce: nonce, confirmCount: len(confirms)})
		}
		return false
	})

	totalDeleted := 0
	for _, o := range orphans {
		k.DeleteValsetConfirms(ctx, o.nonce)
		totalDeleted += o.confirmCount
		// Per-nonce detail is emitted at Debug level to keep the default upgrade log concise.
		ctx.Logger().Debug("Recovery upgrade: deleted orphaned valset confirms",
			"nonce", o.nonce, "confirmCount", o.confirmCount)
	}

	ctx.Logger().Info("Recovery upgrade: orphaned valset confirm cleanup complete",
		"orphanedNonces", len(orphans), "confirmsDeleted", totalDeleted)
}

// cleanupOrphanedBatchConfirms deletes batch confirmations (validator signatures) whose
// corresponding outgoing tx batch no longer exists in state. After the ERC20 remap, the malicious
// tokens' batches are gone but their historical confirms linger; more generally, confirms for any
// executed/timed-out batch can outlive the batch. These confirms are inert (batch slashing only
// iterates batches that still exist and no batch can be re-submitted for a confirm with no batch),
// so removing them is safe while the bridge is paused and keeps the resume state clean.
func cleanupOrphanedBatchConfirms(ctx sdk.Context, k *gravitykeeper.Keeper) {
	// Collect distinct (tokenContract, nonce) pairs that have confirms, counting confirms per pair,
	// before deleting anything so we never mutate the store mid-iteration.
	type pair struct {
		contract string
		nonce    uint64
	}
	counts := map[pair]int{}
	order := []pair{}

	k.IterateBatchConfirms(ctx, func(_ []byte, confirm types.MsgConfirmBatch) bool {
		p := pair{contract: confirm.TokenContract, nonce: confirm.Nonce}
		if _, seen := counts[p]; !seen {
			order = append(order, p)
		}
		counts[p]++
		return false
	})

	orphanedPairs := 0
	totalDeleted := 0
	for _, p := range order {
		ethAddr, err := types.NewEthAddress(p.contract)
		if err != nil {
			ctx.Logger().Error("Recovery upgrade: batch confirm has invalid token contract, skipping",
				"tokenContract", p.contract, "nonce", p.nonce, "error", err)
			continue
		}

		// If a batch still exists for this (contract, nonce) the confirms are live; leave them.
		if k.GetOutgoingTXBatch(ctx, *ethAddr, p.nonce) != nil {
			continue
		}

		// Orphaned: delete every confirm for this (contract, nonce).
		k.DeleteBatchConfirms(ctx, types.InternalOutgoingTxBatch{
			BatchNonce:    p.nonce,
			TokenContract: *ethAddr,
		})
		orphanedPairs++
		totalDeleted += counts[p]
		// Per-nonce detail is emitted at Debug level to keep the default upgrade log concise.
		ctx.Logger().Debug("Recovery upgrade: deleted orphaned batch confirms",
			"erc20", ethAddr.GetAddress().Hex(), "nonce", p.nonce, "confirmCount", counts[p])
	}

	ctx.Logger().Info("Recovery upgrade: orphaned batch confirm cleanup complete",
		"orphanedBatchNonces", orphanedPairs, "confirmsDeleted", totalDeleted)
}
