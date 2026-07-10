package recovery

import (
	"context"
	"fmt"

	errorsmod "cosmossdk.io/errors"
	sdkmath "cosmossdk.io/math"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	crisiskeeper "github.com/cosmos/cosmos-sdk/x/crisis/keeper"

	gravitykeeper "github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/keeper"
	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
)

// gravityModuleBalance is a small debugging helper that returns the gravity module account's
// balance of the given denom. It is used purely for logging so operators can see exactly what
// the module account holds at each step of the recovery upgrade.
func gravityModuleBalance(ctx sdk.Context, bk *bankkeeper.BaseKeeper, denom string) sdk.Coin {
	modAddr := authtypes.NewModuleAddress(types.ModuleName)
	return bk.GetBalance(ctx, modAddr, denom)
}

// sumTrackedEscrowForContract is a debugging helper that adds up every source of escrowed balance:
// unconfirmed batches, unbatched txs, and pending IBC Auto Forwards
func sumTrackedEscrowForContract(ctx sdk.Context, k *gravitykeeper.Keeper, tokenContract types.EthAddress, denom string) sdkmath.Int {
	total := sdkmath.ZeroInt()

	k.IterateOutgoingTxBatches(ctx, func(_ []byte, batch types.InternalOutgoingTxBatch) bool {
		if batch.TokenContract.GetAddress() == tokenContract.GetAddress() {
			for _, tx := range batch.Transactions {
				total = total.Add(tx.Erc20Token.Amount).Add(tx.Erc20Fee.Amount)
			}
		}
		return false
	})

	k.IterateUnbatchedTransactionsByContract(ctx, tokenContract, func(_ []byte, tx *types.InternalOutgoingTransferTx) bool {
		total = total.Add(tx.Erc20Token.Amount).Add(tx.Erc20Fee.Amount)
		return false
	})

	for _, forward := range k.PendingIbcAutoForwards(ctx, 0) {
		if forward.Token.Denom == denom {
			total = total.Add(forward.Token.Amount)
		}
	}

	return total
}

// recoveryVoidAddress is a deterministically-derived address with no known private key and no
// registered module account behavior anywhere in the app. It exists solely as a destination for
// permanently removing leftover, unclassifiable token balances discovered during the recovery
// upgrade (see sweepOrphanedDenomToBurnAddress). Using an address like this ensures the tokens
// can't be used for anything (unlike the community pool) and allows the invariants to pass
var recoveryVoidAddress = authtypes.NewModuleAddress("gravity/recovery-void")

func sweepMaliciousDenomToVoidAddress(ctx sdk.Context, bk *bankkeeper.BaseKeeper, denom string) error {
	modAddr := authtypes.NewModuleAddress(types.ModuleName)
	bal := bk.GetBalance(ctx, modAddr, denom)
	if bal.Amount.IsZero() {
		ctx.Logger().Info("Recovery upgrade: no balance to sweep for orphaned denom", "denom", denom)
		return nil
	}

	if err := bk.SendCoinsFromModuleToAccount(ctx, types.ModuleName, recoveryVoidAddress, sdk.NewCoins(bal)); err != nil {
		return errorsmod.Wrapf(err, "failed to sweep orphaned denom %s to void address", denom)
	}

	ctx.Logger().Info("Recovery upgrade: swept malicious ibc denom to void address",
		"denom", denom,
		"sweptAmount", bal.Amount.String(),
		"voidAddress", recoveryVoidAddress.String(),
	)

	return nil
}

// breakFalseCosmosOriginatedMapping breaks the malicious cosmos-originated mapping for the given ERC20 tokenContract, if one exists.
func breakFalseCosmosOriginatedMapping(ctx sdk.Context, k *gravitykeeper.Keeper, tokenContract types.EthAddress) (string, bool) {
	var foundDenom string
	var found bool
	k.IterateCosmosOriginatedMappings(ctx, func(denom string, erc20 *types.EthAddress) bool {
		if erc20.GetAddress() == tokenContract.GetAddress() {
			foundDenom = denom
			found = true
			return true // stop iterating, only one denom can map to a given ERC20
		}
		return false
	})

	if !found {
		ctx.Logger().Info("Recovery upgrade: no cosmos-originated mapping found for ERC20, nothing to break",
			"erc20", tokenContract.GetAddress().Hex())
		return "", false
	}

	k.DeleteCosmosOriginatedMapping(ctx, tokenContract, foundDenom)
	ctx.Logger().Info("Recovery upgrade: deleted false cosmos-originated mapping",
		"erc20", tokenContract.GetAddress().Hex(), "denom", foundDenom,
	)
	return foundDenom, true
}

// GetRecoveryUpgradeHandler returns the upgrade handler for the recovery upgrade.
func GetRecoveryUpgradeHandler(
	mm *module.Manager,
	configurator *module.Configurator,
	crisisKeeper *crisiskeeper.Keeper,
	gravityKeeper *gravitykeeper.Keeper,
	bankKeeper *bankkeeper.BaseKeeper,
) func(c context.Context, plan upgradetypes.Plan, vmap module.VersionMap) (module.VersionMap, error) {
	if mm == nil || configurator == nil || crisisKeeper == nil || gravityKeeper == nil || bankKeeper == nil {
		panic("Nil argument to GetRecoveryUpgradeHandler")
	}
	return func(c context.Context, plan upgradetypes.Plan, vmap module.VersionMap) (module.VersionMap, error) {
		ctx := sdk.UnwrapSDKContext(c)

		ctx.Logger().Info("Recovery upgrade: starting",
			"planName", plan.Name,
			"planHeight", plan.Height,
			"blockHeight", ctx.BlockHeight(),
			"incomingVersionMap", vmap,
		)

		ctx.Logger().Info("Recovery upgrade: running module migrations")
		out, err := mm.RunMigrations(ctx, *configurator, vmap)
		if err != nil {
			ctx.Logger().Error("Recovery upgrade: module migrations failed", "error", err)
			return out, err
		}
		ctx.Logger().Info("Recovery upgrade: module migrations complete", "outgoingVersionMap", out)

		ctx.Logger().Info("Recovery upgrade: remapping affected ERC20 tokens")

		migrateRemappedERC20s(ctx, gravityKeeper, bankKeeper)

		ctx.Logger().Info("Recovery upgrade: registering cosmos-originated tokens as CosmosBridgeableTokens")
		if err := registerCosmosBridgeableTokens(ctx, gravityKeeper, bankKeeper); err != nil {
			ctx.Logger().Error("Recovery upgrade: registerCosmosBridgeableTokens failed", "error", err)
			return out, err
		}

		ctx.Logger().Info("Recovery upgrade: checking for pending IBC Auto Forwards")
		assertNoPendingRemappedIbcAutoForwards(ctx, gravityKeeper)

		ctx.Logger().Info("Recovery upgrade: disabling the bridge to Ethereum until governance re-enables it")
		setBridgeActive(ctx, gravityKeeper, false)

		params, err := gravityKeeper.GetParams(ctx)
		if err != nil {
			ctx.Logger().Error("Recovery upgrade: unable to get params", "error", err)
			panic(fmt.Sprintf("Recover upgrade: Unable to get params: %v", err))
		}
		bridgeActive := params.GetBridgeActive()

		ctx.Logger().Info("Recovery upgrade: BridgeActive param set", "bridgeActive", bridgeActive)

		ctx.Logger().Info("Recovery upgrade: dumping gravity module account balances before asserting invariants")
		modAddr := authtypes.NewModuleAddress(types.ModuleName)
		unclassifiable := 0
		for _, coin := range bankKeeper.GetAllBalances(ctx, modAddr) {
			if _, err := gravityKeeper.ClassifyDenom(ctx, coin.Denom); err != nil {
				unclassifiable++
				ctx.Logger().Error("Recovery upgrade: unable to classify gravity module balance denom, this will break the module-balance invariant",
					"denom", coin.Denom, "amount", coin.Amount.String(), "error", err,
				)
			}
		}

		ctx.Logger().Info("Recovery upgrade: asserting invariants")
		crisisKeeper.AssertInvariants(ctx)

		if unclassifiable > 0 {
			panic(fmt.Sprintf("Recovery upgrade FAILURE: %d unclassifiable gravity module balance denoms found", unclassifiable))
		}

		ctx.Logger().Info("Recovery upgrade: SUCCESS")
		return out, nil
	}
}

// remapEntry holds the details of a single remapped ERC20
type remapEntry struct {
	erc20            types.EthAddress
	problemOldDenom  string // a CosmosOriginated ERC20 denom
	remappedNewDenom string // gravity2...  - after the remap, given on deposits and allowed to be withdrawn
}

// migrateRemappedERC20s detects every DenomToERC20 entry whose denom contains an embedded
// Ethereum address which is on the ERC20ToDenom mapping, and will:
//   - cancel and refund all in-flight outgoing transactions,
//   - deletes the cosmos-originated mapping,
//   - sets the RemappedERC20 entry so new deposits use gravity2 and bridge-out of old vouchers is blocked,
//   - asserts that no bank metadata exists for either the old or new denoms
func migrateRemappedERC20s(ctx sdk.Context, k *gravitykeeper.Keeper, bk *bankkeeper.BaseKeeper) {
	var remapped []remapEntry

	for _, erc20 := range tokensToRemap {
		ethAddress, err := types.NewEthAddress(erc20)
		if err != nil {
			ctx.Logger().Error("Recovery upgrade: invalid erc20 address in tokensToRemap", "erc20", erc20, "error", err)
			panic(fmt.Sprintf("invalid erc20 address in tokensToRemap: %s", erc20))
		}
		gravityDenom := types.GravityDenom(*ethAddress)
		gravity2Denom := types.Gravity2Denom(*ethAddress)
		remapped = append(remapped, remapEntry{
			erc20:            *ethAddress,
			problemOldDenom:  gravityDenom,
			remappedNewDenom: gravity2Denom,
		})
	}

	if len(remapped) == 0 {
		panic("Recovery upgrade: no ERC20 tokens to remap?")
	}

	ctx.Logger().Info(fmt.Sprintf("Recovery upgrade: ERC20 tokens to remap: %d", len(remapped)))

	for _, e := range remapped {
		balBefore := gravityModuleBalance(ctx, bk, e.problemOldDenom)
		trackedBefore := sumTrackedEscrowForContract(ctx, k, e.erc20, e.problemOldDenom)
		untrackedBefore := balBefore.Amount.Sub(trackedBefore)
		ctx.Logger().Info("Recovery upgrade: remapping ERC20",
			"erc20", e.erc20.GetAddress().Hex(),
			"problemOldDenom", e.problemOldDenom,
			"remappedNewDenom", e.remappedNewDenom,
			"moduleBalanceOldDenomBeforeCancel", balBefore.Amount.String(),
			"trackedEscrowBeforeCancel", trackedBefore.String(),
			"untrackedBalanceBeforeCancel", untrackedBefore.String(),
		)
		if !untrackedBefore.IsZero() {
			ctx.Logger().Error("Recovery upgrade: untracked balance exists for ERC20, this will break the module-balance invariant",
				"erc20", e.erc20.GetAddress().Hex(),
				"problemOldDenom", e.problemOldDenom,
				"untrackedBalance", untrackedBefore.String(),
			)
		}

		// Break malicious mappings
		if falseDenom, found := breakFalseCosmosOriginatedMapping(ctx, k, e.erc20); found {
			// If the module holds any malicious balances, this will break the module-balance invariant at some point.
			if err := sweepMaliciousDenomToVoidAddress(ctx, bk, falseDenom); err != nil {
				ctx.Logger().Error("Recovery upgrade: failed to sweep malicious denom balance",
					"erc20", e.erc20.GetAddress().Hex(), "denom", falseDenom, "error", err)
				panic(fmt.Sprintf("recovery: failed to sweep orphaned denom %s for %s: %v",
					falseDenom, e.erc20.GetAddress().Hex(), err))
			}
		}

		// cancel and refund all in-flight outgoing transactions
		if err := CancelAllOutgoingTxsForContract(ctx, k, bk, e.erc20); err != nil {
			ctx.Logger().Error("Recovery upgrade: failed to cancel outgoing txs",
				"erc20", e.erc20.GetAddress().Hex(), "error", err)
			panic(fmt.Sprintf("recovery: failed to cancel outgoing txs for %s: %v",
				e.erc20.GetAddress().Hex(), err))
		}

		// set the remapped flag
		k.SetRemappedERC20(ctx, e.erc20)
		ctx.Logger().Info("Recovery upgrade: set RemappedERC20 flag", "erc20", e.erc20.GetAddress().Hex())

		// Check that denom metadata DOES NOT exist for either the old or the new denoms.
		if _, exists := bk.GetDenomMetaData(ctx, e.problemOldDenom); exists {
			ctx.Logger().Error("Recovery upgrade: unexpected bank metadata for old gravity denom", "denom", e.problemOldDenom)
			panic(fmt.Sprintf("Recovery upgrade: denom metadata exists for old gravity denom? (%s)", e.problemOldDenom))
		}
		if _, exists := bk.GetDenomMetaData(ctx, e.remappedNewDenom); exists {
			ctx.Logger().Error("Recovery upgrade: unexpected bank metadata for new gravity2 denom", "denom", e.remappedNewDenom)
			panic(fmt.Sprintf("Recovery upgrade: denom metadata exists for new gravity2 denom? (%s)", e.remappedNewDenom))
		}

		ctx.Logger().Info("Recovery upgrade: ERC20 remapped successfully",
			"erc20", e.erc20.GetAddress().Hex(),
			"gravity2_denom", e.remappedNewDenom,
		)
	}

	ctx.Logger().Info(fmt.Sprintf(
		"Recovery upgrade: remapped %d ERC20 token(s). ",
		len(remapped),
	))
}

// registerCosmosBridgeableTokens adds all cosmos-originated ERC20 tokens that remain after
// remapping (i.e. the non-problem tokens) to the CosmosBridgeableTokens allowlist in the keeper.
// This must be called after migrateRemappedERC20s, which deletes remapped entries from the store,
// so only legitimate tokens are seen here.
func registerCosmosBridgeableTokens(ctx sdk.Context, k *gravitykeeper.Keeper, bk *bankkeeper.BaseKeeper) error {
	// Ensure that no CosmosBridgeableTokens exist yet
	existing := k.GetAllCosmosBridgeableTokens(ctx)
	for _, d := range existing {
		ctx.Logger().Error("Recovery upgrade: CosmosBridgeableTokens already exists", "denom", d.Base)
		panic(fmt.Sprintf("Recovery upgrade: CosmosBridgeableTokens already exist in keeper? (%s)", d.Base))
	}

	var newEntries []banktypes.Metadata
	var mappingCount int
	k.IterateCosmosOriginatedMappings(ctx, func(denom string, erc20 *types.EthAddress) bool {
		mappingCount++
		meta, found := bk.GetDenomMetaData(ctx, denom)
		if !found || meta.Base == "" {
			ctx.Logger().Error("Recovery upgrade: cosmos-originated denom has no bank metadata", "denom", denom)
			panic(fmt.Sprintf(
				"recovery: cosmos-originated ERC20 denom %q has no bank metadata",
				denom,
			))
		}
		newEntries = append(newEntries, meta)
		return false
	})

	if len(newEntries) == 0 {
		ctx.Logger().Error("Recovery upgrade: no new cosmos-originated tokens to add to CosmosBridgeableTokens?")
		panic("Recovery upgrade: no new cosmos-originated tokens to add to CosmosBridgeableTokens?")
	}

	for _, m := range newEntries {
		ctx.Logger().Info("Recovery upgrade: registering cosmos-originated token as CosmosBridgeableToken",
			"denom", m.Base,
		)
		k.SetCosmosBridgeableToken(ctx, m)
	}

	ctx.Logger().Info(fmt.Sprintf(
		"Recovery upgrade: registered %d cosmos-originated token(s) as CosmosBridgeableTokens",
		len(newEntries),
	))
	return nil
}
func setBridgeActive(ctx sdk.Context, k *gravitykeeper.Keeper, v bool) {
	params, err := k.GetParams(ctx)
	if err != nil {
		ctx.Logger().Error("Recovery upgrade: failed to get params", "error", err)
		panic(fmt.Sprintf("failed to get params: %v", err))
	}

	params.BridgeActive = v
	if err := k.SetParams(ctx, params); err != nil {
		ctx.Logger().Error("Recovery upgrade: failed to set params", "error", err)
		panic(fmt.Sprintf("failed to set params: %v", err))
	}
}

// CancelAllOutgoingTxsForContract cancels and refunds all pending outgoing bridge
// transactions (both unconfirmed batches and unbatched pool entries) for the given
// ERC20 contract address.
//
// This must be called while the contract's cosmos-originated denom mapping is still
// intact (before DeleteCosmosOriginatedDenomToERC20 and SetRemappedERC20) so that
// ERC20ToDenomLookup returns the old gravity0x denom and RemoveFromOutgoingPoolAndRefund
// refunds users in the correct pre-remap denom.
//
// Note on fees: the chain fee (MsgSendToEth.ChainFee) is paid to stakers/auction before
// the transaction ever enters the pool and is therefore not returned.  Only the send
// amount and bridge fee are escrowed in the module account and will be refunded.
func CancelAllOutgoingTxsForContract(ctx sdk.Context, k *gravitykeeper.Keeper, bk *bankkeeper.BaseKeeper, tokenContract types.EthAddress) error {
	var batchNonces []uint64
	batchTxTotal := sdkmath.ZeroInt()
	batchTxCount := 0
	k.IterateOutgoingTxBatches(ctx, func(_ []byte, batch types.InternalOutgoingTxBatch) bool {
		if batch.TokenContract.GetAddress() == tokenContract.GetAddress() {
			batchNonces = append(batchNonces, batch.BatchNonce)
			batchTotal := sdkmath.ZeroInt()
			for _, tx := range batch.Transactions {
				txTotal := tx.Erc20Token.Amount.Add(tx.Erc20Fee.Amount)
				batchTotal = batchTotal.Add(txTotal)
			}
			batchTxTotal = batchTxTotal.Add(batchTotal)
			batchTxCount += len(batch.Transactions)
		}
		return false
	})
	ctx.Logger().Info("Recovery upgrade: found unconfirmed batches to cancel",
		"erc20", tokenContract.GetAddress().Hex(), "batchCount", len(batchNonces), "nonces", batchNonces,
		"totalTxsInBatches", batchTxCount, "totalEscrowedInBatches", batchTxTotal.String())

	for _, nonce := range batchNonces {
		if err := k.CancelOutgoingTXBatch(ctx, tokenContract, nonce); err != nil {
			ctx.Logger().Error("Recovery upgrade: failed to cancel batch",
				"erc20", tokenContract.GetAddress().Hex(), "nonce", nonce, "error", err)
			return errorsmod.Wrapf(err, "recovery: failed to cancel batch with nonce %d for contract %s",
				nonce, tokenContract.GetAddress().Hex())
		}
		ctx.Logger().Info("Recovery upgrade: canceled batch successfully",
			"erc20", tokenContract.GetAddress().Hex(), "nonce", nonce)
	}
	ctx.Logger().Info("Recovery upgrade: finished canceling batches, all transactions moved back into unbatched pool",
		"erc20", tokenContract.GetAddress().Hex(), "batchCount", len(batchNonces))

	// Refund all unbatched pool entries for this contract.
	type txEntry struct {
		id     uint64
		sender sdk.AccAddress
		amount sdkmath.Int
		fee    sdkmath.Int
	}
	var pending []txEntry
	pendingTotal := sdkmath.ZeroInt()
	k.IterateUnbatchedTransactionsByContract(ctx, tokenContract, func(_ []byte, tx *types.InternalOutgoingTransferTx) bool {
		pending = append(pending, txEntry{id: tx.Id, sender: tx.Sender, amount: tx.Erc20Token.Amount, fee: tx.Erc20Fee.Amount})
		pendingTotal = pendingTotal.Add(tx.Erc20Token.Amount).Add(tx.Erc20Fee.Amount)
		return false
	})
	ctx.Logger().Info("Recovery upgrade: found unbatched pool entries to refund",
		"erc20", tokenContract.GetAddress().Hex(), "pendingCount", len(pending), "pendingTotal", pendingTotal.String())

	refundedTotal := sdkmath.ZeroInt()
	for _, entry := range pending {
		if err := k.RemoveFromOutgoingPoolAndRefund(ctx, entry.id, entry.sender); err != nil {
			ctx.Logger().Error("Recovery upgrade: failed to refund unbatched tx",
				"erc20", tokenContract.GetAddress().Hex(), "txId", entry.id, "sender", entry.sender.String(), "error", err)
			return errorsmod.Wrapf(err, "recovery: failed to refund tx %d for contract %s",
				entry.id, tokenContract.GetAddress().Hex())
		}
		refundedTotal = refundedTotal.Add(entry.amount).Add(entry.fee)
	}
	ctx.Logger().Info("Recovery upgrade: finished refunding unbatched pool entries",
		"erc20", tokenContract.GetAddress().Hex(), "refundedCount", len(pending), "refundedTotal", refundedTotal.String())

	return nil
}

// assertNoPendingRemappedIbcAutoForwards asserts that there are no pending IBC Auto Forwards for any of the remapped ERC20s
func assertNoPendingRemappedIbcAutoForwards(ctx sdk.Context, k *gravitykeeper.Keeper) {
	forwards := k.PendingIbcAutoForwards(ctx, 0)
	for _, forward := range forwards {
		// Try gravity2 denom first - none of these should exist before the upgrade
		if tc, err := types.Gravity2DenomToERC20(forward.Token.Denom); err == nil {
			ctx.Logger().Error("Recovery upgrade: found pending IBC Auto Forward for a gravity2 denom before the upgrade ran",
				"erc20", tc.GetAddress().Hex(), "denom", forward.Token.Denom)
			panic(fmt.Sprintf("Recovery upgrade: found pending IBC Auto Forward for remapped ERC20?: %s (%s)",
				tc.GetAddress().Hex(), forward.Token.Denom))
		}
		// Then gravity denom
		if tc, err := types.GravityDenomToERC20(forward.Token.Denom); err == nil {
			if k.IsRemappedERC20(ctx, *tc) {
				ctx.Logger().Error("Recovery upgrade: found pending IBC Auto Forward for a remapped ERC20's old denom",
					"erc20", tc.GetAddress().Hex(), "denom", forward.Token.Denom)
				panic(fmt.Sprintf("Recovery upgrade: found pending IBC Auto Forward for remapped ERC20: %s (gravity2 denom: %s)",
					tc.GetAddress().Hex(), forward.Token.Denom))
			}
			continue
		}
		// Cosmos-originated or unknown - not a remapped ERC20
	}
	ctx.Logger().Info("Recovery upgrade: no pending IBC Auto Forwards found for remapped ERC20s")
}
