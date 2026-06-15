package types

import (
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/stretchr/testify/require"
)

// Tests for ValidateStrictDenom which provides hard restrictions on token denoms that can interact with the bridge
// these are not banned from the bank module but rejected by the Gravity module to reduce the scope of denom validation
// downstream of the check.
func TestValidateStrictDenom(t *testing.T) {
	tests := []struct {
		name    string
		denom   string
		wantErr bool
	}{
		// Length tests
		{"exactly 256 chars - pass", strings.Repeat("a", 256), false},
		{"257 chars - fail", strings.Repeat("a", 257), true},
		{"empty string - fail", "", true},

		// ASCII tests
		{"pure ASCII - pass", "ugraviton", false},
		{"non-ASCII \u00e0 - fail", "gravity\u00e9", true},
		{"non-ASCII high byte - fail", "gravity\xff", true},

		// Separator tests
		{"attestation separator - fail", "ibc/" + AttestationSeparator, true},

		// Backslash tests
		{"backslash - fail", "gravity\\token", true},

		// Slash / IBC tests
		{"valid ibc hash - pass", "ibc/27394FB092D2ECCD56123C74F36E4C1F926001CEADA9CA97EA622B25F41E5EB2", false},
		{"ibc with extra slash - fail", "ibc/27394FB092D2/ECCD56123C", true},
		{"ibc too short (just prefix) - fail", "ibc/", true},
		{"uppercase IBC prefix - fail", "IBC/27394FB092D2ECCD56123C74F36E4C1F926001CEADA9CA97EA622B25F41E5EB2", true},
		{"uppercase Ibc prefix - fail", "Ibc/27394FB092D2ECCD56123C74F36E4C1F926001CEADA9CA97EA622B25F41E5EB2", true},
		{"non-IBC with slash - fail", "gravity/0xabc", true},
		{"non-IBC no slash - pass", "ugravity", false},

		// Forbidden substring tests (IBC)
		{"ibc with gravity - fail", "ibc/gravity0xabc123", true},
		{"ibc with graviton - fail", "ibc/graviton0xabc123", true},
		{"ibc with 0x - fail", "ibc/evil0xdead", true},

		// Allowed substrings (non-IBC)
		{"gravity prefix non-IBC - pass", "gravity0xabc123", false},
		{"graviton prefix non-IBC - pass", "graviton0xabc123", false},
		{"0x in non-IBC - pass", "mytoken0xabc", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateStrictDenom(tt.denom)
			if tt.wantErr {
				require.Error(t, err, "expected error but got none for denom: %q", tt.denom)
			} else {
				require.NoError(t, err, "unexpected error for denom: %q", tt.denom)
			}
		})
	}
}

// This test ensures the AttestationSeparator retains the desired properties, specifically that it is a non-ASCII string
// that compiles at compile time into a single heavily modified unicode character that is unique enough to never appear in any
// reasonable token name. This test is intended to fail if the separator is somehow modified to not meet these criteria.
func TestAttestationSeparatorIntegrity(t *testing.T) {
	sep := AttestationSeparator

	// Must be valid UTF-8
	require.True(t, utf8.ValidString(sep), "separator must be valid UTF-8")

	// Must not contain a literal backslash byte (0x5C) — the \uXXXX escapes
	// must be resolved at compile time
	require.False(t, strings.ContainsRune(sep, '\\'),
		"separator must not contain literal backslash bytes at runtime")

	// Must not be pure ASCII — separator is inteded to be marked up unicode
	require.False(t, isASCII(sep),
		"separator must contain non-ASCII bytes so ASCII-only denoms can never contain it")

	// Must start with a recognizable ASCII anchor (the 'G')
	// this and the check below further guard against the separator somehow being modified
	// to a less unique value.
	require.True(t, len(sep) > 0 && sep[0] == 'G',
		"separator should start with 'G' as a stable anchor character")

	// All runes after the first must be Unicode combining marks (Mn category)
	runes := []rune(sep)
	require.Greater(t, len(runes), 1, "separator must have combining marks after the anchor")
	for i, r := range runes[1:] {
		require.True(t, unicode.Is(unicode.Mn, r),
			"rune at position %d (U+%04X) must be a combining mark (category Mn)", i+1, r)
	}

	// Must have consistent byte length across compilations
	require.Equal(t, 63, len(sep),
		"separator UTF-8 byte length changed — source may have been corrupted")

	// Must have consistent rune count
	require.Equal(t, 32, utf8.RuneCountInString(sep),
		"separator rune count changed — source may have been corrupted")

	// Must be rejected by ValidateStrictDenom
	require.Error(t, ValidateStrictDenom(sep),
		"separator itself must fail denom validation")
}
