package apptesting

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"

	auctiontypes "github.com/Gravity-Bridge/Gravity-Bridge/module/x/auction/types"
)

// Mints `amounts` and sends them to `addr`
func (s *AppTestHelper) FundAccount(ctx sdk.Context, addr sdk.AccAddress, amounts sdk.Coins) {
	bankkeeper := s.App.AuctionKeeper.BankKeeper
	err := bankkeeper.MintCoins(ctx, minttypes.ModuleName, amounts)
	if err != nil {
		panic("Failed to mint coins")
	}
	err = bankkeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, addr, amounts)
	if err != nil {
		panic("Failed to send coins to account")
	}
}

// Mints `amounts` and sends them to the module with the given `moduleName`
func (s *AppTestHelper) FundModule(ctx sdk.Context, moduleName string, amounts sdk.Coins) {
	bankkeeper := s.App.AuctionKeeper.BankKeeper
	err := bankkeeper.MintCoins(ctx, minttypes.ModuleName, amounts)
	if err != nil {
		panic("Failed to mint coins")
	}
	err = bankkeeper.SendCoinsFromModuleToModule(ctx, minttypes.ModuleName, moduleName, amounts)
	if err != nil {
		panic("Failed to send coins to module")
	}
}

// Mints tokens and places them in the community pool
func (s *AppTestHelper) FundCommunityPool(ctx sdk.Context, amounts sdk.Coins) {
	decAmounts := sdk.NewDecCoinsFromCoins(amounts...)
	distKeeper := s.App.AuctionKeeper.DistKeeper
	s.FundModule(ctx, distrtypes.ModuleName, amounts)
	communityPool := distKeeper.GetFeePool(ctx)
	communityPool.CommunityPool = communityPool.CommunityPool.Add(decAmounts...)

	distKeeper.SetFeePool(ctx, communityPool)
}

// Mints tokens and places them in the auction module pool
func (s *AppTestHelper) FundAuctionPool(ctx sdk.Context, amounts sdk.Coins) {
	s.FundModule(ctx, auctiontypes.AuctionPoolAccountName, amounts)
}

// Creates `numAccounts` accounts and mints for them the specified `amounts`
func (s *AppTestHelper) CreateAndFundRandomAccounts(numAccounts int, amounts sdk.Coins) []sdk.AccAddress {
	addrs := CreateRandomAccounts(numAccounts)

	for _, addr := range addrs {
		s.FundAccount(s.Ctx, addr, amounts)

		// Assert that the balances actually updated
		for _, coin := range amounts {
			balance := s.App.BankKeeper.GetBalance(s.Ctx, addr, coin.Denom)
			if !balance.IsGTE(coin) {
				panic("Invalid balance")
			}
		}
	}
	s.TestAccs = addrs

	return addrs
}
