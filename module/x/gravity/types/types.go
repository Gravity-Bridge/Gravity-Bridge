package types

import (
	"encoding/binary"
	fmt "fmt"
	"strconv"
	"strings"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/bech32"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

// UInt64FromBytes create uint from binary big endian representation
func UInt64FromBytes(s []byte) uint64 {
	return binary.BigEndian.Uint64(s)
}

// UInt64Bytes uses the SDK byte marshaling to encode a uint64
func UInt64Bytes(n uint64) []byte {
	return sdk.Uint64ToBigEndian(n)
}

// UInt64FromString to parse out a uint64 for a nonce
func UInt64FromString(s string) (uint64, error) {
	return strconv.ParseUint(s, 10, 64)
}

// IBCAddressFromBech32 decodes an IBC-compatible Address from a Bech32
// encoded string
// TODO: This is very similar to the sdk's GetFromBech32 method, but makes no
// assertions about the bech32 prefix (aka "human readable part"), when Gravity
// IBC Forwarding is completed, this function should return invalid prefix errors
func IBCAddressFromBech32(bech32str string) ([]byte, error) {
	if len(bech32str) == 0 {
		return nil, ErrEmpty
	}

	_, bz, err := bech32.DecodeAndConvert(bech32str)
	if err != nil {
		return nil, err
	}

	err = sdk.VerifyAddressFormat(bz)
	if err != nil {
		return nil, err
	}

	return bz, nil
}

// GetPrefixFromBech32 returns the human readable part of a bech32 string (excluding the 1 byte)
// Returns an error on too short input or when the 1 byte cannot be found
// Note: This is an excerpt from the Decode function for bech32 strings
func GetPrefixFromBech32(bech32str string) (string, error) {
	if len(bech32str) < 8 {
		return "", fmt.Errorf("invalid bech32 string length %d",
			len(bech32str))
	}
	one := strings.LastIndexByte(bech32str, '1')
	if one < 1 || one+7 > len(bech32str) {
		return "", fmt.Errorf("invalid index of 1")
	}

	return bech32str[:one], nil
}

// GetNativePrefixedAccAddressString treats the input as an AccAddress and re-prefixes the string
// with this chain's configured Bech32AccountAddrPrefix
// Returns an error when input is not a bech32 string or the original string it is already natively prefixed
func GetNativePrefixedAccAddressString(foreignStr string) (string, error) {
	prefix, err := GetPrefixFromBech32(foreignStr)
	if err != nil {
		return "", sdkerrors.Wrap(err, "invalid bech32 string")
	}
	nativePrefix := sdk.GetConfig().GetBech32AccountAddrPrefix()
	if prefix == nativePrefix {
		return foreignStr, nil
	}

	return nativePrefix + foreignStr[len(prefix):], nil
}

// GetNativePrefixedAccAddress re-prefixes the input AccAddress with this chain's configured Bech32AccountAddrPrefix
func GetNativePrefixedAccAddress(foreignAddr sdk.AccAddress) (sdk.AccAddress, error) {
	nativeStr, err := GetNativePrefixedAccAddressString(foreignAddr.String())
	if err != nil {
		return nil, err
	}
	return sdk.AccAddressFromBech32(nativeStr)
}
