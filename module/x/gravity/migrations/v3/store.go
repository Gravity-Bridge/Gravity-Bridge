package v3

import (
	v2 "github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/migrations/v2"
	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/store/prefix"
	storetypes "github.com/cosmos/cosmos-sdk/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

const EthereumChainPrefix string = "gravity"

// MigrateStore performs in-place store migrations from v2 to v3. The migration
// includes:
//
// - Moving currently existing chain specific data to use the new keys that include chain prefix.
func MigrateStore(ctx sdk.Context, storeKey storetypes.StoreKey, cdc codec.BinaryCodec) error {
	ctx.Logger().Info("Enter MigrateStore")
	store := ctx.KVStore(storeKey)

	// TODO: insert Eth into new key for chain info

	updateKeysWithEthPrefix(store, v2.ValsetRequestKey)
	updateKeysWithEthPrefix(store, v2.ValsetConfirmKey)
	updateKeysWithEthPrefix(store, v2.OracleAttestationKey)
	updateKeysWithEthPrefix(store, v2.OutgoingTXPoolKey)
	updateKeysWithEthPrefix(store, v2.OutgoingTXBatchKey)
	updateKeysWithEthPrefix(store, v2.BatchConfirmKey)
	updateKeysWithEthPrefix(store, v2.LastEventNonceByValidatorKey)
	updateKeysWithEthPrefix(store, v2.LastObservedEventNonceKey)
	updateKeysWithEthPrefix(store, v2.KeyLastTXPoolID)
	updateKeysWithEthPrefix(store, v2.KeyLastOutgoingBatchID)
	updateKeysWithEthPrefix(store, v2.KeyOutgoingLogicCall)
	updateKeysWithEthPrefix(store, v2.KeyOutgoingLogicConfirm)
	updateKeysWithEthPrefix(store, v2.DenomToERC20Key)
	updateKeysWithEthPrefix(store, v2.ERC20ToDenomKey)
	updateKeysWithEthPrefix(store, v2.LastSlashedValsetNonce)
	updateKeysWithEthPrefix(store, v2.LatestValsetNonce)
	updateKeysWithEthPrefix(store, v2.LastSlashedBatchBlock)
	updateKeysWithEthPrefix(store, v2.LastSlashedLogicCallBlock)
	updateKeysWithEthPrefix(store, v2.LastObservedValsetKey)

	updateKeysPrefixToEvm(store, v2.PastEthSignatureCheckpointKey, types.PastEvmSignatureCheckpointKey)
	updateKeysPrefixToEvmWithoutChain(store, v2.ValidatorByEthAddressKey, types.ValidatorByEvmAddressKey)
	updateKeysPrefixToEvmWithoutChain(store, v2.EthAddressByValidatorKey, types.EvmAddressByValidatorKey)
	updateKeyPrefixToEvm(store, v2.LastObservedEthereumBlockHeightKey, types.AppendChainPrefix(types.LastObservedEvmBlockHeightKey, EthereumChainPrefix))
	updateKeyPrefixToEvm(store, v2.LastEventNonceByValidatorKey, types.AppendChainPrefix(types.LastEventNonceByValidatorKey, EthereumChainPrefix))

	return nil
}

func updateKeysWithEthPrefix(store storetypes.KVStore, keyPrefix []byte) {
	prefixStore := prefix.NewStore(store, keyPrefix)
	oldStoreIter := prefixStore.Iterator(nil, nil)
	defer oldStoreIter.Close()

	for ; oldStoreIter.Valid(); oldStoreIter.Next() {
		// Set new oldKey on store. Values don't change.
		oldKey := oldStoreIter.Key()
		newKey := types.AppendBytes([]byte(EthereumChainPrefix), oldKey)
		prefixStore.Set(newKey, oldStoreIter.Value())
		prefixStore.Delete(oldKey)
	}
}

func updateKeysPrefixToEvm(store storetypes.KVStore, oldKeyPrefix, newKeyPrefix []byte) {
	oldPrefixStore := prefix.NewStore(store, oldKeyPrefix)
	oldStoreIter := oldPrefixStore.Iterator(nil, nil)
	defer oldStoreIter.Close()

	for ; oldStoreIter.Valid(); oldStoreIter.Next() {
		// Set new oldKey on store. Values don't change.
		oldKey := oldStoreIter.Key()
		newKey := types.AppendBytes(newKeyPrefix, []byte(EthereumChainPrefix), oldKey)
		store.Set(newKey, oldStoreIter.Value())
		oldPrefixStore.Delete(oldKey)
	}
}

func updateKeysPrefixToEvmWithoutChain(store storetypes.KVStore, oldKeyPrefix, newKeyPrefix []byte) {
	oldPrefixStore := prefix.NewStore(store, oldKeyPrefix)
	oldStoreIter := oldPrefixStore.Iterator(nil, nil)
	defer oldStoreIter.Close()

	for ; oldStoreIter.Valid(); oldStoreIter.Next() {
		// Set new oldKey on store. Values don't change.
		oldKey := oldStoreIter.Key()
		newKey := types.AppendBytes(newKeyPrefix, oldKey)
		store.Set(newKey, oldStoreIter.Value())
		oldPrefixStore.Delete(oldKey)
	}
}

func updateKeyPrefixToEvm(store sdk.KVStore, oldKey, newKey []byte) {
	value := store.Get(oldKey)
	if len(value) == 0 {
		return
	}
	store.Set(newKey, value)
	store.Delete(oldKey)
}
