package keeper

import (
	"testing"

	"github.com/althea-net/cosmos-gravity-bridge/module/x/gravity/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	disttypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//nolint: exhaustivestruct
func TestAirdropProposal(t *testing.T) {
	input := CreateTestEnv(t)
	ctx := input.Context
	goodAirdrop := types.AirdropProposal{
		Title:       "test tile",
		Description: "test description",
		Amount:      sdk.NewInt64Coin("grav", 1000),
		Recipients:  []string{"gravity1ahx7f8wyertuus9r20284ej0asrs085ceqtfnm"},
	}
	airdropTooBig := goodAirdrop
	airdropTooBig.Amount = sdk.NewInt64Coin("grav", 100000)
	airdropBadToken := goodAirdrop
	airdropBadToken.Amount = sdk.NewInt64Coin("notreal", 1000)
	airdropBadDest := goodAirdrop
	airdropBadDest.Recipients = []string{"gravity1junk"}

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

	err = gk.HandleAirdropProposal(ctx, &airdropBadToken)
	require.Error(t, err)

	err = gk.HandleAirdropProposal(ctx, &airdropBadDest)
	require.Error(t, err)

	err = gk.HandleAirdropProposal(ctx, &goodAirdrop)
	require.NoError(t, err)
	feePool = gk.DistKeeper.GetFeePool(ctx)
	assert.Equal(t, feePool.CommunityPool.AmountOf("grav"), sdk.NewInt64DecCoin("grav", 9000).Amount)

}
