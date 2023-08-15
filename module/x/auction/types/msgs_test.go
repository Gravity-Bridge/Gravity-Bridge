package types

import (
	"bytes"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
)

func TestMsgBid_ValidateBasic(t *testing.T) {

	var (
		validAddress sdk.AccAddress = bytes.Repeat([]byte{0x1}, 20)
	)
	invalidAddress := "invalid-address"

	validCoin, err := sdk.ParseCoinNormalized("100atom")
	require.NoError(t, err)

	invalidCoin, err := sdk.ParseCoinNormalized("invalid-coin")
	require.Error(t, err)

	tests := []struct {
		name    string
		bidder  string
		amount  sdk.Coin
		wantErr bool
	}{
		{
			name:    "valid bid",
			bidder:  validAddress.String(),
			amount:  validCoin,
			wantErr: false,
		},
		{
			name:    "invalid address",
			bidder:  invalidAddress,
			amount:  validCoin,
			wantErr: true,
		},
		{
			name:    "invalid coin",
			bidder:  validAddress.String(),
			amount:  invalidCoin,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := NewMsgBid(1, tt.bidder, tt.amount)
			err := msg.ValidateBasic()
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
