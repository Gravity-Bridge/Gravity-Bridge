package types

import (
	"crypto/rand"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
)

func TestMsgBid_ValidateBasic(t *testing.T) {
	addrBz := make([]byte, 20)
	_, err := rand.Read(addrBz)
	require.NoError(t, err)
	validAddress := sdk.AccAddress(addrBz)
	invalidAddress := "invalid-address"

	validAmt := uint64(100)

	validFee := uint64(0) // Any fee value will pass ValidateBasic

	invalidAmt := uint64(0)

	tests := []struct {
		name    string
		bidder  string
		amount  uint64
		fee     uint64
		wantErr bool
	}{
		{
			name:    "valid bid",
			bidder:  validAddress.String(),
			amount:  validAmt,
			fee:     validFee,
			wantErr: false,
		},
		{
			name:    "invalid address",
			bidder:  invalidAddress,
			amount:  validAmt,
			fee:     validFee,
			wantErr: true,
		},
		{
			name:    "invalid amount",
			bidder:  validAddress.String(),
			amount:  invalidAmt,
			fee:     validFee,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := NewMsgBid(1, tt.bidder, tt.amount, tt.fee)
			err := msg.ValidateBasic()
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
