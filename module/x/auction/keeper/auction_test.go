package keeper_test

import (
	"crypto/rand"
	"fmt"
	"math/big"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/auction/types"
)

// Creates auctions in the store and tests the auction storage and query functions
func (suite *KeeperTestSuite) TestAuctionStorage() {
	accounts := suite.CreateAndFundRandomAccounts(5, sdk.NewCoins(sdk.NewCoin("Hello", sdk.NewInt(1))))
	t := suite.T()
	ctx := suite.Ctx
	ak := suite.App.AuctionKeeper
	ak.DeleteAllAuctions(ctx)

	// Create and store multiple Auctions
	auction := types.NewAuction(1, sdk.NewCoin("test", sdk.OneInt()))
	ak.StoreAuction(ctx, auction)
	stored := ak.GetAllAuctions(ctx)
	require.Equal(t, 1, len(stored))
	require.Equal(t, auction, stored[0])

	// Create 30 more auctions after the initial one
	for i := 1; i < 30; i++ {
		random, err := rand.Int(rand.Reader, (&big.Int{}).Exp(big.NewInt(2), big.NewInt(256), nil))
		require.NoError(t, err)
		auction := types.NewAuction(uint64(i+1), sdk.NewCoin(fmt.Sprintf("test%d", i+2), sdk.NewIntFromBigInt(random)))
		ak.StoreAuction(ctx, auction)
	}

	// Fetch auctions using all the functions
	count := 0
	ak.IterateAuctions(ctx, func(_ []byte, _ types.Auction) bool {
		count += 1
		return false
	})
	require.Equal(t, 30, count)

	allAuctions := ak.GetAllAuctions(ctx)
	require.NotEmpty(t, allAuctions)
	require.Equal(t, 30, len(allAuctions))

	// Update the highest bidder (fail with a bad address first)
	bid := types.Bid{BidAmount: 1, BidderAddress: "hello"}
	err := ak.UpdateHighestBidder(ctx, 1, bid)
	require.Error(t, err)
	bid.BidderAddress = accounts[0].String()
	err = ak.UpdateHighestBidder(ctx, 1, bid)
	require.NoError(t, err)
	auction = *ak.GetAuctionById(ctx, 1)
	require.Equal(t, bid, *auction.HighestBid)

	// Update the highest bidder via UpdateAuction
	updatedAuction := auction
	updatedAuction.HighestBid = &types.Bid{
		BidAmount:     2,
		BidderAddress: accounts[1].String(),
	}
	err = ak.UpdateAuction(ctx, updatedAuction)
	require.NoError(t, err)

	auc := ak.GetAuctionById(ctx, 1)
	require.Equal(t, updatedAuction, *auc)

	// Get all auctions by the current winning bidder
	auctions := ak.GetAllAuctionsByBidder(ctx, accounts[0].String())
	require.Empty(t, auctions)
	auctions = ak.GetAllAuctionsByBidder(ctx, accounts[1].String())
	require.NotEmpty(t, auctions)
	require.Equal(t, updatedAuction, auctions[0])

	// Update a second auction to have the same winning bidder
	secondAuction := *ak.GetAuctionById(ctx, 2)
	secondAuction.HighestBid = &types.Bid{
		BidAmount:     2,
		BidderAddress: accounts[1].String(),
	}
	err = ak.UpdateAuction(ctx, secondAuction)
	require.NoError(t, err)

	// Ensure GetAllAuctionsByBidder returns both results
	auctions = ak.GetAllAuctionsByBidder(ctx, accounts[1].String())
	require.NotEmpty(t, auctions)
	require.Equal(t, 2, len(auctions))
	require.Equal(t, updatedAuction, auctions[0])
	require.Equal(t, secondAuction, auctions[1])

	nonce := ak.GetAuctionNonce(ctx)
	require.Equal(t, uint64(30), nonce.Id)

	next := ak.GetNextAuctionId(ctx)
	require.Equal(t, uint64(31), next)

	incr := ak.IncrementAuctionNonce(ctx)
	require.Equal(t, uint64(30), incr.Id)

	// Delete the auctions and ensure we don't get any iteration results
	ak.DeleteAllAuctions(ctx)
	ak.IterateAuctions(ctx, func(_ []byte, _ types.Auction) bool {
		panic("Should not enter iteration callback func with no auctions")
	})

	// Ensure the nonce persists
	nonce = ak.GetAuctionNonce(ctx)
	require.Equal(t, uint64(31), nonce.Id)

	// And that the next ID is also correct after the auctions are gone
	next = ak.GetNextAuctionId(ctx)
	require.Equal(t, uint64(32), next)
}

// Tests the auction functions when there are no auctions
func (suite *KeeperTestSuite) TestEmptyAuctionFunctions() {
	accounts := suite.CreateAndFundRandomAccounts(5, sdk.NewCoins(sdk.NewCoin("Hello", sdk.NewInt(1))))
	ctx := suite.Ctx
	ak := suite.App.AuctionKeeper

	// Must delete auctions first, potentially some value in community pool on chain init
	ak.DeleteAllAuctions(ctx)

	ak.IterateAuctions(ctx, func(_ []byte, _ types.Auction) bool {
		panic("Should not enter iteration callback func with no auctions")
	})

	t := suite.T()

	allAuctions := ak.GetAllAuctions(ctx)
	require.Empty(t, allAuctions)

	err := ak.UpdateHighestBidder(ctx, 1, types.Bid{})
	require.Error(t, err)

	err = ak.UpdateAuction(ctx, types.NewAuction(1, sdk.Coin{}))
	require.Error(t, err)

	auc := ak.GetAuctionById(ctx, 1)
	require.Nil(t, auc)

	auctions := ak.GetAllAuctionsByBidder(ctx, accounts[0].String())
	require.Empty(t, auctions)

	nonce := ak.GetAuctionNonce(ctx)
	require.Equal(t, uint64(0), nonce.Id)

	next := ak.GetNextAuctionId(ctx)
	require.Equal(t, uint64(1), next)

	incr := ak.IncrementAuctionNonce(ctx)
	require.Equal(t, uint64(0), incr.Id)

	nonce = ak.GetAuctionNonce(ctx)
	require.Equal(t, uint64(1), nonce.Id)
}
