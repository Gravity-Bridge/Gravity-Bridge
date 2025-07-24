package keeper

import (
	"bytes"
	"testing"
	"time"

	"cosmossdk.io/log"
	sdkmath "cosmossdk.io/math"
	"cosmossdk.io/store"
	"cosmossdk.io/store/metrics"
	storetypes "cosmossdk.io/store/types"
	"cosmossdk.io/x/evidence"
	"cosmossdk.io/x/upgrade"
	upgradekeeper "cosmossdk.io/x/upgrade/keeper"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	tmversion "github.com/cometbft/cometbft/proto/tendermint/version"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	ccrypto "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	"github.com/cosmos/cosmos-sdk/std"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/cosmos/cosmos-sdk/types/tx"
	"github.com/cosmos/cosmos-sdk/x/auth"
	authcodec "github.com/cosmos/cosmos-sdk/x/auth/codec"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	"github.com/cosmos/cosmos-sdk/x/auth/migrations/legacytx"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/cosmos/cosmos-sdk/x/auth/vesting"
	"github.com/cosmos/cosmos-sdk/x/bank"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/cosmos/cosmos-sdk/x/crisis"
	"github.com/cosmos/cosmos-sdk/x/distribution"
	distrkeeper "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	"github.com/cosmos/cosmos-sdk/x/genutil"
	"github.com/cosmos/cosmos-sdk/x/gov"
	govclient "github.com/cosmos/cosmos-sdk/x/gov/client"
	govkeeper "github.com/cosmos/cosmos-sdk/x/gov/keeper"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	govv1beta1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1beta1"
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
	"github.com/cosmos/ibc-go/modules/capability"
	capabilitykeeper "github.com/cosmos/ibc-go/modules/capability/keeper"
	capabilitytypes "github.com/cosmos/ibc-go/modules/capability/types"
	gethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"

	ibctransferkeeper "github.com/cosmos/ibc-go/v8/modules/apps/transfer/keeper"
	ibctransfertypes "github.com/cosmos/ibc-go/v8/modules/apps/transfer/types"
	ibcexported "github.com/cosmos/ibc-go/v8/modules/core/exported"
	ibckeeper "github.com/cosmos/ibc-go/v8/modules/core/keeper"

	bech32ibckeeper "github.com/Gravity-Bridge/Gravity-Bridge/module/x/bech32ibc/keeper"
	bech32ibctypes "github.com/Gravity-Bridge/Gravity-Bridge/module/x/bech32ibc/types"

	ethermintcryptocodec "github.com/evmos/ethermint/crypto/codec"
	ethermintcodec "github.com/evmos/ethermint/encoding/codec"
	etherminttypes "github.com/evmos/ethermint/types"

	gravityparams "github.com/Gravity-Bridge/Gravity-Bridge/module/app/params"
	auctionkeeper "github.com/Gravity-Bridge/Gravity-Bridge/module/x/auction/keeper"
	auctiontypes "github.com/Gravity-Bridge/Gravity-Bridge/module/x/auction/types"
	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
)

var (
	// ModuleBasics is a mock module basic manager for testing
	ModuleBasics = module.NewBasicManager(
		auth.AppModuleBasic{},
		genutil.AppModuleBasic{},
		bank.AppModuleBasic{},
		capability.AppModuleBasic{},
		staking.AppModuleBasic{},
		mint.AppModuleBasic{},
		distribution.AppModuleBasic{},
		gov.NewAppModuleBasic(
			[]govclient.ProposalHandler{paramsclient.ProposalHandler},
		),
		params.AppModuleBasic{},
		crisis.AppModuleBasic{},
		slashing.AppModuleBasic{},
		upgrade.AppModuleBasic{},
		evidence.AppModuleBasic{},
		vesting.AppModuleBasic{},
	)
)

var (
	// ConsPrivKeys generate ed25519 ConsPrivKeys to be used for validator operator keys
	ConsPrivKeys = []ccrypto.PrivKey{
		ed25519.GenPrivKey(),
		ed25519.GenPrivKey(),
		ed25519.GenPrivKey(),
		ed25519.GenPrivKey(),
		ed25519.GenPrivKey(),
	}

	// ConsPubKeys holds the consensus public keys to be used for validator operator keys
	ConsPubKeys = []ccrypto.PubKey{
		ConsPrivKeys[0].PubKey(),
		ConsPrivKeys[1].PubKey(),
		ConsPrivKeys[2].PubKey(),
		ConsPrivKeys[3].PubKey(),
		ConsPrivKeys[4].PubKey(),
	}

	// AccPrivKeys generate secp256k1 pubkeys to be used for account pub keys
	AccPrivKeys = []ccrypto.PrivKey{
		secp256k1.GenPrivKey(),
		secp256k1.GenPrivKey(),
		secp256k1.GenPrivKey(),
		secp256k1.GenPrivKey(),
		secp256k1.GenPrivKey(),
	}

	// AccPubKeys holds the pub keys for the account keys
	AccPubKeys = []ccrypto.PubKey{
		AccPrivKeys[0].PubKey(),
		AccPrivKeys[1].PubKey(),
		AccPrivKeys[2].PubKey(),
		AccPrivKeys[3].PubKey(),
		AccPrivKeys[4].PubKey(),
	}

	// AccAddrs holds the sdk.AccAddresses
	AccAddrs = []sdk.AccAddress{
		sdk.AccAddress(AccPubKeys[0].Address()),
		sdk.AccAddress(AccPubKeys[1].Address()),
		sdk.AccAddress(AccPubKeys[2].Address()),
		sdk.AccAddress(AccPubKeys[3].Address()),
		sdk.AccAddress(AccPubKeys[4].Address()),
	}

	// ValAddrs holds the sdk.ValAddresses
	ValAddrs = []sdk.ValAddress{
		sdk.ValAddress(AccPubKeys[0].Address()),
		sdk.ValAddress(AccPubKeys[1].Address()),
		sdk.ValAddress(AccPubKeys[2].Address()),
		sdk.ValAddress(AccPubKeys[3].Address()),
		sdk.ValAddress(AccPubKeys[4].Address()),
	}

	// AccPubKeys holds the pub keys for the account keys
	OrchPubKeys = []ccrypto.PubKey{
		OrchPrivKeys[0].PubKey(),
		OrchPrivKeys[1].PubKey(),
		OrchPrivKeys[2].PubKey(),
		OrchPrivKeys[3].PubKey(),
		OrchPrivKeys[4].PubKey(),
	}

	// Orchestrator private keys
	OrchPrivKeys = []ccrypto.PrivKey{
		secp256k1.GenPrivKey(),
		secp256k1.GenPrivKey(),
		secp256k1.GenPrivKey(),
		secp256k1.GenPrivKey(),
		secp256k1.GenPrivKey(),
	}

	// AccAddrs holds the sdk.AccAddresses
	OrchAddrs = []sdk.AccAddress{
		sdk.AccAddress(OrchPubKeys[0].Address()),
		sdk.AccAddress(OrchPubKeys[1].Address()),
		sdk.AccAddress(OrchPubKeys[2].Address()),
		sdk.AccAddress(OrchPubKeys[3].Address()),
		sdk.AccAddress(OrchPubKeys[4].Address()),
	}

	// TODO: generate the eth priv keys here and
	// derive the address from them.

	// EthAddrs holds etheruem addresses
	EthAddrs = []gethcommon.Address{
		gethcommon.BytesToAddress(bytes.Repeat([]byte{byte(1)}, 20)),
		gethcommon.BytesToAddress(bytes.Repeat([]byte{byte(2)}, 20)),
		gethcommon.BytesToAddress(bytes.Repeat([]byte{byte(3)}, 20)),
		gethcommon.BytesToAddress(bytes.Repeat([]byte{byte(4)}, 20)),
		gethcommon.BytesToAddress(bytes.Repeat([]byte{byte(5)}, 20)),
	}

	// TokenContractAddrs holds example token contract addresses
	TokenContractAddrs = []string{
		"0x6b175474e89094c44da98b954eedeac495271d0f", // DAI
		"0x0bc529c00c6401aef6d220be8c6ea1667f6ad93e", // YFI
		"0x1f9840a85d5af5bf1d1762f925bdaddc4201f984", // UNI
		"0xc00e94cb662c3520282e6f5717214004a7f26888", // COMP
		"0xc011a73ee8576fb46f5e1c5751ca3b9fe0af2a6f", // SNX
	}

	// InitTokens holds the number of tokens to initialize an account with
	InitTokens = sdk.TokensFromConsensusPower(110, sdk.DefaultPowerReduction)

	// InitCoins holds the number of coins to initialize an account with
	InitCoins = sdk.NewCoins(sdk.NewCoin(TestingStakeParams.BondDenom, InitTokens))

	// StakingAmount holds the staking power to start a validator with
	StakingAmount = sdk.TokensFromConsensusPower(10, sdk.DefaultPowerReduction)

	// StakingCoins holds the staking coins to start a validator with
	StakingCoins = sdk.NewCoins(sdk.NewCoin(TestingStakeParams.BondDenom, StakingAmount))

	// TestingStakeParams is a set of staking params for testing
	TestingStakeParams = stakingtypes.Params{
		UnbondingTime:     100,
		MaxValidators:     10,
		MaxEntries:        10,
		HistoricalEntries: 10000,
		BondDenom:         "stake",
		MinCommissionRate: sdkmath.LegacyNewDecWithPrec(5, 2), // 5%
	}

	// TestingGravityParams is a set of gravity params for testing
	TestingGravityParams = types.Params{
		GravityId:                    "testgravityid",
		ContractSourceHash:           "62328f7bc12efb28f86111d08c29b39285680a906ea0e524e0209d6f6657b713",
		BridgeEthereumAddress:        "0x8858eeb3dfffa017d4bce9801d340d36cf895ccf",
		BridgeChainId:                11,
		SignedValsetsWindow:          10,
		SignedBatchesWindow:          10,
		SignedLogicCallsWindow:       10,
		TargetBatchTimeout:           60001,
		AverageBlockTime:             5000,
		AverageEthereumBlockTime:     15000,
		SlashFractionValset:          sdkmath.LegacyNewDecWithPrec(1, 2),
		SlashFractionBatch:           sdkmath.LegacyNewDecWithPrec(1, 2),
		SlashFractionLogicCall:       sdkmath.LegacyDec{},
		UnbondSlashingValsetsWindow:  15,
		SlashFractionBadEthSignature: sdkmath.LegacyNewDecWithPrec(1, 2),
		ValsetReward:                 sdk.Coin{Denom: "", Amount: sdkmath.ZeroInt()},
		BridgeActive:                 true,
		EthereumBlacklist:            []string{},
		MinChainFeeBasisPoints:       0,
		ChainFeeAuctionPoolFraction:  sdkmath.LegacyNewDecWithPrec(50, 2), // 50%
	}
)

// TestInput stores the various keepers required to test gravity
type TestInput struct {
	GravityKeeper     Keeper
	AccountKeeper     authkeeper.AccountKeeper
	StakingKeeper     stakingkeeper.Keeper
	SlashingKeeper    slashingkeeper.Keeper
	DistKeeper        distrkeeper.Keeper
	BankKeeper        bankkeeper.BaseKeeper
	GovKeeper         govkeeper.Keeper
	IbcKeeper         ibckeeper.Keeper
	IbcTransferKeeper ibctransferkeeper.Keeper
	MintKeeper        mintkeeper.Keeper
	AuctionKeeper     auctionkeeper.Keeper
	Context           sdk.Context
	Marshaler         codec.Codec
	LegacyAmino       *codec.LegacyAmino
	EncodingConfig    gravityparams.EncodingConfig
	GravityStoreKey   *storetypes.KVStoreKey
}

// SetupFiveValChain does all the initialization for a 5 Validator chain using the keys here
func SetupFiveValChain(t *testing.T) (TestInput, sdk.Context) {
	t.Helper()
	input := CreateTestEnv(t)

	// Set the params for our modules
	input.StakingKeeper.SetParams(input.Context, TestingStakeParams)

	// Initialize each of the validators
	sMsgServer := stakingkeeper.NewMsgServerImpl(&input.StakingKeeper)
	// sh := staking.NewHandler(input.StakingKeeper)
	for i := range []int{0, 1, 2, 3, 4} {

		// Initialize the account for the key
		acc := input.AccountKeeper.NewAccount(
			input.Context,
			authtypes.NewBaseAccount(AccAddrs[i], AccPubKeys[i], uint64(i), 0),
		)

		// Set the balance for the account
		require.NoError(t, input.BankKeeper.MintCoins(input.Context, types.ModuleName, InitCoins))
		require.NoError(t, input.BankKeeper.SendCoinsFromModuleToAccount(input.Context, types.ModuleName, acc.GetAddress(), InitCoins))

		// Set the account in state
		input.AccountKeeper.SetAccount(input.Context, acc)

		// Create a validator for that account using some of the tokens in the account
		// and the staking handler
		_, err := sMsgServer.CreateValidator(
			input.Context,
			NewTestMsgCreateValidator(ValAddrs[i], ConsPubKeys[i], StakingAmount),
		)

		// Return error if one exists
		require.NoError(t, err)
	}

	// Run the staking endblocker to ensure valset is correct in state
	input.StakingKeeper.EndBlocker(input.Context)

	// Register eth addresses and orchestrator address for each validator
	for i, addr := range ValAddrs {
		ethAddr, err := types.NewEthAddress(EthAddrs[i].String())
		if err != nil {
			panic("found invalid address in EthAddrs")
		}
		input.GravityKeeper.SetEthAddressForValidator(input.Context, addr, *ethAddr)

		input.GravityKeeper.SetOrchestratorValidator(input.Context, addr, OrchAddrs[i])
	}

	// Return the test input
	return input, input.Context
}

// SetupTestChain sets up a test environment with the provided validator voting weights
func SetupTestChain(t *testing.T, weights []uint64, setDelegateAddresses bool) (TestInput, sdk.Context) {
	t.Helper()
	input := CreateTestEnv(t)

	// Set the params for our modules
	TestingStakeParams.MaxValidators = 100
	input.StakingKeeper.SetParams(input.Context, TestingStakeParams)

	// Initialize each of the validators
	sMsgServer := stakingkeeper.NewMsgServerImpl(&input.StakingKeeper)
	for i, weight := range weights {
		consPrivKey := ed25519.GenPrivKey()
		consPubKey := consPrivKey.PubKey()
		valPrivKey := secp256k1.GenPrivKey()
		valPubKey := valPrivKey.PubKey()
		valAddr := sdk.ValAddress(valPubKey.Address())
		accAddr := sdk.AccAddress(valPubKey.Address())

		// Initialize the account for the key
		acc := input.AccountKeeper.NewAccount(
			input.Context,
			authtypes.NewBaseAccount(accAddr, valPubKey, uint64(i), 0),
		)

		// Set the balance for the account
		weightCoins := sdk.NewCoins(sdk.NewInt64Coin(TestingStakeParams.BondDenom, int64(weight)))
		require.NoError(t, input.BankKeeper.MintCoins(input.Context, types.ModuleName, weightCoins))
		require.NoError(t, input.BankKeeper.SendCoinsFromModuleToAccount(input.Context, types.ModuleName, accAddr, weightCoins))

		// Set the account in state
		input.AccountKeeper.SetAccount(input.Context, acc)

		// Create a validator for that account using some of the tokens in the account
		// and the staking handler
		_, err := sMsgServer.CreateValidator(
			input.Context,
			NewTestMsgCreateValidator(valAddr, consPubKey, sdkmath.NewIntFromUint64(weight)),
		)
		require.NoError(t, err)

		// Run the staking endblocker to ensure valset is correct in state
		input.StakingKeeper.EndBlocker(input.Context)

		if setDelegateAddresses {
			// set the delegate addresses for this key
			ethAddr, err := types.NewEthAddress(gethcommon.BytesToAddress(bytes.Repeat([]byte{byte(i)}, 20)).String())
			if err != nil {
				panic("found invalid address in EthAddrs")
			}
			input.GravityKeeper.SetEthAddressForValidator(input.Context, valAddr, *ethAddr)
			input.GravityKeeper.SetOrchestratorValidator(input.Context, valAddr, accAddr)

			// increase block height by 100 blocks
			input.Context = input.Context.WithBlockHeight(input.Context.BlockHeight() + 100)

			// Run the staking endblocker to ensure valset is correct in state
			input.StakingKeeper.EndBlocker(input.Context)

			// set a request every time.
			input.GravityKeeper.SetValsetRequest(input.Context)
		}

	}

	// some inputs can cause the validator creation ot not work, this checks that
	// everything was successful
	validators, err := input.StakingKeeper.GetBondedValidatorsByPower(input.Context)
	require.NoError(t, err)
	require.Equal(t, len(weights), len(validators))

	// Return the test input
	return input, input.Context
}

// CreateTestEnv creates the keeper testing environment for gravity
func CreateTestEnv(t *testing.T) TestInput {
	t.Helper()

	// Initialize store keys
	gravityKey := storetypes.NewKVStoreKey(types.StoreKey)
	keyAcc := storetypes.NewKVStoreKey(authtypes.StoreKey)
	keyStaking := storetypes.NewKVStoreKey(stakingtypes.StoreKey)
	keyBank := storetypes.NewKVStoreKey(banktypes.StoreKey)
	keyDistro := storetypes.NewKVStoreKey(distrtypes.StoreKey)
	keyParams := storetypes.NewKVStoreKey(paramstypes.StoreKey)
	tkeyParams := storetypes.NewTransientStoreKey(paramstypes.TStoreKey)
	keyGov := storetypes.NewKVStoreKey(govtypes.StoreKey)
	keySlashing := storetypes.NewKVStoreKey(slashingtypes.StoreKey)
	keyCapability := storetypes.NewKVStoreKey(capabilitytypes.StoreKey)
	keyUpgrade := storetypes.NewKVStoreKey(upgradetypes.StoreKey)
	keyIbc := storetypes.NewKVStoreKey(ibcexported.StoreKey)
	keyIbcTransfer := storetypes.NewKVStoreKey(ibctransfertypes.StoreKey)
	keyBech32Ibc := storetypes.NewKVStoreKey(bech32ibctypes.StoreKey)
	keyMint := storetypes.NewKVStoreKey(minttypes.StoreKey)
	keyAuction := storetypes.NewKVStoreKey(auctiontypes.StoreKey)

	// Initialize memory database and mount stores on it
	db := dbm.NewMemDB()
	logger := log.NewNopLogger()
	ms := store.NewCommitMultiStore(db, logger, metrics.NewNoOpMetrics())
	ms.MountStoreWithDB(gravityKey, storetypes.StoreTypeIAVL, db)
	ms.MountStoreWithDB(keyAcc, storetypes.StoreTypeIAVL, db)
	ms.MountStoreWithDB(keyParams, storetypes.StoreTypeIAVL, db)
	ms.MountStoreWithDB(keyStaking, storetypes.StoreTypeIAVL, db)
	ms.MountStoreWithDB(keyBank, storetypes.StoreTypeIAVL, db)
	ms.MountStoreWithDB(keyDistro, storetypes.StoreTypeIAVL, db)
	ms.MountStoreWithDB(tkeyParams, storetypes.StoreTypeTransient, db)
	ms.MountStoreWithDB(keyGov, storetypes.StoreTypeIAVL, db)
	ms.MountStoreWithDB(keySlashing, storetypes.StoreTypeIAVL, db)
	ms.MountStoreWithDB(keyCapability, storetypes.StoreTypeIAVL, db)
	ms.MountStoreWithDB(keyUpgrade, storetypes.StoreTypeIAVL, db)
	ms.MountStoreWithDB(keyIbc, storetypes.StoreTypeIAVL, db)
	ms.MountStoreWithDB(keyIbcTransfer, storetypes.StoreTypeIAVL, db)
	ms.MountStoreWithDB(keyBech32Ibc, storetypes.StoreTypeIAVL, db)
	ms.MountStoreWithDB(keyMint, storetypes.StoreTypeIAVL, db)
	ms.MountStoreWithDB(keyAuction, storetypes.StoreTypeIAVL, db)
	err := ms.LoadLatestVersion()
	require.Nil(t, err)

	// Create sdk.Context
	ctx := sdk.NewContext(ms, tmproto.Header{
		Version: tmversion.Consensus{
			Block: 0,
			App:   0,
		},
		ChainID: "gravity-test-1",
		Height:  1234567,
		Time:    time.Date(2020, time.April, 22, 12, 0, 0, 0, time.UTC),
		LastBlockId: tmproto.BlockID{
			Hash: []byte{},
			PartSetHeader: tmproto.PartSetHeader{
				Total: 0,
				Hash:  []byte{},
			},
		},
		LastCommitHash:     []byte{},
		DataHash:           []byte{},
		ValidatorsHash:     []byte{},
		NextValidatorsHash: []byte{},
		ConsensusHash:      []byte{},
		AppHash:            []byte{},
		LastResultsHash:    []byte{},
		EvidenceHash:       []byte{},
		ProposerAddress:    []byte{},
	}, false, logger)

	encodingConfig := MakeTestEncodingConfig()
	appCodec := encodingConfig.Marshaler
	legacytx.RegressionTestingAminoCodec = encodingConfig.Amino

	paramsKeeper := paramskeeper.NewKeeper(appCodec, encodingConfig.Amino, keyParams, tkeyParams)
	paramsKeeper.Subspace(authtypes.ModuleName)
	paramsKeeper.Subspace(banktypes.ModuleName)
	paramsKeeper.Subspace(stakingtypes.ModuleName)
	paramsKeeper.Subspace(distrtypes.ModuleName)
	paramsKeeper.Subspace(govtypes.ModuleName)
	paramsKeeper.Subspace(types.DefaultParamspace)
	paramsKeeper.Subspace(slashingtypes.ModuleName)
	paramsKeeper.Subspace(ibcexported.ModuleName)
	paramsKeeper.Subspace(ibctransfertypes.ModuleName)
	paramsKeeper.Subspace(bech32ibctypes.ModuleName)
	paramsKeeper.Subspace(minttypes.ModuleName)
	paramsKeeper.Subspace(auctiontypes.ModuleName)

	// this is also used to initialize module accounts for all the map keys
	maccPerms := map[string][]string{
		authtypes.FeeCollectorName:          nil,
		distrtypes.ModuleName:               nil,
		stakingtypes.BondedPoolName:         {authtypes.Burner, authtypes.Staking},
		stakingtypes.NotBondedPoolName:      {authtypes.Burner, authtypes.Staking},
		govtypes.ModuleName:                 {authtypes.Burner},
		types.ModuleName:                    {authtypes.Minter, authtypes.Burner},
		ibctransfertypes.ModuleName:         {authtypes.Minter, authtypes.Burner},
		minttypes.ModuleName:                {authtypes.Minter, authtypes.Burner},
		auctiontypes.AuctionPoolAccountName: nil,
	}

	govAuthority := authtypes.NewModuleAddress(govtypes.ModuleName).String()
	accountKeeper := authkeeper.NewAccountKeeper(
		appCodec,
		runtime.NewKVStoreService(keyAcc), // target store
		authtypes.ProtoBaseAccount,        // prototype
		maccPerms,
		authcodec.NewBech32Codec("gravity"),
		"gravity",
		govAuthority,
	)
	accountParams := authtypes.DefaultParams()
	accountKeeper.Params.Set(ctx, accountParams)

	blockedAddr := make(map[string]bool, len(maccPerms))
	for acc := range maccPerms {
		blockedAddr[authtypes.NewModuleAddress(acc).String()] = true
	}
	bankKeeper := bankkeeper.NewBaseKeeper(
		appCodec,
		runtime.NewKVStoreService(keyBank),
		accountKeeper,
		blockedAddr,
		govAuthority,
		logger,
	)
	bankKeeper.SetParams(ctx, banktypes.Params{
		SendEnabled:        []*banktypes.SendEnabled{},
		DefaultSendEnabled: true,
	})

	stakingKeeper := stakingkeeper.NewKeeper(
		appCodec,
		runtime.NewKVStoreService(keyStaking),
		accountKeeper,
		bankKeeper,
		govAuthority,
		authcodec.NewBech32Codec("gravityvaloper"),
		authcodec.NewBech32Codec("gravityconsaddr"),
	)
	stakingKeeper.SetParams(ctx, TestingStakeParams)

	distKeeper := distrkeeper.NewKeeper(
		appCodec,
		runtime.NewKVStoreService(keyDistro),
		accountKeeper,
		bankKeeper,
		stakingKeeper,
		authtypes.FeeCollectorName,
		govAuthority,
	)
	distKeeper.Params.Set(ctx, distrtypes.DefaultParams())

	// set genesis items required for distribution
	distKeeper.FeePool.Set(ctx, distrtypes.InitialFeePool())

	// set up initial accounts
	for name, perms := range maccPerms {
		mod := authtypes.NewEmptyModuleAccount(name, perms...)
		if name == distrtypes.ModuleName {
			// some big pot to pay out
			amt := sdk.NewCoins(sdk.NewInt64Coin("stake", 500000))
			err = bankKeeper.MintCoins(ctx, types.ModuleName, amt)
			require.NoError(t, err)
			err = bankKeeper.SendCoinsFromModuleToModule(ctx, types.ModuleName, mod.Name, amt)

			// distribution module balance must be outstanding rewards + community pool in order to pass
			// invariants checks, therefore we must add any amount we add to the module balance to the fee pool
			feePool, err := distKeeper.FeePool.Get(ctx)
			require.NoError(t, err)
			newCoins := feePool.CommunityPool.Add(sdk.NewDecCoinsFromCoins(amt...)...)
			feePool.CommunityPool = newCoins
			distKeeper.FeePool.Set(ctx, feePool)

			require.NoError(t, err)
		}
		// if !accountKeeper.HasAccount(ctx, mod.GetAddress()) {
		// accountKeeper.SetModuleAccount(ctx, mod)
		// }
	}

	moduleAcct := accountKeeper.GetModuleAccount(ctx, stakingtypes.BondedPoolName)
	require.NotNil(t, moduleAcct)

	govRouter := govv1beta1.NewRouter().
		AddRoute(paramsproposal.RouterKey, params.NewParamChangeProposalHandler(paramsKeeper)).
		AddRoute(govtypes.RouterKey, govv1beta1.ProposalHandler)

	govConfig := govtypes.DefaultConfig()

	govKeeper := govkeeper.NewKeeper(
		appCodec,
		runtime.NewKVStoreService(keyGov),
		accountKeeper,
		bankKeeper,
		stakingKeeper,
		distKeeper,
		baseapp.NewMsgServiceRouter(),
		govConfig,
		govAuthority,
	)
	govKeeper.SetLegacyRouter(govRouter)

	slashingKeeper := slashingkeeper.NewKeeper(
		appCodec,
		codec.NewLegacyAmino(),
		runtime.NewKVStoreService(keySlashing),
		stakingKeeper,
		govAuthority,
	)

	bApp := *baseapp.NewBaseApp("test", logger, db, encodingConfig.TxConfig.TxDecoder())
	upgradeKeeper := upgradekeeper.NewKeeper(
		make(map[int64]bool),
		runtime.NewKVStoreService(keyUpgrade),
		appCodec,
		"",
		&bApp,
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
	)

	memKeys := storetypes.NewMemoryStoreKeys(capabilitytypes.MemStoreKey)
	capabilityKeeper := *capabilitykeeper.NewKeeper(
		appCodec,
		keyCapability,
		memKeys[capabilitytypes.MemStoreKey],
	)

	scopedIbcKeeper := capabilityKeeper.ScopeToModule(ibcexported.ModuleName)
	ibcKeeper := *ibckeeper.NewKeeper(
		appCodec,
		keyIbc,
		getSubspace(paramsKeeper, ibcexported.ModuleName),
		stakingKeeper,
		upgradeKeeper,
		scopedIbcKeeper,
		govAuthority,
	)

	scopedTransferKeeper := capabilityKeeper.ScopeToModule(ibctransfertypes.ModuleName)
	ibcTransferKeeper := ibctransferkeeper.NewKeeper(
		appCodec, keyIbcTransfer, getSubspace(paramsKeeper, ibctransfertypes.ModuleName),
		ibcKeeper.ChannelKeeper, ibcKeeper.ChannelKeeper, ibcKeeper.PortKeeper,
		accountKeeper, bankKeeper, scopedTransferKeeper, govAuthority,
	)

	bech32IbcKeeper := *bech32ibckeeper.NewKeeper(
		ibcKeeper.ChannelKeeper, appCodec, keyBech32Ibc,
		ibcTransferKeeper,
	)
	// Set the native prefix to the "gravity" value we like in module/config/config.go
	err = bech32IbcKeeper.SetNativeHrp(ctx, sdk.GetConfig().GetBech32AccountAddrPrefix())
	if err != nil {
		panic("Test Env Creation failure, could not set native hrp")
	}

	mintKeeper := mintkeeper.NewKeeper(appCodec, runtime.NewKVStoreService(keyMint), stakingKeeper, accountKeeper, bankKeeper, authtypes.FeeCollectorName, govAuthority)
	mintKeeper.Params.Set(ctx, minttypes.DefaultParams())

	auctionKeeper := auctionkeeper.NewKeeper(keyAuction, getSubspace(paramsKeeper, auctiontypes.ModuleName), appCodec, &bankKeeper, &accountKeeper, &distKeeper, &mintKeeper)
	auctionKeeper.SetParams(ctx, auctiontypes.DefaultParams())

	k := NewKeeper(gravityKey, getSubspace(paramsKeeper, types.DefaultParamspace), appCodec, &bankKeeper,
		stakingKeeper, &slashingKeeper, &distKeeper, &accountKeeper, &ibcTransferKeeper, &bech32IbcKeeper, &auctionKeeper, authtypes.NewModuleAddress(govtypes.ModuleName).String())

	stakingKeeper.SetHooks(
		stakingtypes.NewMultiStakingHooks(
			distKeeper.Hooks(),
			slashingKeeper.Hooks(),
			k.Hooks(),
		),
	)

	// set gravityIDs for batches and tx items, simulating genesis setup
	k.SetLatestValsetNonce(ctx, 0)
	k.setLastObservedEventNonce(ctx, 0)
	k.SetLastSlashedValsetNonce(ctx, 0)
	k.SetLastSlashedBatchBlock(ctx, 0)
	k.SetLastSlashedLogicCallBlock(ctx, 0)
	k.setID(ctx, 0, types.KeyLastTXPoolID)
	k.setID(ctx, 0, types.KeyLastOutgoingBatchID)

	k.SetParams(ctx, TestingGravityParams)

	testInput := TestInput{
		GravityKeeper:     k,
		AccountKeeper:     accountKeeper,
		StakingKeeper:     *stakingKeeper,
		SlashingKeeper:    slashingKeeper,
		DistKeeper:        distKeeper,
		BankKeeper:        bankKeeper,
		GovKeeper:         *govKeeper,
		IbcKeeper:         ibcKeeper,
		IbcTransferKeeper: ibcTransferKeeper,
		MintKeeper:        mintKeeper,
		AuctionKeeper:     auctionKeeper,
		Context:           ctx,
		Marshaler:         appCodec,
		LegacyAmino:       encodingConfig.Amino,
		EncodingConfig:    encodingConfig,
		GravityStoreKey:   gravityKey,
	}
	// check invariants before starting
	testInput.Context.Logger().Info("Asserting invariants on new test env")
	testInput.AssertInvariants()
	return testInput
}

// AssertInvariants tests each modules invariants individually, this is easier than
// dealing with all the init required to get the crisis keeper working properly by
// running appModuleBasic for every module and allowing them to register their invariants
func (t TestInput) AssertInvariants() {
	distrInvariantFunc := distrkeeper.AllInvariants(t.DistKeeper)
	bankInvariantFunc := bankkeeper.AllInvariants(t.BankKeeper)
	govInvariantFunc := govkeeper.ModuleAccountInvariant(&t.GovKeeper, t.BankKeeper)
	stakeInvariantFunc := stakingkeeper.AllInvariants(&t.StakingKeeper)
	gravInvariantFunc := AllInvariants(t.GravityKeeper)

	invariantStr, invariantViolated := distrInvariantFunc(t.Context)
	if invariantViolated {
		panic(invariantStr)
	}
	invariantStr, invariantViolated = bankInvariantFunc(t.Context)
	if invariantViolated {
		panic(invariantStr)
	}
	invariantStr, invariantViolated = govInvariantFunc(t.Context)
	if invariantViolated {
		panic(invariantStr)
	}
	invariantStr, invariantViolated = stakeInvariantFunc(t.Context)
	if invariantViolated {
		panic(invariantStr)
	}
	invariantStr, invariantViolated = gravInvariantFunc(t.Context)
	if invariantViolated {
		panic(invariantStr)
	}

	t.Context.Logger().Info("All invariants successful")
}

// getSubspace returns a param subspace for a given module name.
func getSubspace(k paramskeeper.Keeper, moduleName string) paramstypes.Subspace {
	subspace, _ := k.GetSubspace(moduleName)
	return subspace
}

// This is a copy of the encoding config creation in /app
func MakeTestEncodingConfig() gravityparams.EncodingConfig {
	encodingConfig := gravityparams.MakeEncodingConfig()
	ethermintcodec.RegisterLegacyAminoCodec(encodingConfig.Amino)
	ethermintcryptocodec.RegisterInterfaces(encodingConfig.InterfaceRegistry)
	std.RegisterInterfaces(encodingConfig.InterfaceRegistry)

	ModuleBasics.RegisterLegacyAminoCodec(encodingConfig.Amino)
	ModuleBasics.RegisterInterfaces(encodingConfig.InterfaceRegistry)

	// nolint: exhaustruct
	encodingConfig.InterfaceRegistry.RegisterImplementations(
		(*tx.TxExtensionOptionI)(nil),
		&etherminttypes.ExtensionOptionsWeb3Tx{},
	)

	types.RegisterCodec(encodingConfig.Amino)
	types.RegisterInterfaces(encodingConfig.InterfaceRegistry)

	return encodingConfig
}

// MintVouchersFromAir creates new gravity vouchers given erc20tokens
func MintVouchersFromAir(t *testing.T, ctx sdk.Context, k Keeper, dest sdk.AccAddress, amount types.InternalERC20Token) sdk.Coin {
	coin := amount.GravityCoin()
	vouchers := sdk.Coins{coin}
	err := k.bankKeeper.MintCoins(ctx, types.ModuleName, vouchers)
	require.NoError(t, err)
	err = k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, dest, vouchers)
	require.NoError(t, err)
	return coin
}

func NewTestMsgCreateValidator(address sdk.ValAddress, pubKey ccrypto.PubKey, amt sdkmath.Int) *stakingtypes.MsgCreateValidator {
	fivePercent := sdkmath.LegacyNewDecWithPrec(5, 2)
	commission := stakingtypes.NewCommissionRates(fivePercent, fivePercent, fivePercent)
	out, err := stakingtypes.NewMsgCreateValidator(
		address.String(), pubKey, sdk.NewCoin("stake", amt),
		stakingtypes.Description{
			Moniker:         "moniker",
			Identity:        "identity",
			Website:         "website",
			SecurityContact: "security_contact",
			Details:         "details",
		}, commission, sdkmath.OneInt(),
	)
	if err != nil {
		panic(err)
	}
	return out
}

func NewTestMsgUnDelegateValidator(address sdk.ValAddress, amt sdkmath.Int) *stakingtypes.MsgUndelegate {
	msg := stakingtypes.NewMsgUndelegate(sdk.AccAddress(address).String(), address.String(), sdk.NewCoin("stake", amt))
	return msg
}
