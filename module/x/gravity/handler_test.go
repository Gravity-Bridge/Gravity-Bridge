package gravity

import (
	"bytes"
	"fmt"
	"math/big"
	"testing"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/althea-net/cosmos-gravity-bridge/module/x/gravity/keeper"
	"github.com/althea-net/cosmos-gravity-bridge/module/x/gravity/types"
)

//nolint: exhaustivestruct
func TestHandleMsgSendToEth(t *testing.T) {
	var (
		userCosmosAddr, _               = sdk.AccAddressFromBech32("cosmos1990z7dqsvh8gthw9pa5sn4wuy2xrsd80mg5z6y")
		blockTime                       = time.Date(2020, 9, 14, 15, 20, 10, 0, time.UTC)
		blockHeight           int64     = 200
		denom                           = "gravity0x0bc529c00c6401aef6d220be8c6ea1667f6ad93e"
		startingCoinAmount, _           = sdk.NewIntFromString("150000000000000000000") // 150 ETH worth, required to reach above u64 limit (which is about 18 ETH)
		sendAmount, _                   = sdk.NewIntFromString("50000000000000000000")  // 50 ETH
		feeAmount, _                    = sdk.NewIntFromString("5000000000000000000")   // 5 ETH
		startingCoins         sdk.Coins = sdk.Coins{sdk.NewCoin(denom, startingCoinAmount)}
		sendingCoin           sdk.Coin  = sdk.NewCoin(denom, sendAmount)
		feeCoin               sdk.Coin  = sdk.NewCoin(denom, feeAmount)
		ethDestination                  = "0x3c9289da00b02dC623d0D8D907619890301D26d4"
	)

	// we start by depositing some funds into the users balance to send
	input := keeper.CreateTestEnv(t)
	ctx := input.Context
	h := NewHandler(input.GravityKeeper)
	require.NoError(t, input.BankKeeper.MintCoins(ctx, types.ModuleName, startingCoins))
	input.BankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, userCosmosAddr, startingCoins)
	balance1 := input.BankKeeper.GetAllBalances(ctx, userCosmosAddr)
	assert.Equal(t, sdk.Coins{sdk.NewCoin(denom, startingCoinAmount)}, balance1)

	// send some coins
	msg := &types.MsgSendToEth{
		Sender:    userCosmosAddr.String(),
		EthDest:   ethDestination,
		Amount:    sendingCoin,
		BridgeFee: feeCoin}
	ctx = ctx.WithBlockTime(blockTime).WithBlockHeight(blockHeight)
	_, err := h(ctx, msg)
	require.NoError(t, err)
	balance2 := input.BankKeeper.GetAllBalances(ctx, userCosmosAddr)
	assert.Equal(t, sdk.Coins{sdk.NewCoin(denom, startingCoinAmount.Sub(sendAmount).Sub(feeAmount))}, balance2)

	// do the same thing again and make sure it works twice
	msg1 := &types.MsgSendToEth{
		Sender:    userCosmosAddr.String(),
		EthDest:   ethDestination,
		Amount:    sendingCoin,
		BridgeFee: feeCoin}
	ctx = ctx.WithBlockTime(blockTime).WithBlockHeight(blockHeight)
	_, err1 := h(ctx, msg1)
	require.NoError(t, err1)
	balance3 := input.BankKeeper.GetAllBalances(ctx, userCosmosAddr)
	finalAmount3 := startingCoinAmount.Sub(sendAmount).Sub(sendAmount).Sub(feeAmount).Sub(feeAmount)
	assert.Equal(t, sdk.Coins{sdk.NewCoin(denom, finalAmount3)}, balance3)

	// now we should be out of coins and error
	msg2 := &types.MsgSendToEth{
		Sender:    userCosmosAddr.String(),
		EthDest:   ethDestination,
		Amount:    sendingCoin,
		BridgeFee: feeCoin}
	ctx = ctx.WithBlockTime(blockTime).WithBlockHeight(blockHeight)
	_, err2 := h(ctx, msg2)
	require.Error(t, err2)
	balance4 := input.BankKeeper.GetAllBalances(ctx, userCosmosAddr)
	assert.Equal(t, sdk.Coins{sdk.NewCoin(denom, finalAmount3)}, balance4)
}

//nolint: exhaustivestruct
func TestMsgSendToCosmosClaimSingleValidator(t *testing.T) {
	var (
		myOrchestratorAddr sdk.AccAddress = make([]byte, 20)
		myCosmosAddr, _                   = sdk.AccAddressFromBech32("cosmos16ahjkfqxpp6lvfy9fpfnfjg39xr96qett0alj5")
		myValAddr                         = sdk.ValAddress(myOrchestratorAddr) // revisit when proper mapping is impl in keeper
		myNonce                           = uint64(1)
		anyETHAddr                        = "0xf9613b532673Cc223aBa451dFA8539B87e1F666D"
		tokenETHAddr                      = "0x0bc529c00c6401aef6d220be8c6ea1667f6ad93e"
		myBlockTime                       = time.Date(2020, 9, 14, 15, 20, 10, 0, time.UTC)
		amountA, _                        = sdk.NewIntFromString("50000000000000000000")  // 50 ETH
		amountB, _                        = sdk.NewIntFromString("100000000000000000000") // 100 ETH
	)
	input := keeper.CreateTestEnv(t)
	ctx := input.Context
	input.GravityKeeper.StakingKeeper = keeper.NewStakingKeeperMock(myValAddr)
	input.GravityKeeper.SetEthAddressForValidator(ctx, myValAddr, *types.ZeroAddress())
	input.GravityKeeper.SetOrchestratorValidator(ctx, myValAddr, myOrchestratorAddr)
	h := NewHandler(input.GravityKeeper)

	myErc20 := types.ERC20Token{
		Amount:   amountA,
		Contract: tokenETHAddr,
	}

	ethClaim := types.MsgSendToCosmosClaim{
		EventNonce:     myNonce,
		TokenContract:  myErc20.Contract,
		Amount:         myErc20.Amount,
		EthereumSender: anyETHAddr,
		CosmosReceiver: myCosmosAddr.String(),
		Orchestrator:   myOrchestratorAddr.String(),
	}

	// when
	ctx = ctx.WithBlockTime(myBlockTime)
	_, err := h(ctx, &ethClaim)
	EndBlocker(ctx, input.GravityKeeper)
	require.NoError(t, err)

	// and attestation persisted
	hash, err := ethClaim.ClaimHash()
	require.NoError(t, err)
	a := input.GravityKeeper.GetAttestation(ctx, myNonce, hash)
	require.NotNil(t, a)
	// and vouchers added to the account
	balance := input.BankKeeper.GetAllBalances(ctx, myCosmosAddr)
	assert.Equal(t, sdk.Coins{sdk.NewCoin("gravity0x0bc529c00c6401aef6d220be8c6ea1667f6ad93e", amountA)}, balance)

	// Test to reject duplicate deposit
	// when
	ctx = ctx.WithBlockTime(myBlockTime)
	_, err = h(ctx, &ethClaim)
	EndBlocker(ctx, input.GravityKeeper)
	// then
	require.Error(t, err)
	balance = input.BankKeeper.GetAllBalances(ctx, myCosmosAddr)
	assert.Equal(t, sdk.Coins{sdk.NewCoin("gravity0x0bc529c00c6401aef6d220be8c6ea1667f6ad93e", amountA)}, balance)

	// Test to reject skipped nonce
	ethClaim = types.MsgSendToCosmosClaim{
		EventNonce:     uint64(3),
		TokenContract:  tokenETHAddr,
		Amount:         amountA,
		EthereumSender: anyETHAddr,
		CosmosReceiver: myCosmosAddr.String(),
		Orchestrator:   myOrchestratorAddr.String(),
	}

	// when
	ctx = ctx.WithBlockTime(myBlockTime)
	_, err = h(ctx, &ethClaim)
	EndBlocker(ctx, input.GravityKeeper)
	// then
	require.Error(t, err)
	balance = input.BankKeeper.GetAllBalances(ctx, myCosmosAddr)
	assert.Equal(t, sdk.Coins{sdk.NewCoin("gravity0x0bc529c00c6401aef6d220be8c6ea1667f6ad93e", amountA)}, balance)

	// Test to finally accept consecutive nonce
	ethClaim = types.MsgSendToCosmosClaim{
		EventNonce:     uint64(2),
		Amount:         amountA,
		TokenContract:  tokenETHAddr,
		EthereumSender: anyETHAddr,
		CosmosReceiver: myCosmosAddr.String(),
		Orchestrator:   myOrchestratorAddr.String(),
	}

	// when
	ctx = ctx.WithBlockTime(myBlockTime)
	_, err = h(ctx, &ethClaim)
	EndBlocker(ctx, input.GravityKeeper)

	// then
	require.NoError(t, err)
	balance = input.BankKeeper.GetAllBalances(ctx, myCosmosAddr)
	assert.Equal(t, sdk.Coins{sdk.NewCoin("gravity0x0bc529c00c6401aef6d220be8c6ea1667f6ad93e", amountB)}, balance)
}

const biggestInt = "115792089237316195423570985008687907853269984665640564039457584007913129639935" // 2^256 - 1

// We rely on BitLen() to detect Uint256 overflow, here we ensure BitLen() returns what we expect
func TestUint256BitLen(t *testing.T) {
	biggestBigInt, _ := new(big.Int).SetString(biggestInt, 10)
	require.Equal(t, 256, biggestBigInt.BitLen(), "expected 2^256 - 1 to be represented in 256 bits");
	biggerThanUint256 := biggestBigInt.Add(biggestBigInt, new(big.Int).SetInt64(1)) // add 1 to the max value of Uint256
	require.Equal(t, 257, biggerThanUint256.BitLen(), "expected 2^256 to be represented in 257 bits");
}

func TestMsgSendToCosmosOverflow(t *testing.T) {
	const grandeInt = "115792089237316195423570985008687907853269984665640564039457584007913129639835" // 2^256 - 101
	var (
		biggestBigInt, _                  = new(big.Int).SetString(biggestInt, 10);
		grandeBigInt, _ 				  = new(big.Int).SetString(grandeInt, 10);
		myOrchestratorAddr sdk.AccAddress = make([]byte, 20)
		myCosmosAddr, _                   = sdk.AccAddressFromBech32("cosmos16ahjkfqxpp6lvfy9fpfnfjg39xr96qett0alj5")
		myValAddr                         = sdk.ValAddress(myOrchestratorAddr) // revisit when proper mapping is impl in keeper
		myNonce                           = uint64(1)
		anyETHAddr                        = "0xf9613b532673Cc223aBa451dFA8539B87e1F666D"
		tokenETHAddr1                     = "0x0bc529c00c6401aef6d220be8c6ea1667f6ad93e"
		tokenETHAddr2                     = "0x429881672B9AE42b8EbA0E26cD9C73711b891Ca6"
		myBlockTime                       = time.Date(2020, 9, 14, 15, 20, 10, 0, time.UTC)
		tokenEthAddress1, _               = types.NewEthAddress(tokenETHAddr1)
		tokenEthAddress2, _               = types.NewEthAddress(tokenETHAddr2)
		denom1                            = types.GravityDenom(*tokenEthAddress1)
		denom2                            = types.GravityDenom(*tokenEthAddress2)
	)

	// Totally valid, but we're 101 away from the supply limit
	almostTooMuch := types.ERC20Token{
		Amount:   sdk.NewIntFromBigInt(grandeBigInt),
		Contract: tokenETHAddr1,
	}
	// This takes us past the supply limit of 2^256 - 1
	exactlyTooMuch := types.ERC20Token{
		Amount:   sdk.NewInt(101),
		Contract: tokenETHAddr1,
	}
	almostTooMuchClaim := types.MsgSendToCosmosClaim{
		EventNonce:     myNonce,
		TokenContract:  almostTooMuch.Contract,
		Amount:         almostTooMuch.Amount,
		EthereumSender: anyETHAddr,
		CosmosReceiver: myCosmosAddr.String(),
		Orchestrator:   myOrchestratorAddr.String(),
	}
	exactlyTooMuchClaim := types.MsgSendToCosmosClaim{
		EventNonce:     myNonce+1,
		TokenContract:  exactlyTooMuch.Contract,
		Amount:         exactlyTooMuch.Amount,
		EthereumSender: anyETHAddr,
		CosmosReceiver: myCosmosAddr.String(),
		Orchestrator:   myOrchestratorAddr.String(),
	}
	// Absoulte max value of 2^256 - 1. Previous versions (v0.43 or v0.44) of cosmos-sdk did not support sdk.Int of this size
	maxSend := types.ERC20Token{
		Amount: sdk.NewIntFromBigInt(biggestBigInt),
		Contract: tokenETHAddr2,
	}
	maxSendClaim := types.MsgSendToCosmosClaim{
		EventNonce:     myNonce+2,
		TokenContract:  maxSend.Contract,
		Amount:         maxSend.Amount,
		EthereumSender: anyETHAddr,
		CosmosReceiver: myCosmosAddr.String(),
		Orchestrator:   myOrchestratorAddr.String(),
	}
	// Setup
	input := keeper.CreateTestEnv(t)
	ctx := input.Context
	input.GravityKeeper.StakingKeeper = keeper.NewStakingKeeperMock(myValAddr)
	input.GravityKeeper.SetEthAddressForValidator(ctx, myValAddr, *types.ZeroAddress())
	input.GravityKeeper.SetOrchestratorValidator(ctx, myValAddr, myOrchestratorAddr)
	h := NewHandler(input.GravityKeeper)

	// Require that no tokens were bridged previously
	preSupply1 := input.BankKeeper.GetSupply(ctx, denom1)
	require.Equal(t, sdk.NewInt(0), preSupply1.Amount)

	fmt.Println("<<<<START Expecting to see 'minted coins from module account		module=x/bank amount={2^256 - 101}'")
	// Execute the 2^256 - 101 transaction
	ctx = ctx.WithBlockTime(myBlockTime)
	_, err := h(ctx, &almostTooMuchClaim)
	EndBlocker(ctx, input.GravityKeeper)
	require.NoError(t, err)
	// Require that the actual bridged amount is equal to the amount in our almostTooBigClaim
	middleSupply := input.BankKeeper.GetSupply(ctx, denom1)
	require.Equal(t, almostTooMuch.Amount, middleSupply.Amount.Sub(preSupply1.Amount))
	fmt.Println("END>>>>")

	fmt.Println("<<<<START Expecting to see an error about 'invalid supply after SendToCosmos attestation'")
	// Execute the 101 transaction
	ctx = ctx.WithBlockTime(myBlockTime)
	_, err = h(ctx, &exactlyTooMuchClaim)
	EndBlocker(ctx, input.GravityKeeper)
	require.NoError(t, err)
	// Require that the overflowing amount 101 was not bridged, and instead reverted
	endSupply := input.BankKeeper.GetSupply(ctx, denom1)
	require.Equal(t, almostTooMuch.Amount, endSupply.Amount.Sub(preSupply1.Amount))
	fmt.Println("END>>>>")

	// Require that no tokens were bridged previously
	preSupply2 := input.BankKeeper.GetSupply(ctx, denom2)
	require.Equal(t, sdk.NewInt(0), preSupply2.Amount)

	fmt.Println("<<<<START Expecting to see 'minted coins from module account		module=x/bank amount={2^256 - 1}'")
	// Execute the 2^256 - 1 transaction
	ctx = ctx.WithBlockTime(myBlockTime)
	_, err = h(ctx, &maxSendClaim)
	EndBlocker(ctx, input.GravityKeeper)
	require.NoError(t, err)
	// Require that the actual bridged amount is equal to the amount in our almostTooBigClaim
	maxSendSupply := input.BankKeeper.GetSupply(ctx, denom2)
	require.Equal(t, maxSend.Amount, maxSendSupply.Amount.Sub(preSupply2.Amount))
	fmt.Println("END>>>>")
}

//nolint: exhaustivestruct
func TestMsgSendToCosmosClaimsMultiValidator(t *testing.T) {
	var (
		orchestratorAddr1, _ = sdk.AccAddressFromBech32("cosmos1dg55rtevlfxh46w88yjpdd08sqhh5cc3xhkcej")
		orchestratorAddr2, _ = sdk.AccAddressFromBech32("cosmos164knshrzuuurf05qxf3q5ewpfnwzl4gj4m4dfy")
		orchestratorAddr3, _ = sdk.AccAddressFromBech32("cosmos193fw83ynn76328pty4yl7473vg9x86alq2cft7")
		validatorEthAddr1, _ = types.NewEthAddress("0x0000000000000000000000000000000000000001")
		validatorEthAddr2, _ = types.NewEthAddress("0x0000000000000000000000000000000000000002")
		validatorEthAddr3, _ = types.NewEthAddress("0x0000000000000000000000000000000000000003")
		myCosmosAddr, _      = sdk.AccAddressFromBech32("cosmos16ahjkfqxpp6lvfy9fpfnfjg39xr96qett0alj5")
		valAddr1             = sdk.ValAddress(orchestratorAddr1) // revisit when proper mapping is impl in keeper
		valAddr2             = sdk.ValAddress(orchestratorAddr2) // revisit when proper mapping is impl in keeper
		valAddr3             = sdk.ValAddress(orchestratorAddr3) // revisit when proper mapping is impl in keeper
		myNonce              = uint64(1)
		anyETHAddr           = "0xf9613b532673Cc223aBa451dFA8539B87e1F666D"
		tokenETHAddr         = "0x0bc529c00c6401aef6d220be8c6ea1667f6ad93e"
		myBlockTime          = time.Date(2020, 9, 14, 15, 20, 10, 0, time.UTC)
	)
	input := keeper.CreateTestEnv(t)
	ctx := input.Context
	input.GravityKeeper.StakingKeeper = keeper.NewStakingKeeperMock(valAddr1, valAddr2, valAddr3)
	input.GravityKeeper.SetEthAddressForValidator(ctx, valAddr1, *validatorEthAddr1)
	input.GravityKeeper.SetEthAddressForValidator(ctx, valAddr2, *validatorEthAddr2)
	input.GravityKeeper.SetEthAddressForValidator(ctx, valAddr3, *validatorEthAddr3)
	input.GravityKeeper.SetOrchestratorValidator(ctx, valAddr1, orchestratorAddr1)
	input.GravityKeeper.SetOrchestratorValidator(ctx, valAddr2, orchestratorAddr2)
	input.GravityKeeper.SetOrchestratorValidator(ctx, valAddr3, orchestratorAddr3)
	h := NewHandler(input.GravityKeeper)

	myErc20 := types.ERC20Token{
		Amount:   sdk.NewInt(12),
		Contract: tokenETHAddr,
	}

	ethClaim1 := types.MsgSendToCosmosClaim{
		EventNonce:     myNonce,
		TokenContract:  myErc20.Contract,
		Amount:         myErc20.Amount,
		EthereumSender: anyETHAddr,
		CosmosReceiver: myCosmosAddr.String(),
		Orchestrator:   orchestratorAddr1.String(),
	}
	ethClaim2 := types.MsgSendToCosmosClaim{
		EventNonce:     myNonce,
		TokenContract:  myErc20.Contract,
		Amount:         myErc20.Amount,
		EthereumSender: anyETHAddr,
		CosmosReceiver: myCosmosAddr.String(),
		Orchestrator:   orchestratorAddr2.String(),
	}
	ethClaim3 := types.MsgSendToCosmosClaim{
		EventNonce:     myNonce,
		TokenContract:  myErc20.Contract,
		Amount:         myErc20.Amount,
		EthereumSender: anyETHAddr,
		CosmosReceiver: myCosmosAddr.String(),
		Orchestrator:   orchestratorAddr3.String(),
	}

	// when
	ctx = ctx.WithBlockTime(myBlockTime)
	_, err := h(ctx, &ethClaim1)
	EndBlocker(ctx, input.GravityKeeper)
	require.NoError(t, err)
	// and attestation persisted
	hash1, err := ethClaim1.ClaimHash()
	require.NoError(t, err)
	a1 := input.GravityKeeper.GetAttestation(ctx, myNonce, hash1)
	require.NotNil(t, a1)
	// and vouchers not yet added to the account
	balance1 := input.BankKeeper.GetAllBalances(ctx, myCosmosAddr)
	assert.NotEqual(t, sdk.Coins{sdk.NewInt64Coin("gravity0x0bc529c00c6401aef6d220be8c6ea1667f6ad93e", 12)}, balance1)

	// when
	ctx = ctx.WithBlockTime(myBlockTime)
	_, err = h(ctx, &ethClaim2)
	EndBlocker(ctx, input.GravityKeeper)
	require.NoError(t, err)

	// and attestation persisted
	a2 := input.GravityKeeper.GetAttestation(ctx, myNonce, hash1)
	require.NotNil(t, a2)
	// and vouchers now added to the account
	balance2 := input.BankKeeper.GetAllBalances(ctx, myCosmosAddr)
	assert.Equal(t, sdk.Coins{sdk.NewInt64Coin("gravity0x0bc529c00c6401aef6d220be8c6ea1667f6ad93e", 12)}, balance2)

	// when
	ctx = ctx.WithBlockTime(myBlockTime)
	_, err = h(ctx, &ethClaim3)
	EndBlocker(ctx, input.GravityKeeper)
	require.NoError(t, err)

	// and attestation persisted
	a3 := input.GravityKeeper.GetAttestation(ctx, myNonce, hash1)
	require.NotNil(t, a3)
	// and no additional added to the account
	balance3 := input.BankKeeper.GetAllBalances(ctx, myCosmosAddr)
	assert.Equal(t, sdk.Coins{sdk.NewInt64Coin("gravity0x0bc529c00c6401aef6d220be8c6ea1667f6ad93e", 12)}, balance3)
}

//nolint: exhaustivestruct
func TestMsgSetOrchestratorAddresses(t *testing.T) {
	var (
		ethAddress, _                 = types.NewEthAddress("0xb462864E395d88d6bc7C5dd5F3F5eb4cc2599255")
		cosmosAddress  sdk.AccAddress = bytes.Repeat([]byte{0x1}, 20)
		ethAddress2, _                = types.NewEthAddress("0x26126048c706fB45a5a6De8432F428e794d0b952")
		cosmosAddress2 sdk.AccAddress = bytes.Repeat([]byte{0x2}, 20)
		valAddress     sdk.ValAddress = bytes.Repeat([]byte{0x2}, 20)
		blockTime                     = time.Date(2020, 9, 14, 15, 20, 10, 0, time.UTC)
		blockTime2                    = time.Date(2020, 9, 15, 15, 20, 10, 0, time.UTC)
		blockHeight    int64          = 200
		blockHeight2   int64          = 210
	)
	input := keeper.CreateTestEnv(t)
	input.GravityKeeper.StakingKeeper = keeper.NewStakingKeeperMock(valAddress)
	ctx := input.Context
	wctx := sdk.WrapSDKContext(ctx)
	k := input.GravityKeeper
	h := NewHandler(input.GravityKeeper)
	ctx = ctx.WithBlockTime(blockTime)

	// test setting keys
	msg := types.NewMsgSetOrchestratorAddress(valAddress, cosmosAddress, *ethAddress)
	ctx = ctx.WithBlockTime(blockTime).WithBlockHeight(blockHeight)
	_, err := h(ctx, msg)
	require.NoError(t, err)

	// test all lookup methods

	// individual lookups
	ethLookup, found := k.GetEthAddressByValidator(ctx, valAddress)
	assert.True(t, found)
	assert.Equal(t, ethLookup, ethAddress)

	valLookup, found := k.GetOrchestratorValidator(ctx, cosmosAddress)
	assert.True(t, found)
	assert.Equal(t, valLookup.GetOperator(), valAddress)

	// query endpoints
	queryO := types.QueryDelegateKeysByOrchestratorAddress{
		OrchestratorAddress: cosmosAddress.String(),
	}
	_, err = k.GetDelegateKeyByOrchestrator(wctx, &queryO)
	require.NoError(t, err)

	queryE := types.QueryDelegateKeysByEthAddress{
		EthAddress: ethAddress.GetAddress(),
	}
	_, err = k.GetDelegateKeyByEth(wctx, &queryE)
	require.NoError(t, err)

	// try to set values again. This should fail see issue #344 for why allowing this
	// would require keeping a history of all validators delegate keys forever
	msg = types.NewMsgSetOrchestratorAddress(valAddress, cosmosAddress2, *ethAddress2)
	ctx = ctx.WithBlockTime(blockTime2).WithBlockHeight(blockHeight2)
	_, err = h(ctx, msg)
	require.Error(t, err)
}
