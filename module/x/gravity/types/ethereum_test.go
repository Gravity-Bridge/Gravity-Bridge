package types

import (
	"testing"

	"github.com/stretchr/testify/require"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func TestInternalERC20TokensAddition(t *testing.T) {
	var (
		token1, e1 = NewEthAddress("0x0A7254b318dd742A3086882321C27779B4B642a6")
		token2, e2 = NewEthAddress("0x454330deAaB759468065d08F2b3B0562caBe1dD1")
		token3, e3 = NewEthAddress("0x479FFc856Cdfa0f5D1AE6Fa61915b01351A7773D")
		token4, e4 = NewEthAddress("0x6db48cBBCeD754bDc760720e38E456144e83269b")
		token5, e5 = NewEthAddress("0x8E91960d704Df3fF24ECAb78AB9df1B5D9144140")
	)
	require.NoError(t, e1)
	require.NoError(t, e2)
	require.NoError(t, e3)
	require.NoError(t, e4)
	require.NoError(t, e5)

	var (
		amount1   = int64(1234567)
		amount2   = int64(4567890)
		amount3   = int64(1111111)
		amount4   = int64(2222222)
		amount5   = int64(3333333)
		expected1 = int64(0)
		expected2 = int64(3333323)
		expected3 = int64(-123456)
		expected4 = int64(987655)
		expected5 = int64(2098766)
	)

	tokensA := InternalERC20Tokens{
		{Amount: sdk.NewInt(amount1), Contract: *token1},
		{Amount: sdk.NewInt(amount2), Contract: *token2},
		{Amount: sdk.NewInt(amount3), Contract: *token3},
		{Amount: sdk.NewInt(amount4), Contract: *token4},
		{Amount: sdk.NewInt(amount5), Contract: *token5},
	}
	tokensA.Sort()
	tokensB := InternalERC20Tokens{
		{Amount: sdk.NewInt(amount1), Contract: *token1},
		{Amount: sdk.NewInt(amount1), Contract: *token2},
		{Amount: sdk.NewInt(amount1), Contract: *token3},
		{Amount: sdk.NewInt(amount1), Contract: *token4},
		{Amount: sdk.NewInt(amount1), Contract: *token5},
	}
	tokensB.Sort()

	tokensC := tokensA.SubSorted(tokensB)
	t.Logf("tokensA after subtraction: %v\n\n\ntokensB after subtraction: %v\n\n\ntokensC after subtraction: %v", tokensA, tokensB, tokensC)

	for _, token := range tokensA {
		if token.Contract.GetAddress() == token1.GetAddress() {
			require.Equal(t, amount1, token.Amount.Int64())
		}
		if token.Contract.GetAddress() == token2.GetAddress() {
			require.Equal(t, amount2, token.Amount.Int64())
		}
		if token.Contract.GetAddress() == token3.GetAddress() {
			require.Equal(t, amount3, token.Amount.Int64())
		}
		if token.Contract.GetAddress() == token4.GetAddress() {
			require.Equal(t, amount4, token.Amount.Int64())
		}
		if token.Contract.GetAddress() == token5.GetAddress() {
			require.Equal(t, amount5, token.Amount.Int64())
		}
	}
	for _, token := range tokensB {
		require.Equal(t, amount1, token.Amount.Int64())
	}
	for _, token := range tokensC {
		if token.Contract.GetAddress() == token1.GetAddress() {
			require.Equal(t, expected1, token.Amount.Int64())
		}
		if token.Contract.GetAddress() == token2.GetAddress() {
			require.Equal(t, expected2, token.Amount.Int64())
		}
		if token.Contract.GetAddress() == token3.GetAddress() {
			require.Equal(t, expected3, token.Amount.Int64())
		}
		if token.Contract.GetAddress() == token4.GetAddress() {
			require.Equal(t, expected4, token.Amount.Int64())
		}
		if token.Contract.GetAddress() == token5.GetAddress() {
			require.Equal(t, expected5, token.Amount.Int64())
		}
	}
}

// Creates 3 tokens with 2 amounts each, then tests they are consolidated down to 1 entry each
func TestInternalERC20TokensConsolidate(t *testing.T) {
	var (
		token1, e1 = NewEthAddress("0x0A7254b318dd742A3086882321C27779B4B642a6")
		token2, e2 = NewEthAddress("0x454330deAaB759468065d08F2b3B0562caBe1dD1")
		token3, e3 = NewEthAddress("0x479FFc856Cdfa0f5D1AE6Fa61915b01351A7773D")
	)
	require.NoError(t, e1)
	require.NoError(t, e2)
	require.NoError(t, e3)
	var (
		amount1 = int64(1234567)
		amount2 = int64(-123456)

		expected1 = int64(1111111)
		expected2 = int64(2469134)
		expected3 = int64(-246912)
	)

	tokens := InternalERC20Tokens{
		{Amount: sdk.NewInt(amount1), Contract: *token1},
		{Amount: sdk.NewInt(amount2), Contract: *token1},
		{Amount: sdk.NewInt(amount1), Contract: *token2},
		{Amount: sdk.NewInt(amount1), Contract: *token2},
		{Amount: sdk.NewInt(amount2), Contract: *token3},
		{Amount: sdk.NewInt(amount2), Contract: *token3},
	}
	tokens.Sort()
	t.Logf("Sorted tokens: %v", tokens.String())
	require.Equal(t, 6, tokens.Len())

	tokens.Consolidate()
	t.Logf("Consolidated tokens: %v", tokens.String())
	require.Equal(t, &InternalERC20Token{Amount: sdk.NewInt(expected1), Contract: *token1}, tokens[0])
	require.Equal(t, &InternalERC20Token{Amount: sdk.NewInt(expected2), Contract: *token2}, tokens[1])
	require.Equal(t, &InternalERC20Token{Amount: sdk.NewInt(expected3), Contract: *token3}, tokens[2])
	require.Equal(t, 3, tokens.Len())
}
