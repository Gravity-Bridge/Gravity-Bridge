package keeper

import (
	v1 "github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/migrations/v1"
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

// Migrate1to2 migrates from consensus version 1 to 2.
func (m Migrator) Migrate1to2(ctx sdk.Context) error {
	ctx.Logger().Info("Mercury Upgrade: Enter Migrate1to2()")
	return v1.MigrateStore(ctx, m.keeper.storeKey, m.keeper.cdc)
}
