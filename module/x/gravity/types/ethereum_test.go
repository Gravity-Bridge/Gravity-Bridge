package types

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestGravity2Denom verifies that Gravity2Denom produces the expected format and
// that Gravity2DenomToERC20 can parse it back, forming a correct round-trip.
func TestGravity2Denom(t *testing.T) {
	addr, err := NewEthAddress("0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48")
	require.NoError(t, err)

	denom := Gravity2Denom(*addr)

	// Must start with the gravity2 prefix
	require.True(t, strings.HasPrefix(denom, Gravity2DenomPrefix))
	// Total length must match constant
	require.Equal(t, Gravity2DenomLen, len(denom))
	// Must contain the 0x-prefixed address
	require.Contains(t, denom, "0x")
	// Expected exact output
	require.Equal(t, "gravity20xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48", denom)
}

// TestGravity2DenomToERC20 verifies parsing of gravity2 denoms, both valid and invalid.
func TestGravity2DenomToERC20(t *testing.T) {
	tests := []struct {
		name    string
		denom   string
		wantErr bool
		wantHex string
	}{
		{
			name:    "valid gravity2 denom",
			denom:   "gravity20xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48",
			wantErr: false,
			wantHex: "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48",
		},
		{
			name:    "wrong prefix (gravity0x)",
			denom:   "gravity0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48",
			wantErr: true,
		},
		{
			name:    "too short",
			denom:   "gravity20xA0b86991",
			wantErr: true,
		},
		{
			name:    "too long",
			denom:   "gravity20xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48FF",
			wantErr: true,
		},
		{
			name:    "empty string",
			denom:   "",
			wantErr: true,
		},
		{
			name:    "plain gravity prefix",
			denom:   "gravity",
			wantErr: true,
		},
		{
			name:    "invalid hex in address",
			denom:   "gravity20xZZZZ86991c6218b36c1d19D4a2e9Eb0cE3606eB48",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addr, err := Gravity2DenomToERC20(tt.denom)
			if tt.wantErr {
				require.Error(t, err)
				require.Nil(t, addr)
			} else {
				require.NoError(t, err)
				require.NotNil(t, addr)
				require.Equal(t, tt.wantHex, addr.GetAddress().Hex())
			}
		})
	}
}

// TestGravity2DenomRoundTrip verifies that Gravity2Denom -> Gravity2DenomToERC20 produces
// the original address, and that the reverse also holds.
func TestGravity2DenomRoundTrip(t *testing.T) {
	addresses := []string{
		"0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48",
		"0xdAC17F958D2ee523a2206206994597C13D831ec7",
		"0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2",
		"0x0000000000000000000000000000000000000000",
	}

	for _, hex := range addresses {
		addr, err := NewEthAddress(hex)
		require.NoError(t, err)

		denom := Gravity2Denom(*addr)
		parsed, err := Gravity2DenomToERC20(denom)
		require.NoError(t, err)
		require.Equal(t, addr.GetAddress(), parsed.GetAddress(),
			"round-trip failed for %s", hex)
	}
}

// TestGravityDenomToERC20_RejectsGravity2Prefix ensures that GravityDenomToERC20
// explicitly rejects gravity2-prefixed denoms with a clear error, rather than
// attempting to parse them as a regular gravity denom.
func TestGravityDenomToERC20_RejectsGravity2Prefix(t *testing.T) {
	denom := "gravity20xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48"
	addr, err := GravityDenomToERC20(denom)
	require.Error(t, err)
	require.Nil(t, addr)
	require.Contains(t, err.Error(), "gravity2 prefix")
}

// TestGravityDenomLen verifies that the GravityDenomLen and Gravity2DenomLen constants
// match actual output lengths.
func TestGravityDenomLen(t *testing.T) {
	addr, err := NewEthAddress("0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48")
	require.NoError(t, err)

	gravityDenom := GravityDenom(*addr)
	require.Equal(t, GravityDenomLen, len(gravityDenom),
		"GravityDenom output length must match GravityDenomLen constant")

	gravity2Denom := Gravity2Denom(*addr)
	require.Equal(t, Gravity2DenomLen, len(gravity2Denom),
		"Gravity2Denom output length must match Gravity2DenomLen constant")
}

// TestGravityDenomPrefixDisambiguation verifies that the two denom families
// (gravity0x... and gravity20x...) are never confused by either parser.
func TestGravityDenomPrefixDisambiguation(t *testing.T) {
	addr, err := NewEthAddress("0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48")
	require.NoError(t, err)

	g1 := GravityDenom(*addr)
	g2 := Gravity2Denom(*addr)

	// Gravity denom should NOT parse as gravity2
	_, err = Gravity2DenomToERC20(g1)
	require.Error(t, err, "gravity0x denom must not parse as gravity2")

	// Gravity2 denom should NOT parse as gravity
	_, err = GravityDenomToERC20(g2)
	require.Error(t, err, "gravity20x denom must not parse as gravity")

	// Each should parse with its own parser
	parsed1, err := GravityDenomToERC20(g1)
	require.NoError(t, err)
	require.Equal(t, addr.GetAddress(), parsed1.GetAddress())

	parsed2, err := Gravity2DenomToERC20(g2)
	require.NoError(t, err)
	require.Equal(t, addr.GetAddress(), parsed2.GetAddress())
}

func TestERC20ToDenom_ValidateBasic_Denom(t *testing.T) {
	validAddr := "0xb462864E395d88d6bc7C5dd5F3F5eb4cc2599255"

	tests := []struct {
		name    string
		denom   string
		wantErr bool
	}{
		{"valid", "ugraviton", false},
		{"invalid backslash", "gravity\\0xabc", true},
		{"invalid gravity in ibc", "ibc/gravity0xabc", true},
		{"too long", strings.Repeat("a", 257), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := ERC20ToDenom{Erc20: validAddr, Denom: tt.denom}
			err := m.ValidateBasic()
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
