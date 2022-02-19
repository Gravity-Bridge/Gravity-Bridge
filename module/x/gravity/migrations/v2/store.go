package v2

import (
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/store/prefix"
	storetypes "github.com/cosmos/cosmos-sdk/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	ethcmn "github.com/ethereum/go-ethereum/common"
)

// MigrateStore performs in-place store migrations from v1 to v2. The migration
// includes:
//
// - Change all Cosmos orginated ERC20 mappings from (HEX) string to bytes.
// - Change all validator Ethereum keys from (HEX) string to bytes.
func MigrateStore(ctx sdk.Context, storeKey storetypes.StoreKey, cdc codec.BinaryCodec) error {
	store := ctx.KVStore(storeKey)

	// Denoms
	if err := migrateCosmosOriginatedDenomToERC20(store); err != nil {
		return err
	}

	if err := migrateCosmosOriginatedERC20ToDenom(store); err != nil {
		return err
	}

	// Validators' Ethereum addresses
	if err := migrateEthAddressByValidator(store); err != nil {
		return err
	}

	if err := migrateValidatorByEthAddressKey(store); err != nil {
		return err
	}

	// Batch confirmations
	if err := migrateBatchConfirms(store); err != nil {
		return err
	}

	if err := migrateOutgoingTxs(store); err != nil {
		return err
	}

	if err := migrateOutgoingTxBatches(store); err != nil {
		return err
	}

	return nil
}

func migrateCosmosOriginatedDenomToERC20(store storetypes.KVStore) error {
	prefixStore := prefix.NewStore(store, []byte(DenomToERC20Key))
	iterator := prefixStore.Iterator(nil, nil)
	defer iterator.Close()

	for ; iterator.Valid(); iterator.Next() {
		addrStr := string(iterator.Value())
		addr := ethcmn.HexToAddress(addrStr)

		prefixStore.Set(iterator.Key(), addr.Bytes())
	}

	return nil
}

func migrateCosmosOriginatedERC20ToDenom(store storetypes.KVStore) error {
	prefixStore := prefix.NewStore(store, []byte(ERC20ToDenomKey))
	iterator := prefixStore.Iterator(nil, nil)
	defer iterator.Close()

	for ; iterator.Valid(); iterator.Next() {
		oldKey := iterator.Key()

		newKey := string(ethcmn.HexToAddress(string(oldKey)).Bytes())

		prefixStore.Delete(oldKey)
		prefixStore.Set([]byte(newKey), iterator.Value())
	}

	return nil
}

func migrateEthAddressByValidator(store storetypes.KVStore) error {
	prefixStore := prefix.NewStore(store, []byte(EthAddressByValidatorKey))
	iterator := prefixStore.Iterator(nil, nil)
	defer iterator.Close()

	for ; iterator.Valid(); iterator.Next() {
		addrStr := string(iterator.Value())
		addr := ethcmn.HexToAddress(addrStr)
		prefixStore.Set(iterator.Key(), addr.Bytes())
	}

	return nil
}

func migrateValidatorByEthAddressKey(store storetypes.KVStore) error {
	// TODO: There's a chance that we have duplicated keys here. We should
	// keep only the well encoded key and discard the other. This is possible
	// given that the all lower case keys were never accepted and no confirms
	// made.
	prefixStore := prefix.NewStore(store, []byte(ValidatorByEthAddressKey))
	iterator := prefixStore.Iterator(nil, nil)
	defer iterator.Close()

	for ; iterator.Valid(); iterator.Next() {
		oldKey := iterator.Key()
		addr := ethcmn.HexToAddress(string(oldKey))
		newKey := string(addr.Bytes())

		prefixStore.Delete(oldKey)
		if addr.Hex() != string(oldKey) {
			// This address is not well encoded, and wasn't able to sign any
			// claims. Thus we need to remove it (in this case skip it)
			continue
		}
		prefixStore.Set([]byte(newKey), iterator.Value())
	}

	return nil
}

func migrateBatchConfirms(store storetypes.KVStore) error {
	// previously:  BatchConfirmKey + tokenContract.GetAddress() + ConvertByteArrToString(UInt64Bytes(batchNonce)) + string(validator.Bytes())

	prefixStore := prefix.NewStore(store, []byte(BatchConfirmKey))
	iterator := prefixStore.Iterator(nil, nil)
	defer iterator.Close()

	for ; iterator.Valid(); iterator.Next() {
		oldKey := iterator.Key()
		tokenAddr := ethcmn.HexToAddress(string(oldKey[:42]))
		newKey := tokenAddr.Bytes()
		newKey = append(newKey, oldKey[42:]...)
		prefixStore.Delete(oldKey)
		prefixStore.Set([]byte(newKey), iterator.Value())
	}

	return nil
}

func migrateOutgoingTxs(store storetypes.KVStore) error {
	prefixStore := prefix.NewStore(store, []byte(OutgoingTXPoolKey))
	iterator := prefixStore.Iterator(nil, nil)
	defer iterator.Close()

	for ; iterator.Valid(); iterator.Next() {
		oldKey := iterator.Key()
		tokenAddr := ethcmn.HexToAddress(string(oldKey[:42]))
		newKey := tokenAddr.Bytes()
		newKey = append(newKey, oldKey[42:]...)
		prefixStore.Delete(oldKey)
		prefixStore.Set([]byte(newKey), iterator.Value())
	}

	return nil
}

func migrateOutgoingTxBatches(store storetypes.KVStore) error {
	// OutgoingTXBatchKey + tokenContract.GetAddress() + ConvertByteArrToString(UInt64Bytes(nonce))
	// OutgoingTXBatchKey + string(tokenContract.GetAddress().Bytes()) + ConvertByteArrToString(UInt64Bytes(nonce))
	prefixStore := prefix.NewStore(store, []byte(OutgoingTXBatchKey))
	iterator := prefixStore.Iterator(nil, nil)
	defer iterator.Close()

	for ; iterator.Valid(); iterator.Next() {
		oldKey := iterator.Key()
		tokenAddr := ethcmn.HexToAddress(string(oldKey[:42]))
		newKey := tokenAddr.Bytes()
		newKey = append(newKey, oldKey[42:]...)
		prefixStore.Delete(oldKey)
		prefixStore.Set(newKey, iterator.Value())
	}

	return nil
}
