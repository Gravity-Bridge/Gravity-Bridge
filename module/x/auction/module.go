package auction

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/gorilla/mux"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	authKeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	abci "github.com/tendermint/tendermint/abci/types"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/auction/client/cli"
	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/auction/keeper"
	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/auction/types"
)

// type check to ensure the interface is properly implemented
// nolint: exhaustruct
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
	return cdc.MustMarshalJSON(types.DefaultGenesis())
}

// ValidateGenesis implements app module basic
func (AppModuleBasic) ValidateGenesis(cdc codec.JSONCodec, _ client.TxEncodingConfig, bz json.RawMessage) error {
	var data types.GenesisState
	if err := cdc.UnmarshalJSON(bz, &data); err != nil {
		return fmt.Errorf("failed to unmarshal %s genesis state: %w", types.ModuleName, err)
	}

	return data.ValidateBasic()
}

// RegisterRESTRoutes implements app module basic
func (AppModuleBasic) RegisterRESTRoutes(ctx client.Context, rtr *mux.Router) {
	// Do nothing, legacy REST routes are not supported by x/auction
	// REST queries should be provided by the proto definitions with gRPC on (default) port 1317
}

// GetQueryCmd implements app module basic
func (AppModuleBasic) GetQueryCmd() *cobra.Command {
	return cli.GetQueryCmd()
}

// GetTxCmd implements app module basic
func (AppModuleBasic) GetTxCmd() *cobra.Command {
	return cli.GetTxCmd()
}

// RegisterGRPCGatewayRoutes registers the gRPC Gateway routes for the distribution module.
// also implements app module basic
func (AppModuleBasic) RegisterGRPCGatewayRoutes(clientCtx client.Context, mux *runtime.ServeMux) {
	err := types.RegisterQueryHandlerClient(context.Background(), mux, types.NewQueryClient(clientCtx))

	if err != nil {
		panic("Failed to register query handler")
	}
}

// RegisterInterfaces implements app module basic
func (b AppModuleBasic) RegisterInterfaces(registry codectypes.InterfaceRegistry) {
	types.RegisterInterfaces(registry)
}

// ___________________________________________________________________________

// AppModule object for module implementation
type AppModule struct {
	AppModuleBasic
	keeper keeper.Keeper
}

func (am AppModule) ConsensusVersion() uint64 {
	return 1
}

// NewAppModule creates a new AppModule Object
func NewAppModule(k keeper.Keeper, bankKeeper bankkeeper.Keeper, accountkeeper authKeeper.AccountKeeper) AppModule {
	return AppModule{
		AppModuleBasic: AppModuleBasic{},
		keeper:         k,
	}
}

// Name implements app module
func (AppModule) Name() string {
	return types.ModuleName
}

// RegisterInvariants implements app module
func (am AppModule) RegisterInvariants(ir sdk.InvariantRegistry) {
	ir.RegisterRoute(types.ModuleName, "module-balance", keeper.ModuleBalanceInvariant(am.keeper))
	ir.RegisterRoute(types.ModuleName, "valid-auctions", keeper.ValidAuctionsInvariant(am.keeper))
}

// Route implements app module
func (am AppModule) Route() sdk.Route {
	return sdk.NewRoute(types.RouterKey, NewHandler(am.keeper))
}

// QuerierRoute implements app module
func (am AppModule) QuerierRoute() string {
	return types.QuerierRoute
}

// LegacyQuerierHandler returns the module sdk.Querier.
func (am AppModule) LegacyQuerierHandler(legacyQuerierCdc *codec.LegacyAmino) sdk.Querier {
	return func(_ sdk.Context, path []string, req abci.RequestQuery) ([]byte, error) {
		return nil, fmt.Errorf("legacy querier not implemented, use gRPC queries instead")
	}
}

// RegisterServices registers module services.
func (am AppModule) RegisterServices(cfg module.Configurator) {
	types.RegisterMsgServer(cfg.MsgServer(), keeper.NewMsgServerImpl(am.keeper))
	types.RegisterQueryServer(cfg.QueryServer(), am.keeper)
}

// InitGenesis initializes the genesis state for this module and implements app module.
func (am AppModule) InitGenesis(ctx sdk.Context, cdc codec.JSONCodec, data json.RawMessage) []abci.ValidatorUpdate {
	var genesisState types.GenesisState
	cdc.MustUnmarshalJSON(data, &genesisState)
	return keeper.InitGenesis(ctx, am.keeper, genesisState)
}

// ExportGenesis exports the current genesis state to a json.RawMessage
func (am AppModule) ExportGenesis(ctx sdk.Context, cdc codec.JSONCodec) json.RawMessage {
	genState := keeper.ExportGenesis(ctx, am.keeper)
	return cdc.MustMarshalJSON(genState)
}

// BeginBlock handles the start of every block
func (am AppModule) BeginBlock(ctx sdk.Context, _ abci.RequestBeginBlock) {
	// Auction module does not use BeginBlock
}

// EndBlock handles the end of every block
func (am AppModule) EndBlock(ctx sdk.Context, _ abci.RequestEndBlock) []abci.ValidatorUpdate {
	// End auctions, transfer balances, start new auctions
	EndBlocker(ctx, am.keeper)
	return []abci.ValidatorUpdate{}
}
