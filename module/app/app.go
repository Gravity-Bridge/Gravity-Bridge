package app

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/spf13/cast"

	abci "github.com/cometbft/cometbft/abci/types"
	tmjson "github.com/cometbft/cometbft/libs/json"
	tmos "github.com/cometbft/cometbft/libs/os"
	dbm "github.com/cosmos/cosmos-db"

	// Cosmos SDK
	"cosmossdk.io/client/v2/autocli"
	"cosmossdk.io/core/appmodule"
	"cosmossdk.io/log"
	"cosmossdk.io/simapp"
	simappparams "cosmossdk.io/simapp/params"
	storetypes "cosmossdk.io/store/types"
	"cosmossdk.io/x/evidence"
	evidencekeeper "cosmossdk.io/x/evidence/keeper"
	evidencetypes "cosmossdk.io/x/evidence/types"
	"cosmossdk.io/x/tx/signing"
	"cosmossdk.io/x/upgrade"
	upgradekeeper "cosmossdk.io/x/upgrade/keeper"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/grpc/cmtservice"
	nodeservice "github.com/cosmos/cosmos-sdk/client/grpc/node"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/address"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	runtimeservices "github.com/cosmos/cosmos-sdk/runtime/services"
	"github.com/cosmos/cosmos-sdk/server"
	"github.com/cosmos/cosmos-sdk/server/api"
	"github.com/cosmos/cosmos-sdk/server/config"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	"github.com/cosmos/cosmos-sdk/std"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/mempool"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/cosmos/cosmos-sdk/types/msgservice"
	"github.com/cosmos/cosmos-sdk/types/tx"
	"github.com/cosmos/cosmos-sdk/version"
	"github.com/cosmos/cosmos-sdk/x/auth"
	sdkante "github.com/cosmos/cosmos-sdk/x/auth/ante"
	authcodec "github.com/cosmos/cosmos-sdk/x/auth/codec"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	"github.com/cosmos/cosmos-sdk/x/auth/posthandler"
	authsims "github.com/cosmos/cosmos-sdk/x/auth/simulation"
	authtx "github.com/cosmos/cosmos-sdk/x/auth/tx"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/cosmos/cosmos-sdk/x/auth/vesting"
	vestingtypes "github.com/cosmos/cosmos-sdk/x/auth/vesting/types"
	"github.com/cosmos/cosmos-sdk/x/authz"
	authzkeeper "github.com/cosmos/cosmos-sdk/x/authz/keeper"
	authzmodule "github.com/cosmos/cosmos-sdk/x/authz/module"
	"github.com/cosmos/cosmos-sdk/x/bank"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/cosmos/cosmos-sdk/x/consensus"
	consensusparamkeeper "github.com/cosmos/cosmos-sdk/x/consensus/keeper"
	consensusparamtypes "github.com/cosmos/cosmos-sdk/x/consensus/types"
	"github.com/cosmos/cosmos-sdk/x/crisis"
	crisiskeeper "github.com/cosmos/cosmos-sdk/x/crisis/keeper"
	crisistypes "github.com/cosmos/cosmos-sdk/x/crisis/types"
	distr "github.com/cosmos/cosmos-sdk/x/distribution"
	distrkeeper "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	"github.com/cosmos/cosmos-sdk/x/genutil"
	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"
	"github.com/cosmos/cosmos-sdk/x/gov"
	govclient "github.com/cosmos/cosmos-sdk/x/gov/client"
	govkeeper "github.com/cosmos/cosmos-sdk/x/gov/keeper"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	govv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	govv1beta1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1beta1"
	"github.com/cosmos/cosmos-sdk/x/group"
	groupkeeper "github.com/cosmos/cosmos-sdk/x/group/keeper"
	groupmodule "github.com/cosmos/cosmos-sdk/x/group/module"
	"github.com/cosmos/cosmos-sdk/x/mint"
	mintkeeper "github.com/cosmos/cosmos-sdk/x/mint/keeper"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	"github.com/cosmos/cosmos-sdk/x/params"
	paramsclient "github.com/cosmos/cosmos-sdk/x/params/client"
	paramskeeper "github.com/cosmos/cosmos-sdk/x/params/keeper"
	paramstypes "github.com/cosmos/cosmos-sdk/x/params/types"
	paramsproposal "github.com/cosmos/cosmos-sdk/x/params/types/proposal"
	"github.com/cosmos/cosmos-sdk/x/slashing"
	slashingkeeper "github.com/cosmos/cosmos-sdk/x/slashing/keeper"
	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
	"github.com/cosmos/cosmos-sdk/x/staking"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/cosmos/gogoproto/proto"

	// Cosmos IBC-Go
	"github.com/cosmos/ibc-go/modules/capability"
	capabilitykeeper "github.com/cosmos/ibc-go/modules/capability/keeper"
	capabilitytypes "github.com/cosmos/ibc-go/modules/capability/types"
	ica "github.com/cosmos/ibc-go/v8/modules/apps/27-interchain-accounts"
	icahost "github.com/cosmos/ibc-go/v8/modules/apps/27-interchain-accounts/host"
	icahostkeeper "github.com/cosmos/ibc-go/v8/modules/apps/27-interchain-accounts/host/keeper"
	icahosttypes "github.com/cosmos/ibc-go/v8/modules/apps/27-interchain-accounts/host/types"
	icatypes "github.com/cosmos/ibc-go/v8/modules/apps/27-interchain-accounts/types"
	transfer "github.com/cosmos/ibc-go/v8/modules/apps/transfer"
	ibctransferkeeper "github.com/cosmos/ibc-go/v8/modules/apps/transfer/keeper"
	ibctransfertypes "github.com/cosmos/ibc-go/v8/modules/apps/transfer/types"
	ibc "github.com/cosmos/ibc-go/v8/modules/core"
	porttypes "github.com/cosmos/ibc-go/v8/modules/core/05-port/types"
	ibcexported "github.com/cosmos/ibc-go/v8/modules/core/exported"
	ibckeeper "github.com/cosmos/ibc-go/v8/modules/core/keeper"

	// Osmosis-Labs Bech32-IBC
	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/bech32ibc"
	bech32ibckeeper "github.com/Gravity-Bridge/Gravity-Bridge/module/x/bech32ibc/keeper"
	bech32ibctypes "github.com/Gravity-Bridge/Gravity-Bridge/module/x/bech32ibc/types"

	// unnamed import of statik for swagger UI support
	// _ "github.com/cosmos/cosmos-sdk/client/docs/statik"

	// Tharsis Ethermint
	ethante "github.com/evmos/ethermint/app/ante"
	ethermintcryptocodec "github.com/evmos/ethermint/crypto/codec"
	ethermintcodec "github.com/evmos/ethermint/encoding/codec"
	etherminttypes "github.com/evmos/ethermint/types"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/app/ante"
	"github.com/Gravity-Bridge/Gravity-Bridge/module/app/upgrades"
	"github.com/Gravity-Bridge/Gravity-Bridge/module/app/upgrades/antares"
	"github.com/Gravity-Bridge/Gravity-Bridge/module/app/upgrades/apollo"
	"github.com/Gravity-Bridge/Gravity-Bridge/module/app/upgrades/neutrino"
	v2 "github.com/Gravity-Bridge/Gravity-Bridge/module/app/upgrades/v2"
	gravityconfig "github.com/Gravity-Bridge/Gravity-Bridge/module/config"
	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/auction"
	auckeeper "github.com/Gravity-Bridge/Gravity-Bridge/module/x/auction/keeper"
	auctiontypes "github.com/Gravity-Bridge/Gravity-Bridge/module/x/auction/types"
	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity"
	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/keeper"
	gravitytypes "github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
)

const appName = "app"

var (
	// DefaultNodeHome sets the folder where the applcation data and configuration will be stored
	DefaultNodeHome string

	// TODO Remove module basics?
	/*
		// ModuleBasics The module BasicManager is in charge of setting up basic,
		// non-dependant module elements, such as codec registration
		// and genesis verification.
		ModuleBasics = module.NewBasicManager(
			auth.AppModuleBasic{},
			authzmodule.AppModuleBasic{},
			genutil.AppModuleBasic{},
			bank.AppModuleBasic{},
			capability.AppModuleBasic{},
			staking.AppModuleBasic{},
			mint.AppModuleBasic{},
			distr.AppModuleBasic{},
			gov.NewAppModuleBasic(
				[]govclient.ProposalHandler{
					paramsclient.ProposalHandler,
					distrclient.ProposalHandler,
					upgradeclient.LegacyProposalHandler,
					upgradeclient.LegacyCancelProposalHandler,
					ibcclientclient.UpdateClientProposalHandler,
					ibcclientclient.UpgradeProposalHandler,
				},
			),
			params.AppModuleBasic{},
			crisis.AppModuleBasic{},
			slashing.AppModuleBasic{},
			ibc.AppModuleBasic{},
			upgrade.AppModuleBasic{},
			evidence.AppModuleBasic{},
			transfer.AppModuleBasic{},
			vesting.AppModuleBasic{},
			gravity.AppModuleBasic{},
			auction.AppModuleBasic{},
			bech32ibc.AppModuleBasic{},
			ica.AppModuleBasic{},
			groupmodule.AppModuleBasic{},
		)
	*/

	// module account permissions
	// NOTE: We believe that this is giving various modules access to functions of the supply module? We will probably need to use this.
	maccPerms = map[string][]string{
		authtypes.FeeCollectorName:          nil,
		distrtypes.ModuleName:               nil,
		minttypes.ModuleName:                {authtypes.Minter},
		stakingtypes.BondedPoolName:         {authtypes.Burner, authtypes.Staking},
		stakingtypes.NotBondedPoolName:      {authtypes.Burner, authtypes.Staking},
		govtypes.ModuleName:                 {authtypes.Burner},
		ibctransfertypes.ModuleName:         {authtypes.Minter, authtypes.Burner},
		gravitytypes.ModuleName:             {authtypes.Minter, authtypes.Burner},
		auctiontypes.ModuleName:             {authtypes.Minter, authtypes.Burner},
		auctiontypes.AuctionPoolAccountName: nil,
		icatypes.ModuleName:                 nil,
	}

	// module accounts that are allowed to receive tokens
	allowedReceivingModAcc = map[string]bool{
		distrtypes.ModuleName: true,
		// TODO: Why was gov enabled in ethermint?
		govtypes.ModuleName: true,
	}

	// verify app interface at compile time
	_ runtime.AppI            = (*Gravity)(nil)
	_ servertypes.Application = (*Gravity)(nil)

	// enable checks that run on the first BeginBlocker execution after an upgrade/genesis init/node restart
	firstBlock sync.Once
)

// Gravity extended ABCI application
type Gravity struct {
	*baseapp.BaseApp
	legacyAmino       *codec.LegacyAmino
	AppCodec          codec.Codec
	TxConfig          client.TxConfig
	InterfaceRegistry codectypes.InterfaceRegistry
	EncodingConfig    simappparams.EncodingConfig

	invCheckPeriod uint

	// keys to access the substores
	keys    map[string]*storetypes.KVStoreKey
	tKeys   map[string]*storetypes.TransientStoreKey
	memKeys map[string]*storetypes.MemoryStoreKey

	// keepers
	// NOTE: If you add anything to this struct, add a nil check to ValidateMembers below!
	AccountKeeper         *authkeeper.AccountKeeper
	AuthzKeeper           *authzkeeper.Keeper
	BankKeeper            *bankkeeper.BaseKeeper
	CapabilityKeeper      *capabilitykeeper.Keeper
	StakingKeeper         *stakingkeeper.Keeper
	SlashingKeeper        *slashingkeeper.Keeper
	MintKeeper            *mintkeeper.Keeper
	DistrKeeper           *distrkeeper.Keeper
	GovKeeper             *govkeeper.Keeper
	CrisisKeeper          *crisiskeeper.Keeper
	UpgradeKeeper         *upgradekeeper.Keeper
	ParamsKeeper          *paramskeeper.Keeper
	IbcKeeper             *ibckeeper.Keeper
	EvidenceKeeper        *evidencekeeper.Keeper
	IbcTransferKeeper     *ibctransferkeeper.Keeper
	GravityKeeper         *keeper.Keeper
	AuctionKeeper         *auckeeper.Keeper
	Bech32IbcKeeper       *bech32ibckeeper.Keeper
	IcaHostKeeper         *icahostkeeper.Keeper
	GroupKeeper           *groupkeeper.Keeper
	ConsensusParamsKeeper *consensusparamkeeper.Keeper

	// make scoped keepers public for test purposes
	// NOTE: If you add anything to this struct, add a nil check to ValidateMembers below!
	ScopedIBCKeeper      *capabilitykeeper.ScopedKeeper
	ScopedTransferKeeper *capabilitykeeper.ScopedKeeper
	ScopedIcaHostKeeper  *capabilitykeeper.ScopedKeeper

	// Module Manager
	ModuleManager      *module.Manager
	ModuleBasicManager *module.BasicManager

	// simulation manager
	sm *module.SimulationManager

	// configurator
	configurator *module.Configurator
}

// ValidateMembers checks for nil members
func (app Gravity) ValidateMembers() {
	if app.legacyAmino == nil {
		panic("Nil legacyAmino!")
	}
	if app.TxConfig == nil {
		panic("Nil TxConfig!")
	}
	if app.AppCodec == nil {
		panic("Nil AppCodec!")
	}
	if app.InterfaceRegistry == nil {
		panic("Nil InterfaceRegistry!")
	}

	// keepers
	if app.AccountKeeper == nil {
		panic("Nil accountKeeper!")
	}
	if app.AuthzKeeper == nil {
		panic("Nil authzKeeper!")
	}
	if app.BankKeeper == nil {
		panic("Nil bankKeeper!")
	}
	if app.CapabilityKeeper == nil {
		panic("Nil capabilityKeeper!")
	}
	if app.StakingKeeper == nil {
		panic("Nil stakingKeeper!")
	}
	if app.SlashingKeeper == nil {
		panic("Nil slashingKeeper!")
	}
	if app.MintKeeper == nil {
		panic("Nil mintKeeper!")
	}
	if app.DistrKeeper == nil {
		panic("Nil distrKeeper!")
	}
	if app.GovKeeper == nil {
		panic("Nil govKeeper!")
	}
	if app.CrisisKeeper == nil {
		panic("Nil crisisKeeper!")
	}
	if app.UpgradeKeeper == nil {
		panic("Nil upgradeKeeper!")
	}
	if app.ParamsKeeper == nil {
		panic("Nil paramsKeeper!")
	}
	if app.IbcKeeper == nil {
		panic("Nil ibcKeeper!")
	}
	if app.EvidenceKeeper == nil {
		panic("Nil evidenceKeeper!")
	}
	if app.IbcTransferKeeper == nil {
		panic("Nil ibcTransferKeeper!")
	}
	if app.GravityKeeper == nil {
		panic("Nil gravityKeeper!")
	}
	if app.AuctionKeeper == nil {
		panic("Nil auctionKeeper!")
	}
	if app.Bech32IbcKeeper == nil {
		panic("Nil bech32IbcKeeper!")
	}
	if app.IcaHostKeeper == nil {
		panic("Nil icaHostKeeper!")
	}
	if app.GroupKeeper == nil {
		panic("Nil groupKeeper!")
	}
	if app.ConsensusParamsKeeper == nil {
		panic("Nil ConsensusParamsKeeper!")
	}

	// scoped keepers
	if app.ScopedIBCKeeper == nil {
		panic("Nil ScopedIBCKeeper!")
	}
	if app.ScopedTransferKeeper == nil {
		panic("Nil ScopedTransferKeeper!")
	}

	// managers
	if app.ModuleManager == nil {
		panic("Nil ModuleManager!")
	}
	if app.sm == nil {
		panic("Nil ModuleManager!")
	}
}

func init() {
	userHomeDir, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}

	DefaultNodeHome = filepath.Join(userHomeDir, ".gravity")
}

func NewGravityApp(
	logger log.Logger,
	db dbm.DB,
	traceStore io.Writer,
	loadLatest bool,
	skipUpgradeHeights map[int64]bool,
	homePath string,
	invCheckPeriod uint,
	appOpts servertypes.AppOptions,
	baseAppOptions ...func(*baseapp.BaseApp),
) *Gravity {
	legacyAmino := codec.NewLegacyAmino()
	signingOptions := signing.Options{
		AddressCodec: address.Bech32Codec{
			Bech32Prefix: sdk.GetConfig().GetBech32AccountAddrPrefix(),
		},
		ValidatorAddressCodec: address.Bech32Codec{
			Bech32Prefix: sdk.GetConfig().GetBech32ValidatorAddrPrefix(),
		},
	}
	interfaceRegistry, _ := codectypes.NewInterfaceRegistryWithOptions(codectypes.InterfaceRegistryOptions{
		ProtoFiles:     proto.HybridResolver,
		SigningOptions: signingOptions,
	})
	appCodec := codec.NewProtoCodec(interfaceRegistry)
	txConfig := authtx.NewTxConfig(appCodec, authtx.DefaultSignModes)

	encodingConfig := simappparams.EncodingConfig{
		InterfaceRegistry: interfaceRegistry,
		Codec:             appCodec,
		TxConfig:          txConfig,
		Amino:             legacyAmino,
	}
	ethermintcodec.RegisterLegacyAminoCodec(encodingConfig.Amino)
	ethermintcryptocodec.RegisterInterfaces(encodingConfig.InterfaceRegistry)
	std.RegisterInterfaces(encodingConfig.InterfaceRegistry)

	// create and set dummy vote extension handler
	voteExtOp := func(bApp *baseapp.BaseApp) {
		voteExtHandler := simapp.NewVoteExtensionHandler()
		voteExtHandler.SetHandlers(bApp)
	}
	mempoolSelection := func(app *baseapp.BaseApp) {
		mempool := mempool.NoOpMempool{}
		app.SetMempool(mempool)
		handler := baseapp.NewDefaultProposalHandler(mempool, app)
		app.SetPrepareProposal(handler.PrepareProposalHandler())
		app.SetProcessProposal(handler.ProcessProposalHandler())
	}
	baseAppOptions = append(baseAppOptions, voteExtOp, baseapp.SetOptimisticExecution(), mempoolSelection)

	bApp := *baseapp.NewBaseApp(appName, logger, db, encodingConfig.TxConfig.TxDecoder(), baseAppOptions...)
	bApp.SetCommitMultiStoreTracer(traceStore)
	bApp.SetVersion(version.Version)
	bApp.SetInterfaceRegistry(interfaceRegistry)
	bApp.SetTxEncoder(txConfig.TxEncoder())

	keys := storetypes.NewKVStoreKeys(
		authtypes.StoreKey, authzkeeper.StoreKey, banktypes.StoreKey,
		stakingtypes.StoreKey, minttypes.StoreKey, distrtypes.StoreKey,
		slashingtypes.StoreKey, govtypes.StoreKey, paramstypes.StoreKey,
		ibcexported.StoreKey, upgradetypes.StoreKey, evidencetypes.StoreKey,
		ibctransfertypes.StoreKey, capabilitytypes.StoreKey,
		icahosttypes.StoreKey, group.StoreKey, crisistypes.StoreKey, consensusparamtypes.StoreKey,

		gravitytypes.StoreKey, auctiontypes.StoreKey, bech32ibctypes.StoreKey,
	)
	tKeys := storetypes.NewTransientStoreKeys(paramstypes.TStoreKey)
	memKeys := storetypes.NewMemoryStoreKeys(capabilitytypes.MemStoreKey)

	// Register streaming services
	if err := bApp.RegisterStreamingServices(appOpts, keys); err != nil {
		panic(err)
	}

	// nolint: exhaustruct
	var app = &Gravity{
		BaseApp:           &bApp,
		legacyAmino:       legacyAmino,
		AppCodec:          appCodec,
		TxConfig:          txConfig,
		InterfaceRegistry: interfaceRegistry,
		invCheckPeriod:    invCheckPeriod,
		keys:              keys,
		tKeys:             tKeys,
		memKeys:           memKeys,
	}

	paramsKeeper := initParamsKeeper(appCodec, legacyAmino, keys[paramstypes.StoreKey], tKeys[paramstypes.TStoreKey])
	app.ParamsKeeper = &paramsKeeper

	consensusParamsKeeper := consensusparamkeeper.NewKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[consensusparamtypes.StoreKey]),
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
		runtime.EventService{},
	)
	app.ConsensusParamsKeeper = &consensusParamsKeeper
	bApp.SetParamStore(consensusParamsKeeper.ParamsStore)

	capabilityKeeper := *capabilitykeeper.NewKeeper(
		appCodec,
		keys[capabilitytypes.StoreKey],
		memKeys[capabilitytypes.MemStoreKey],
	)
	app.CapabilityKeeper = &capabilityKeeper

	scopedIBCKeeper := capabilityKeeper.ScopeToModule(ibcexported.ModuleName)
	app.ScopedIBCKeeper = &scopedIBCKeeper

	scopedTransferKeeper := capabilityKeeper.ScopeToModule(ibctransfertypes.ModuleName)
	app.ScopedTransferKeeper = &scopedTransferKeeper

	scopedIcaHostKeeper := capabilityKeeper.ScopeToModule(icahosttypes.SubModuleName)
	app.ScopedIcaHostKeeper = &scopedIcaHostKeeper

	// Applications that wish to enforce statically created ScopedKeepers should call `Seal` after creating
	// their scoped modules in `NewApp` with `ScopeToModule`
	capabilityKeeper.Seal()

	govAuthority := authtypes.NewModuleAddress(govtypes.ModuleName).String()

	accountKeeper := authkeeper.NewAccountKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[authtypes.StoreKey]),
		authtypes.ProtoBaseAccount,
		maccPerms,
		authcodec.NewBech32Codec(sdk.GetConfig().GetBech32AccountAddrPrefix()),
		Bech32Prefix,
		govAuthority,
	)
	app.AccountKeeper = &accountKeeper

	authzKeeper := authzkeeper.NewKeeper(
		runtime.NewKVStoreService(keys[authzkeeper.StoreKey]),
		appCodec,
		app.MsgServiceRouter(),
		accountKeeper,
	)
	app.AuthzKeeper = &authzKeeper

	bankKeeper := bankkeeper.NewBaseKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[banktypes.StoreKey]),
		accountKeeper,
		app.BlockedAddrs(),
		govAuthority,
		logger,
	)
	app.BankKeeper = &bankKeeper

	// optional: enable sign mode textual by overwriting the default tx config (after setting the bank keeper)
	// enabledSignModes := append(tx.DefaultSignModes, sigtypes.SignMode_SIGN_MODE_TEXTUAL)
	// txConfigOpts := tx.ConfigOptions{
	//      EnabledSignModes:           enabledSignModes,
	//      TextualCoinMetadataQueryFn: txmodule.NewBankKeeperCoinMetadataQueryFn(app.BankKeeper),
	// }
	// txConfig, err := tx.NewTxConfigWithOptions(
	//      appCodec,
	//      txConfigOpts,
	// )
	// if err != nil {
	//      panic(err)
	// }
	// app.txConfig = txConfig

	stakingKeeper := *stakingkeeper.NewKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[stakingtypes.StoreKey]),
		accountKeeper,
		bankKeeper,
		govAuthority,
		authcodec.NewBech32Codec(gravityconfig.Bech32PrefixValAddr),
		authcodec.NewBech32Codec(gravityconfig.Bech32PrefixConsAddr),
	)
	app.StakingKeeper = &stakingKeeper

	distrKeeper := distrkeeper.NewKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[distrtypes.StoreKey]),
		accountKeeper,
		bankKeeper,
		stakingKeeper,
		authtypes.FeeCollectorName,
		govAuthority,
	)
	app.DistrKeeper = &distrKeeper

	slashingKeeper := slashingkeeper.NewKeeper(
		appCodec,
		legacyAmino,
		runtime.NewKVStoreService(keys[slashingtypes.StoreKey]),
		&stakingKeeper,
		govAuthority,
	)
	app.SlashingKeeper = &slashingKeeper

	upgradeKeeper := *upgradekeeper.NewKeeper(
		skipUpgradeHeights,
		runtime.NewKVStoreService(keys[upgradetypes.StoreKey]),
		appCodec,
		homePath,
		&bApp,
		govAuthority,
	)
	app.UpgradeKeeper = &upgradeKeeper

	ibcKeeper := *ibckeeper.NewKeeper(
		appCodec,
		keys[ibcexported.StoreKey],
		app.GetSubspace(ibcexported.ModuleName),
		stakingKeeper,
		upgradeKeeper,
		scopedIBCKeeper,
		govAuthority,
	)
	app.IbcKeeper = &ibcKeeper

	ibcTransferKeeper := ibctransferkeeper.NewKeeper(
		appCodec, keys[ibctransfertypes.StoreKey], app.GetSubspace(ibctransfertypes.ModuleName),
		ibcKeeper.ChannelKeeper, ibcKeeper.ChannelKeeper, ibcKeeper.PortKeeper,
		accountKeeper, bankKeeper, scopedTransferKeeper,
		govAuthority,
	)
	app.IbcTransferKeeper = &ibcTransferKeeper

	bech32IbcKeeper := *bech32ibckeeper.NewKeeper(
		ibcKeeper.ChannelKeeper, appCodec, keys[bech32ibctypes.StoreKey],
		ibcTransferKeeper,
	)
	app.Bech32IbcKeeper = &bech32IbcKeeper

	icaHostKeeper := icahostkeeper.NewKeeper(
		appCodec, keys[icahosttypes.StoreKey], app.GetSubspace(icahosttypes.SubModuleName),
		ibcKeeper.ChannelKeeper, ibcKeeper.ChannelKeeper, ibcKeeper.PortKeeper,
		accountKeeper, scopedIcaHostKeeper, app.MsgServiceRouter(),
		govAuthority,
	)
	app.IcaHostKeeper = &icaHostKeeper
	icaHostKeeper.WithQueryRouter(bApp.GRPCQueryRouter())

	mintKeeper := mintkeeper.NewKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[minttypes.StoreKey]),
		stakingKeeper,
		accountKeeper,
		bankKeeper,
		authtypes.FeeCollectorName,
		govAuthority,
	)
	app.MintKeeper = &mintKeeper

	auctionKeeper := auckeeper.NewKeeper(
		keys[auctiontypes.StoreKey],
		app.GetSubspace(auctiontypes.ModuleName),
		appCodec,
		&bankKeeper,
		&accountKeeper,
		&distrKeeper,
		&mintKeeper,
	)
	app.AuctionKeeper = &auctionKeeper

	govModuleAddress := authtypes.NewModuleAddress(govtypes.ModuleName).String()
	gravityKeeper := keeper.NewKeeper(
		keys[gravitytypes.StoreKey],
		app.GetSubspace(gravitytypes.ModuleName),
		appCodec,
		&bankKeeper,
		&stakingKeeper,
		&slashingKeeper,
		&distrKeeper,
		&accountKeeper,
		&ibcTransferKeeper,
		&bech32IbcKeeper,
		&auctionKeeper,
		govModuleAddress,
	)
	app.GravityKeeper = &gravityKeeper

	// Add the staking hooks from distribution, slashing, and gravity to staking
	stakingKeeper.SetHooks(
		stakingtypes.NewMultiStakingHooks(
			distrKeeper.Hooks(),
			slashingKeeper.Hooks(),
			gravityKeeper.Hooks(),
		),
	)

	crisisKeeper := *crisiskeeper.NewKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[crisistypes.StoreKey]),
		invCheckPeriod,
		bankKeeper,
		authtypes.FeeCollectorName,
		govAuthority,
		accountKeeper.AddressCodec(),
	)
	app.CrisisKeeper = &crisisKeeper

	govRouter := govv1beta1.NewRouter()
	govRouter.AddRoute(govtypes.RouterKey, govv1beta1.ProposalHandler).
		AddRoute(paramsproposal.RouterKey, params.NewParamChangeProposalHandler(paramsKeeper)).
		AddRoute(gravitytypes.RouterKey, keeper.NewGravityProposalHandler(gravityKeeper)).
		AddRoute(bech32ibctypes.RouterKey, bech32ibc.NewBech32IBCProposalHandler(*app.Bech32IbcKeeper))

	govConfig := govtypes.DefaultConfig()
	govKeeper := *govkeeper.NewKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[govtypes.StoreKey]),
		accountKeeper,
		bankKeeper,
		stakingKeeper,
		distrKeeper,
		app.MsgServiceRouter(),
		govConfig,
		govAuthority,
	)
	govKeeper.SetLegacyRouter(govRouter)
	govKeeper = *govKeeper.SetHooks(govtypes.NewMultiGovHooks(
	// Register any governance hooks here
	))
	app.GovKeeper = &govKeeper

	ibcTransferAppModule := transfer.NewAppModule(ibcTransferKeeper)
	ibcTransferIBCModule := transfer.NewIBCModule(ibcTransferKeeper)
	icaAppModule := ica.NewAppModule(nil, &icaHostKeeper)
	icaHostIBCModule := icahost.NewIBCModule(icaHostKeeper)

	ibcRouter := porttypes.NewRouter()
	ibcRouter.AddRoute(ibctransfertypes.ModuleName, ibcTransferIBCModule).
		AddRoute(icahosttypes.SubModuleName, icaHostIBCModule)
	ibcKeeper.SetRouter(ibcRouter)

	evidenceKeeper := *evidencekeeper.NewKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[evidencetypes.StoreKey]),
		&stakingKeeper,
		slashingKeeper,
		accountKeeper.AddressCodec(),
		runtime.ProvideCometInfoService(),
	)
	app.EvidenceKeeper = &evidenceKeeper

	groupConfig := group.DefaultConfig()
	groupKeeper := groupkeeper.NewKeeper(keys[group.StoreKey], appCodec, app.MsgServiceRouter(), app.AccountKeeper, groupConfig)
	app.GroupKeeper = &groupKeeper

	var skipGenesisInvariants = cast.ToBool(appOpts.Get(crisis.FlagSkipGenesisInvariants))

	app.registerStoreLoaders()

	moduleManager := *module.NewManager(
		genutil.NewAppModule(
			accountKeeper,
			stakingKeeper,
			app,
			txConfig,
		),
		auth.NewAppModule(
			appCodec,
			accountKeeper,
			authsims.RandomGenesisAccounts,
			app.GetSubspace(authtypes.ModuleName),
		),
		authzmodule.NewAppModule(
			appCodec,
			authzKeeper,
			accountKeeper,
			bankKeeper,
			app.InterfaceRegistry,
		),
		vesting.NewAppModule(
			accountKeeper,
			bankKeeper,
		),
		bank.NewAppModule(
			appCodec,
			bankKeeper,
			accountKeeper,
			app.GetSubspace(banktypes.ModuleName),
		),
		capability.NewAppModule(
			appCodec,
			capabilityKeeper,
			false,
		),
		crisis.NewAppModule(
			&crisisKeeper,
			skipGenesisInvariants,
			app.GetSubspace(crisistypes.ModuleName),
		),
		gov.NewAppModule(
			appCodec,
			&govKeeper,
			accountKeeper,
			bankKeeper,
			app.GetSubspace(govtypes.ModuleName),
		),
		mint.NewAppModule(
			appCodec,
			mintKeeper,
			accountKeeper,
			nil,
			app.GetSubspace(minttypes.ModuleName),
		),
		slashing.NewAppModule(
			appCodec,
			slashingKeeper,
			accountKeeper,
			bankKeeper,
			stakingKeeper,
			app.GetSubspace(slashingtypes.ModuleName),
			interfaceRegistry,
		),
		distr.NewAppModule(
			appCodec,
			distrKeeper,
			accountKeeper,
			bankKeeper,
			stakingKeeper,
			app.GetSubspace(distrtypes.ModuleName),
		),
		staking.NewAppModule(
			appCodec,
			&stakingKeeper,
			accountKeeper,
			bankKeeper,
			app.GetSubspace(stakingtypes.ModuleName),
		),
		upgrade.NewAppModule(&upgradeKeeper, accountKeeper.AddressCodec()),
		evidence.NewAppModule(evidenceKeeper),
		ibc.NewAppModule(&ibcKeeper),
		params.NewAppModule(paramsKeeper),
		ibcTransferAppModule,
		gravity.NewAppModule(
			gravityKeeper,
			bankKeeper,
		),
		auction.NewAppModule(
			auctionKeeper,
			bankKeeper,
			accountKeeper,
		),
		bech32ibc.NewAppModule(
			appCodec,
			bech32IbcKeeper,
		),
		icaAppModule,
		groupmodule.NewAppModule(appCodec, groupKeeper, accountKeeper, bankKeeper, interfaceRegistry),
		consensus.NewAppModule(appCodec, consensusParamsKeeper),
	)
	app.ModuleManager = &moduleManager

	moduleBasicManager := module.NewBasicManagerFromManager(
		app.ModuleManager,
		map[string]module.AppModuleBasic{
			genutiltypes.ModuleName: genutil.NewAppModuleBasic(genutiltypes.DefaultMessageValidator),
			govtypes.ModuleName: gov.NewAppModuleBasic(
				[]govclient.ProposalHandler{
					paramsclient.ProposalHandler,
				},
			),
		},
	)

	moduleBasicManager.RegisterLegacyAminoCodec(encodingConfig.Amino)
	moduleBasicManager.RegisterInterfaces(encodingConfig.InterfaceRegistry)
	app.ModuleBasicManager = &moduleBasicManager
	// nolint: exhaustruct
	encodingConfig.InterfaceRegistry.RegisterImplementations(
		(*tx.TxExtensionOptionI)(nil),
		&etherminttypes.ExtensionOptionsWeb3Tx{},
	)
	app.EncodingConfig = encodingConfig

	// NOTE: upgrade module is required to be prioritized
	app.ModuleManager.SetOrderPreBlockers(
		upgradetypes.ModuleName,
	)

	// NOTE: capability module's BeginBlocker must come before any modules using capabilities (e.g. IBC)
	moduleManager.SetOrderBeginBlockers(
		upgradetypes.ModuleName,
		capabilitytypes.ModuleName,
		minttypes.ModuleName,
		distrtypes.ModuleName,
		slashingtypes.ModuleName,
		evidencetypes.ModuleName,
		stakingtypes.ModuleName,
		ibcexported.ModuleName,
		banktypes.ModuleName,
		crisistypes.ModuleName,
		authtypes.ModuleName,
		vestingtypes.ModuleName,
		ibctransfertypes.ModuleName,
		bech32ibctypes.ModuleName,
		gravitytypes.ModuleName,
		auctiontypes.ModuleName,
		genutiltypes.ModuleName,
		authz.ModuleName,
		govtypes.ModuleName,
		paramstypes.ModuleName,
		icatypes.ModuleName,
		group.ModuleName,
		consensusparamtypes.ModuleName,
	)
	moduleManager.SetOrderEndBlockers(
		crisistypes.ModuleName,
		govtypes.ModuleName,
		stakingtypes.ModuleName,
		icatypes.ModuleName,
		gravitytypes.ModuleName,
		auctiontypes.ModuleName,
		upgradetypes.ModuleName,
		capabilitytypes.ModuleName,
		minttypes.ModuleName,
		distrtypes.ModuleName,
		slashingtypes.ModuleName,
		evidencetypes.ModuleName,
		ibcexported.ModuleName,
		banktypes.ModuleName,
		authtypes.ModuleName,
		vestingtypes.ModuleName,
		ibctransfertypes.ModuleName,
		bech32ibctypes.ModuleName,
		genutiltypes.ModuleName,
		authz.ModuleName,
		paramstypes.ModuleName,
		group.ModuleName,
		consensusparamtypes.ModuleName,
	)
	moduleManager.SetOrderInitGenesis(
		capabilitytypes.ModuleName,
		authtypes.ModuleName,
		banktypes.ModuleName,
		distrtypes.ModuleName,
		stakingtypes.ModuleName,
		slashingtypes.ModuleName,
		govtypes.ModuleName,
		minttypes.ModuleName,
		upgradetypes.ModuleName,
		ibcexported.ModuleName,
		genutiltypes.ModuleName,
		evidencetypes.ModuleName,
		ibctransfertypes.ModuleName,
		authz.ModuleName,
		bech32ibctypes.ModuleName, // Must go before gravity so that pending ibc auto forwards can be restored
		gravitytypes.ModuleName,
		auctiontypes.ModuleName, // Must go after bank module to verify balances
		crisistypes.ModuleName,
		vestingtypes.ModuleName,
		paramstypes.ModuleName,
		icatypes.ModuleName,
		group.ModuleName,
		consensusparamtypes.ModuleName,
	)

	moduleManager.RegisterInvariants(&crisisKeeper)
	configurator := module.NewConfigurator(appCodec, app.MsgServiceRouter(), app.GRPCQueryRouter())
	app.configurator = &configurator
	err := moduleManager.RegisterServices(*app.configurator)
	if err != nil {
		panic(err)
	}

	overrideModules := map[string]module.AppModuleSimulation{
		authtypes.ModuleName: auth.NewAppModule(appCodec, accountKeeper, authsims.RandomGenesisAccounts, app.GetSubspace(authtypes.ModuleName)),
	}
	sm := *module.NewSimulationManagerFromAppModules(moduleManager.Modules, overrideModules)
	app.sm = &sm

	sm.RegisterStoreDecoders()

	app.MountKVStores(keys)
	app.MountTransientStores(tKeys)
	app.MountMemoryStores(memKeys)

	app.SetInitChainer(app.InitChainer)
	app.SetPreBlocker(app.PreBlocker)
	app.SetBeginBlocker(app.BeginBlocker)

	app.SetEndBlocker(app.EndBlocker)

	app.setAnteHandler(encodingConfig)
	app.setPostHandler()

	app.registerUpgradeHandlers()

	protoFiles, err := proto.MergedRegistry()
	if err != nil {
		panic(err)
	}
	err = msgservice.ValidateProtoAnnotations(protoFiles)
	if err != nil {
		panic(err)
	}

	if loadLatest {
		if err := app.LoadLatestVersion(); err != nil {
			tmos.Exit(err.Error())
		}
	}

	keeper.RegisterProposalTypes()

	// We don't allow anything to be nil
	app.ValidateMembers()
	return app
}

func (app *Gravity) setAnteHandler(encodingConfig simappparams.EncodingConfig) {
	options := sdkante.HandlerOptions{
		AccountKeeper:          app.AccountKeeper,
		BankKeeper:             app.BankKeeper,
		FeegrantKeeper:         nil,
		SignModeHandler:        encodingConfig.TxConfig.SignModeHandler(),
		SigGasConsumer:         ethante.DefaultSigVerificationGasConsumer,
		ExtensionOptionChecker: nil,
		TxFeeChecker:           nil,
	}

	// Note: If feegrant keeper is added, add it to the NewAnteHandler call instead of nil
	ah, err := ante.NewAnteHandler(options, app.GravityKeeper, app.AccountKeeper, app.BankKeeper, nil, app.IbcKeeper, app.AppCodec, gravityconfig.GravityEvmChainIDs)
	if err != nil {
		panic("invalid antehandler created")
	}
	app.SetAnteHandler(*ah)
}
func (app *Gravity) setPostHandler() {
	postHandler, err := posthandler.NewPostHandler(
		posthandler.HandlerOptions{},
	)
	if err != nil {
		panic(err)
	}

	app.SetPostHandler(postHandler)
}

// MakeCodecs constructs the *std.Codec and *codec.LegacyAmino instances used by
// simapp. It is useful for tests and clients who do not want to construct the
// full simapp
func MakeCodecs() (codec.Codec, *codec.LegacyAmino) {
	config := MakeEncodingConfig()
	return config.Codec, config.Amino
}

// Name returns the name of the App
func (app *Gravity) Name() string { return app.BaseApp.Name() }

func (app *Gravity) LegacyAmino() *codec.LegacyAmino {
	return app.legacyAmino
}

// PreBlocker updates every pre begin block
func (app *Gravity) PreBlocker(ctx sdk.Context, _ *abci.RequestFinalizeBlock) (*sdk.ResponsePreBlock, error) {
	return app.ModuleManager.PreBlock(ctx)
}

// BeginBlocker application updates every begin block
func (app *Gravity) BeginBlocker(ctx sdk.Context) (sdk.BeginBlock, error) {
	out, err := app.ModuleManager.BeginBlock(ctx)
	if err != nil {
		// nolint: exhaustruct
		return sdk.BeginBlock{}, err
	}
	firstBlock.Do(func() { // Run the startup firstBeginBlocker assertions only once
		app.firstBeginBlocker(ctx)
	})

	return out, nil
}

// Perform necessary checks at the start of this node's first BeginBlocker execution
// Note: This should ONLY be called once, it should be called at the top of BeginBlocker guarded by firstBlock
func (app *Gravity) firstBeginBlocker(ctx sdk.Context) {
	app.assertBech32PrefixMatches(ctx)
	app.assertNativeTokenMatchesConstant(ctx)
	app.assertNativeTokenIsNonAuctionable(ctx)
}

// EndBlocker application updates every end block
func (app *Gravity) EndBlocker(ctx sdk.Context) (sdk.EndBlock, error) {
	return app.ModuleManager.EndBlock(ctx)
}

// InitChainer application update at chain initialization
func (app *Gravity) InitChainer(ctx sdk.Context, req *abci.RequestInitChain) (*abci.ResponseInitChain, error) {
	var genesisState simapp.GenesisState
	if err := tmjson.Unmarshal(req.AppStateBytes, &genesisState); err != nil {
		panic(err)
	}

	app.UpgradeKeeper.SetModuleVersionMap(ctx, app.ModuleManager.GetVersionMap())

	return app.ModuleManager.InitGenesis(ctx, app.AppCodec, genesisState)
}

// LoadHeight loads a particular height
func (app *Gravity) LoadHeight(height int64) error {
	return app.LoadVersion(height)
}

// ModuleAccountAddrs returns all the app's module account addresses.
func (app *Gravity) ModuleAccountAddrs() map[string]bool {
	modAccAddrs := make(map[string]bool)
	for acc := range maccPerms {
		modAccAddrs[authtypes.NewModuleAddress(acc).String()] = true
	}

	return modAccAddrs
}

// BlockedAddrs returns all the app's module account addresses that are not
// allowed to receive external tokens.
func (app *Gravity) BlockedAddrs() map[string]bool {
	blockedAddrs := make(map[string]bool)
	for acc := range maccPerms {
		blockedAddrs[authtypes.NewModuleAddress(acc).String()] = !allowedReceivingModAcc[acc]
	}

	return blockedAddrs
}

// GetSubspace returns a param subspace for a given module name.
func (app *Gravity) GetSubspace(moduleName string) paramstypes.Subspace {
	subspace, _ := app.ParamsKeeper.GetSubspace(moduleName)
	return subspace
}

// SimulationManager implements the SimulationApp interface
func (app *Gravity) SimulationManager() *module.SimulationManager {
	return app.sm
}

// GetTxConfig implements the TestingApp interface.
func (app *Gravity) GetTxConfig() client.TxConfig {
	cfg := MakeEncodingConfig()
	return cfg.TxConfig
}

// GetBaseApp returns the base app of the application
func (app *Gravity) GetBaseApp() *baseapp.BaseApp { return app.BaseApp }

// RegisterAPIRoutes registers all application module routes with the provided
// API server.
func (app *Gravity) RegisterAPIRoutes(apiSvr *api.Server, apiConfig config.APIConfig) {
	clientCtx := apiSvr.ClientCtx
	authtx.RegisterGRPCGatewayRoutes(clientCtx, apiSvr.GRPCGatewayRouter)
	cmtservice.RegisterGRPCGatewayRoutes(clientCtx, apiSvr.GRPCGatewayRouter)
	nodeservice.RegisterGRPCGatewayRoutes(clientCtx, apiSvr.GRPCGatewayRouter)
	app.ModuleBasicManager.RegisterGRPCGatewayRoutes(clientCtx, apiSvr.GRPCGatewayRouter)

	if apiConfig.Swagger {
		if err := server.RegisterSwaggerAPI(apiSvr.ClientCtx, apiSvr.Router, apiConfig.Swagger); err != nil {
			panic(err)
		}
	}
}

// RegisterTxService implements the Application.RegisterTxService method.
func (app *Gravity) RegisterTxService(clientCtx client.Context) {
	authtx.RegisterTxService(app.BaseApp.GRPCQueryRouter(), clientCtx, app.BaseApp.Simulate, app.InterfaceRegistry)
}

// RegisterTendermintService implements the Application.RegisterTendermintService method.
func (app *Gravity) RegisterTendermintService(clientCtx client.Context) {
	cmtservice.RegisterTendermintService(clientCtx, app.BaseApp.GRPCQueryRouter(), app.InterfaceRegistry, app.Query)
}

func (app *Gravity) RegisterNodeService(clientCtx client.Context, cfg config.Config) {
	nodeservice.RegisterNodeService(clientCtx, app.GRPCQueryRouter(), cfg)
}

// GetMaccPerms returns a mapping of the application's module account permissions.
func GetMaccPerms() map[string][]string {
	modAccPerms := make(map[string][]string)
	for k, v := range maccPerms {
		modAccPerms[k] = v
	}
	return modAccPerms
}

// initParamsKeeper init params keeper and its subspaces
func initParamsKeeper(appCodec codec.BinaryCodec, legacyAmino *codec.LegacyAmino, key, tkey storetypes.StoreKey) paramskeeper.Keeper {
	paramsKeeper := paramskeeper.NewKeeper(appCodec, legacyAmino, key, tkey)

	paramsKeeper.Subspace(authtypes.ModuleName)
	paramsKeeper.Subspace(banktypes.ModuleName)
	paramsKeeper.Subspace(stakingtypes.ModuleName)
	paramsKeeper.Subspace(minttypes.ModuleName)
	paramsKeeper.Subspace(distrtypes.ModuleName)
	paramsKeeper.Subspace(slashingtypes.ModuleName)
	paramsKeeper.Subspace(govtypes.ModuleName).WithKeyTable(govv1.ParamKeyTable())
	paramsKeeper.Subspace(crisistypes.ModuleName)
	paramsKeeper.Subspace(ibctransfertypes.ModuleName)
	paramsKeeper.Subspace(gravitytypes.ModuleName)
	paramsKeeper.Subspace(auctiontypes.ModuleName)
	paramsKeeper.Subspace(ibcexported.ModuleName)
	paramsKeeper.Subspace(icahosttypes.SubModuleName)

	return paramsKeeper
}

// Registers handlers for all our upgrades
func (app *Gravity) registerUpgradeHandlers() {
	upgrades.RegisterUpgradeHandlers(
		app.ModuleManager, app.configurator, app.AccountKeeper, app.BankKeeper, app.Bech32IbcKeeper, app.DistrKeeper,
		app.MintKeeper, app.StakingKeeper, app.UpgradeKeeper, app.CrisisKeeper, app.IbcTransferKeeper, app.AuctionKeeper,
	)
}

// Sets up the StoreLoader for new, deleted, or renamed modules
func (app *Gravity) registerStoreLoaders() {
	// Read the upgrade height and name from previous execution
	upgradeInfo, err := app.UpgradeKeeper.ReadUpgradeInfoFromDisk()
	if err != nil {
		panic(fmt.Sprintf("failed to read upgrade info from disk %s", err))
	}
	if app.UpgradeKeeper.IsSkipHeight(upgradeInfo.Height) {
		return
	}

	// STORE LOADER CONFIGURATION:
	// Added: []string{"newmodule"}, // We are adding these modules
	// Renamed: []storetypes.StoreRename{{"foo", "bar"}}, example foo to bar rename
	// Deleted: []string{"bazmodule"}, example deleted bazmodule

	// v1->v2 STORE LOADER SETUP
	// Register the new v2 modules and the special StoreLoader to add them
	if upgradeInfo.Name == v2.V1ToV2PlanName {
		// Register the bech32ibc module as a new module that needs a new store allocated
		storeUpgrades := storetypes.StoreUpgrades{
			Added:   []string{bech32ibctypes.ModuleName},
			Renamed: nil,
			Deleted: nil,
		}

		app.SetStoreLoader(upgradetypes.UpgradeStoreLoader(upgradeInfo.Height, &storeUpgrades))
	}
	// ANTARES ICA Host module store loader setup
	if upgradeInfo.Name == antares.OrionToAntaresPlanName {
		// Register the ICA Host module as a new module that needs a new store allocated
		storeUpgrades := storetypes.StoreUpgrades{
			Added:   []string{icahosttypes.StoreKey},
			Renamed: nil,
			Deleted: nil,
		}

		app.SetStoreLoader(upgradetypes.UpgradeStoreLoader(upgradeInfo.Height, &storeUpgrades))
	}
	// Apollo Auction module store loader setup
	if upgradeInfo.Name == apollo.AntaresToApolloPlanName {
		// Register the Auction module as a new module that needs a new store allocated
		storeUpgrades := storetypes.StoreUpgrades{
			Added:   []string{auctiontypes.StoreKey},
			Renamed: nil,
			Deleted: nil,
		}

		app.SetStoreLoader(upgradetypes.UpgradeStoreLoader(upgradeInfo.Height, &storeUpgrades))
	}
	// Neutrino Group module store loader setup
	if upgradeInfo.Name == neutrino.ApolloToNeutrinoPlanName {
		// Register the Group module as a new module that needs a new store allocated
		storeUpgrades := storetypes.StoreUpgrades{
			Added:   []string{group.StoreKey},
			Renamed: nil,
			Deleted: nil,
		}

		app.SetStoreLoader(upgradetypes.UpgradeStoreLoader(upgradeInfo.Height, &storeUpgrades))
	}
}

// AutoCliOpts returns the autocli options for the app.
func (app *Gravity) AutoCliOpts() autocli.AppOptions {
	modules := make(map[string]appmodule.AppModule, 0)
	for _, m := range app.ModuleManager.Modules {
		if moduleWithName, ok := m.(module.HasName); ok {
			moduleName := moduleWithName.Name()
			if appModule, ok := moduleWithName.(appmodule.AppModule); ok {
				modules[moduleName] = appModule
			}
		}
	}

	return autocli.AppOptions{
		Modules:               modules,
		ModuleOptions:         runtimeservices.ExtractAutoCLIOptions(app.ModuleManager.Modules),
		AddressCodec:          authcodec.NewBech32Codec(sdk.GetConfig().GetBech32AccountAddrPrefix()),
		ValidatorAddressCodec: authcodec.NewBech32Codec(sdk.GetConfig().GetBech32ValidatorAddrPrefix()),
		ConsensusAddressCodec: authcodec.NewBech32Codec(sdk.GetConfig().GetBech32ConsensusAddrPrefix()),
	}
}

// DefaultGenesis returns a default genesis from the registered AppModuleBasic's.
func (app *Gravity) DefaultGenesis() map[string]json.RawMessage {
	return app.ModuleBasicManager.DefaultGenesis(app.AppCodec)
}

// GetStoreKeys returns all the stored store keys.
func (app *Gravity) GetStoreKeys() []storetypes.StoreKey {
	keys := make([]storetypes.StoreKey, len(app.keys))
	for _, key := range app.keys {
		keys = append(keys, key)
	}

	return keys
}
