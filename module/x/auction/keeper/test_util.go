package keeper

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
)

// Asserts that actualCoins contains exactly the expectedCoins. If strict is true no additional tokens are allowed.
// The amounts of the expectedCoins must exactly equal the amounts of the respective coins in actualCoins
func ExpectedCoinsPresent(t *testing.T, expectedCoins sdk.Coins, actualCoins sdk.Coins, strict bool) {
	if strict {
		require.True(t, actualCoins.IsEqual(expectedCoins), "expected coins and actual coins are not strictly equal")
	} else {
		require.True(t, len(expectedCoins) <= len(actualCoins), "expected coins has more members than actual coins")
		for _, expected := range expectedCoins {
			actual := actualCoins.AmountOf(expected.Denom)
			require.Equalf(t, expected.Amount, actual, "expected amount != actual amount for denom %s", expected.Denom)
		}
	}
}
