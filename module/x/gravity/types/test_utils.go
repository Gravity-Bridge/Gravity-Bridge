package types

import (
	betterrand "crypto/rand"
	"encoding/hex"
	"math/big"
	"math/rand"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

// Creates a random nonzero uint64 test value
func NonzeroUint64() (ret uint64) {
	for ret == 0 {
		ret = rand.Uint64()
	}
	return
}

// Creates a random nonempty 20 byte sdk.AccAddress string test value
func NonemptySdkAccAddress() (ret sdk.AccAddress) {
	for ret.Empty() {
		addr := make([]byte, 20)
		if _, err := betterrand.Read(addr); err != nil {
			panic(err)
		}
		ret = sdk.AccAddress(addr)
	}
	return
}

// Creates a random nonempty 20 byte address hex string test value
func NonemptyEthAddress() (ret string) {
	for ret == "" {
		addr := make([]byte, 20)
		if _, err := betterrand.Read(addr); err != nil {
			panic(err)
		}
		ret = hex.EncodeToString(addr)
	}
	ret = "0x" + ret
	return
}

// Creates a random nonzero sdkmath.Int test value
func NonzeroSdkInt() (ret sdkmath.Int) {
	amount := big.NewInt(0)
	for amount.Cmp(big.NewInt(0)) == 0 {
		amountBz := make([]byte, 32)
		if _, err := betterrand.Read(amountBz); err != nil {
			panic(err)
		}
		amount = big.NewInt(0).SetBytes(amountBz)
	}
	ret = sdkmath.NewIntFromBigInt(amount)
	return
}

// minMeta builds a minimal valid banktypes.Metadata for a given denom.
// Useful across types-package tests that need to populate CosmosBridgeableTokens.
func minMeta(denom string) banktypes.Metadata {
	// nolint: exhaustruct
	return banktypes.Metadata{
		Name:    denom,
		Symbol:  denom,
		Base:    denom,
		Display: denom,
		// nolint: exhaustruct
		DenomUnits: []*banktypes.DenomUnit{{Denom: denom, Exponent: 0}},
	}
}
