package gravity

import (
	"encoding/json"
	"math/rand"

	"github.com/gorilla/mux"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/spf13/cobra"
	abci "github.com/tendermint/tendermint/abci/types"

	// Cosmos SDK

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/cosmos/cosmos-sdk/types/module"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	authzkeeper "github.com/cosmos/cosmos-sdk/x/authz/keeper"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	capabilitykeeper "github.com/cosmos/cosmos-sdk/x/capability/keeper"
	distrkeeper "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
	evidencekeeper "github.com/cosmos/cosmos-sdk/x/evidence/keeper"
	govkeeper "github.com/cosmos/cosmos-sdk/x/gov/keeper"
	mintkeeper "github.com/cosmos/cosmos-sdk/x/mint/keeper"
	paramskeeper "github.com/cosmos/cosmos-sdk/x/params/keeper"
	slashingkeeper "github.com/cosmos/cosmos-sdk/x/slashing/keeper"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	upgradekeeper "github.com/cosmos/cosmos-sdk/x/upgrade/keeper"

	// Cosmos IBC-Go
	ibctransferkeeper "github.com/cosmos/ibc-go/v3/modules/apps/transfer/keeper"
	ibckeeper "github.com/cosmos/ibc-go/v3/modules/core/keeper"

	// Osmosis-Labs Bech32-IBC
	bech32ibckeeper "github.com/osmosis-labs/bech32-ibc/x/bech32ibc/keeper"

	gravitykeeper "github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/keeper"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/orbit/keeper"
	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/orbit/types"
)

// type check to ensure the interface is properly implemented
var (
	_ module.AppModule      = AppModule{}
	_ module.AppModuleBasic = AppModuleBasic{}
)

// AppModuleBasic object for module implementation
type AppModuleBasic struct{}

// Name implements app module basic
func (AppModuleBasic) Name() string {
	return types.ModuleName
}

// RegisterLegacyAminoCodec implements app module basic
func (AppModuleBasic) RegisterLegacyAminoCodec(cdc *codec.LegacyAmino) {
	types.RegisterCodec(cdc)
}

// DefaultGenesis implements app module basic
func (AppModuleBasic) DefaultGenesis(cdc codec.JSONCodec) json.RawMessage {
	return cdc.MustMarshalJSON(types.DefaultGenesisState())
}

// ValidateGenesis implements app module basic
func (AppModuleBasic) ValidateGenesis(cdc codec.JSONCodec, _ client.TxEncodingConfig, bz json.RawMessage) error {
	return nil
}

// RegisterRESTRoutes implements app module basic
func (AppModuleBasic) RegisterRESTRoutes(ctx client.Context, rtr *mux.Router) {
}

// GetQueryCmd implements app module basic
func (AppModuleBasic) GetQueryCmd() *cobra.Command {
	return nil
}

// GetTxCmd implements app module basic
func (AppModuleBasic) GetTxCmd() *cobra.Command {
	return nil
}

// RegisterGRPCGatewayRoutes registers the gRPC Gateway routes for the distribution module.
// also implements app module basic
func (AppModuleBasic) RegisterGRPCGatewayRoutes(clientCtx client.Context, mux *runtime.ServeMux) {}

// RegisterInterfaces implements app module basic
func (b AppModuleBasic) RegisterInterfaces(registry codectypes.InterfaceRegistry) {
	types.RegisterInterfaces(registry)
}

// ___________________________________________________________________________

// AppModule object for module implementation
type AppModule struct {
	AppModuleBasic
	OrbitKeeper       *keeper.Keeper
	BankKeeper        *bankkeeper.BaseKeeper
	GravityKeeper     *gravitykeeper.Keeper
	IbcTransferKeeper *ibctransferkeeper.Keeper
	AccountKeeper     *authkeeper.AccountKeeper
	AuthzKeeper       *authzkeeper.Keeper
	CapabilityKeeper  *capabilitykeeper.Keeper
	GovKeeper         *govkeeper.Keeper
	MintKeeper        *mintkeeper.Keeper
	SlashingKeeper    *slashingkeeper.Keeper
	DistrKeeper       *distrkeeper.Keeper
	StakingKeeper     *stakingkeeper.Keeper
	UpgradeKeeper     *upgradekeeper.Keeper
	EvidenceKeeper    *evidencekeeper.Keeper
	ParamsKeeper      *paramskeeper.Keeper
	Bech32IbcKeeper   *bech32ibckeeper.Keeper
}

func (am AppModule) ConsensusVersion() uint64 {
	return 1
}

// NewAppModule creates a new AppModule Object
func NewAppModule(
	k *keeper.Keeper, bankKeeper *bankkeeper.BaseKeeper, gravityKeeper *gravitykeeper.Keeper,
	ibcTransferKeeper *ibctransferkeeper.Keeper, ibcKeeper *ibckeeper.Keeper, accountKeeper *authkeeper.AccountKeeper,
	authzKeeper *authzkeeper.Keeper, capabilityKeeper *capabilitykeeper.Keeper, govKeeper *govkeeper.Keeper,
	mintKeeper *mintkeeper.Keeper, slashingKeeper *slashingkeeper.Keeper, distrKeeper *distrkeeper.Keeper,
	stakingKeeper *stakingkeeper.Keeper, upgradeKeeper *upgradekeeper.Keeper, evidenceKeeper *evidencekeeper.Keeper,
	paramsKeeper *paramskeeper.Keeper, bech32IbcKeeper *bech32ibckeeper.Keeper,
) AppModule {
	return AppModule{
		AppModuleBasic:    AppModuleBasic{},
		OrbitKeeper:       k,
		BankKeeper:        bankKeeper,
		GravityKeeper:     gravityKeeper,
		IbcTransferKeeper: ibcTransferKeeper,
		AccountKeeper:     accountKeeper,
		AuthzKeeper:       authzKeeper,
		CapabilityKeeper:  capabilityKeeper,
		GovKeeper:         govKeeper,
		MintKeeper:        mintKeeper,
		SlashingKeeper:    slashingKeeper,
		DistrKeeper:       distrKeeper,
		StakingKeeper:     stakingKeeper,
		UpgradeKeeper:     upgradeKeeper,
		EvidenceKeeper:    evidenceKeeper,
		ParamsKeeper:      paramsKeeper,
		Bech32IbcKeeper:   bech32IbcKeeper,
	}
}

// Name implements app module
func (AppModule) Name() string {
	return types.ModuleName
}

// RegisterInvariants implements app module
func (am AppModule) RegisterInvariants(ir sdk.InvariantRegistry) {
	ir.RegisterRoute(types.ModuleName, "failing-invariant", keeper.FailingInvariant(*am.OrbitKeeper))
}

// Route implements app module
func (am AppModule) Route() sdk.Route {
	return sdk.NewRoute(types.RouterKey, NewHandler(*am.OrbitKeeper))
}

// QuerierRoute implements app module
func (am AppModule) QuerierRoute() string {
	return types.QuerierRoute
}

// LegacyQuerierHandler returns the distribution module sdk.Querier.
func (am AppModule) LegacyQuerierHandler(legacyQuerierCdc *codec.LegacyAmino) sdk.Querier {

	return func(ctx sdk.Context, path []string, req abci.RequestQuery) (res []byte, err error) {

		return nil, sdkerrors.Wrapf(sdkerrors.ErrUnknownRequest, "unknown %s query endpoint", types.ModuleName)
	}
}

// RegisterServices registers module services.
func (am AppModule) RegisterServices(cfg module.Configurator) {
	types.RegisterMsgServer(cfg.MsgServer(), keeper.NewMsgServerImpl(*am.OrbitKeeper))
	types.RegisterQueryServer(cfg.QueryServer(), am.OrbitKeeper)
}

// InitGenesis initializes the genesis state for this module and implements app module.
func (am AppModule) InitGenesis(ctx sdk.Context, cdc codec.JSONCodec, data json.RawMessage) []abci.ValidatorUpdate {
	var genesisState types.GenesisState
	cdc.MustUnmarshalJSON(data, &genesisState)
	keeper.InitGenesis(ctx, *am.OrbitKeeper, genesisState)
	return []abci.ValidatorUpdate{}
}

// ExportGenesis exports the current genesis state to a json.RawMessage
func (am AppModule) ExportGenesis(ctx sdk.Context, cdc codec.JSONCodec) json.RawMessage {
	gs := types.GenesisState{}
	return cdc.MustMarshalJSON(&gs)
}

// BeginBlock implements app module
func (am AppModule) BeginBlock(ctx sdk.Context, _ abci.RequestBeginBlock) {}

// EndBlock implements app module
func (am AppModule) EndBlock(ctx sdk.Context, _ abci.RequestEndBlock) []abci.ValidatorUpdate {
	EndBlocker(ctx, *am.OrbitKeeper)
	return []abci.ValidatorUpdate{}
}

// ___________________________________________________________________________

// AppModuleSimulation functions

// GenerateGenesisState creates a randomized GenState of the distribution module.
func (AppModule) GenerateGenesisState(simState *module.SimulationState) {
	// TODO: implement gravity simulation stuffs
	// simulation.RandomizedGenState(simState)
}

// ProposalContents returns all the distribution content functions used to
// simulate governance proposals.
func (am AppModule) ProposalContents(simState module.SimulationState) []simtypes.WeightedProposalContent {
	// TODO: implement gravity simulation stuffs
	return nil
}

// RandomizedParams creates randomized distribution param changes for the simulator.
func (AppModule) RandomizedParams(r *rand.Rand) []simtypes.ParamChange {
	// TODO: implement gravity simulation stuffs
	return nil
}

// RegisterStoreDecoder registers a decoder for distribution module's types
func (am AppModule) RegisterStoreDecoder(sdr sdk.StoreDecoderRegistry) {
	// TODO: implement gravity simulation stuffs
	// sdr[types.StoreKey] = simulation.NewDecodeStore(am.cdc)
}

// WeightedOperations returns the all the gov module operations with their respective weights.
func (am AppModule) WeightedOperations(simState module.SimulationState) []simtypes.WeightedOperation {
	// TODO: implement gravity simulation stuffs
	// return simulation.WeightedOperations(
	// simState.AppParams, simState.Cdc, am.accountKeeper, am.bankKeeper, am.keeper, am.stakingKeeper,
	// )
	return nil
}
