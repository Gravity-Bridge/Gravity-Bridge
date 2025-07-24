package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	paramstypes "github.com/cosmos/cosmos-sdk/x/params/types"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/auction/types"
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

func (m Migrator) Migrate1to2(ctx sdk.Context) error {
	ctx.Logger().Info("Begin Auction v1 -> v2 migration")

	// nolint: exhaustruct
	params := types.Params{}

	m.legacyParamSpace.GetParamSet(ctx, &params)

	err := m.keeper.SetParams(ctx, params)
	if err != nil {
		return err
	}

	ctx.Logger().Info("Auction v2 migration finished!")
	return nil
}
