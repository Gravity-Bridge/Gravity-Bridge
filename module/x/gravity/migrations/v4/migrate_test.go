package v4_test

import (
	"fmt"
	"testing"

	_ "github.com/Gravity-Bridge/Gravity-Bridge/module/config"
	v3 "github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/migrations/v3"
	v4 "github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/migrations/v4"
	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	ccodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	"github.com/cosmos/cosmos-sdk/std"
	"github.com/cosmos/cosmos-sdk/store"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/cosmos/cosmos-sdk/x/auth"
	"github.com/cosmos/cosmos-sdk/x/auth/vesting"
	"github.com/cosmos/cosmos-sdk/x/bank"
	"github.com/cosmos/cosmos-sdk/x/capability"
	"github.com/cosmos/cosmos-sdk/x/crisis"
	"github.com/cosmos/cosmos-sdk/x/distribution"
	distrclient "github.com/cosmos/cosmos-sdk/x/distribution/client"
	"github.com/cosmos/cosmos-sdk/x/evidence"
	"github.com/cosmos/cosmos-sdk/x/genutil"
	"github.com/cosmos/cosmos-sdk/x/gov"
	"github.com/cosmos/cosmos-sdk/x/mint"
	"github.com/cosmos/cosmos-sdk/x/params"
	paramsclient "github.com/cosmos/cosmos-sdk/x/params/client"
	paramskeeper "github.com/cosmos/cosmos-sdk/x/params/keeper"
	paramstypes "github.com/cosmos/cosmos-sdk/x/params/types"
	"github.com/cosmos/cosmos-sdk/x/slashing"
	"github.com/cosmos/cosmos-sdk/x/staking"
	"github.com/cosmos/cosmos-sdk/x/upgrade"
	upgradeclient "github.com/cosmos/cosmos-sdk/x/upgrade/client"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/libs/log"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	dbm "github.com/tendermint/tm-db"
)

func TestMigrateParams(t *testing.T) {
	params := v3.Params{

		SignedValsetsWindow:    0,
		SignedBatchesWindow:    0,
		SignedLogicCallsWindow: 0,
		TargetBatchTimeout:     60000,
		AverageBlockTime:       100,

		SlashFractionValset:          sdk.NewDecWithPrec(5, 2),
		SlashFractionBatch:           sdk.NewDecWithPrec(4, 2),
		SlashFractionLogicCall:       sdk.NewDecWithPrec(3, 2),
		UnbondSlashingValsetsWindow:  100,
		SlashFractionBadEthSignature: sdk.NewDecWithPrec(2, 2),
		ValsetReward: sdk.Coin{
			Denom:  types.GravityDenomPrefix,
			Amount: sdk.NewInt(1000),
		},
		GravityId:                "defaultid",
		ContractSourceHash:       "foobar",
		BridgeEthereumAddress:    "0x8858eeb3dfffa017d4bce9801d340d36cf895ccf",
		BridgeChainId:            1,
		AverageEthereumBlockTime: 5000,
		BridgeActive:             false,
		EthereumBlacklist:        []string{"0x8858eeb3dfffa017d4bce9801d340d36cf895ccf"},
	}

	gravityKey := sdk.NewKVStoreKey("gravity")
	keyParams := sdk.NewKVStoreKey(paramstypes.StoreKey)
	tkeyParams := sdk.NewTransientStoreKey(paramstypes.TStoreKey)

	db := dbm.NewMemDB()
	ms := store.NewCommitMultiStore(db)
	ms.MountStoreWithDB(gravityKey, sdk.StoreTypeIAVL, db)
	ms.MountStoreWithDB(keyParams, sdk.StoreTypeIAVL, db)
	ms.MountStoreWithDB(tkeyParams, sdk.StoreTypeTransient, db)

	err := ms.LoadLatestVersion()
	if err != nil {
		panic(err)
	}

	ctx := sdk.NewContext(ms, tmproto.Header{}, false, log.NewNopLogger())
	marshaler := MakeTestMarshaler()
	cdc := MakeTestCodec()

	paramsKeeper := paramskeeper.NewKeeper(marshaler, cdc, keyParams, tkeyParams)
	paramsKeeper.Subspace(types.DefaultParamspace).WithKeyTable(v3.ParamKeyTable())
	subspace, exists := paramsKeeper.GetSubspace(types.DefaultParamspace)
	require.Equal(t, true, exists)

	// set v3 params then migrate
	subspace.SetParamSet(ctx, &params)

	// // init param keeper v4
	paramsKeeper = paramskeeper.NewKeeper(marshaler, cdc, keyParams, tkeyParams)
	paramsKeeper.Subspace(types.DefaultParamspace).WithKeyTable(types.ParamKeyTable())
	v4Subspace, exists := paramsKeeper.GetSubspace(types.DefaultParamspace)
	require.Equal(t, true, exists)

	v4.MigrateParams(ctx, v4Subspace, subspace)
	v4Params := types.Params{}
	v4Subspace.GetParamSet(ctx, &v4Params)
	fmt.Printf("v4 params in testing: %v\n", v4Params)

	evmChainParam := v4Params.EvmChainParams[0]
	require.Equal(t, types.GravityDenomPrefix, evmChainParam.EvmChainPrefix)
	require.Equal(t, params.AverageEthereumBlockTime, evmChainParam.AverageEthereumBlockTime)
	require.Equal(t, params.GravityId, evmChainParam.GravityId)
	require.Equal(t, params.ContractSourceHash, evmChainParam.ContractSourceHash)
	require.Equal(t, params.BridgeEthereumAddress, evmChainParam.BridgeEthereumAddress)
	require.Equal(t, params.BridgeChainId, evmChainParam.BridgeChainId)
	require.Equal(t, params.BridgeActive, evmChainParam.BridgeActive)
	require.Equal(t, params.EthereumBlacklist, evmChainParam.EthereumBlacklist)
}

// MakeTestCodec creates a legacy amino codec for testing
func MakeTestCodec() *codec.LegacyAmino {
	var cdc = codec.NewLegacyAmino()
	auth.AppModuleBasic{}.RegisterLegacyAminoCodec(cdc)
	bank.AppModuleBasic{}.RegisterLegacyAminoCodec(cdc)
	staking.AppModuleBasic{}.RegisterLegacyAminoCodec(cdc)
	distribution.AppModuleBasic{}.RegisterLegacyAminoCodec(cdc)
	sdk.RegisterLegacyAminoCodec(cdc)
	ccodec.RegisterCrypto(cdc)
	params.AppModuleBasic{}.RegisterLegacyAminoCodec(cdc)
	types.RegisterCodec(cdc)
	return cdc
}

// MakeTestMarshaler creates a proto codec for use in testing
func MakeTestMarshaler() codec.Codec {
	interfaceRegistry := codectypes.NewInterfaceRegistry()
	std.RegisterInterfaces(interfaceRegistry)
	ModuleBasics.RegisterInterfaces(interfaceRegistry)
	types.RegisterInterfaces(interfaceRegistry)
	return codec.NewProtoCodec(interfaceRegistry)
}

// Need to duplicate these because of cyclical imports
// ModuleBasics is a mock module basic manager for testing
var (
	ModuleBasics = module.NewBasicManager(
		auth.AppModuleBasic{},
		genutil.AppModuleBasic{},
		bank.AppModuleBasic{},
		capability.AppModuleBasic{},
		staking.AppModuleBasic{},
		mint.AppModuleBasic{},
		distribution.AppModuleBasic{},
		gov.NewAppModuleBasic(
			paramsclient.ProposalHandler, distrclient.ProposalHandler, upgradeclient.ProposalHandler, upgradeclient.CancelProposalHandler,
		),
		params.AppModuleBasic{},
		crisis.AppModuleBasic{},
		slashing.AppModuleBasic{},
		upgrade.AppModuleBasic{},
		evidence.AppModuleBasic{},
		vesting.AppModuleBasic{},
	)
)
