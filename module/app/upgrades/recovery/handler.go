package recovery

import (
	"context"
	"fmt"
	"regexp"

	upgradetypes "cosmossdk.io/x/upgrade/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	crisiskeeper "github.com/cosmos/cosmos-sdk/x/crisis/keeper"

	gravitykeeper "github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/keeper"
	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
)

// embeddedEthAddrRegex matches any eip-55 eth address embedded within an IBC denom, which is not allowed
// to be sent across the bridge
var embeddedEthAddrRegex = regexp.MustCompile(`0x[0-9a-fA-F]{40}`)

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

		ctx.Logger().Info("Recovery upgrade: scanning for ERC20 tokens to remap")
		if err := migrateRemappedERC20s(ctx, gravityKeeper, bankKeeper); err != nil {
			return out, err
		}

		ctx.Logger().Info("Recovery upgrade: registering cosmos-originated tokens as CosmosBridgeableTokens")
		if err := registerCosmosBridgeableTokens(ctx, gravityKeeper); err != nil {
			return out, err
		}

		ctx.Logger().Info("Recovery upgrade: asserting invariants")
		crisisKeeper.AssertInvariants(ctx)

		ctx.Logger().Info("Recovery upgrade: complete")
		return out, nil
	}
}

// remapEntry holds the details of a single remapped ERC20
type remapEntry struct {
	erc20           types.EthAddress
	problemDenom    string // a CosmosOriginated ERC20 denom
	frozenVoucher   string // gravity0x... - before the remap, not allowed to be bridged out
	recoveryVoucher string // gravity2...  - after the remap, given on deposits and allowed to be withdrawn
}

// migrateRemappedERC20s detects every DenomToERC20 entry whose denom contains an embedded
// Ethereum address which is on the ERC20ToDenom mapping, and will:
//   - cancel and refund all in-flight outgoing transactions,
//   - deletes the cosmos-originated mapping,
//   - sets the RemappedERC20 entry so new deposits use gravity2 and bridge-out of old vouchers is blocked,
//   - registers bank metadata for the new gravity2 denom.
func migrateRemappedERC20s(ctx sdk.Context, k *gravitykeeper.Keeper, bk *bankkeeper.BaseKeeper) error {
	var remapped []remapEntry

	k.IterateCosmosOriginatedERC20s(ctx, func(key []byte, erc20 *types.EthAddress) (stop bool) {
		denom := string(key) // prefix store strips DenomToERC20Key

		// denom must embed an eip-55 address
		if !embeddedEthAddrRegex.MatchString(denom) {
			return false
		}

		// extract the targeted ERC20 address from the denom
		embeddedAddrStr := embeddedEthAddrRegex.FindString(denom)
		if embeddedAddrStr == "" {
			return false
		}
		embeddedAddr, err := types.NewEthAddress(embeddedAddrStr)
		if err != nil {
			panic("invalid erc20 address embedded in cosmos-originated denom: " + embeddedAddrStr)
		}
		remapped = append(remapped, remapEntry{
			erc20:           *embeddedAddr,
			problemDenom:    denom,
			frozenVoucher:   types.GravityDenom(*embeddedAddr),
			recoveryVoucher: types.Gravity2Denom(*embeddedAddr),
		})
		return false
	})

	if len(remapped) == 0 {
		panic("Recovery upgrade: no ERC20 tokens to remap were discovered")
	}

	ctx.Logger().Info(fmt.Sprintf("Recovery upgrade: found %d ERC20 token(s) to remap", len(remapped)))

	for _, e := range remapped {
		ctx.Logger().Info("Recovery upgrade: remapping ERC20",
			"erc20", e.erc20.GetAddress().Hex(),
			"problemDenom", e.problemDenom,
			"frozen_voucher", e.frozenVoucher,
			"recovery_voucher", e.recoveryVoucher,
		)

		// cancel and refund all in-flight outgoing transactions
		// Must happen BEFORE deleting the cosmos-originated mapping and setting the
		// remapped flag
		if err := k.CancelAllOutgoingTxsForContract(ctx, e.erc20); err != nil {
			return fmt.Errorf("recovery: failed to cancel outgoing txs for %s: %w",
				e.erc20.GetAddress().Hex(), err)
		}

		// delete the problem cosmos-originated denom ERC20 mapping
		k.DeleteCosmosOriginatedDenomToERC20(ctx, e.erc20, e.problemDenom)

		// set the remapped flag
		k.SetRemappedERC20(ctx, e.erc20)

		// register bank metadata for the new gravity2 denom
		if existing, ok := bk.GetDenomMetaData(ctx, e.recoveryVoucher); !ok || existing.Base == "" {
			meta := buildGravity2Metadata(ctx, bk, e.erc20, e.recoveryVoucher, e.frozenVoucher)
			bk.SetDenomMetaData(ctx, meta)
		} else {
			ctx.Logger().Info("Recovery upgrade: gravity2 denom metadata already set, skipping",
				"gravity2_denom", e.recoveryVoucher,
			)
		}

		ctx.Logger().Info("Recovery upgrade: ERC20 remapped successfully",
			"erc20", e.erc20.GetAddress().Hex(),
			"gravity2_denom", e.recoveryVoucher,
		)
	}

	ctx.Logger().Info(fmt.Sprintf(
		"Recovery upgrade: remapped %d ERC20 token(s). ",
		len(remapped),
	))
	return nil
}

// buildGravity2Metadata constructs BankDenomMetadata for a new gravity2 denom.
// It copies existing metadata from the old gravity denom when available, updating
// Base, Display, any matching DenomUnit, and the Description.
// If no prior metadata exists, a minimal fallback entry is produced.
func buildGravity2Metadata(
	ctx sdk.Context,
	bk *bankkeeper.BaseKeeper,
	addr types.EthAddress,
	gravity2Denom string,
	oldGravityDenom string,
) banktypes.Metadata {
	if existing, ok := bk.GetDenomMetaData(ctx, oldGravityDenom); ok && existing.Base != "" {
		meta := existing
		meta.Base = gravity2Denom
		meta.Display = gravity2Denom
		for i := range meta.DenomUnits {
			if meta.DenomUnits[i].Denom == oldGravityDenom {
				meta.DenomUnits[i].Denom = gravity2Denom
			}
			// Also update any aliases that referenced the old denom (e.g. set by governance
			// when the token was originally registered).
			for j, alias := range meta.DenomUnits[i].Aliases {
				if alias == oldGravityDenom {
					meta.DenomUnits[i].Aliases[j] = gravity2Denom
				}
			}
		}
		return meta
	}

	panic("Recovery upgrade: no existing metadata found for old gravity denom " + oldGravityDenom)
}

// registerCosmosBridgeableTokens adds all cosmos-originated ERC20 tokens that remain after
// remapping (i.e. the non-problem tokens) to the CosmosBridgeableTokens allowlist in params.
// This must be called after migrateRemappedERC20s, which deletes remapped entries from the store,
// so only legitimate tokens are seen here.
func registerCosmosBridgeableTokens(ctx sdk.Context, k *gravitykeeper.Keeper) error {
	params, err := k.GetParams(ctx)
	if err != nil {
		return fmt.Errorf("recovery: failed to get params for CosmosBridgeableTokens update: %w", err)
	}

	// Build a set of already-registered denoms to avoid duplicates
	existing := make(map[string]struct{}, len(params.CosmosBridgeableTokens))
	for _, d := range params.CosmosBridgeableTokens {
		existing[d] = struct{}{}
	}

	var newDenoms []string
	k.IterateCosmosOriginatedERC20s(ctx, func(key []byte, _ *types.EthAddress) (stop bool) {
		denom := string(key)
		if _, alreadyPresent := existing[denom]; !alreadyPresent {
			newDenoms = append(newDenoms, denom)
		}
		return false
	})

	if len(newDenoms) == 0 {
		ctx.Logger().Info("Recovery upgrade: no new cosmos-originated tokens to add to CosmosBridgeableTokens")
		return nil
	}

	for _, denom := range newDenoms {
		ctx.Logger().Info("Recovery upgrade: registering cosmos-originated token as CosmosBridgeableToken",
			"denom", denom,
		)
	}

	params.CosmosBridgeableTokens = append(params.CosmosBridgeableTokens, newDenoms...)
	if err := k.SetParams(ctx, params); err != nil {
		return fmt.Errorf("recovery: failed to set params after CosmosBridgeableTokens update: %w", err)
	}

	ctx.Logger().Info(fmt.Sprintf(
		"Recovery upgrade: registered %d cosmos-originated token(s) as CosmosBridgeableTokens",
		len(newDenoms),
	))
	return nil
}
