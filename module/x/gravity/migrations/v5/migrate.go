package v5

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	paramstypes "github.com/cosmos/cosmos-sdk/x/params/types"

	v4 "github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/migrations/v4"
	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
)

// MigrateParams performs in-place migrations from v4 to v5. The migration includes:
//
// - Set the ChainFeeCommunityPoolFraction to the default value
func MigrateParams(ctx sdk.Context, paramSpace paramstypes.Subspace) types.Params {
	ctx.Logger().Info("Gravity v5 Migration: Migrating to new Params")
	v4Params := GetParams(ctx, paramSpace)
	v5Params := V4ToV5Params(v4Params)

	ctx.Logger().Info("Gravity v5 Migration: Params migration finished")
	return v5Params
}

// GetParams returns the parameters from the store
func GetParams(ctx sdk.Context, paramSpace paramstypes.Subspace) (params v4.Params) {
	paramSpace.GetParamSet(ctx, &params)
	return
}

// V4ToV5Params Adds any new params to the given v4Params, using the new default values
func V4ToV5Params(v4Params v4.Params) types.Params {
	v5DefaultParams := types.DefaultParams()
	// NEW PARAMS: ChainFeeAuctionPoolFraction
	chainFeeAuctionPoolFraction := v5DefaultParams.ChainFeeAuctionPoolFraction

	// nolint: exhaustruct
	v5Params := types.Params{
		GravityId:                    v4Params.GravityId,
		ContractSourceHash:           v4Params.ContractSourceHash,
		BridgeEthereumAddress:        v4Params.BridgeEthereumAddress,
		BridgeChainId:                v4Params.BridgeChainId,
		SignedValsetsWindow:          v4Params.SignedValsetsWindow,
		SignedBatchesWindow:          v4Params.SignedBatchesWindow,
		SignedLogicCallsWindow:       v4Params.SignedLogicCallsWindow,
		TargetBatchTimeout:           v4Params.TargetBatchTimeout,
		AverageBlockTime:             v4Params.AverageBlockTime,
		AverageEthereumBlockTime:     v4Params.AverageEthereumBlockTime,
		SlashFractionValset:          v4Params.SlashFractionValset,
		SlashFractionBatch:           v4Params.SlashFractionBatch,
		SlashFractionLogicCall:       v4Params.SlashFractionLogicCall,
		UnbondSlashingValsetsWindow:  v4Params.UnbondSlashingValsetsWindow,
		SlashFractionBadEthSignature: v4Params.SlashFractionBadEthSignature,
		ValsetReward:                 v4Params.ValsetReward,
		BridgeActive:                 v4Params.BridgeActive,
		EthereumBlacklist:            v4Params.EthereumBlacklist,
		MinChainFeeBasisPoints:       v4Params.MinChainFeeBasisPoints,
		ChainFeeAuctionPoolFraction:  chainFeeAuctionPoolFraction,
	}
	return v5Params
}
