package keeper

import (
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sdkmath "cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	disttypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	govkeeper "github.com/cosmos/cosmos-sdk/x/gov/keeper"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	govv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	govv1beta1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1beta1"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
	typesv2 "github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types/v2"
)

// nolint: exhaustruct
func TestAirdropProposal(t *testing.T) {
	input := CreateTestEnv(t)
	defer func() { input.Context.Logger().Info("Asserting invariants at test end"); input.AssertInvariants() }()

	ctx := input.Context

	testAddr := []string{"gravity1ahx7f8wyertuus9r20284ej0asrs085ceqtfnm", "gravity1n38caqg63jf9hefycw3yp95fpkpk669nvekqy2", "gravity1qz4zm5s0vwfuu46lg3q0vmnwsukd8e9yfmcgjj"}

	parsedRecipients := make([]sdk.AccAddress, len(testAddr))
	for i, v := range testAddr {
		parsed, err := sdk.AccAddressFromBech32(v)
		require.NoError(t, err)
		parsedRecipients[i] = parsed
	}
	byteEncodedRecipients := []byte{}
	for _, v := range parsedRecipients {
		byteEncodedRecipients = append(byteEncodedRecipients, v.Bytes()...)
	}

	extremelyLargeAmount := sdkmath.NewInt(1000000000000).Mul(sdkmath.NewInt(1000000000000))
	require.False(t, extremelyLargeAmount.IsUint64())

	goodAirdrop := types.AirdropProposal{
		Title:       "test tile",
		Description: "test description",
		Denom:       "grav",
		Amounts:     []uint64{1000, 900, 1100},
		Recipients:  byteEncodedRecipients,
	}
	airdropTooBig := goodAirdrop
	airdropTooBig.Amounts = []uint64{100000, 100000, 100000}
	airdropLarge := goodAirdrop
	airdropLarge.Amounts = []uint64{18446744073709551614, 18446744073709551614, 18446744073709551614}
	airdropBadToken := goodAirdrop
	airdropBadToken.Denom = "notreal"
	airdropAmountsMismatch := goodAirdrop
	airdropAmountsMismatch.Amounts = []uint64{1000, 1000}
	airdropBadDest := goodAirdrop
	airdropBadDest.Recipients = []byte{0, 1, 2, 3, 4}
	gk := input.GravityKeeper

	feePoolBalance := sdk.NewInt64Coin("grav", 10000)
	feePool, err := gk.DistKeeper.FeePool.Get(ctx)
	require.NoError(t, err)
	newCoins := feePool.CommunityPool.Add(sdk.NewDecCoins(sdk.NewDecCoinFromCoin(feePoolBalance))...)
	feePool.CommunityPool = newCoins
	err = gk.DistKeeper.FeePool.Set(ctx, feePool)
	require.NoError(t, err)
	// test that we are actually setting the fee pool
	fp, err := input.DistKeeper.FeePool.Get(ctx)
	require.NoError(t, err)
	assert.Equal(t, fp, feePool)
	// mint the actual coins
	require.NoError(t, input.BankKeeper.MintCoins(ctx, types.ModuleName, sdk.NewCoins(feePoolBalance)))
	require.NoError(t, input.BankKeeper.SendCoinsFromModuleToModule(ctx, types.ModuleName, disttypes.ModuleName, sdk.NewCoins(feePoolBalance)))

	err = gk.HandleAirdropProposal(ctx, &airdropTooBig)
	require.Error(t, err)
	input.AssertInvariants()

	err = gk.HandleAirdropProposal(ctx, &airdropBadToken)
	require.Error(t, err)
	input.AssertInvariants()

	err = gk.HandleAirdropProposal(ctx, &airdropAmountsMismatch)
	require.Error(t, err)
	input.AssertInvariants()

	err = gk.HandleAirdropProposal(ctx, &airdropBadDest)
	require.Error(t, err)
	input.AssertInvariants()

	err = gk.HandleAirdropProposal(ctx, &goodAirdrop)
	require.NoError(t, err)
	feePool, err = gk.DistKeeper.FeePool.Get(ctx)
	require.NoError(t, err)
	assert.Equal(t, feePool.CommunityPool.AmountOf("grav"), sdk.NewInt64DecCoin("grav", 7000).Amount)
	input.AssertInvariants()

	// now we test with extremely large amounts, specifically to get to rounding errors
	feePoolBalance = sdk.NewCoin("grav", extremelyLargeAmount)
	feePool, err = gk.DistKeeper.FeePool.Get(ctx)
	require.NoError(t, err)
	newCoins = feePool.CommunityPool.Add(sdk.NewDecCoins(sdk.NewDecCoinFromCoin(feePoolBalance))...)
	feePool.CommunityPool = newCoins
	err = gk.DistKeeper.FeePool.Set(ctx, feePool)
	require.NoError(t, err)
	// test that we are actually setting the fee pool
	fp, err = input.DistKeeper.FeePool.Get(ctx)
	require.NoError(t, err)
	assert.Equal(t, fp, feePool)
	// mint the actual coins
	require.NoError(t, input.BankKeeper.MintCoins(ctx, types.ModuleName, sdk.NewCoins(feePoolBalance)))
	require.NoError(t, input.BankKeeper.SendCoinsFromModuleToModule(ctx, types.ModuleName, disttypes.ModuleName, sdk.NewCoins(feePoolBalance)))

	err = gk.HandleAirdropProposal(ctx, &airdropLarge)
	require.NoError(t, err)
	feePool, err = gk.DistKeeper.FeePool.Get(ctx)
	require.NoError(t, err)
	input.AssertInvariants()
}

// Tests the new airdrop proposal message handler's verification of the authority by submitting
// a valid proposal with the correct authority and then submitting the same proposal with
// a large number of random authorities to ensure that the authority check is functioning correctly.
func TestMsgAirdropProposal(t *testing.T) {
	numFalseAuthorities := 10000
	input := CreateTestEnv(t)
	defer func() { input.Context.Logger().Info("Asserting invariants at test end"); input.AssertInvariants() }()

	ctx := input.Context

	testAddr := []string{"gravity1ahx7f8wyertuus9r20284ej0asrs085ceqtfnm", "gravity1n38caqg63jf9hefycw3yp95fpkpk669nvekqy2", "gravity1qz4zm5s0vwfuu46lg3q0vmnwsukd8e9yfmcgjj"}

	parsedRecipients := make([]sdk.AccAddress, len(testAddr))
	for i, v := range testAddr {
		parsed, err := sdk.AccAddressFromBech32(v)
		require.NoError(t, err)
		parsedRecipients[i] = parsed
	}
	byteEncodedRecipients := []byte{}
	for _, v := range parsedRecipients {
		byteEncodedRecipients = append(byteEncodedRecipients, v.Bytes()...)
	}

	gk := input.GravityKeeper
	feePoolBalance := sdk.NewInt64Coin("ugraviton", 1000000000000)
	feePool, err := gk.DistKeeper.FeePool.Get(ctx)
	require.NoError(t, err)
	newCoins := feePool.CommunityPool.Add(sdk.NewDecCoins(sdk.NewDecCoinFromCoin(feePoolBalance))...)
	feePool.CommunityPool = newCoins
	err = gk.DistKeeper.FeePool.Set(ctx, feePool)
	require.NoError(t, err)
	// test that we are actually setting the fee pool
	updatedFeePool, err := gk.DistKeeper.FeePool.Get(ctx)
	require.NoError(t, err)
	assert.Equal(t, updatedFeePool, feePool)
	// mint the actual coins
	require.NoError(t, input.BankKeeper.MintCoins(ctx, types.ModuleName, sdk.NewCoins(feePoolBalance)))
	require.NoError(t, input.BankKeeper.SendCoinsFromModuleToModule(ctx, types.ModuleName, disttypes.ModuleName, sdk.NewCoins(feePoolBalance)))

	proposal := types.AirdropProposal{
		Title:       "Airdrop Proposal",
		Description: "Proposal description",
		Denom:       "ugraviton",
		Amounts:     []uint64{1000, 900, 1100},
		Recipients:  byteEncodedRecipients,
	}

	msgServer := msgServer{input.GravityKeeper}

	authority := input.GravityKeeper.GetAuthority()
	msgProposal := typesv2.MsgAirdropProposal{
		Authority: authority,
		Proposal:  &proposal,
	}

	// Test the Airdrop Proposal with the correct authority
	_, err = msgServer.AirdropProposal(ctx, &msgProposal)
	require.NoError(t, err)

	for range numFalseAuthorities {
		privKey := secp256k1.GenPrivKey()
		address := sdk.AccAddress(privKey.PubKey().Address())
		authority = address.String()
		if authority == input.GravityKeeper.GetAuthority() {
			continue
		}
		msgProposal.Authority = authority
		// Test the Airdrop Proposal with an incorrect authority
		_, err = msgServer.AirdropProposal(ctx, &msgProposal)
		require.Error(t, err, "Expected error for authority %s", authority)
	}
}

func TestMsgUnhaltBridgeProposal(t *testing.T) {
	numFalseAuthorities := 10000
	invalidAuthorityError := "expected gov account as only signer for proposal message"

	input := CreateTestEnv(t)
	defer func() { input.Context.Logger().Info("Asserting invariants at test end"); input.AssertInvariants() }()

	ctx := input.Context
	proposal := types.UnhaltBridgeProposal{
		Title:       "test tile",
		Description: "test description",
		TargetNonce: 1,
	}

	msgServer := msgServer{input.GravityKeeper}

	// nolint: exhaustruct
	msgProposal := typesv2.MsgUnhaltBridgeProposal{
		Proposal: &proposal,
	}

	for range numFalseAuthorities {
		privKey := secp256k1.GenPrivKey()
		address := sdk.AccAddress(privKey.PubKey().Address())
		authority := address.String()
		if authority == input.GravityKeeper.GetAuthority() {
			continue
		}

		msgProposal.Authority = authority
		// Test the Airdrop Proposal with an incorrect authority
		_, err := msgServer.UnhaltBridgeProposal(ctx, &msgProposal)
		require.Contains(t, err.Error(), invalidAuthorityError)
	}

	// Finally, perform a good test with the proper authority
	authority := input.GravityKeeper.GetAuthority()
	msgProposal.Authority = authority
	_, err := msgServer.UnhaltBridgeProposal(ctx, &msgProposal)
	require.NoError(t, err)
}

func TestMsgUpdateParamsProposal(t *testing.T) {
	input := CreateTestEnv(t)
	defer func() { input.Context.Logger().Info("Asserting invariants at test end"); input.AssertInvariants() }()

	govAddress := authtypes.NewModuleAddress(govtypes.ModuleName)

	ctx := input.Context

	//nolint: exhaustruct
	gravityId := typesv2.Param{
		Key:   "GravityId",
		Value: "1",
	}
	//nolint: exhaustruct
	contractSourceHash := typesv2.Param{
		Key:   "ContractSourceHash",
		Value: "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
	}
	//nolint: exhaustruct
	bridgeEthereumAddress := typesv2.Param{
		Key:   "BridgeEthereumAddress",
		Value: "0x0000000000000000000000000000000000000000",
	}
	//nolint: exhaustruct
	bridgeChainId := typesv2.Param{
		Key:   "BridgeChainId",
		Value: "1",
	}
	//nolint: exhaustruct
	signedValsetsWindow := typesv2.Param{
		Key:   "SignedValsetsWindow",
		Value: "100",
	}
	//nolint: exhaustruct
	signedBatchesWindow := typesv2.Param{
		Key:   "SignedBatchesWindow",
		Value: "100",
	}
	//nolint: exhaustruct
	signedLogicCallsWindow := typesv2.Param{
		Key:   "SignedLogicCallsWindow",
		Value: "100",
	}
	//nolint: exhaustruct
	targetBatchTimeout := typesv2.Param{
		Key:   "TargetBatchTimeout",
		Value: "1000000",
	}
	//nolint: exhaustruct
	averageBlockTime := typesv2.Param{
		Key:   "AverageBlockTime",
		Value: "1000",
	}
	//nolint: exhaustruct
	averageEthereumBlockTime := typesv2.Param{
		Key:   "AverageEthereumBlockTime",
		Value: "25000",
	}
	//nolint: exhaustruct
	slashFractionValset := typesv2.Param{
		Key:   "SlashFractionValset",
		Value: "0.020000000000000000",
	}
	//nolint: exhaustruct
	slashFractionBatch := typesv2.Param{
		Key:   "SlashFractionBatch",
		Value: "0.020000000000000000",
	}
	//nolint: exhaustruct
	slashFractionLogicCall := typesv2.Param{
		Key:   "SlashFractionLogicCall",
		Value: "0.010000000000000000",
	}
	//nolint: exhaustruct
	unbondSlashingValsetsWindow := typesv2.Param{
		Key:   "UnbondSlashingValsetsWindow",
		Value: "100",
	}
	//nolint: exhaustruct
	slashFractionBadEthSignature := typesv2.Param{
		Key:   "SlashFractionBadEthSignature",
		Value: "0.020000000000000000",
	}
	//nolint: exhaustruct
	valsetReward := typesv2.Param{
		Key:   "ValsetReward",
		Value: "10ugraviton",
	}
	//nolint: exhaustruct
	bridgeActive := typesv2.Param{
		Key:   "BridgeActive",
		Value: "false",
	}
	//nolint: exhaustruct
	ethereumBlacklist := typesv2.Param{
		Key:   "EthereumBlacklist",
		Value: "[\"0x0000000000000000000000000000000000000000\"]",
	}
	//nolint: exhaustruct
	minChainFeeBasisPoints := typesv2.Param{
		Key:   "MinChainFeeBasisPoints",
		Value: "100",
	}
	//nolint: exhaustruct
	chainFeeAuctionPoolFraction := typesv2.Param{
		Key:   "ChainFeeAuctionPoolFraction",
		Value: "0.100000000000000000",
	}

	testCases := []struct {
		name           string
		msg            typesv2.MsgUpdateParamsProposal
		expectError    bool
		expectedParams func(input TestInput) types.Params
	}{
		{
			name: "All fields set",
			msg: typesv2.MsgUpdateParamsProposal{
				Authority: govAddress.String(),
				//nolint: exhaustruct
				ParamUpdates: []*typesv2.Param{
					&gravityId, &contractSourceHash, &bridgeEthereumAddress, &bridgeChainId,
					&signedValsetsWindow, &signedBatchesWindow, &signedLogicCallsWindow,
					&targetBatchTimeout, &averageBlockTime, &averageEthereumBlockTime,
					&slashFractionValset, &slashFractionBatch, &slashFractionLogicCall,
					&unbondSlashingValsetsWindow, &slashFractionBadEthSignature,
					&valsetReward, &bridgeActive, &ethereumBlacklist,
					&minChainFeeBasisPoints, &chainFeeAuctionPoolFraction,
				},
			},
			expectError: false,
			expectedParams: func(input TestInput) types.Params {
				return types.Params{
					GravityId:                    gravityId.Value,
					ContractSourceHash:           contractSourceHash.Value,
					BridgeEthereumAddress:        bridgeEthereumAddress.Value,
					BridgeChainId:                1,
					SignedValsetsWindow:          100,
					SignedBatchesWindow:          100,
					SignedLogicCallsWindow:       100,
					TargetBatchTimeout:           1000000,
					AverageBlockTime:             1000,
					AverageEthereumBlockTime:     25000,
					SlashFractionValset:          sdkmath.LegacyMustNewDecFromStr(slashFractionValset.Value),
					SlashFractionBatch:           sdkmath.LegacyMustNewDecFromStr(slashFractionBatch.Value),
					SlashFractionLogicCall:       sdkmath.LegacyMustNewDecFromStr(slashFractionLogicCall.Value),
					UnbondSlashingValsetsWindow:  100,
					SlashFractionBadEthSignature: sdkmath.LegacyMustNewDecFromStr(slashFractionBadEthSignature.Value),
					ValsetReward:                 sdk.NewCoin("ugraviton", sdkmath.NewInt(10)),
					BridgeActive:                 false,
					EthereumBlacklist:            []string{"0x0000000000000000000000000000000000000000"},
					MinChainFeeBasisPoints:       100,
					ChainFeeAuctionPoolFraction:  sdkmath.LegacyMustNewDecFromStr(chainFeeAuctionPoolFraction.Value),
				}
			},
		},
		{
			name: "Update only GravityId",
			msg: typesv2.MsgUpdateParamsProposal{
				Authority: govAddress.String(),
				//nolint: exhaustruct
				ParamUpdates: []*typesv2.Param{
					&gravityId,
				},
			},
			expectError: false,
			expectedParams: func(input TestInput) types.Params {
				params, err := input.GravityKeeper.GetParams(input.Context)
				require.NoError(t, err)
				params.GravityId = gravityId.Value
				return params
			},
		},
		{
			name: "Update only ContractSourceHash",
			msg: typesv2.MsgUpdateParamsProposal{
				Authority: govAddress.String(),
				//nolint: exhaustruct
				ParamUpdates: []*typesv2.Param{
					&contractSourceHash,
				},
			},
			expectError: false,
			expectedParams: func(input TestInput) types.Params {
				params, err := input.GravityKeeper.GetParams(input.Context)
				require.NoError(t, err)
				params.ContractSourceHash = contractSourceHash.Value
				return params
			},
		},
		{
			name: "Update only BridgeEthereumAddress",
			msg: typesv2.MsgUpdateParamsProposal{
				Authority: govAddress.String(),
				//nolint: exhaustruct
				ParamUpdates: []*typesv2.Param{
					&bridgeEthereumAddress,
				},
			},
			expectError: false,
			expectedParams: func(input TestInput) types.Params {
				params, err := input.GravityKeeper.GetParams(input.Context)
				require.NoError(t, err)
				params.BridgeEthereumAddress = bridgeEthereumAddress.Value
				return params
			},
		},
		{
			name: "Update only BridgeChainId",
			msg: typesv2.MsgUpdateParamsProposal{
				Authority: govAddress.String(),
				//nolint: exhaustruct
				ParamUpdates: []*typesv2.Param{
					&bridgeChainId,
				},
			},
			expectError: false,
			expectedParams: func(input TestInput) types.Params {
				params, err := input.GravityKeeper.GetParams(input.Context)
				require.NoError(t, err)
				params.BridgeChainId = 1
				return params
			},
		},
		{
			name: "Update only SignedValsetsWindow",
			msg: typesv2.MsgUpdateParamsProposal{
				Authority: govAddress.String(),
				//nolint: exhaustruct
				ParamUpdates: []*typesv2.Param{
					&signedValsetsWindow,
				},
			},
			expectError: false,
			expectedParams: func(input TestInput) types.Params {
				params, err := input.GravityKeeper.GetParams(input.Context)
				require.NoError(t, err)
				params.SignedValsetsWindow = 100
				return params
			},
		},
		{
			name: "Update only SignedBatchesWindow",
			msg: typesv2.MsgUpdateParamsProposal{
				Authority: govAddress.String(),
				//nolint: exhaustruct
				ParamUpdates: []*typesv2.Param{
					&signedBatchesWindow,
				},
			},
			expectError: false,
			expectedParams: func(input TestInput) types.Params {
				params, err := input.GravityKeeper.GetParams(input.Context)
				require.NoError(t, err)
				params.SignedBatchesWindow = 100
				return params
			},
		},
		{
			name: "Update only SignedLogicCallsWindow",
			msg: typesv2.MsgUpdateParamsProposal{
				Authority: govAddress.String(),
				//nolint: exhaustruct
				ParamUpdates: []*typesv2.Param{
					&signedLogicCallsWindow,
				},
			},
			expectError: false,
			expectedParams: func(input TestInput) types.Params {
				params, err := input.GravityKeeper.GetParams(input.Context)
				require.NoError(t, err)
				params.SignedLogicCallsWindow = 100
				return params
			},
		},
		{
			name: "Update only TargetBatchTimeout",
			msg: typesv2.MsgUpdateParamsProposal{
				Authority: govAddress.String(),
				//nolint: exhaustruct
				ParamUpdates: []*typesv2.Param{
					&targetBatchTimeout,
				},
			},
			expectError: false,
			expectedParams: func(input TestInput) types.Params {
				params, err := input.GravityKeeper.GetParams(input.Context)
				require.NoError(t, err)
				params.TargetBatchTimeout = 1000000
				return params
			},
		},
		{
			name: "Update only AverageBlockTime",
			msg: typesv2.MsgUpdateParamsProposal{
				Authority: govAddress.String(),
				//nolint: exhaustruct
				ParamUpdates: []*typesv2.Param{
					&averageBlockTime,
				},
			},
			expectError: false,
			expectedParams: func(input TestInput) types.Params {
				params, err := input.GravityKeeper.GetParams(input.Context)
				require.NoError(t, err)
				params.AverageBlockTime = 1000
				return params
			},
		},
		{
			name: "Update only AverageEthereumBlockTime",
			msg: typesv2.MsgUpdateParamsProposal{
				Authority: govAddress.String(),
				//nolint: exhaustruct
				ParamUpdates: []*typesv2.Param{
					&averageEthereumBlockTime,
				},
			},
			expectError: false,
			expectedParams: func(input TestInput) types.Params {
				params, err := input.GravityKeeper.GetParams(input.Context)
				require.NoError(t, err)
				params.AverageEthereumBlockTime = 25000
				return params
			},
		},
		{
			name: "Update only SlashFractionValset",
			msg: typesv2.MsgUpdateParamsProposal{
				Authority: govAddress.String(),
				//nolint: exhaustruct
				ParamUpdates: []*typesv2.Param{
					&slashFractionValset,
				},
			},
			expectError: false,
			expectedParams: func(input TestInput) types.Params {
				params, err := input.GravityKeeper.GetParams(input.Context)
				require.NoError(t, err)
				params.SlashFractionValset = sdkmath.LegacyMustNewDecFromStr(slashFractionValset.Value)
				return params
			},
		},
		{
			name: "Update only SlashFractionBatch",
			msg: typesv2.MsgUpdateParamsProposal{
				Authority: govAddress.String(),
				//nolint: exhaustruct
				ParamUpdates: []*typesv2.Param{
					&slashFractionBatch,
				},
			},
			expectError: false,
			expectedParams: func(input TestInput) types.Params {
				params, err := input.GravityKeeper.GetParams(input.Context)
				require.NoError(t, err)
				params.SlashFractionBatch = sdkmath.LegacyMustNewDecFromStr(slashFractionBatch.Value)
				return params
			},
		},
		{
			name: "Update only SlashFractionLogicCall",
			msg: typesv2.MsgUpdateParamsProposal{
				Authority: govAddress.String(),
				//nolint: exhaustruct
				ParamUpdates: []*typesv2.Param{
					&slashFractionLogicCall,
				},
			},
			expectError: false,
			expectedParams: func(input TestInput) types.Params {
				params, err := input.GravityKeeper.GetParams(input.Context)
				require.NoError(t, err)
				params.SlashFractionLogicCall = sdkmath.LegacyMustNewDecFromStr(slashFractionLogicCall.Value)
				return params
			},
		},
		{
			name: "Update only UnbondSlashingValsetsWindow",
			msg: typesv2.MsgUpdateParamsProposal{
				Authority: govAddress.String(),
				//nolint: exhaustruct
				ParamUpdates: []*typesv2.Param{
					&unbondSlashingValsetsWindow,
				},
			},
			expectError: false,
			expectedParams: func(input TestInput) types.Params {
				params, err := input.GravityKeeper.GetParams(input.Context)
				require.NoError(t, err)
				params.UnbondSlashingValsetsWindow = 100
				return params
			},
		},
		{
			name: "Update only SlashFractionBadEthSignature",
			msg: typesv2.MsgUpdateParamsProposal{
				Authority: govAddress.String(),
				//nolint: exhaustruct
				ParamUpdates: []*typesv2.Param{
					&slashFractionBadEthSignature,
				},
			},
			expectError: false,
			expectedParams: func(input TestInput) types.Params {
				params, err := input.GravityKeeper.GetParams(input.Context)
				require.NoError(t, err)
				params.SlashFractionBadEthSignature = sdkmath.LegacyMustNewDecFromStr(slashFractionBadEthSignature.Value)
				return params
			},
		},
		{
			name: "Update only ValsetReward",
			msg: typesv2.MsgUpdateParamsProposal{
				Authority: govAddress.String(),
				//nolint: exhaustruct
				ParamUpdates: []*typesv2.Param{
					&valsetReward,
				},
			},
			expectError: false,
			expectedParams: func(input TestInput) types.Params {
				params, err := input.GravityKeeper.GetParams(input.Context)
				require.NoError(t, err)
				params.ValsetReward = sdk.NewCoin("ugraviton", sdkmath.NewInt(10))
				return params
			},
		},
		{
			name: "Update only BridgeActive",
			msg: typesv2.MsgUpdateParamsProposal{
				Authority: govAddress.String(),
				//nolint: exhaustruct
				ParamUpdates: []*typesv2.Param{
					&bridgeActive,
				},
			},
			expectError: false,
			expectedParams: func(input TestInput) types.Params {
				params, err := input.GravityKeeper.GetParams(input.Context)
				require.NoError(t, err)
				params.BridgeActive = false
				return params
			},
		},
		{
			name: "Update only EthereumBlacklist",
			msg: typesv2.MsgUpdateParamsProposal{
				Authority: govAddress.String(),
				//nolint: exhaustruct
				ParamUpdates: []*typesv2.Param{
					&ethereumBlacklist,
				},
			},
			expectError: false,
			expectedParams: func(input TestInput) types.Params {
				params, err := input.GravityKeeper.GetParams(input.Context)
				require.NoError(t, err)
				params.EthereumBlacklist = []string{"0x0000000000000000000000000000000000000000"}
				return params
			},
		},
		{
			name: "Update only MinChainFeeBasisPoints",
			msg: typesv2.MsgUpdateParamsProposal{
				Authority: govAddress.String(),
				//nolint: exhaustruct
				ParamUpdates: []*typesv2.Param{
					&minChainFeeBasisPoints,
				},
			},
			expectError: false,
			expectedParams: func(input TestInput) types.Params {
				params, err := input.GravityKeeper.GetParams(input.Context)
				require.NoError(t, err)
				params.MinChainFeeBasisPoints = 100
				return params
			},
		},
		{
			name: "Update only ChainFeeAuctionPoolFraction",
			msg: typesv2.MsgUpdateParamsProposal{
				Authority: govAddress.String(),
				//nolint: exhaustruct
				ParamUpdates: []*typesv2.Param{
					&chainFeeAuctionPoolFraction,
				},
			},
			expectError: false,
			expectedParams: func(input TestInput) types.Params {
				params, err := input.GravityKeeper.GetParams(input.Context)
				require.NoError(t, err)
				params.ChainFeeAuctionPoolFraction = sdkmath.LegacyMustNewDecFromStr(chainFeeAuctionPoolFraction.Value)
				return params
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cacheCtx, _ := ctx.CacheContext()
			msgServer := msgServer{input.GravityKeeper}
			expectedParams := tc.expectedParams(input)
			_, err := msgServer.UpdateParamsProposal(cacheCtx, &tc.msg)
			if tc.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				params, err := input.GravityKeeper.GetParams(cacheCtx)
				require.NoError(t, err)

				fmt.Println("Expected Params:", expectedParams)
				fmt.Println("Actual Params:", params)
				require.Equal(t, expectedParams.GravityId, params.GravityId, "Expected gravity id to match after proposal execution")
				require.Equal(t, expectedParams.ContractSourceHash, params.ContractSourceHash, "Expected contract source hash to match after proposal execution")
				require.Equal(t, expectedParams.BridgeEthereumAddress, params.BridgeEthereumAddress, "Expected bridge ethereum address to match after proposal execution")
				require.Equal(t, expectedParams.BridgeChainId, params.BridgeChainId, "Expected bridge chain id to match after proposal execution")
				require.Equal(t, expectedParams.SignedValsetsWindow, params.SignedValsetsWindow, "Expected signed valsets window to match after proposal execution")
				require.Equal(t, expectedParams.SignedBatchesWindow, params.SignedBatchesWindow, "Expected signed batches window to match after proposal execution")
				require.Equal(t, expectedParams.SignedLogicCallsWindow, params.SignedLogicCallsWindow, "Expected signed logic calls window to match after proposal execution")
				require.Equal(t, expectedParams.TargetBatchTimeout, params.TargetBatchTimeout, "Expected target batch timeout to match after proposal execution")
				require.Equal(t, expectedParams.AverageBlockTime, params.AverageBlockTime, "Expected average block time to match after proposal execution")
				require.Equal(t, expectedParams.AverageEthereumBlockTime, params.AverageEthereumBlockTime, "Expected average ethereum block time to match after proposal execution")
				require.Equal(t, expectedParams.SlashFractionValset, params.SlashFractionValset, "Expected slash fraction valset to match after proposal execution")
				require.Equal(t, expectedParams.SlashFractionBatch, params.SlashFractionBatch, "Expected slash fraction batch to match after proposal execution")
				require.Equal(t, expectedParams.SlashFractionLogicCall, params.SlashFractionLogicCall, "Expected slash fraction logic call to match after proposal execution")
				require.Equal(t, expectedParams.UnbondSlashingValsetsWindow, params.UnbondSlashingValsetsWindow, "Expected unbond slashing valsets window to match after proposal execution")
				require.Equal(t, expectedParams.SlashFractionBadEthSignature, params.SlashFractionBadEthSignature, "Expected slash fraction bad eth signature to match after proposal execution")
				require.Equal(t, expectedParams.ValsetReward, params.ValsetReward, "Expected valset reward to match after proposal execution")
				require.Equal(t, expectedParams.BridgeActive, params.BridgeActive, "Expected bridge active to match after proposal execution")
				require.Equal(t, expectedParams.EthereumBlacklist, params.EthereumBlacklist, "Expected ethereum blacklist to match after proposal execution")
				require.Equal(t, expectedParams.MinChainFeeBasisPoints, params.MinChainFeeBasisPoints, "Expected min chain fee basis points to match after proposal execution")
				require.Equal(t, expectedParams.ChainFeeAuctionPoolFraction, params.ChainFeeAuctionPoolFraction, "Expected chain fee auction pool fraction to match after proposal execution")
			}
		})
	}
}

// ------------------------------------------------------
// nolint: exhaustruct
func TestAirdropProposal_BadDenom(t *testing.T) {
	input := CreateTestEnv(t)
	defer func() { input.Context.Logger().Info("Asserting invariants at test end"); input.AssertInvariants() }()

	ctx := input.Context
	testAddr := []string{
		"gravity1ahx7f8wyertuus9r20284ej0asrs085ceqtfnm",
		"gravity1n38caqg63jf9hefycw3yp95fpkpk669nvekqy2",
		"gravity1qz4zm5s0vwfuu46lg3q0vmnwsukd8e9yfmcgjj",
	}
	parsedRecipients := make([]sdk.AccAddress, len(testAddr))
	for i, v := range testAddr {
		parsed, err := sdk.AccAddressFromBech32(v)
		require.NoError(t, err)
		parsedRecipients[i] = parsed
	}
	byteEncodedRecipients := []byte{}
	for _, v := range parsedRecipients {
		byteEncodedRecipients = append(byteEncodedRecipients, v.Bytes()...)
	}

	airdropBadDenom := types.AirdropProposal{
		Title:       "test title",
		Description: "test description",
		Denom:       "ibc/gravity0xbad",
		Amounts:     []uint64{1000},
		Recipients:  byteEncodedRecipients,
	}

	gk := input.GravityKeeper

	feePoolBalance := sdk.NewInt64Coin("ugraviton", 1000000)
	feePool, err := gk.DistKeeper.FeePool.Get(ctx)
	require.NoError(t, err)
	newCoins := feePool.CommunityPool.Add(sdk.NewDecCoins(sdk.NewDecCoinFromCoin(feePoolBalance))...)
	feePool.CommunityPool = newCoins
	err = gk.DistKeeper.FeePool.Set(ctx, feePool)
	require.NoError(t, err)
	require.NoError(t, input.BankKeeper.MintCoins(ctx, types.ModuleName, sdk.NewCoins(feePoolBalance)))
	require.NoError(t, input.BankKeeper.SendCoinsFromModuleToModule(ctx, types.ModuleName, disttypes.ModuleName, sdk.NewCoins(feePoolBalance)))

	err = gk.HandleAirdropProposal(ctx, &airdropBadDenom)
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrInvalidDenom)
}

// Raw x/gov store value of gravity-bridge-3 proposal 7 at height 23791067.
const mainnetProposal7Hex = "08071296040a232f636f736d6f732e676f762e76312e4d7367457865634c6567616379436f6e74656e7412ee030abb03" +
	"0a1f2f677261766974792e76312e4942434d6574616461746150726f706f73616c1297030a1b4e594d5420746573746e" +
	"657420746f6b656e206d65746164617461124b50726f706f73616c20746f20696e636c75646520746865204942432072" +
	"6570726573656e746174696f6e206f6620746865204e796d2053616e64626f7820746573746e657420746f6b656e1ae4" +
	"010a2b546865206e617469766520746f6b656e206f6620746865204e796d2053616e64626f7820746573746e6574124d" +
	"0a446962632f343945343531304230343132323132414431464534364430324339424135373131393239314645353536" +
	"344635323743314432413435303333364245343737331a05756e796d74120e0a046e796d7410061a046e796d741a4469" +
	"62632f343945343531304230343132323132414431464534364430324339424135373131393239314645353536344635" +
	"3237433144324134353033333642453437373322046e796d742a046e796d7432046e796d7422446962632f3439453435" +
	"313042303431323231324144314645343644303243394241353731313932393146453535363446353237433144324134" +
	"3530333336424534373733122e67726176697479313064303779323635676d6d757674347a30773961773838306a6e73" +
	"723730306a376a70616e6d180322270a0f323238393638343036373434373835120e3932343335363431363730363637" +
	"1a01302201302a0b0890a49f8f0610e1f1b508320b0890eaa98f0610e1f1b5083a150a09756772617669746f6e120831" +
	"31303030303030420c08fac39f8f0610cf95baaf014a0c08fa89aa8f0610cf95baaf01"

// Historical IBCMetadataProposals must decode for gov queries but must not be executable again
func TestHistoricalIBCMetadataProposalDecodes(t *testing.T) {
	input := CreateTestEnv(t)
	defer func() { input.Context.Logger().Info("Asserting invariants at test end"); input.AssertInvariants() }()
	cdc := input.EncodingConfig.Marshaler

	raw, err := hex.DecodeString(mainnetProposal7Hex)
	require.NoError(t, err)

	var proposal govv1.Proposal
	require.NoError(t, cdc.Unmarshal(raw, &proposal))
	require.Equal(t, uint64(7), proposal.Id)
	require.Len(t, proposal.Messages, 1)

	legacy, ok := proposal.Messages[0].GetCachedValue().(*govv1.MsgExecLegacyContent)
	require.True(t, ok)
	content, err := govv1.LegacyContentFromMessage(legacy)
	require.NoError(t, err)
	metadataProposal, ok := content.(*types.IBCMetadataProposal)
	require.True(t, ok)
	require.Equal(t, "NYMT testnet token metadata", metadataProposal.Title)
	require.Equal(t, "ibc/49E4510B0412212AD1FE46D02C9BA57119291FE5564F527C1D2A450336BE4773", metadataProposal.IbcDenom)
	require.Equal(t, metadataProposal.IbcDenom, metadataProposal.Metadata.Base)

	jsonBz, err := cdc.MarshalJSON(&proposal)
	require.NoError(t, err)
	require.Contains(t, string(jsonBz), "/gravity.v1.IBCMetadataProposal")

	// the list queries used by gbt and the REST gateway iterate over every stored proposal
	require.NoError(t, input.GovKeeper.Proposals.Set(input.Context, proposal.Id, proposal))
	queryServer := govkeeper.NewQueryServer(&input.GovKeeper)
	listed, err := queryServer.Proposals(input.Context, &govv1.QueryProposalsRequest{})
	require.NoError(t, err)
	require.Len(t, listed.Proposals, 1)
	votingPeriod, err := queryServer.Proposals(input.Context, &govv1.QueryProposalsRequest{ProposalStatus: govv1.StatusVotingPeriod})
	require.NoError(t, err)
	require.Empty(t, votingPeriod.Proposals)
	legacyListed, err := govkeeper.NewLegacyQueryServer(&input.GovKeeper).Proposals(input.Context, &govv1beta1.QueryProposalsRequest{})
	require.NoError(t, err)
	require.Len(t, legacyListed.Proposals, 1)
	require.Equal(t, "/gravity.v1.IBCMetadataProposal", legacyListed.Proposals[0].Content.TypeUrl)

	// v1beta1 submissions are rejected by the type check, v1 submissions by the handler
	require.False(t, govv1beta1.IsValidProposalType(types.ProposalTypeIBCMetadata))
	err = NewGravityProposalHandler(input.GravityKeeper)(input.Context, content)
	require.ErrorIs(t, err, sdkerrors.ErrUnknownRequest)
}
