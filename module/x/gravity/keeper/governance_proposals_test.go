package keeper

import (
	"testing"

	sdkmath "cosmossdk.io/math"
	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
	typesv2 "github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types/v2"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	disttypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	feePool := gk.DistKeeper.GetFeePool(ctx)
	newCoins := feePool.CommunityPool.Add(sdk.NewDecCoins(sdk.NewDecCoinFromCoin(feePoolBalance))...)
	feePool.CommunityPool = newCoins
	gk.DistKeeper.SetFeePool(ctx, feePool)
	// test that we are actually setting the fee pool
	assert.Equal(t, input.DistKeeper.GetFeePool(ctx), feePool)
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
	_, err := msgServer.AirdropProposal(sdk.WrapSDKContext(ctx), &msgProposal)
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
		_, err = msgServer.AirdropProposal(sdk.WrapSDKContext(ctx), &msgProposal)
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
