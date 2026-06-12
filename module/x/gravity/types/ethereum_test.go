package types

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

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
