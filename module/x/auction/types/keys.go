package types

const (
	// ModuleName defines the module name
	ModuleName = "auction"

	// StoreKey defines the primary module store key
	StoreKey = ModuleName

	// RouterKey is the message route for slashing
	RouterKey = ModuleName

	// QuerierRoute defines the module's query routing key
	QuerierRoute = ModuleName

	// MemStoreKey defines the in-memory store key
	MemStoreKey = "mem_auction"

	// Version defines the current version the IBC module supports
	Version = "auction-1"

	// PortID is the default port id that module binds to
	PortID = "auction"

	// ParamsKey defines the store key for auction module parameters
	ParamsKey = "params"

	// Auction
	// KeyPrefixAuction is the key used to store the auction in the KVStore
	KeyPrefixAuction = "auction-value-"

	// Bid
	// KeyPrefixBid defines the prefix to store bid
	KeyPrefixBid = "bid-value-"

	// AuctionPeriod
	// KeyPrefixAuctionPeriod defines the prefix to store auction period
	KeyPrefixAuctionPeriod = "auctionPeriod-value-"

	KeyPrefixLastAuctionPeriodBlockHeight = "last-auctionPeriod-"

	KeyAuctionPeriodBlockHeight = "block-height"

	KeyPrefixEstimateNextAuctionPeriodBlockHeight = "estimate-next-auctionPeriod-"
)
