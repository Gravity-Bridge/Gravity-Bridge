package gravity

import (
	"testing"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/keeper"
	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

// Have the validators put in a erc20<>denom relation with ERC20DeployedEvent
// Send some coins of that denom into the cosmos module
// Check that the coins are locked, not burned
// Have the validators put in a deposit event for that ERC20
// Check that the coins are unlocked and sent to the right account

func TestCosmosOriginated(t *testing.T) {
	tv := initializeTestingVars(t)
	defer func() {
		tv.input.Context.Logger().Info("Asserting invariants at test end")
		tv.input.AssertInvariants()
	}()
	addDenomToERC20Relation(tv)
	// we only create a relation here, we don't perform
	// the other tests with the IBC representation as the
	// results should be the same
	addIbcDenomToERC20Relation(tv)
	lockCoinsInModule(tv)
	acceptDepositEvent(tv)
}

// TestERC20DeployedClaimAllowlist verifies that handleErc20Deployed enforces the
// CosmosBridgeableTokens allowlist when deciding whether to register an ERC20 for
// a cosmos-originated denom. An empty allowlist blocks all denoms; a non-empty
// allowlist restricts registration to only the listed denoms.
func TestERC20DeployedClaimAllowlist(t *testing.T) {
	input, ctx := keeper.SetupFiveValChain(t)
	defer func() {
		ctx.Logger().Info("Asserting invariants at test end")
		input.AssertInvariants()
	}()
	msgServer := keeper.NewMsgServerImpl(input.GravityKeeper)

	// denom metadata for two test tokens
	fooMetadata := banktypes.Metadata{
		Description: "Foo Token",
		DenomUnits:  []*banktypes.DenomUnit{{Denom: "footoken", Exponent: 0}, {Denom: "foo", Exponent: 6}},
		Base:        "footoken",
		Display:     "foo",
		Name:        "Foo Token",
		Symbol:      "FOO",
		URI:         "",
		URIHash:     "",
	}
	barMetadata := banktypes.Metadata{
		Description: "Bar Token",
		DenomUnits:  []*banktypes.DenomUnit{{Denom: "bartoken", Exponent: 0}, {Denom: "bar", Exponent: 6}},
		Base:        "bartoken",
		Display:     "bar",
		Name:        "Bar Token",
		Symbol:      "BAR",
		URI:         "",
		URIHash:     "",
	}
	input.BankKeeper.SetDenomMetaData(ctx, fooMetadata)
	input.BankKeeper.SetDenomMetaData(ctx, barMetadata)

	submitERC20Claim := func(nonce uint64, cosmosDenom, tokenContract, name, symbol string, decimals uint64) {
		for _, orchAddr := range keeper.OrchAddrs {
			claim := types.MsgERC20DeployedClaim{
				EventNonce:     nonce,
				EthBlockHeight: 0,
				CosmosDenom:    cosmosDenom,
				TokenContract:  tokenContract,
				Name:           name,
				Symbol:         symbol,
				Decimals:       decimals,
				Orchestrator:   orchAddr.String(),
			}
			_, err := msgServer.ERC20DeployedClaim(ctx, &claim)
			require.NoError(t, err)
		}
		EndBlocker(ctx, input.GravityKeeper)
	}

	// Case 1: allowlist has footoken — footoken ERC20 is registered
	input.GravityKeeper.SetCosmosBridgeableToken(ctx, fooMetadata)

	submitERC20Claim(1, "footoken", "0x1111111111111111111111111111111111111111", "Foo Token", "FOO", 6)
	isCosmosOriginated, _, err := input.GravityKeeper.DenomToERC20Lookup(ctx, "footoken")
	require.NoError(t, err)
	require.True(t, isCosmosOriginated, "footoken ERC20 should be registered when footoken is on the allowlist")

	// Case 2: allowlist has bartoken — bartoken ERC20 is registered
	input.GravityKeeper.DeleteCosmosBridgeableToken(ctx, fooMetadata.Base)
	input.GravityKeeper.SetCosmosBridgeableToken(ctx, barMetadata)

	submitERC20Claim(2, "bartoken", "0x2222222222222222222222222222222222222222", "Bar Token", "BAR", 6)
	isCosmosOriginated, _, err = input.GravityKeeper.DenomToERC20Lookup(ctx, "bartoken")
	require.NoError(t, err)
	require.True(t, isCosmosOriginated, "bartoken ERC20 should be registered when bartoken is on the allowlist")

	// Case 3: allowlist has bartoken only — unlisted denom (baztoken) is rejected
	// the allowlist still contains only "bartoken"
	//nolint: exhaustruct
	bazMetadata := banktypes.Metadata{
		Description: "Baz Token",
		DenomUnits:  []*banktypes.DenomUnit{{Denom: "baztoken", Exponent: 0}, {Denom: "baz", Exponent: 6}},
		Base:        "baztoken",
		Display:     "baz",
		Name:        "Baz Token",
		Symbol:      "BAZ",
		URI:         "",
		URIHash:     "",
	}
	input.BankKeeper.SetDenomMetaData(ctx, bazMetadata)

	submitERC20Claim(3, "baztoken", "0x3333333333333333333333333333333333333333", "Baz Token", "BAZ", 6)
	_, _, err = input.GravityKeeper.DenomToERC20Lookup(ctx, "baztoken")
	require.Error(t, err, "baztoken ERC20 should NOT be registered when baztoken is not on the allowlist")

	// Case 4: empty allowlist — all cosmos-originated denoms are blocked
	input.GravityKeeper.DeleteCosmosBridgeableToken(ctx, barMetadata.Base)

	quxMetadata := banktypes.Metadata{
		Description: "Qux Token",
		DenomUnits:  []*banktypes.DenomUnit{{Denom: "quxtoken", Exponent: 0}, {Denom: "qux", Exponent: 6}},
		Base:        "quxtoken",
		Display:     "qux",
		Name:        "Qux Token",
		Symbol:      "QUX",
	}
	input.BankKeeper.SetDenomMetaData(ctx, quxMetadata)

	submitERC20Claim(4, "quxtoken", "0x4444444444444444444444444444444444444444", "Qux Token", "QUX", 6)
	_, _, err = input.GravityKeeper.DenomToERC20Lookup(ctx, "quxtoken")
	require.Error(t, err, "quxtoken ERC20 should NOT be registered when the allowlist is empty")
}

type testingVars struct {
	erc20     string
	denom     string
	input     keeper.TestInput
	ctx       sdk.Context
	msgServer types.MsgServer
	t         *testing.T
}

func initializeTestingVars(t *testing.T) *testingVars {
	var tv testingVars

	tv.t = t

	tv.erc20 = "0x0bc529c00C6401aEF6D220BE8C6Ea1667F6Ad93e"
	tv.denom = "ugraviton"

	tv.input, tv.ctx = keeper.SetupFiveValChain(t)
	tv.msgServer = keeper.NewMsgServerImpl(tv.input.GravityKeeper)

	return &tv
}

func addDenomToERC20Relation(tv *testingVars) {
	ugravitonMeta := banktypes.Metadata{
		Description: "The native staking token of the Cosmos Gravity Bridge",
		DenomUnits:  []*banktypes.DenomUnit{{Denom: "ugraviton", Exponent: uint32(0), Aliases: []string{"micrograviton"}}, {Denom: "mgraviton", Exponent: uint32(3), Aliases: []string{"milligraviton"}}, {Denom: "graviton", Exponent: uint32(6), Aliases: []string{}}},
		Base:        "ugraviton",
		Display:     "graviton",
		Name:        "Graviton",
		Symbol:      "GRAV",
		URI:         "",
		URIHash:     "",
	}
	tv.input.BankKeeper.SetDenomMetaData(tv.ctx, ugravitonMeta)

	// Add ugraviton to the allowlist so the ERC20 deployed handler accepts it
	tv.input.GravityKeeper.SetCosmosBridgeableToken(tv.ctx, ugravitonMeta)

	var (
		myNonce = uint64(1)
	)

	// have all five validators observe this event
	for _, v := range keeper.OrchAddrs {
		ethClaim := types.MsgERC20DeployedClaim{
			EventNonce:     myNonce,
			EthBlockHeight: 0,
			CosmosDenom:    tv.denom,
			TokenContract:  tv.erc20,
			Name:           "Graviton",
			Symbol:         "GRAV",
			Decimals:       6,
			Orchestrator:   v.String(),
		}
		_, err := tv.msgServer.ERC20DeployedClaim(tv.ctx, &ethClaim)
		require.NoError(tv.t, err)

		// check if attestations persisted
		hash, err := ethClaim.ClaimHash()
		require.NoError(tv.t, err)
		a := tv.input.GravityKeeper.GetAttestation(tv.ctx, myNonce, hash)
		require.NotNil(tv.t, a)
	}

	EndBlocker(tv.ctx, tv.input.GravityKeeper)

	// check if erc20<>denom relation added to db
	isCosmosOriginated, gotERC20, err := tv.input.GravityKeeper.DenomToERC20Lookup(tv.ctx, tv.denom)
	require.NoError(tv.t, err)
	assert.True(tv.t, isCosmosOriginated)

	ethAddr, err := types.NewEthAddress(tv.erc20)
	require.NoError(tv.t, err)
	isCosmosOriginated, gotDenom := tv.input.GravityKeeper.ERC20ToDenomLookup(tv.ctx, *ethAddr)
	assert.True(tv.t, isCosmosOriginated)

	assert.Equal(tv.t, tv.denom, gotDenom)
	assert.Equal(tv.t, tv.erc20, gotERC20.GetAddress().Hex())
}

func lockCoinsInModule(tv *testingVars) {
	var (
		userCosmosAddr, err             = sdk.AccAddressFromBech32("gravity1990z7dqsvh8gthw9pa5sn4wuy2xrsd80lcx6lv")
		denom                           = "ugraviton"
		startingCoinAmount  sdkmath.Int = sdkmath.NewIntFromUint64(150)
		sendAmount          sdkmath.Int = sdkmath.NewIntFromUint64(50)
		feeAmount           sdkmath.Int = sdkmath.NewIntFromUint64(5)
		startingCoins       sdk.Coins   = sdk.Coins{sdk.NewCoin(denom, startingCoinAmount)}
		sendingCoin         sdk.Coin    = sdk.NewCoin(denom, sendAmount)
		feeCoin             sdk.Coin    = sdk.NewCoin(denom, feeAmount)
		ethDestination                  = "0x3c9289da00b02dC623d0D8D907619890301D26d4"
	)
	assert.Nil(tv.t, err)

	// Add ugraviton to the CosmosBridgeableTokens allowlist so SendToEth accepts it
	ugravitonMeta, found := tv.input.BankKeeper.GetDenomMetaData(tv.ctx, denom)
	require.True(tv.t, found, "expected ugraviton bank metadata to be set")
	tv.input.GravityKeeper.SetCosmosBridgeableToken(tv.ctx, ugravitonMeta)

	// we start by depositing some funds into the users balance to send
	require.NoError(tv.t, tv.input.BankKeeper.MintCoins(tv.ctx, types.ModuleName, startingCoins))
	err = tv.input.BankKeeper.SendCoinsFromModuleToAccount(tv.ctx, types.ModuleName, userCosmosAddr, startingCoins)
	require.NoError(tv.t, err)
	balance1 := tv.input.BankKeeper.GetAllBalances(tv.ctx, userCosmosAddr)
	assert.Equal(tv.t, sdk.Coins{sdk.NewCoin(denom, startingCoinAmount)}, balance1)

	// send some coins
	// nolint: exhaustruct
	zeroAmount := sdkmath.ZeroInt()
	msg := &types.MsgSendToEth{
		Sender:    userCosmosAddr.String(),
		EthDest:   ethDestination,
		Amount:    sendingCoin,
		BridgeFee: feeCoin,
		ChainFee:  sdk.NewCoin(denom, zeroAmount),
	}

	_, err = tv.msgServer.SendToEth(tv.ctx, msg)
	require.NoError(tv.t, err)

	// Check that user balance has gone down
	balance2 := tv.input.BankKeeper.GetAllBalances(tv.ctx, userCosmosAddr)
	assert.Equal(tv.t, sdk.Coins{sdk.NewCoin(denom, startingCoinAmount.Sub(sendAmount).Sub(feeAmount))}, balance2)

	// Check that gravity balance has gone up
	gravityAddr := tv.input.AccountKeeper.GetModuleAddress(types.ModuleName)
	assert.Equal(tv.t,
		sdk.Coins{sdk.NewCoin(denom, sendAmount.Add(feeAmount))},
		tv.input.BankKeeper.GetAllBalances(tv.ctx, gravityAddr),
	)
}

func acceptDepositEvent(tv *testingVars) {
	var (
		myCosmosAddr, err = sdk.AccAddressFromBech32("gravity16ahjkfqxpp6lvfy9fpfnfjg39xr96qet0l08hu")
		myNonce           = uint64(3)
		anyETHAddr        = "0xf9613b532673Cc223aBa451dFA8539B87e1F666D"
	)
	require.NoError(tv.t, err)

	myErc20 := types.ERC20Token{
		Amount:   sdkmath.NewInt(12),
		Contract: tv.erc20,
	}

	// have all five validators observe this event
	for _, v := range keeper.OrchAddrs {
		ethClaim := types.MsgSendToCosmosClaim{
			EventNonce:     myNonce,
			EthBlockHeight: 0,
			TokenContract:  myErc20.Contract,
			Amount:         myErc20.Amount,
			EthereumSender: anyETHAddr,
			CosmosReceiver: myCosmosAddr.String(),
			Orchestrator:   v.String(),
		}

		_, err := tv.msgServer.SendToCosmosClaim(tv.ctx, &ethClaim)
		require.NoError(tv.t, err)
		EndBlocker(tv.ctx, tv.input.GravityKeeper)

		// check that attestation persisted
		hash, err := ethClaim.ClaimHash()
		require.NoError(tv.t, err)
		a := tv.input.GravityKeeper.GetAttestation(tv.ctx, myNonce, hash)
		require.NotNil(tv.t, a)
	}

	// Check that user balance has gone up
	assert.Equal(tv.t,
		sdk.Coins{sdk.NewCoin(tv.denom, myErc20.Amount)},
		tv.input.BankKeeper.GetAllBalances(tv.ctx, myCosmosAddr))

	// Check that gravity balance has gone down
	gravityAddr := tv.input.AccountKeeper.GetModuleAddress(types.ModuleName)
	assert.Equal(tv.t,
		sdk.Coins{sdk.NewCoin(tv.denom, sdkmath.NewIntFromUint64(55).Sub(myErc20.Amount))},
		tv.input.BankKeeper.GetAllBalances(tv.ctx, gravityAddr),
	)
}

func addIbcDenomToERC20Relation(tv *testingVars) {

	tokenContract := "0xE486cC1a00aA806C3e40224EDAd5FdCA93dDdA62"
	ibcDenom := "ibc/46B44899322F3CD854D2D46DEEF881958467CDD4B3B10086DA49296BBED94BED"
	metadata := banktypes.Metadata{
		Description: "Atom",
		DenomUnits:  []*banktypes.DenomUnit{{Denom: ibcDenom, Exponent: 0}, {Denom: "Atom", Exponent: 6}},
		Base:        ibcDenom,
		Display:     "Atom",
		Name:        "Atom",
		Symbol:      "ATOM",
		URI:         "",
		URIHash:     "",
	}
	tv.input.BankKeeper.SetDenomMetaData(tv.ctx, metadata)

	// Add ibcDenom to the allowlist so the ERC20 deployed handler accepts it
	tv.input.GravityKeeper.SetCosmosBridgeableToken(tv.ctx, metadata)

	var (
		myNonce = uint64(2)
	)

	// have all five validators observe this event
	for _, v := range keeper.OrchAddrs {
		ethClaim := types.MsgERC20DeployedClaim{
			EventNonce:     myNonce,
			EthBlockHeight: 0,
			CosmosDenom:    ibcDenom,
			TokenContract:  tokenContract,
			Name:           "Atom",
			Symbol:         "ATOM",
			Decimals:       6,
			Orchestrator:   v.String(),
		}
		_, err := tv.msgServer.ERC20DeployedClaim(tv.ctx, &ethClaim)
		require.NoError(tv.t, err)

		// check if attestations persisted
		hash, err := ethClaim.ClaimHash()
		require.NoError(tv.t, err)
		a := tv.input.GravityKeeper.GetAttestation(tv.ctx, myNonce, hash)
		require.NotNil(tv.t, a)
	}

	EndBlocker(tv.ctx, tv.input.GravityKeeper)

	// check if erc20<>denom relation added to db
	isCosmosOriginated, gotERC20, err := tv.input.GravityKeeper.DenomToERC20Lookup(tv.ctx, tv.denom)
	require.NoError(tv.t, err)
	assert.True(tv.t, isCosmosOriginated)

	ethAddr, err := types.NewEthAddress(tv.erc20)
	require.NoError(tv.t, err)
	isCosmosOriginated, gotDenom := tv.input.GravityKeeper.ERC20ToDenomLookup(tv.ctx, *ethAddr)
	assert.True(tv.t, isCosmosOriginated)

	assert.Equal(tv.t, tv.denom, gotDenom)
	assert.Equal(tv.t, tv.erc20, gotERC20.GetAddress().Hex())
}
