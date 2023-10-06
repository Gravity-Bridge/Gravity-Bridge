package keeper

import (
	v4 "github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/migrations/v4"
	v5 "github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/migrations/v5"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// Migrator is a struct for handling in-place store migrations.
type Migrator struct {
	keeper Keeper
}

// NewMigrator returns a new Migrator.
func NewMigrator(keeper Keeper) Migrator {
	return Migrator{keeper: keeper}
}

// Migrate3to4 migrates from consensus version 3 to 4.
func (m Migrator) Migrate3to4(ctx sdk.Context) error {
	ctx.Logger().Info("Pleiades Upgrade part 2: Enter Migrate3to4()")
	newParams := v4.MigrateParams(ctx, m.keeper.paramSpace)
	m.keeper.SetParams(ctx, newParams)
	ctx.Logger().Info("Pleiades Upgrade part 2: Gravity module migration is complete!")
	return nil
}

// Migrate4to5 migrates from consensus version 4 to 5.
func (m Migrator) Migrate4to5(ctx sdk.Context) error {
	ctx.Logger().Info("Begin Gravity v4 -> v5 migration")
	newParams := v5.MigrateParams(ctx, m.keeper.paramSpace)
	m.keeper.SetParams(ctx, newParams)
	ctx.Logger().Info("Gravity migration finished!")
	return nil
}
