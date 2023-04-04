package keeper

import (
	"testing"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
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

	extremelyLargeAmount := sdk.NewInt(1000000000000).Mul(sdk.NewInt(1000000000000))
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
	feePool := gk.DistKeeper.GetFeePool(ctx)
	newCoins := feePool.CommunityPool.Add(sdk.NewDecCoins(sdk.NewDecCoinFromCoin(feePoolBalance))...)
	feePool.CommunityPool = newCoins
	gk.DistKeeper.SetFeePool(ctx, feePool)
	// test that we are actually setting the fee pool
	assert.Equal(t, input.DistKeeper.GetFeePool(ctx), feePool)
	// mint the actual coins
	require.NoError(t, input.BankKeeper.MintCoins(ctx, types.ModuleName, sdk.NewCoins(feePoolBalance)))
	require.NoError(t, input.BankKeeper.SendCoinsFromModuleToModule(ctx, types.ModuleName, disttypes.ModuleName, sdk.NewCoins(feePoolBalance)))

	err := gk.HandleAirdropProposal(ctx, &airdropTooBig)
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
	feePool = gk.DistKeeper.GetFeePool(ctx)
	assert.Equal(t, feePool.CommunityPool.AmountOf("grav"), sdk.NewInt64DecCoin("grav", 7000).Amount)
	input.AssertInvariants()

	// now we test with extremely large amounts, specifically to get to rounding errors
	feePoolBalance = sdk.NewCoin("grav", extremelyLargeAmount)
	feePool = gk.DistKeeper.GetFeePool(ctx)
	newCoins = feePool.CommunityPool.Add(sdk.NewDecCoins(sdk.NewDecCoinFromCoin(feePoolBalance))...)
	feePool.CommunityPool = newCoins
	gk.DistKeeper.SetFeePool(ctx, feePool)
	// test that we are actually setting the fee pool
	assert.Equal(t, input.DistKeeper.GetFeePool(ctx), feePool)
	// mint the actual coins
	require.NoError(t, input.BankKeeper.MintCoins(ctx, types.ModuleName, sdk.NewCoins(feePoolBalance)))
	require.NoError(t, input.BankKeeper.SendCoinsFromModuleToModule(ctx, types.ModuleName, disttypes.ModuleName, sdk.NewCoins(feePoolBalance)))

	err = gk.HandleAirdropProposal(ctx, &airdropLarge)
	require.NoError(t, err)
	feePool = gk.DistKeeper.GetFeePool(ctx)
	input.AssertInvariants()
}

// nolint: exhaustruct
func TestIBCMetadataProposal(t *testing.T) {
	input := CreateTestEnv(t)
	defer func() { input.Context.Logger().Info("Asserting invariants at test end"); input.AssertInvariants() }()

	ctx := input.Context
	evmChain := input.GravityKeeper.GetEvmChainData(ctx, EthChainPrefix)
	ibcDenom := "ibc/46B44899322F3CD854D2D46DEEF881958467CDD4B3B10086DA49296BBED94BED/grav"
	goodProposal := types.IBCMetadataProposal{
		Title:          "test tile",
		Description:    "test description",
		EvmChainPrefix: evmChain.EvmChainPrefix,
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
func TestAddEvmChainProposal(t *testing.T) {
	input := CreateTestEnv(t)
	defer func() { input.Context.Logger().Info("Asserting invariants at test end"); input.AssertInvariants() }()

	ctx := input.Context
	goodProposal := types.AddEvmChainProposal{
		Title:          "test tile",
		Description:    "test description",
		EvmChainPrefix: "dummy",
		EvmChainName:   "Dummy",
	}

	gk := input.GravityKeeper

	err := gk.HandleAddEvmChainProposal(ctx, &goodProposal)
	require.NoError(t, err)

	evmChain := input.GravityKeeper.GetEvmChainData(ctx, "dummy")
	require.NotNil(t, evmChain)

	// set evm chain params so later we can try to override them with new ones when we try to update
	params := gk.GetParams(ctx)
	exists := false
	for _, param := range params.EvmChainParams {
		if param.EvmChainPrefix == evmChain.EvmChainPrefix {
			param.GravityId = "sample-gravity-id"
			exists = true
		}
	}
	require.Equal(t, exists, true) // when adding correctly, the new evm chain param should exist
	require.Equal(t, len(params.EvmChainParams), 3)
	gk.SetParams(ctx, params)

	// does not have a zero base unit
	badEvmChainPrefix := "dummy" // already exists above
	goodProposal.EvmChainPrefix = badEvmChainPrefix
	goodProposal.EvmChainName = "foobar"

	err = gk.HandleAddEvmChainProposal(ctx, &goodProposal)
	require.NoError(t, err)

	// when we update the chain based on the evm prefix, it should be updated
	chains := gk.GetEvmChains(ctx)
	for _, chain := range chains {
		if chain.EvmChainPrefix == "dummy" {
			require.Equal(t, chain.EvmChainName, "foobar")
		}
	}
	require.Equal(t, len(chains), 3) // only has three chains, BSC, ETH and newly updated dummy chain

	params = gk.GetParams(ctx)
	for _, param := range params.EvmChainParams {
		if param.EvmChainPrefix == evmChain.EvmChainPrefix {
			require.Equal(t, param.GravityId, "")
		}
	}
	require.Equal(t, len(params.EvmChainParams), 3) // should update params only, not append
}

func TestRemoveEvmChainProposal(t *testing.T) {

	input := CreateTestEnv(t)
	defer func() { input.Context.Logger().Info("Asserting invariants at test end"); input.AssertInvariants() }()

	ctx := input.Context
	addProposal := types.AddEvmChainProposal{
		Title:          "test tile",
		Description:    "test description",
		EvmChainPrefix: "dummy",
		EvmChainName:   "Dummy",
	}

	gk := input.GravityKeeper

	err := gk.HandleAddEvmChainProposal(ctx, &addProposal)
	require.NoError(t, err)

	evmChain := gk.GetEvmChainData(ctx, "dummy")
	require.NotNil(t, evmChain)

	gk.setLastObservedEventNonce(ctx, "dummy", 1)

	length := 10
	msgs, anys, _ := createAttestations(t, ctx, gk, length, addProposal.EvmChainPrefix)

	recentAttestations := gk.GetMostRecentAttestations(ctx, addProposal.EvmChainPrefix, uint64(length))
	require.True(t, len(recentAttestations) == length,
		"recentAttestations should have len %v but instead has %v", length, len(recentAttestations))
	for n, attest := range recentAttestations {
		require.Equal(t, attest.Claim.GetCachedValue(), anys[n].GetCachedValue(),
			"The %vth claim does not match our message: claim %v\n message %v", n, attest.Claim, msgs[n])
	}

	removeProposal := types.RemoveEvmChainProposal{
		Title:          "test tile",
		Description:    "test description",
		EvmChainPrefix: "dummy",
	}
	err = gk.HandleRemoveEvmChainProposal(ctx, &removeProposal)
	require.NoError(t, err)

	// now evmChain is empty
	evmChain = gk.GetEvmChainData(ctx, "dummy")
	require.Nil(t, evmChain)

	// no attestation after removal
	recentAttestations = gk.GetMostRecentAttestations(ctx, "Dummy", uint64(length))
	require.Equal(t, len(recentAttestations), 0)

	// also evm chain params
	evmChainParam := gk.GetEvmChainParam(ctx, "dummy")
	require.Nil(t, evmChainParam)

	// when we try to re-add the chain, the list of attestations should be empty
	err = gk.HandleAddEvmChainProposal(ctx, &addProposal)
	require.NoError(t, err)

	recentAttestations = gk.GetMostRecentAttestations(ctx, "Dummy", uint64(length))
	require.Equal(t, len(recentAttestations), 0)
}
