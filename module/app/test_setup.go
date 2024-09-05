package app

import (
	"encoding/json"
	"time"

	"cosmossdk.io/math"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/log"
	dbm "github.com/tendermint/tm-db"

	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	ccrypto "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/cosmos/cosmos-sdk/simapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	govv1beta1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1beta1"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/config"
)

const Bech32Prefix = "gravity"

var (
	AccPrivKeys = []ccrypto.PrivKey{
		secp256k1.GenPrivKey(),
		secp256k1.GenPrivKey(),
	}
	AccPubKeys = []ccrypto.PubKey{
		AccPrivKeys[0].PubKey(),
		AccPrivKeys[1].PubKey(),
	}
	AccAddresses = []sdk.AccAddress{
		sdk.AccAddress(AccPubKeys[0].Address()),
		sdk.AccAddress(AccPubKeys[1].Address()),
	}
)

// Initializes a new GravityApp without IBC functionality
func InitGravityTestApp(initChain bool) *Gravity {
	db := dbm.NewMemDB()
	app := NewGravityApp(
		log.NewNopLogger(),
		db,
		nil,
		true,
		map[int64]bool{},
		DefaultNodeHome,
		5,
		MakeEncodingConfig(),
		simapp.EmptyAppOptions{},
	)
	if initChain {
		genesisState := NewDefaultGenesisState()
		addValidators(&genesisState, app.AppCodec)
		replaceStakeWithGrav(&genesisState, app.AppCodec)
		stateBytes, err := json.MarshalIndent(genesisState, "", " ")
		if err != nil {
			panic(err)
		}

		app.BaseApp.InitChain(
			// nolint: exhaustruct
			abci.RequestInitChain{
				Validators:      []abci.ValidatorUpdate{},
				ConsensusParams: simapp.DefaultConsensusParams,
				AppStateBytes:   stateBytes,
			},
		)
	}

	return app
}

func replaceStakeWithGrav(genState *GenesisState, cdc codec.Codec) {
	stake := "stake"

	bankGenesis := (*genState)[banktypes.ModuleName]
	if bankGenesis == nil {
		panic("Nil bank genesis")
	}
	var bankGenState banktypes.GenesisState
	cdc.MustUnmarshalJSON(bankGenesis, &bankGenState)
	for _, mdta := range bankGenState.DenomMetadata {
		if mdta.Base == stake {
			mdta.Base = config.NativeTokenDenom
			mdta.DenomUnits = []*banktypes.DenomUnit{{
				Denom:    config.NativeTokenDenom,
				Exponent: 0,
			}}
			mdta.Display = config.NativeTokenDenom
		}
	}
	(*genState)[banktypes.ModuleName] = cdc.MustMarshalJSON(&bankGenState)

	govGenesis := (*genState)[govtypes.ModuleName]
	if govGenesis == nil {
		panic("Nil gov genesis")
	}
	var govGenState govv1beta1.GenesisState
	cdc.MustUnmarshalJSON(govGenesis, &govGenState)
	govGenState.DepositParams.MinDeposit[0].Denom = config.NativeTokenDenom
	(*genState)[govtypes.ModuleName] = cdc.MustMarshalJSON(&govGenState)

	mintGenesis := (*genState)[minttypes.ModuleName]
	if mintGenesis == nil {
		panic("Nil mint genesis")
	}
	var mintGenState minttypes.GenesisState
	cdc.MustUnmarshalJSON(mintGenesis, &mintGenState)
	mintGenState.Params.MintDenom = config.NativeTokenDenom
	(*genState)[minttypes.ModuleName] = cdc.MustMarshalJSON(&mintGenState)

	stakingGenesis := (*genState)[stakingtypes.ModuleName]
	if stakingGenesis == nil {
		panic("Nil staking genesis")
	}
	var stakingGenState stakingtypes.GenesisState
	cdc.MustUnmarshalJSON(stakingGenesis, &stakingGenState)
	stakingGenState.Params.BondDenom = config.NativeTokenDenom
	(*genState)[stakingtypes.ModuleName] = cdc.MustMarshalJSON(&stakingGenState)
}

func addValidators(genState *GenesisState, cdc codec.Codec) {
	authGenesis := (*genState)[authtypes.ModuleName]
	if authGenesis == nil {
		panic("Nil auth genesis")
	}
	var authGenState authtypes.GenesisState
	cdc.MustUnmarshalJSON(authGenesis, &authGenState)
	for _, pubkey := range AccPubKeys {
		validatorAcc, err := codectypes.NewAnyWithValue(
			authtypes.NewBaseAccount(pubkey.Address().Bytes(), pubkey, uint64(len(authGenState.Accounts)), 0),
		)
		if err != nil {
			panic(err)
		}
		authGenState.Accounts = append(
			authGenState.Accounts,
			validatorAcc,
		)
	}

	bankGenesis := (*genState)[banktypes.ModuleName]
	if bankGenesis == nil {
		panic("Nil bank genesis")
	}
	var bankGenState banktypes.GenesisState
	cdc.MustUnmarshalJSON(bankGenesis, &bankGenState)
	for _, pubkey := range AccPubKeys {
		bankGenState.Balances = append(
			bankGenState.Balances,
			banktypes.Balance{
				Address: sdk.MustBech32ifyAddressBytes("gravity", pubkey.Address().Bytes()),
				Coins:   sdk.NewCoins(sdk.NewInt64Coin(config.NativeTokenDenom, 1000000000)),
			},
		)
		bankGenState.Supply = bankGenState.Supply.Add(sdk.NewInt64Coin(config.NativeTokenDenom, 1000000000))
	}

	mintGenesis := (*genState)[minttypes.ModuleName]
	if mintGenesis == nil {
		panic("Nil mint genesis")
	}
	var mintGenState minttypes.GenesisState
	cdc.MustUnmarshalJSON(mintGenesis, &mintGenState)
	mintGenState.Params.MintDenom = config.NativeTokenDenom

	stakingGenesis := (*genState)[stakingtypes.ModuleName]
	if stakingGenesis == nil {
		panic("Nil staking genesis")
	}
	var stakingGenState stakingtypes.GenesisState
	cdc.MustUnmarshalJSON(stakingGenesis, &stakingGenState)

	valTokens := sdk.TokensFromConsensusPower(1, sdk.DefaultPowerReduction)

	params := stakingtypes.DefaultParams()
	var delegations []stakingtypes.Delegation

	pk0, err := codectypes.NewAnyWithValue(AccPubKeys[0])
	if err != nil {
		panic(err)
	}

	pk1, err := codectypes.NewAnyWithValue(AccPubKeys[1])
	if err != nil {
		panic(err)
	}

	// initialize the validators
	valShares := sdk.NewDecFromInt(valTokens)
	bondedVal1 := stakingtypes.Validator{
		OperatorAddress: sdk.ValAddress(AccAddresses[0]).String(),
		ConsensusPubkey: pk0,
		Jailed:          false,
		Status:          stakingtypes.Bonded,
		Tokens:          valTokens,
		DelegatorShares: valShares,
		Description:     stakingtypes.NewDescription("hoop", "", "", "", ""),
		UnbondingHeight: 0,
		UnbondingTime:   time.Time{},
		// nolint: exhaustruct
		Commission:        stakingtypes.Commission{},
		MinSelfDelegation: math.Int{},
	}
	delegations = append(delegations, stakingtypes.Delegation{ValidatorAddress: sdk.ValAddress(AccAddresses[0]).String(), DelegatorAddress: AccAddresses[0].String(), Shares: valShares})
	bondedVal2 := stakingtypes.Validator{
		OperatorAddress: sdk.ValAddress(AccAddresses[1]).String(),
		ConsensusPubkey: pk1,
		Jailed:          false,
		Status:          stakingtypes.Bonded,
		Tokens:          valTokens,
		DelegatorShares: valShares,
		Description:     stakingtypes.NewDescription("bloop", "", "", "", ""),
		UnbondingHeight: 0,
		UnbondingTime:   time.Time{},
		// nolint: exhaustruct
		Commission:        stakingtypes.Commission{},
		MinSelfDelegation: math.Int{},
	}
	delegations = append(delegations, stakingtypes.Delegation{ValidatorAddress: sdk.ValAddress(AccAddresses[1]).String(), DelegatorAddress: AccAddresses[1].String(), Shares: valShares})

	bondedPool := authtypes.NewModuleAddress(stakingtypes.BondedPoolName)
	bondedTokens := valTokens.Mul(math.NewInt(2))
	bankGenState.Balances = append(
		bankGenState.Balances,
		banktypes.Balance{Address: bondedPool.String(), Coins: sdk.NewCoins(sdk.NewCoin(config.NativeTokenDenom, bondedTokens))},
	)

	bankGenState.Supply = bankGenState.Supply.Add(sdk.NewInt64Coin(config.NativeTokenDenom, bondedTokens.Int64()))
	stakingGenState.Validators = append(stakingGenState.Validators, bondedVal1, bondedVal2)
	stakingGenState.Params.BondDenom = config.NativeTokenDenom
	stakingGenState.Delegations = delegations
	stakingGenState.Params = params

	(*genState)[minttypes.ModuleName] = cdc.MustMarshalJSON(&mintGenState)
	(*genState)[banktypes.ModuleName] = cdc.MustMarshalJSON(&bankGenState)
	(*genState)[stakingtypes.ModuleName] = cdc.MustMarshalJSON(&stakingGenState)
}
