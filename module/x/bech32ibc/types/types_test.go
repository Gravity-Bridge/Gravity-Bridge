package types_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/bech32ibc/types"
)

func TestValidateHrp(t *testing.T) {
	tests := []struct {
		name    string
		hrp     string
		wantErr bool
	}{
		{name: "valid lowercase hrp", hrp: "gravity", wantErr: false},
		{name: "valid single character", hrp: "a", wantErr: false},
		{name: "empty hrp", hrp: "", wantErr: true},
		{name: "uppercase hrp", hrp: "GRAVITY", wantErr: true},
		{name: "mixed case hrp", hrp: "Gravity", wantErr: true},
		{name: "contains a character below the allowed range", hrp: "grav\x20ity", wantErr: true},
		{name: "contains a character above the allowed range", hrp: "grav\x7fity", wantErr: true},
		{name: "valid hrp with digits and punctuation", hrp: "gravity-1_2.3", wantErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := types.ValidateHrp(tt.hrp)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
