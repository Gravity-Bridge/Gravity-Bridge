package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	paramstypes "github.com/cosmos/cosmos-sdk/x/params/types"

	v7 "github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/migrations/v7"
	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
)

// Migrator is a struct for handling in-place store migrations.
type Migrator struct {
	keeper           Keeper
	legacyParamSpace paramstypes.Subspace
}

// NewMigrator returns a new Migrator.
func NewMigrator(keeper Keeper, legacyParamSpace paramstypes.Subspace) Migrator {
	return Migrator{keeper: keeper, legacyParamSpace: legacyParamSpace}
}

// The following migrations are commented out because they rely on an old paramSpace field
// which no longer exists and will cause trouble to keep trying to use.
// They are kept here for reference for future migrations.

// // Migrate3to4 migrates from consensus version 3 to 4.
// func (m Migrator) Migrate3to4(ctx sdk.Context) error {
// 	ctx.Logger().Info("Pleiades Upgrade part 2: Enter Migrate3to4()")
// 	newParams := v4.MigrateParams(ctx, m.keeper.paramSpace)
// 	m.keeper.SetParams(ctx, newParams)
// 	ctx.Logger().Info("Pleiades Upgrade part 2: Gravity module migration is complete!")
// 	return nil
// }

// // Migrate4to5 migrates from consensus version 4 to 5.
// func (m Migrator) Migrate4to5(ctx sdk.Context) error {
// 	ctx.Logger().Info("Begin Gravity v4 -> v5 migration")
// 	newParams := v5.MigrateParams(ctx, m.keeper.paramSpace)
// 	m.keeper.SetParams(ctx, newParams)
// 	ctx.Logger().Info("Gravity migration finished!")
// 	return nil
// }

func (m Migrator) Migrate5to6(ctx sdk.Context) error {
	ctx.Logger().Info("Begin Gravity v5 -> v6 migration")

	// nolint: exhaustruct
	params := types.Params{}

	m.legacyParamSpace.GetParamSet(ctx, &params)

	err := m.keeper.SetParams(ctx, params)
	if err != nil {
		return err
	}

	ctx.Logger().Info("Gravity v6 migration finished!")
	return nil
}

func (m Migrator) Migrate6to7(ctx sdk.Context) error {
	ctx.Logger().Info("Begin Gravity v6 -> v7 migration")

	// Migrate all attestations to use the new attestation separator in their claim hashes
	err := v7.MigrateAttestationsToAttestationSeparator(ctx, m.keeper)
	if err != nil {
		return err
	}
	err = v7.MigrateAttestationsToClaimHashComponents(ctx, m.keeper)
	if err != nil {
		return err
	}

	ctx.Logger().Info("Gravity v7 migration finished!")
	return nil
}

// WARNING DO NOT USE THIS FUNCTION OUTSIDE OF THE v7 MIGRATION!
//
// DeleteRawAttestationKey deletes whatever is stored under the exact given raw attestation store
// key. Unlike Keeper.DeleteAttestation, this does not recompute the key from the attestation's
// claim, which makes it useful for migrations (e.g. the v7 claim hash separator migration) where
// the key was computed using since-changed hashing logic and can no longer be recomputed.
//
// WARNING DO NOT USE THIS FUNCTION OUTSIDE OF THE v7 MIGRATION!
func (k Keeper) DeleteRawAttestationKey(ctx sdk.Context, key []byte) {
	store := ctx.KVStore(k.storeKey)
	store.Delete(key)
}
