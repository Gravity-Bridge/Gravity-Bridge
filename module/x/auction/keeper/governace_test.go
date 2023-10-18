package keeper_test

import (
	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/auction/types"
)

func (suite *KeeperTestSuite) TestHandleUpdateAllowListProposal() {
	// Assume that AllowList currently has two denoms: "atom" and "juno"
	testCases := map[string]struct {
		update            *types.UpdateAllowListProposal
		expectedAllowList map[string]bool
	}{
		"add denom osmo": {
			update: &types.UpdateAllowListProposal{
				Denom:       "osmo",
				ActionType:  types.ActionType_ACTION_TYPE_ADD_TOKEN,
				Title:       "",
				Description: "",
			},
			expectedAllowList: map[string]bool{
				"atom": true,
				"juno": true,
				"osmo": true,
			},
		},
		"delete denom atom": {
			update: &types.UpdateAllowListProposal{
				Denom:       "atom",
				ActionType:  types.ActionType_ACTION_TYPE_REMOVE_TOKEN,
				Title:       "",
				Description: "",
			},
			expectedAllowList: map[string]bool{
				"juno": true,
			},
		},
	}
	for name, tc := range testCases {
		suite.Run(name, func() {
			// Set up test app
			suite.SetupTest()

			// Set allowlist tokens
			params := suite.App.GetAuctionKeeper().GetParams(suite.Ctx)
			params.AllowTokens["atom"] = true
			params.AllowTokens["juno"] = true
			suite.App.GetAuctionKeeper().SetParams(suite.Ctx, params)

			// HandleUpdate
			err := suite.App.GetAuctionKeeper().HandleUpdateAllowListProposal(suite.Ctx, tc.update)
			suite.Require().NoError(err)
			preParams := suite.App.GetAuctionKeeper().GetParams(suite.Ctx)
			suite.Require().Equal(preParams.AllowTokens, tc.expectedAllowList)
		})
	}
}
