package types

import (
	betterrand "crypto/rand"
	"encoding/hex"
	"math/big"
	"math/rand"

	math "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
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

// Creates a random nonzero math.Int test value
func NonzeroSdkInt() (ret math.Int) {
	amount := big.NewInt(0)
	for amount.Cmp(big.NewInt(0)) == 0 {
		amountBz := make([]byte, 32)
		if _, err := betterrand.Read(amountBz); err != nil {
			panic(err)
		}
		amount = big.NewInt(0).SetBytes(amountBz)
	}
	ret = sdk.NewIntFromBigInt(amount)
	return
}
