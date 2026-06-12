package types

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

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
