package v4

import (
	"github.com/cosmos/cosmos-sdk/codec"
	storetypes "github.com/cosmos/cosmos-sdk/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// MigrateStore performs in-place store migrations from v2 to v3. The migration
// includes:
//
// - Moving currently existing chain specific data to use the new keys that include chain prefix.
func MigrateStore(ctx sdk.Context, storeKey storetypes.StoreKey, cdc codec.BinaryCodec) error {
	// removeEvmChain()
	return nil
}

func removeEvmChain(store storetypes.KVStore, evmChainPrefix []byte) {
}
