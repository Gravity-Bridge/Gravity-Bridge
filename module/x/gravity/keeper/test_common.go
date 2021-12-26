package keeper

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	ccodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	ccrypto "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/cosmos/cosmos-sdk/std"
	"github.com/cosmos/cosmos-sdk/store"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/cosmos/cosmos-sdk/x/auth"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/cosmos/cosmos-sdk/x/auth/vesting"
	"github.com/cosmos/cosmos-sdk/x/bank"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/cosmos/cosmos-sdk/x/capability"
	"github.com/cosmos/cosmos-sdk/x/crisis"
	"github.com/cosmos/cosmos-sdk/x/distribution"
	distrclient "github.com/cosmos/cosmos-sdk/x/distribution/client"
	distrkeeper "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	"github.com/cosmos/cosmos-sdk/x/evidence"
	"github.com/cosmos/cosmos-sdk/x/genutil"
	"github.com/cosmos/cosmos-sdk/x/gov"
	govkeeper "github.com/cosmos/cosmos-sdk/x/gov/keeper"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	"github.com/cosmos/cosmos-sdk/x/mint"
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
	gethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/libs/log"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	tmversion "github.com/tendermint/tendermint/proto/tendermint/version"
	dbm "github.com/tendermint/tm-db"

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
		SlashFractionValset:          sdk.NewDecWithPrec(1, 2),
		SlashFractionBatch:           sdk.NewDecWithPrec(1, 2),
		SlashFractionLogicCall:       sdk.Dec{},
		UnbondSlashingValsetsWindow:  15,
		SlashFractionBadEthSignature: sdk.NewDecWithPrec(1, 2),
		ValsetReward:                 sdk.Coin{Denom: "", Amount: sdk.ZeroInt()},
		BridgeActive:                 true,
	}
)

// TestInput stores the various keepers required to test gravity
type TestInput struct {
	GravityKeeper  Keeper
	AccountKeeper  authkeeper.AccountKeeper
	StakingKeeper  stakingkeeper.Keeper
	SlashingKeeper slashingkeeper.Keeper
	DistKeeper     distrkeeper.Keeper
	BankKeeper     bankkeeper.BaseKeeper
	GovKeeper      govkeeper.Keeper
	Context        sdk.Context
	Marshaler      codec.Codec
	LegacyAmino    *codec.LegacyAmino
}

// SetupFiveValChain does all the initialization for a 5 Validator chain using the keys here
func SetupFiveValChain(t *testing.T) (TestInput, sdk.Context) {
	t.Helper()
	input := CreateTestEnv(t)

	// Set the params for our modules
	input.StakingKeeper.SetParams(input.Context, TestingStakeParams)

	// Initialize each of the validators
	sh := staking.NewHandler(input.StakingKeeper)
	for i := range []int{0, 1, 2, 3, 4} {

		// Initialize the account for the key
		acc := input.AccountKeeper.NewAccount(
			input.Context,
			authtypes.NewBaseAccount(AccAddrs[i], AccPubKeys[i], uint64(i), 0),
		)

		// Set the balance for the account
		require.NoError(t, input.BankKeeper.MintCoins(input.Context, types.ModuleName, InitCoins))
		input.BankKeeper.SendCoinsFromModuleToAccount(input.Context, types.ModuleName, acc.GetAddress(), InitCoins)

		// Set the account in state
		input.AccountKeeper.SetAccount(input.Context, acc)

		// Create a validator for that account using some of the tokens in the account
		// and the staking handler
		_, err := sh(
			input.Context,
			NewTestMsgCreateValidator(ValAddrs[i], ConsPubKeys[i], StakingAmount),
		)

		// Return error if one exists
		require.NoError(t, err)
	}

	// Run the staking endblocker to ensure valset is correct in state
	staking.EndBlocker(input.Context, input.StakingKeeper)

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
	sh := staking.NewHandler(input.StakingKeeper)
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
		_, err := sh(
			input.Context,
			NewTestMsgCreateValidator(valAddr, consPubKey, sdk.NewIntFromUint64(weight)),
		)
		require.NoError(t, err)

		// Run the staking endblocker to ensure valset is correct in state
		staking.EndBlocker(input.Context, input.StakingKeeper)

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
			staking.EndBlocker(input.Context, input.StakingKeeper)

			// set a request every time.
			input.GravityKeeper.SetValsetRequest(input.Context)
		}

	}

	// some inputs can cause the validator creation ot not work, this checks that
	// everything was successful
	validators := input.StakingKeeper.GetBondedValidatorsByPower(input.Context)
	require.Equal(t, len(weights), len(validators))

	// Return the test input
	return input, input.Context
}

// CreateTestEnv creates the keeper testing environment for gravity
func CreateTestEnv(t *testing.T) TestInput {
	t.Helper()

	// Initialize store keys
	gravityKey := sdk.NewKVStoreKey(types.StoreKey)
	keyAcc := sdk.NewKVStoreKey(authtypes.StoreKey)
	keyStaking := sdk.NewKVStoreKey(stakingtypes.StoreKey)
	keyBank := sdk.NewKVStoreKey(banktypes.StoreKey)
	keyDistro := sdk.NewKVStoreKey(distrtypes.StoreKey)
	keyParams := sdk.NewKVStoreKey(paramstypes.StoreKey)
	tkeyParams := sdk.NewTransientStoreKey(paramstypes.TStoreKey)
	keyGov := sdk.NewKVStoreKey(govtypes.StoreKey)
	keySlashing := sdk.NewKVStoreKey(slashingtypes.StoreKey)

	// Initialize memory database and mount stores on it
	db := dbm.NewMemDB()
	ms := store.NewCommitMultiStore(db)
	ms.MountStoreWithDB(gravityKey, sdk.StoreTypeIAVL, db)
	ms.MountStoreWithDB(keyAcc, sdk.StoreTypeIAVL, db)
	ms.MountStoreWithDB(keyParams, sdk.StoreTypeIAVL, db)
	ms.MountStoreWithDB(keyStaking, sdk.StoreTypeIAVL, db)
	ms.MountStoreWithDB(keyBank, sdk.StoreTypeIAVL, db)
	ms.MountStoreWithDB(keyDistro, sdk.StoreTypeIAVL, db)
	ms.MountStoreWithDB(tkeyParams, sdk.StoreTypeTransient, db)
	ms.MountStoreWithDB(keyGov, sdk.StoreTypeIAVL, db)
	ms.MountStoreWithDB(keySlashing, sdk.StoreTypeIAVL, db)
	err := ms.LoadLatestVersion()
	require.Nil(t, err)

	// Create sdk.Context
	ctx := sdk.NewContext(ms, tmproto.Header{
		Version: tmversion.Consensus{
			Block: 0,
			App:   0,
		},
		ChainID: "",
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
	}, false, log.TestingLogger())

	cdc := MakeTestCodec()
	marshaler := MakeTestMarshaler()

	paramsKeeper := paramskeeper.NewKeeper(marshaler, cdc, keyParams, tkeyParams)
	paramsKeeper.Subspace(authtypes.ModuleName)
	paramsKeeper.Subspace(banktypes.ModuleName)
	paramsKeeper.Subspace(stakingtypes.ModuleName)
	paramsKeeper.Subspace(distrtypes.ModuleName)
	paramsKeeper.Subspace(govtypes.ModuleName)
	paramsKeeper.Subspace(types.DefaultParamspace)
	paramsKeeper.Subspace(slashingtypes.ModuleName)

	// this is also used to initialize module accounts for all the map keys
	maccPerms := map[string][]string{
		authtypes.FeeCollectorName:     nil,
		distrtypes.ModuleName:          nil,
		stakingtypes.BondedPoolName:    {authtypes.Burner, authtypes.Staking},
		stakingtypes.NotBondedPoolName: {authtypes.Burner, authtypes.Staking},
		govtypes.ModuleName:            {authtypes.Burner},
		types.ModuleName:               {authtypes.Minter, authtypes.Burner},
	}

	accountKeeper := authkeeper.NewAccountKeeper(
		marshaler,
		keyAcc, // target store
		getSubspace(paramsKeeper, authtypes.ModuleName),
		authtypes.ProtoBaseAccount, // prototype
		maccPerms,
	)

	blockedAddr := make(map[string]bool, len(maccPerms))
	for acc := range maccPerms {
		blockedAddr[authtypes.NewModuleAddress(acc).String()] = true
	}
	bankKeeper := bankkeeper.NewBaseKeeper(
		marshaler,
		keyBank,
		accountKeeper,
		getSubspace(paramsKeeper, banktypes.ModuleName),
		blockedAddr,
	)
	bankKeeper.SetParams(ctx, banktypes.Params{
		SendEnabled:        []*banktypes.SendEnabled{},
		DefaultSendEnabled: true,
	})

	stakingKeeper := stakingkeeper.NewKeeper(marshaler, keyStaking, accountKeeper, bankKeeper, getSubspace(paramsKeeper, stakingtypes.ModuleName))
	stakingKeeper.SetParams(ctx, TestingStakeParams)

	distKeeper := distrkeeper.NewKeeper(marshaler, keyDistro, getSubspace(paramsKeeper, distrtypes.ModuleName), accountKeeper, bankKeeper, stakingKeeper, authtypes.FeeCollectorName, nil)
	distKeeper.SetParams(ctx, distrtypes.DefaultParams())

	// set genesis items required for distribution
	distKeeper.SetFeePool(ctx, distrtypes.InitialFeePool())

	// total supply to track this
	totalSupply := sdk.NewCoins(sdk.NewInt64Coin("stake", 100000000))

	// set up initial accounts
	for name, perms := range maccPerms {
		mod := authtypes.NewEmptyModuleAccount(name, perms...)
		if name == stakingtypes.NotBondedPoolName {
			err = bankKeeper.MintCoins(ctx, types.ModuleName, totalSupply)
			require.NoError(t, err)
			err = bankKeeper.SendCoinsFromModuleToModule(ctx, types.ModuleName, mod.Name, totalSupply)
			require.NoError(t, err)
		} else if name == distrtypes.ModuleName {
			// some big pot to pay out
			amt := sdk.NewCoins(sdk.NewInt64Coin("stake", 500000))
			err = bankKeeper.MintCoins(ctx, types.ModuleName, amt)
			require.NoError(t, err)
			err = bankKeeper.SendCoinsFromModuleToModule(ctx, types.ModuleName, mod.Name, amt)
			require.NoError(t, err)
		}
		accountKeeper.SetModuleAccount(ctx, mod)
	}

	stakeAddr := authtypes.NewModuleAddress(stakingtypes.BondedPoolName)
	moduleAcct := accountKeeper.GetAccount(ctx, stakeAddr)
	require.NotNil(t, moduleAcct)

	router := baseapp.NewRouter()
	router.AddRoute(bank.AppModule{
		AppModuleBasic: bank.AppModuleBasic{},
	}.Route())
	router.AddRoute(staking.AppModule{
		AppModuleBasic: staking.AppModuleBasic{},
	}.Route())
	router.AddRoute(distribution.AppModule{
		AppModuleBasic: distribution.AppModuleBasic{},
	}.Route())

	// Load default wasm config

	govRouter := govtypes.NewRouter().
		AddRoute(paramsproposal.RouterKey, params.NewParamChangeProposalHandler(paramsKeeper)).
		AddRoute(govtypes.RouterKey, govtypes.ProposalHandler)

	govKeeper := govkeeper.NewKeeper(
		marshaler, keyGov, getSubspace(paramsKeeper, govtypes.ModuleName).WithKeyTable(govtypes.ParamKeyTable()), accountKeeper, bankKeeper, stakingKeeper, govRouter,
	)

	govKeeper.SetProposalID(ctx, govtypes.DefaultStartingProposalID)
	govKeeper.SetDepositParams(ctx, govtypes.DefaultDepositParams())
	govKeeper.SetVotingParams(ctx, govtypes.DefaultVotingParams())
	govKeeper.SetTallyParams(ctx, govtypes.DefaultTallyParams())

	slashingKeeper := slashingkeeper.NewKeeper(
		marshaler,
		keySlashing,
		&stakingKeeper,
		getSubspace(paramsKeeper, slashingtypes.ModuleName).WithKeyTable(slashingtypes.ParamKeyTable()),
	)

	k := NewKeeper(gravityKey, getSubspace(paramsKeeper, types.DefaultParamspace), marshaler, &bankKeeper, &stakingKeeper, &slashingKeeper, &distKeeper, &accountKeeper)

	stakingKeeper = *stakingKeeper.SetHooks(
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
	k.setID(ctx, 0, []byte(types.KeyLastTXPoolID))
	k.setID(ctx, 0, []byte(types.KeyLastOutgoingBatchID))

	k.SetParams(ctx, TestingGravityParams)
	params := k.GetParams(ctx)

	fmt.Println(params)

	return TestInput{
		GravityKeeper:  k,
		AccountKeeper:  accountKeeper,
		BankKeeper:     bankKeeper,
		StakingKeeper:  stakingKeeper,
		SlashingKeeper: slashingKeeper,
		DistKeeper:     distKeeper,
		GovKeeper:      govKeeper,
		Context:        ctx,
		Marshaler:      marshaler,
		LegacyAmino:    cdc,
	}
}

// getSubspace returns a param subspace for a given module name.
func getSubspace(k paramskeeper.Keeper, moduleName string) paramstypes.Subspace {
	subspace, _ := k.GetSubspace(moduleName)
	return subspace
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

func NewTestMsgCreateValidator(address sdk.ValAddress, pubKey ccrypto.PubKey, amt sdk.Int) *stakingtypes.MsgCreateValidator {
	commission := stakingtypes.NewCommissionRates(sdk.ZeroDec(), sdk.ZeroDec(), sdk.ZeroDec())
	out, err := stakingtypes.NewMsgCreateValidator(
		address, pubKey, sdk.NewCoin("stake", amt),
		stakingtypes.Description{
			Moniker:         "",
			Identity:        "",
			Website:         "",
			SecurityContact: "",
			Details:         "",
		}, commission, sdk.OneInt(),
	)
	if err != nil {
		panic(err)
	}
	return out
}

func NewTestMsgUnDelegateValidator(address sdk.ValAddress, amt sdk.Int) *stakingtypes.MsgUndelegate {
	msg := stakingtypes.NewMsgUndelegate(sdk.AccAddress(address), address, sdk.NewCoin("stake", amt))
	return msg
}
