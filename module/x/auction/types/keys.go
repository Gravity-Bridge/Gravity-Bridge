package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	// ModuleName defines the module name
	ModuleName = "auction"

	// StoreKey defines the primary module store key
	StoreKey = ModuleName

	// RouterKey is the message route for slashing
	RouterKey = ModuleName

	// QuerierRoute defines the module's query routing key
	QuerierRoute = ModuleName

	// The source name for an account where all auctionable tokens are contained, active auction balances
	// are held by the Auction module account
	// this string is used with AccountKeeper.GetModuleAccount() or BankKeeper.SendCoinsFromAccountToModule()
	AuctionPoolAccountName = "auction_pool"

	// ParamsKey defines the store key for auction module param
	KeyParams = "KeyParams"

	// KeyAuctionPeriod stores the current AuctionPeriod information
	KeyAuctionPeriod = "KeyAuctionPeriod"

	// KeyAuction stores all the active Auctions by their id
	KeyAuction = "KeyAuctions"

	// KeyAuctionNonce stores the most recent auction id created to prevent issues with
	// future auctions using previously used Ids
	KeyAuctionNonce = "KeyAuctionNonce"
)

// GetAuctionKey constructs a prefixed key for the Auction with the given `id`
// Returns [KeyAuction | id]
func GetAuctionKey(id uint64) []byte {
	return AppendBytes([]byte(KeyAuction), UInt64Bytes(id))
}

// UInt64Bytes uses the SDK byte marshaling to encode a uint64
func UInt64Bytes(n uint64) []byte {
	return sdk.Uint64ToBigEndian(n)
}

// AppendBytes will reliably concatenate byte collections
func AppendBytes(args ...[]byte) []byte {
	length := 0
	for _, v := range args {
		length += len(v)
	}

	res := make([]byte, length)

	length = 0
	for _, v := range args {
		copy(res[length:length+len(v)], v)
		length += len(v)
	}

	return res
}
