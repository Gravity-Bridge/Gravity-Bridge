package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func getAllKeys() []string {
	return []string{
		QuerierRoute,
		MemStoreKey,
		Version,
		ParamsKey,
		KeyPrefixAuction,
		KeyPrefixBid,
		KeyPrefixAuctionPeriod,
		KeyPrefixLastAuctionPeriodBlockHeight,
		KeyAuctionPeriodBlockHeight,
		KeyPrefixEstimateNextAuctionPeriodBlockHeight,
	}
}

func TestNoDuplicateKeys(t *testing.T) {
	keys := getAllKeys()

	for i, key := range keys {
		keys[i] = ""
		require.NotContains(t, keys, key, "key %v should not be in keys!", key)
	}
}
