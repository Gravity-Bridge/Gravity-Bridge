package keeper

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sdkmath "cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	disttypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"

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
	gk.DistKeeper.FeePool.Set(ctx, feePool)
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
	newCoins = feePool.CommunityPool.Add(sdk.NewDecCoins(sdk.NewDecCoinFromCoin(feePoolBalance))...)
	feePool.CommunityPool = newCoins
	gk.DistKeeper.FeePool.Set(ctx, feePool)
	// test that we are actually setting the fee pool
	fp, err = input.DistKeeper.FeePool.Get(ctx)
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
	gk.DistKeeper.FeePool.Set(ctx, feePool)
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

// nolint: exhaustruct
func TestIBCMetadataProposal(t *testing.T) {
	input := CreateTestEnv(t)
	defer func() { input.Context.Logger().Info("Asserting invariants at test end"); input.AssertInvariants() }()

	ctx := input.Context
	ibcDenom := "ibc/46B44899322F3CD854D2D46DEEF881958467CDD4B3B10086DA49296BBED94BED/grav"
	goodProposal := types.IBCMetadataProposal{
		Title:       "test tile",
		Description: "test description",
		Metadata: banktypes.Metadata{
			Description: "Atom",
			Name:        "Atom",
			Base:        ibcDenom,
			Display:     "Atom",
			Symbol:      "ATOM",
			DenomUnits: []*banktypes.DenomUnit{
				{
					Denom:    ibcDenom,
					Exponent: 0,
				},
				{
					Denom:    "Atom",
					Exponent: 6,
				},
			},
		},
		IbcDenom: ibcDenom,
	}

	gk := input.GravityKeeper

	err := gk.HandleIBCMetadataProposal(ctx, &goodProposal)
	require.NoError(t, err)
	metadata, exists := gk.bankKeeper.GetDenomMetaData(ctx, ibcDenom)
	require.True(t, exists)
	require.Equal(t, metadata, goodProposal.Metadata)

	// does not have a zero base unit
	badMetadata := goodProposal
	badMetadata.Metadata.DenomUnits = []*banktypes.DenomUnit{
		{
			Denom:    ibcDenom,
			Exponent: 1,
		},
		{
			Denom:    "Atom",
			Exponent: 6,
		},
	}

	err = gk.HandleIBCMetadataProposal(ctx, &badMetadata)
	require.Error(t, err)

	// no denom unit for display
	badMetadata2 := goodProposal
	badMetadata2.Metadata.DenomUnits = []*banktypes.DenomUnit{
		{
			Denom:    ibcDenom,
			Exponent: 0,
		},
		{
			Denom:    "a",
			Exponent: 6,
		},
	}

	err = gk.HandleIBCMetadataProposal(ctx, &badMetadata2)
	require.Error(t, err)

	// incorrect base unit
	badMetadata3 := goodProposal
	badMetadata3.Metadata.DenomUnits = []*banktypes.DenomUnit{
		{
			Denom:    "atom",
			Exponent: 0,
		},
		{
			Denom:    "a",
			Exponent: 6,
		},
	}

	err = gk.HandleIBCMetadataProposal(ctx, &badMetadata3)
	require.Error(t, err)

}

// nolint: exhaustruct
func TestMsgIBCMetadataProposal(t *testing.T) {
	numFalseAuthorities := 10000

	input := CreateTestEnv(t)
	defer func() { input.Context.Logger().Info("Asserting invariants at test end"); input.AssertInvariants() }()

	ctx := input.Context
	ibcDenom := "ibc/46B44899322F3CD854D2D46DEEF881958467CDD4B3B10086DA49296BBED94BED/grav"
	proposal := types.IBCMetadataProposal{
		Title:       "test tile",
		Description: "test description",
		Metadata: banktypes.Metadata{
			Description: "Atom",
			Name:        "Atom",
			Base:        ibcDenom,
			Display:     "Atom",
			Symbol:      "ATOM",
			DenomUnits: []*banktypes.DenomUnit{
				{
					Denom:    ibcDenom,
					Exponent: 0,
				},
				{
					Denom:    "Atom",
					Exponent: 6,
				},
			},
		},
		IbcDenom: ibcDenom,
	}

	msgServer := msgServer{input.GravityKeeper}

	// nolint: exhaustruct
	msgProposal := typesv2.MsgIBCMetadataProposal{
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
		_, err := msgServer.IBCMetadataProposal(sdk.WrapSDKContext(ctx), &msgProposal)
		require.Error(t, err, "Expected error for authority %s", authority)
	}

	// Finally, perform a good test with the proper authority
	authority := input.GravityKeeper.GetAuthority()
	msgProposal.Authority = authority
	_, err := msgServer.IBCMetadataProposal(ctx, &msgProposal)
	require.NoError(t, err)
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
		_, err := msgServer.UnhaltBridgeProposal(sdk.WrapSDKContext(ctx), &msgProposal)
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

	gravityId := typesv2.Param{
		Key:   "GravityId",
		Value: "1",
	}
	contractSourceHash := typesv2.Param{
		Key:   "ContractSourceHash",
		Value: "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
	}
	bridgeEthereumAddress := typesv2.Param{
		Key:   "BridgeEthereumAddress",
		Value: "0x0000000000000000000000000000000000000000",
	}
	bridgeChainId := typesv2.Param{
		Key:   "BridgeChainId",
		Value: "1",
	}
	signedValsetsWindow := typesv2.Param{
		Key:   "SignedValsetsWindow",
		Value: "100",
	}
	signedBatchesWindow := typesv2.Param{
		Key:   "SignedBatchesWindow",
		Value: "100",
	}
	signedLogicCallsWindow := typesv2.Param{
		Key:   "SignedLogicCallsWindow",
		Value: "100",
	}
	targetBatchTimeout := typesv2.Param{
		Key:   "TargetBatchTimeout",
		Value: "1000000",
	}
	averageBlockTime := typesv2.Param{
		Key:   "AverageBlockTime",
		Value: "1000",
	}
	averageEthereumBlockTime := typesv2.Param{
		Key:   "AverageEthereumBlockTime",
		Value: "25000",
	}
	slashFractionValset := typesv2.Param{
		Key:   "SlashFractionValset",
		Value: "0.020000000000000000",
	}
	slashFractionBatch := typesv2.Param{
		Key:   "SlashFractionBatch",
		Value: "0.020000000000000000",
	}
	slashFractionLogicCall := typesv2.Param{
		Key:   "SlashFractionLogicCall",
		Value: "0.010000000000000000",
	}
	unbondSlashingValsetsWindow := typesv2.Param{
		Key:   "UnbondSlashingValsetsWindow",
		Value: "100",
	}
	slashFractionBadEthSignature := typesv2.Param{
		Key:   "SlashFractionBadEthSignature",
		Value: "0.020000000000000000",
	}
	valsetReward := typesv2.Param{
		Key:   "ValsetReward",
		Value: "10ugraviton",
	}
	bridgeActive := typesv2.Param{
		Key:   "BridgeActive",
		Value: "false",
	}
	ethereumBlacklist := typesv2.Param{
		Key:   "EthereumBlacklist",
		Value: "[\"0x0000000000000000000000000000000000000000\"]",
	}
	minChainFeeBasisPoints := typesv2.Param{
		Key:   "MinChainFeeBasisPoints",
		Value: "100",
	}
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
