package auction

import (
	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/auction/keeper"
	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/auction/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
)

// TODO: ADD BeginBlocker function to check for if the auction periods has started or not
// TODO: ADD EndBlocker function to check for if the auction periods has ended or not,
func StartMewAuctionPeriod(ctx sdk.Context, params types.Params, k keeper.Keeper, bk types.BankKeeper, ak types.AccountKeeper) error {
	auctionRate := params.AuctionRate

	increamentId, err := k.IncreamentAuctionPeriodId(ctx)
	if err != nil {
		panic(err)
	}

	newAuctionPeriods := types.AuctionPeriod{
		Id:               increamentId,
		StartBlockHeight: uint64(ctx.BlockHeight()),
		Auctions:         []*types.Auction{},
	}

	for token := range params.AllowTokens {
		balance := bk.GetBalance(ctx, ak.GetModuleAccount(ctx, distrtypes.ModuleName).GetAddress(), token)

		// Compute auction amount to send to auction module account
		amount := sdk.NewDecFromInt(balance.Amount).Mul(auctionRate).TruncateInt()

		sdkcoin := sdk.NewCoin(token, amount)

		//Send fund from community pool to auction module
		err := k.SendToCommunityPool(ctx, sdk.Coins{sdkcoin})
		if err != nil {
			return err
		}

		newAuction := types.Auction{
			Id:            uint64(len(newAuctionPeriods.Auctions)),
			AuctionAmount: &sdkcoin,
			Status:        1,
		}

		// Set new auction to store
		k.SetAuction(ctx, newAuction)

		// Update auction in auction period auction list
		newAuctionPeriods.Auctions = append(newAuctionPeriods.Auctions, &newAuction)
	}

	// Set new auction period to store
	k.SetAuctionPeriod(ctx, newAuctionPeriods)

	return nil

}

func EndAuctionPeriod(ctx sdk.Context, params types.Params, k keeper.Keeper, bk types.BankKeeper, ak types.AccountKeeper) error {
	return nil
}

func BeginBlocker(ctx sdk.Context, k keeper.Keeper, bk types.BankKeeper, ak types.AccountKeeper) {
	params := k.GetParams(ctx)

	// An initial auction period need to be set as a starting point
	lastAuctionPeriods, found := k.GetLatestAuctionPeriod(ctx)
	if !found {
		return
	}

	if uint64(ctx.BlockHeight())-lastAuctionPeriods.StartBlockHeight == params.AuctionEpoch {
		err := StartMewAuctionPeriod(ctx, params, k, bk, ak)
		if err != nil {
			return
		}
	}
}
