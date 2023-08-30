package keeper_test

import (
	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/auction/keeper"
	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/auction/types"
	"github.com/stretchr/testify/require"
)

func (suite *KeeperTestSuite) TestGenesis() {
	t := suite.T()
	ctx := suite.Ctx
	ak := suite.App.AuctionKeeper
	// Test import and export

	genesis := types.GenesisState{
		Params: types.DefaultParams(),
		ActivePeriod: &types.AuctionPeriod{
			StartBlockHeight: 1,
			EndBlockHeight:   1000,
		},
		ActiveAuctions: []types.Auction{},
	}
	keeper.InitGenesis(ctx, *ak, genesis)

	exported := keeper.ExportGenesis(ctx, *ak)
	require.NotNil(t, exported)
	require.Equal(t, genesis.Params, exported.Params)
	require.Equal(t, genesis.ActivePeriod, exported.ActivePeriod)
	require.Equal(t, len(genesis.ActiveAuctions), len(exported.ActiveAuctions))
	require.ElementsMatch(t, genesis.ActiveAuctions, exported.ActiveAuctions)
}
