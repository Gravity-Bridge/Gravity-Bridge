package recovery

import (
	"context"
	"fmt"

	errorsmod "cosmossdk.io/errors"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	crisiskeeper "github.com/cosmos/cosmos-sdk/x/crisis/keeper"

	gravitykeeper "github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/keeper"
	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
)

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

		ctx.Logger().Info("Recovery upgrade: running module migrations")
		out, err := mm.RunMigrations(ctx, *configurator, vmap)
		if err != nil {
			return out, err
		}

		ctx.Logger().Info("Recovery upgrade: remapping affected ERC20 tokens")

		migrateRemappedERC20s(ctx, gravityKeeper, bankKeeper)

		ctx.Logger().Info("Recovery upgrade: registering cosmos-originated tokens as CosmosBridgeableTokens")
		if err := registerCosmosBridgeableTokens(ctx, gravityKeeper, bankKeeper); err != nil {
			return out, err
		}

		ctx.Logger().Info("Recovery upgrade: checking for pending IBC Auto Forwards")
		assertNoPendingRemappedIbcAutoForwards(ctx, gravityKeeper)

		ctx.Logger().Info("Recovery upgrade: disabling the bridge to Ethereum until governance re-enables it")
		setBridgeActive(ctx, gravityKeeper, false)

		params, err := gravityKeeper.GetParams(ctx)
		if err != nil {
			panic(fmt.Sprintf("Recover upgrade: Unable to get params: %v", err))
		}
		bridgeActive := params.GetBridgeActive()

		ctx.Logger().Info("Recovery upgrade: BridgeActive param set to %v", bridgeActive)

		ctx.Logger().Info("Recovery upgrade: asserting invariants")
		crisisKeeper.AssertInvariants(ctx)

		ctx.Logger().Info("Recovery upgrade: complete")
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
		ctx.Logger().Info("Recovery upgrade: remapping ERC20",
			"erc20", e.erc20.GetAddress().Hex(),
			"problemOldDenom", e.problemOldDenom,
			"remappedNewDenom", e.remappedNewDenom,
		)

		// cancel and refund all in-flight outgoing transactions
		// Must happen BEFORE deleting the cosmos-originated mapping and setting the
		// remapped flag
		if err := CancelAllOutgoingTxsForContract(ctx, k, e.erc20); err != nil {
			panic(fmt.Sprintf("recovery: failed to cancel outgoing txs for %s: %v",
				e.erc20.GetAddress().Hex(), err))
		}

		// delete the problem cosmos-originated denom ERC20 mapping
		k.DeleteCosmosOriginatedMapping(ctx, e.erc20, e.problemOldDenom)

		// set the remapped flag
		k.SetRemappedERC20(ctx, e.erc20)

		// Check that denom metadata DOES NOT exist for either the old or the new denoms.
		if _, exists := bk.GetDenomMetaData(ctx, e.problemOldDenom); exists {
			panic(fmt.Sprintf("Recovery upgrade: denom metadata exists for old gravity denom? (%s)", e.problemOldDenom))
		}
		if _, exists := bk.GetDenomMetaData(ctx, e.remappedNewDenom); exists {
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
	for _, d := range k.GetAllCosmosBridgeableTokens(ctx) {
		panic(fmt.Sprintf("Recovery upgrade: CosmosBridgeableTokens already exist in keeper? (%s)", d.Base))
	}

	var newEntries []banktypes.Metadata
	k.IterateCosmosOriginatedMappings(ctx, func(denom string, _ *types.EthAddress) bool {
		meta, found := bk.GetDenomMetaData(ctx, denom)
		if !found || meta.Base == "" {
			panic(fmt.Sprintf(
				"recovery: cosmos-originated ERC20 denom %q has no bank metadata",
				denom,
			))
		}
		newEntries = append(newEntries, meta)
		return false
	})

	if len(newEntries) == 0 {
		ctx.Logger().Info("Recovery upgrade: no new cosmos-originated tokens to add to CosmosBridgeableTokens")
		return nil
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
		panic(fmt.Sprintf("failed to get params: %v", err))
	}

	params.BridgeActive = v

	if err := k.SetParams(ctx, params); err != nil {
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
func CancelAllOutgoingTxsForContract(ctx sdk.Context, k *gravitykeeper.Keeper, tokenContract types.EthAddress) error {
	// Cancel all unconfirmed batches for this contract.
	// CancelOutgoingTXBatch moves each batch's transactions back into the unbatched pool.
	var batchNonces []uint64
	k.IterateOutgoingTxBatches(ctx, func(_ []byte, batch types.InternalOutgoingTxBatch) bool {
		if batch.TokenContract.GetAddress() == tokenContract.GetAddress() {
			batchNonces = append(batchNonces, batch.BatchNonce)
		}
		return false
	})
	for _, nonce := range batchNonces {
		if err := k.CancelOutgoingTXBatch(ctx, tokenContract, nonce); err != nil {
			return errorsmod.Wrapf(err, "recovery: failed to cancel batch with nonce %d for contract %s",
				nonce, tokenContract.GetAddress().Hex())
		}
	}

	// Refund all unbatched pool entries for this contract.
	type txEntry struct {
		id     uint64
		sender sdk.AccAddress
	}
	var pending []txEntry
	k.IterateUnbatchedTransactionsByContract(ctx, tokenContract, func(_ []byte, tx *types.InternalOutgoingTransferTx) bool {
		pending = append(pending, txEntry{id: tx.Id, sender: tx.Sender})
		return false
	})
	for _, entry := range pending {
		if err := k.RemoveFromOutgoingPoolAndRefund(ctx, entry.id, entry.sender); err != nil {
			return errorsmod.Wrapf(err, "recovery: failed to refund tx %d for contract %s",
				entry.id, tokenContract.GetAddress().Hex())
		}
	}
	return nil
}

// assertNoPendingRemappedIbcAutoForwards asserts that there are no pending IBC Auto Forwards for any of the remapped ERC20s
func assertNoPendingRemappedIbcAutoForwards(ctx sdk.Context, k *gravitykeeper.Keeper) {
	forwards := k.PendingIbcAutoForwards(ctx, 0)
	for _, forward := range forwards {
		// Try gravity2 denom first - none of these should exist before the upgrade
		if tc, err := types.Gravity2DenomToERC20(forward.Token.Denom); err == nil {
			panic(fmt.Sprintf("Recovery upgrade: found pending IBC Auto Forward for remapped ERC20?: %s (%s)",
				tc.GetAddress().Hex(), forward.Token.Denom))
		}
		// Then gravity denom
		if tc, err := types.GravityDenomToERC20(forward.Token.Denom); err == nil {
			if k.IsRemappedERC20(ctx, *tc) {
				panic(fmt.Sprintf("Recovery upgrade: found pending IBC Auto Forward for remapped ERC20: %s (gravity2 denom: %s)",
					tc.GetAddress().Hex(), forward.Token.Denom))
			}
			continue
		}
		// Cosmos-originated or unknown - not a remapped ERC20
	}
	ctx.Logger().Info("Recovery upgrade: no pending IBC Auto Forwards found")
}
