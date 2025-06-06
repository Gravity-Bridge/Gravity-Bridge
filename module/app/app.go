package app

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/gorilla/mux"
	"github.com/rakyll/statik/fs"
	"github.com/spf13/cast"

	abci "github.com/tendermint/tendermint/abci/types"
	tmjson "github.com/tendermint/tendermint/libs/json"
	"github.com/tendermint/tendermint/libs/log"
	tmos "github.com/tendermint/tendermint/libs/os"
	dbm "github.com/tendermint/tm-db"

	// Cosmos SDK

	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client"
	nodeservice "github.com/cosmos/cosmos-sdk/client/grpc/node"
	"github.com/cosmos/cosmos-sdk/client/grpc/tmservice"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/types"
	ccodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	"github.com/cosmos/cosmos-sdk/server/api"
	"github.com/cosmos/cosmos-sdk/server/config"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	"github.com/cosmos/cosmos-sdk/simapp"
	"github.com/cosmos/cosmos-sdk/store/streaming"
	storetypes "github.com/cosmos/cosmos-sdk/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/cosmos/cosmos-sdk/version"
	"github.com/cosmos/cosmos-sdk/x/auth"
	sdkante "github.com/cosmos/cosmos-sdk/x/auth/ante"
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
	"github.com/cosmos/cosmos-sdk/x/capability"
	capabilitykeeper "github.com/cosmos/cosmos-sdk/x/capability/keeper"
	capabilitytypes "github.com/cosmos/cosmos-sdk/x/capability/types"
	"github.com/cosmos/cosmos-sdk/x/crisis"
	crisiskeeper "github.com/cosmos/cosmos-sdk/x/crisis/keeper"
	crisistypes "github.com/cosmos/cosmos-sdk/x/crisis/types"
	distr "github.com/cosmos/cosmos-sdk/x/distribution"
	distrclient "github.com/cosmos/cosmos-sdk/x/distribution/client"
	distrkeeper "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	"github.com/cosmos/cosmos-sdk/x/evidence"
	evidencekeeper "github.com/cosmos/cosmos-sdk/x/evidence/keeper"
	evidencetypes "github.com/cosmos/cosmos-sdk/x/evidence/types"
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
	"github.com/cosmos/cosmos-sdk/x/upgrade"
	upgradeclient "github.com/cosmos/cosmos-sdk/x/upgrade/client"
	upgradekeeper "github.com/cosmos/cosmos-sdk/x/upgrade/keeper"
	upgradetypes "github.com/cosmos/cosmos-sdk/x/upgrade/types"

	// Cosmos IBC-Go
	ica "github.com/cosmos/ibc-go/v6/modules/apps/27-interchain-accounts"
	icahost "github.com/cosmos/ibc-go/v6/modules/apps/27-interchain-accounts/host"
	icahostkeeper "github.com/cosmos/ibc-go/v6/modules/apps/27-interchain-accounts/host/keeper"
	icahosttypes "github.com/cosmos/ibc-go/v6/modules/apps/27-interchain-accounts/host/types"
	icatypes "github.com/cosmos/ibc-go/v6/modules/apps/27-interchain-accounts/types"
	transfer "github.com/cosmos/ibc-go/v6/modules/apps/transfer"
	ibctransferkeeper "github.com/cosmos/ibc-go/v6/modules/apps/transfer/keeper"
	ibctransfertypes "github.com/cosmos/ibc-go/v6/modules/apps/transfer/types"
	ibc "github.com/cosmos/ibc-go/v6/modules/core"
	ibcclient "github.com/cosmos/ibc-go/v6/modules/core/02-client"
	ibcclientclient "github.com/cosmos/ibc-go/v6/modules/core/02-client/client"
	ibcclienttypes "github.com/cosmos/ibc-go/v6/modules/core/02-client/types"
	porttypes "github.com/cosmos/ibc-go/v6/modules/core/05-port/types"
	ibchost "github.com/cosmos/ibc-go/v6/modules/core/24-host"
	ibckeeper "github.com/cosmos/ibc-go/v6/modules/core/keeper"

	// Osmosis-Labs Bech32-IBC
	"github.com/althea-net/bech32-ibc/x/bech32ibc"
	bech32ibckeeper "github.com/althea-net/bech32-ibc/x/bech32ibc/keeper"
	bech32ibctypes "github.com/althea-net/bech32-ibc/x/bech32ibc/types"

	// unnamed import of statik for swagger UI support
	_ "github.com/cosmos/cosmos-sdk/client/docs/statik"

	// Tharsis Ethermint
	ethante "github.com/evmos/ethermint/app/ante"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/app/ante"
	gravityparams "github.com/Gravity-Bridge/Gravity-Bridge/module/app/params"
	"github.com/Gravity-Bridge/Gravity-Bridge/module/app/upgrades"
	"github.com/Gravity-Bridge/Gravity-Bridge/module/app/upgrades/antares"
	"github.com/Gravity-Bridge/Gravity-Bridge/module/app/upgrades/apollo"
	"github.com/Gravity-Bridge/Gravity-Bridge/module/app/upgrades/neutrino"
	v2 "github.com/Gravity-Bridge/Gravity-Bridge/module/app/upgrades/v2"
	gravityconfig "github.com/Gravity-Bridge/Gravity-Bridge/module/config"
	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity"
	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/keeper"
	gravitytypes "github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/auction"
	auckeeper "github.com/Gravity-Bridge/Gravity-Bridge/module/x/auction/keeper"
	auctiontypes "github.com/Gravity-Bridge/Gravity-Bridge/module/x/auction/types"
)

const appName = "app"

var (
	// DefaultNodeHome sets the folder where the applcation data and configuration will be stored
	DefaultNodeHome string

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
	}

	// verify app interface at compile time
	_ simapp.App              = (*Gravity)(nil)
	_ servertypes.Application = (*Gravity)(nil)

	// enable checks that run on the first BeginBlocker execution after an upgrade/genesis init/node restart
	firstBlock sync.Once
)

// MakeCodec creates the application codec. The codec is sealed before it is
// returned.
func MakeCodec() *codec.LegacyAmino {
	var cdc = codec.NewLegacyAmino()
	ModuleBasics.RegisterLegacyAminoCodec(cdc)
	vesting.AppModuleBasic{}.RegisterLegacyAminoCodec(cdc)
	sdk.RegisterLegacyAminoCodec(cdc)
	ccodec.RegisterCrypto(cdc)
	cdc.Seal()
	return cdc
}

// Gravity extended ABCI application
type Gravity struct {
	*baseapp.BaseApp
	legacyAmino       *codec.LegacyAmino
	AppCodec          codec.Codec
	InterfaceRegistry types.InterfaceRegistry

	invCheckPeriod uint

	// keys to access the substores
	keys    map[string]*storetypes.KVStoreKey
	tKeys   map[string]*storetypes.TransientStoreKey
	memKeys map[string]*storetypes.MemoryStoreKey

	// keepers
	// NOTE: If you add anything to this struct, add a nil check to ValidateMembers below!
	AccountKeeper     *authkeeper.AccountKeeper
	AuthzKeeper       *authzkeeper.Keeper
	BankKeeper        *bankkeeper.BaseKeeper
	CapabilityKeeper  *capabilitykeeper.Keeper
	StakingKeeper     *stakingkeeper.Keeper
	SlashingKeeper    *slashingkeeper.Keeper
	MintKeeper        *mintkeeper.Keeper
	DistrKeeper       *distrkeeper.Keeper
	GovKeeper         *govkeeper.Keeper
	CrisisKeeper      *crisiskeeper.Keeper
	UpgradeKeeper     *upgradekeeper.Keeper
	ParamsKeeper      *paramskeeper.Keeper
	IbcKeeper         *ibckeeper.Keeper
	EvidenceKeeper    *evidencekeeper.Keeper
	IbcTransferKeeper *ibctransferkeeper.Keeper
	GravityKeeper     *keeper.Keeper
	AuctionKeeper     *auckeeper.Keeper
	Bech32IbcKeeper   *bech32ibckeeper.Keeper
	IcaHostKeeper     *icahostkeeper.Keeper
	GroupKeeper       *groupkeeper.Keeper

	// make scoped keepers public for test purposes
	// NOTE: If you add anything to this struct, add a nil check to ValidateMembers below!
	ScopedIBCKeeper      *capabilitykeeper.ScopedKeeper
	ScopedTransferKeeper *capabilitykeeper.ScopedKeeper
	ScopedIcaHostKeeper  *capabilitykeeper.ScopedKeeper

	// Module Manager
	mm *module.Manager

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

	// scoped keepers
	if app.ScopedIBCKeeper == nil {
		panic("Nil ScopedIBCKeeper!")
	}
	if app.ScopedTransferKeeper == nil {
		panic("Nil ScopedTransferKeeper!")
	}

	// managers
	if app.mm == nil {
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
	logger log.Logger, db dbm.DB, traceStore io.Writer, loadLatest bool, skipUpgradeHeights map[int64]bool,
	homePath string, invCheckPeriod uint, encodingConfig gravityparams.EncodingConfig,
	appOpts servertypes.AppOptions, baseAppOptions ...func(*baseapp.BaseApp),
) *Gravity {
	appCodec := encodingConfig.Marshaler
	legacyAmino := encodingConfig.Amino
	interfaceRegistry := encodingConfig.InterfaceRegistry

	bApp := *baseapp.NewBaseApp(appName, logger, db, encodingConfig.TxConfig.TxDecoder(), baseAppOptions...)
	bApp.SetCommitMultiStoreTracer(traceStore)
	bApp.SetVersion(version.Version)
	bApp.SetInterfaceRegistry(interfaceRegistry)

	keys := sdk.NewKVStoreKeys(
		authtypes.StoreKey, authzkeeper.StoreKey, banktypes.StoreKey,
		stakingtypes.StoreKey, minttypes.StoreKey, distrtypes.StoreKey,
		slashingtypes.StoreKey, govtypes.StoreKey, paramstypes.StoreKey,
		ibchost.StoreKey, upgradetypes.StoreKey, evidencetypes.StoreKey,
		ibctransfertypes.StoreKey, capabilitytypes.StoreKey,
		gravitytypes.StoreKey, auctiontypes.StoreKey, bech32ibctypes.StoreKey,
		icahosttypes.StoreKey, group.StoreKey,
	)
	tKeys := sdk.NewTransientStoreKeys(paramstypes.TStoreKey)
	memKeys := sdk.NewMemoryStoreKeys(capabilitytypes.MemStoreKey)

	// load state streaming if enabled
	if _, _, err := streaming.LoadStreamingServices(&bApp, appOpts, appCodec, keys); err != nil {
		fmt.Printf("failed to load state streaming: %s", err)
		os.Exit(1)
	}

	// nolint: exhaustruct
	var app = &Gravity{
		BaseApp:           &bApp,
		legacyAmino:       legacyAmino,
		AppCodec:          appCodec,
		InterfaceRegistry: interfaceRegistry,
		invCheckPeriod:    invCheckPeriod,
		keys:              keys,
		tKeys:             tKeys,
		memKeys:           memKeys,
	}

	paramsKeeper := initParamsKeeper(appCodec, legacyAmino, keys[paramstypes.StoreKey], tKeys[paramstypes.TStoreKey])
	app.ParamsKeeper = &paramsKeeper

	bApp.SetParamStore(paramsKeeper.Subspace(baseapp.Paramspace).WithKeyTable(paramstypes.ConsensusParamsKeyTable()))

	capabilityKeeper := *capabilitykeeper.NewKeeper(
		appCodec,
		keys[capabilitytypes.StoreKey],
		memKeys[capabilitytypes.MemStoreKey],
	)
	app.CapabilityKeeper = &capabilityKeeper

	scopedIBCKeeper := capabilityKeeper.ScopeToModule(ibchost.ModuleName)
	app.ScopedIBCKeeper = &scopedIBCKeeper

	scopedTransferKeeper := capabilityKeeper.ScopeToModule(ibctransfertypes.ModuleName)
	app.ScopedTransferKeeper = &scopedTransferKeeper

	scopedIcaHostKeeper := capabilityKeeper.ScopeToModule(icahosttypes.SubModuleName)
	app.ScopedIcaHostKeeper = &scopedIcaHostKeeper

	// Applications that wish to enforce statically created ScopedKeepers should call `Seal` after creating
	// their scoped modules in `NewApp` with `ScopeToModule`
	capabilityKeeper.Seal()

	accountKeeper := authkeeper.NewAccountKeeper(
		appCodec,
		keys[authtypes.StoreKey],
		app.GetSubspace(authtypes.ModuleName),
		authtypes.ProtoBaseAccount,
		maccPerms,
		Bech32Prefix,
	)
	app.AccountKeeper = &accountKeeper

	authzKeeper := authzkeeper.NewKeeper(
		keys[authzkeeper.StoreKey],
		appCodec,
		app.MsgServiceRouter(),
		accountKeeper,
	)
	app.AuthzKeeper = &authzKeeper

	bankKeeper := bankkeeper.NewBaseKeeper(
		appCodec,
		keys[banktypes.StoreKey],
		accountKeeper,
		app.GetSubspace(banktypes.ModuleName),
		app.BlockedAddrs(),
	)
	app.BankKeeper = &bankKeeper

	stakingKeeper := stakingkeeper.NewKeeper(
		appCodec,
		keys[stakingtypes.StoreKey],
		accountKeeper,
		bankKeeper,
		app.GetSubspace(stakingtypes.ModuleName),
	)
	app.StakingKeeper = &stakingKeeper

	distrKeeper := distrkeeper.NewKeeper(
		appCodec,
		keys[distrtypes.StoreKey],
		app.GetSubspace(distrtypes.ModuleName),
		accountKeeper,
		bankKeeper,
		stakingKeeper,
		authtypes.FeeCollectorName,
	)
	app.DistrKeeper = &distrKeeper

	slashingKeeper := slashingkeeper.NewKeeper(
		appCodec,
		keys[slashingtypes.StoreKey],
		&stakingKeeper,
		app.GetSubspace(slashingtypes.ModuleName),
	)
	app.SlashingKeeper = &slashingKeeper

	upgradeKeeper := upgradekeeper.NewKeeper(
		skipUpgradeHeights,
		keys[upgradetypes.StoreKey],
		appCodec,
		homePath,
		&bApp,
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
	)
	app.UpgradeKeeper = &upgradeKeeper

	ibcKeeper := *ibckeeper.NewKeeper(
		appCodec,
		keys[ibchost.StoreKey],
		app.GetSubspace(ibchost.ModuleName),
		stakingKeeper,
		upgradeKeeper,
		scopedIBCKeeper,
	)
	app.IbcKeeper = &ibcKeeper

	ibcTransferKeeper := ibctransferkeeper.NewKeeper(
		appCodec, keys[ibctransfertypes.StoreKey], app.GetSubspace(ibctransfertypes.ModuleName),
		ibcKeeper.ChannelKeeper, ibcKeeper.ChannelKeeper, &ibcKeeper.PortKeeper,
		accountKeeper, bankKeeper, scopedTransferKeeper,
	)
	app.IbcTransferKeeper = &ibcTransferKeeper

	bech32IbcKeeper := *bech32ibckeeper.NewKeeper(
		ibcKeeper.ChannelKeeper, appCodec, keys[bech32ibctypes.StoreKey],
		ibcTransferKeeper,
	)
	app.Bech32IbcKeeper = &bech32IbcKeeper

	icaHostKeeper := icahostkeeper.NewKeeper(
		appCodec, keys[icahosttypes.StoreKey], app.GetSubspace(icahosttypes.SubModuleName),
		ibcKeeper.ChannelKeeper, ibcKeeper.ChannelKeeper, &ibcKeeper.PortKeeper,
		accountKeeper, scopedIcaHostKeeper, app.MsgServiceRouter(),
	)
	app.IcaHostKeeper = &icaHostKeeper

	mintKeeper := mintkeeper.NewKeeper(
		appCodec,
		keys[minttypes.StoreKey],
		app.GetSubspace(minttypes.ModuleName),
		stakingKeeper,
		accountKeeper,
		bankKeeper,
		authtypes.FeeCollectorName,
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

	crisisKeeper := crisiskeeper.NewKeeper(
		app.GetSubspace(crisistypes.ModuleName),
		invCheckPeriod,
		bankKeeper,
		authtypes.FeeCollectorName,
	)
	app.CrisisKeeper = &crisisKeeper

	govRouter := govv1beta1.NewRouter()
	govRouter.AddRoute(govtypes.RouterKey, govv1beta1.ProposalHandler).
		AddRoute(paramsproposal.RouterKey, params.NewParamChangeProposalHandler(paramsKeeper)).
		AddRoute(distrtypes.RouterKey, distr.NewCommunityPoolSpendProposalHandler(distrKeeper)).
		AddRoute(upgradetypes.RouterKey, upgrade.NewSoftwareUpgradeProposalHandler(upgradeKeeper)).
		AddRoute(ibcclienttypes.RouterKey, ibcclient.NewClientProposalHandler(ibcKeeper.ClientKeeper)).
		AddRoute(gravitytypes.RouterKey, keeper.NewGravityProposalHandler(gravityKeeper)).
		AddRoute(bech32ibctypes.RouterKey, bech32ibc.NewBech32IBCProposalHandler(*app.Bech32IbcKeeper))

	govConfig := govtypes.DefaultConfig()
	govKeeper := govkeeper.NewKeeper(
		appCodec,
		keys[govtypes.StoreKey],
		app.GetSubspace(govtypes.ModuleName),
		accountKeeper,
		bankKeeper,
		stakingKeeper,
		govRouter,
		app.MsgServiceRouter(),
		govConfig,
	)
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
		keys[evidencetypes.StoreKey],
		&stakingKeeper,
		slashingKeeper,
	)
	app.EvidenceKeeper = &evidenceKeeper

	groupConfig := group.DefaultConfig()
	groupKeeper := groupkeeper.NewKeeper(keys[group.StoreKey], appCodec, app.MsgServiceRouter(), app.AccountKeeper, groupConfig)
	app.GroupKeeper = &groupKeeper

	var skipGenesisInvariants = cast.ToBool(appOpts.Get(crisis.FlagSkipGenesisInvariants))

	app.registerStoreLoaders()

	mm := *module.NewManager(
		genutil.NewAppModule(
			accountKeeper,
			stakingKeeper,
			bApp.DeliverTx,
			encodingConfig.TxConfig,
		),
		auth.NewAppModule(
			appCodec,
			accountKeeper,
			nil,
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
		),
		capability.NewAppModule(
			appCodec,
			capabilityKeeper,
		),
		crisis.NewAppModule(
			&crisisKeeper,
			skipGenesisInvariants,
		),
		gov.NewAppModule(
			appCodec,
			govKeeper,
			accountKeeper,
			bankKeeper,
		),
		mint.NewAppModule(
			appCodec,
			mintKeeper,
			accountKeeper,
			nil,
		),
		slashing.NewAppModule(
			appCodec,
			slashingKeeper,
			accountKeeper,
			bankKeeper,
			stakingKeeper,
		),
		distr.NewAppModule(
			appCodec,
			distrKeeper,
			accountKeeper,
			bankKeeper,
			stakingKeeper,
		),
		staking.NewAppModule(appCodec,
			stakingKeeper,
			accountKeeper,
			bankKeeper,
		),
		upgrade.NewAppModule(upgradeKeeper),
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
	)
	app.mm = &mm

	// NOTE: capability module's BeginBlocker must come before any modules using capabilities (e.g. IBC)
	mm.SetOrderBeginBlockers(
		upgradetypes.ModuleName,
		capabilitytypes.ModuleName,
		minttypes.ModuleName,
		distrtypes.ModuleName,
		slashingtypes.ModuleName,
		evidencetypes.ModuleName,
		stakingtypes.ModuleName,
		ibchost.ModuleName,
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
	)
	mm.SetOrderEndBlockers(
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
		ibchost.ModuleName,
		banktypes.ModuleName,
		authtypes.ModuleName,
		vestingtypes.ModuleName,
		ibctransfertypes.ModuleName,
		bech32ibctypes.ModuleName,
		genutiltypes.ModuleName,
		authz.ModuleName,
		paramstypes.ModuleName,
		group.ModuleName,
	)
	mm.SetOrderInitGenesis(
		capabilitytypes.ModuleName,
		authtypes.ModuleName,
		banktypes.ModuleName,
		distrtypes.ModuleName,
		stakingtypes.ModuleName,
		slashingtypes.ModuleName,
		govtypes.ModuleName,
		minttypes.ModuleName,
		upgradetypes.ModuleName,
		ibchost.ModuleName,
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
	)

	mm.RegisterInvariants(&crisisKeeper)
	mm.RegisterRoutes(app.Router(), app.QueryRouter(), encodingConfig.Amino)
	configurator := module.NewConfigurator(appCodec, app.MsgServiceRouter(), app.GRPCQueryRouter())
	app.configurator = &configurator
	mm.RegisterServices(*app.configurator)

	sm := *module.NewSimulationManager(
		auth.NewAppModule(appCodec, accountKeeper, authsims.RandomGenesisAccounts),
		bank.NewAppModule(appCodec, bankKeeper, accountKeeper),
		capability.NewAppModule(appCodec, capabilityKeeper),
		gov.NewAppModule(appCodec, govKeeper, accountKeeper, bankKeeper),
		mint.NewAppModule(appCodec, mintKeeper, accountKeeper, nil),
		staking.NewAppModule(appCodec, stakingKeeper, accountKeeper, bankKeeper),
		distr.NewAppModule(appCodec, distrKeeper, accountKeeper, bankKeeper, stakingKeeper),
		slashing.NewAppModule(appCodec, slashingKeeper, accountKeeper, bankKeeper, stakingKeeper),
		params.NewAppModule(paramsKeeper),
		evidence.NewAppModule(evidenceKeeper),
		ibc.NewAppModule(&ibcKeeper),
		ibcTransferAppModule,
	)
	app.sm = &sm

	sm.RegisterStoreDecoders()

	app.MountKVStores(keys)
	app.MountTransientStores(tKeys)
	app.MountMemoryStores(memKeys)

	app.SetInitChainer(app.InitChainer)
	app.SetBeginBlocker(app.BeginBlocker)

	app.SetEndBlocker(app.EndBlocker)

	app.setAnteHandler(encodingConfig)
	app.setPostHandler()

	app.registerUpgradeHandlers()

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

func (app *Gravity) setAnteHandler(encodingConfig gravityparams.EncodingConfig) {
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
	ah, err := ante.NewAnteHandler(options, app.GravityKeeper, app.AccountKeeper, app.BankKeeper, nil, app.IbcKeeper, app.AppCodec, gravityconfig.GravityEvmChainID)
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
	return config.Marshaler, config.Amino
}

// Name returns the name of the App
func (app *Gravity) Name() string { return app.BaseApp.Name() }

// BeginBlocker application updates every begin block
func (app *Gravity) BeginBlocker(ctx sdk.Context, req abci.RequestBeginBlock) abci.ResponseBeginBlock {
	out := app.mm.BeginBlock(ctx, req)
	firstBlock.Do(func() { // Run the startup firstBeginBlocker assertions only once
		app.firstBeginBlocker(ctx)
	})

	return out
}

// Perform necessary checks at the start of this node's first BeginBlocker execution
// Note: This should ONLY be called once, it should be called at the top of BeginBlocker guarded by firstBlock
func (app *Gravity) firstBeginBlocker(ctx sdk.Context) {
	app.assertBech32PrefixMatches(ctx)
	app.assertNativeTokenMatchesConstant(ctx)
	app.assertNativeTokenIsNonAuctionable(ctx)
}

// EndBlocker application updates every end block
func (app *Gravity) EndBlocker(ctx sdk.Context, req abci.RequestEndBlock) abci.ResponseEndBlock {
	return app.mm.EndBlock(ctx, req)
}

// InitChainer application update at chain initialization
func (app *Gravity) InitChainer(ctx sdk.Context, req abci.RequestInitChain) abci.ResponseInitChain {
	var genesisState simapp.GenesisState
	if err := tmjson.Unmarshal(req.AppStateBytes, &genesisState); err != nil {
		panic(err)
	}

	app.UpgradeKeeper.SetModuleVersionMap(ctx, app.mm.GetVersionMap())

	return app.mm.InitGenesis(ctx, app.AppCodec, genesisState)
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
func (app *Gravity) LegacyAmino() *codec.LegacyAmino {
	return app.legacyAmino
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
	tmservice.RegisterGRPCGatewayRoutes(clientCtx, apiSvr.GRPCGatewayRouter)
	nodeservice.RegisterGRPCGatewayRoutes(clientCtx, apiSvr.GRPCGatewayRouter)
	ModuleBasics.RegisterGRPCGatewayRoutes(clientCtx, apiSvr.GRPCGatewayRouter)

	if apiConfig.Swagger {
		RegisterSwaggerAPI(clientCtx, apiSvr.Router)
	}
}

// RegisterTxService implements the Application.RegisterTxService method.
func (app *Gravity) RegisterTxService(clientCtx client.Context) {
	authtx.RegisterTxService(app.BaseApp.GRPCQueryRouter(), clientCtx, app.BaseApp.Simulate, app.InterfaceRegistry)
}

// RegisterTendermintService implements the Application.RegisterTendermintService method.
func (app *Gravity) RegisterTendermintService(clientCtx client.Context) {
	tmservice.RegisterTendermintService(clientCtx, app.BaseApp.GRPCQueryRouter(), app.InterfaceRegistry, app.Query)
}

func (app *Gravity) RegisterNodeService(clientCtx client.Context) {
	nodeservice.RegisterNodeService(clientCtx, app.GRPCQueryRouter())
}

// RegisterSwaggerAPI registers swagger route with API Server
// TODO: build the custom gravity swagger files and add here?
func RegisterSwaggerAPI(ctx client.Context, rtr *mux.Router) {
	statikFS, err := fs.New()
	if err != nil {
		panic(err)
	}

	staticServer := http.FileServer(statikFS)
	rtr.PathPrefix("/swagger/").Handler(http.StripPrefix("/swagger/", staticServer))
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
	paramsKeeper.Subspace(ibchost.ModuleName)
	paramsKeeper.Subspace(icahosttypes.SubModuleName)

	return paramsKeeper
}

// Registers handlers for all our upgrades
func (app *Gravity) registerUpgradeHandlers() {
	upgrades.RegisterUpgradeHandlers(
		app.mm, app.configurator, app.AccountKeeper, app.BankKeeper, app.Bech32IbcKeeper, app.DistrKeeper,
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
