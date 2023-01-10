package v4

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	paramstypes "github.com/cosmos/cosmos-sdk/x/params/types"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/exported"
	v3 "github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/migrations/v3"
	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"

	_ "github.com/Gravity-Bridge/Gravity-Bridge/module/doc/statik"
)

// MigrateParams performs in-place migrations from v3 to v4. The migration includes:
//
// - Set the MonitoredTokenAddresses param to an empty slice
func MigrateParams(ctx sdk.Context, paramSpace paramstypes.Subspace, legacySubspace exported.Subspace) types.Params {
	log := ctx.Logger()
	log.Info("Pleiades Upgrade part 2: Beginning the migrations for the gravity module")
	v3Params := GetParams(ctx, legacySubspace)
	log.Info("v3 params:", v3Params)
	v4Params := V3ToV4Params(v3Params)
	log.Info("v4 params:", v4Params)
	paramSpace.SetParamSet(ctx, &v4Params)

	log.Info("Pleiades Upgrade part 2: Finished the migrations for the gravity module successfully!")
	return v4Params
}

// GetParams returns the parameters from the store
func GetParams(ctx sdk.Context, paramSpace exported.Subspace) (params v3.Params) {
	paramSpace.GetParamSet(ctx, &params)
	return
}

// V3ToV4Params Adds any new params to the given v3Params, using the new default values
func V3ToV4Params(v3Params v3.Params) types.Params {
	v4DefaultParams := types.DefaultParams()
	// NEW PARAMS: MonitoredTokenAddresses
	minChainFeeBasisPoints := v4DefaultParams.MinChainFeeBasisPoints

	v4Params := types.Params{

		SignedValsetsWindow:    v3Params.SignedValsetsWindow,
		SignedBatchesWindow:    v3Params.SignedBatchesWindow,
		SignedLogicCallsWindow: v3Params.SignedLogicCallsWindow,
		TargetBatchTimeout:     v3Params.TargetBatchTimeout,
		AverageBlockTime:       v3Params.AverageBlockTime,

		SlashFractionValset:          v3Params.SlashFractionValset,
		SlashFractionBatch:           v3Params.SlashFractionBatch,
		SlashFractionLogicCall:       v3Params.SlashFractionLogicCall,
		UnbondSlashingValsetsWindow:  v3Params.UnbondSlashingValsetsWindow,
		SlashFractionBadEthSignature: v3Params.SlashFractionBadEthSignature,
		ValsetReward:                 v3Params.ValsetReward,

		MinChainFeeBasisPoints: minChainFeeBasisPoints,
		EvmChainParams: []*types.EvmChainParam{
			{
				EvmChainPrefix:           v3.EthereumChainPrefix,
				GravityId:                v3Params.GravityId,
				ContractSourceHash:       v3Params.ContractSourceHash,
				BridgeEthereumAddress:    "0xb40C364e70bbD98E8aaab707A41a52A2eAF5733f",
				BridgeChainId:            v3Params.BridgeChainId,
				AverageEthereumBlockTime: v3Params.AverageEthereumBlockTime,
				BridgeActive:             v3Params.BridgeActive,
				EthereumBlacklist:        v3Params.EthereumBlacklist,
			},
		},
	}
	return v4Params
}
