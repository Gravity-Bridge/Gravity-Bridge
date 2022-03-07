package v1

import (
	"encoding/hex"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
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

	return migrateKeys(store, cdc)
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

// Migrate prefix for keys from string to hash of the previous string value
// string_prefix | key_part1 | key_part2 ...
// into format:
// hash(string_prefix) | key_part1 | key_part2 ...
func migrateKeys(store sdk.KVStore, cdc codec.BinaryCodec) error {
	if err := migrateKeysFromValues(store, cdc, OutgoingTXBatchKey, convertBatchKey); err != nil {
		return err
	}
	if err := migrateKeysFromValues(store, cdc, ValsetRequestKey, convertValsetKey); err != nil {
		return err
	}
	if err := migrateKeysFromValues(store, cdc, ValsetConfirmKey, convertValsetConfirmKey); err != nil {
		return err
	}
	if err := migrateKeysFromValues(store, cdc, OracleAttestationKey, convertAttestationKey); err != nil {
		return err
	}
	if err := migrateKeysFromValues(store, cdc, OutgoingTXPoolKey, convertOutgoingTxKey); err != nil {
		return err
	}
	if err := migrateKeysFromValues(store, cdc, BatchConfirmKey, convertBatchConfirmKey); err != nil {
		return err
	}
	if err := migrateKeysFromValues(store, cdc, KeyOutgoingLogicCall, convertLogicCallKey); err != nil {
		return err
	}
	if err := migrateKeysFromValues(store, cdc, KeyOutgoingLogicConfirm, convertLogicCallConfirmKey); err != nil {
		return err
	}
	if err := migrateKeysFromKeys(store, EthAddressByValidatorKey, types.EthAddressByValidatorKey); err != nil {
		return err
	}
	if err := migrateKeysFromKeys(store, ValidatorByEthAddressKey, types.ValidatorByEthAddressKey); err != nil {
		return err
	}
	if err := migrateKeysFromKeys(store, LastEventNonceByValidatorKey, types.LastEventNonceByValidatorKey); err != nil {
		return err
	}
	if err := migrateKeysFromKeys(store, KeyOrchestratorAddress, types.KeyOrchestratorAddress); err != nil {
		return err
	}
	if err := migrateKeysFromKeys(store, DenomToERC20Key, types.DenomToERC20Key); err != nil {
		return err
	}
	if err := migrateKeysFromKeys(store, ERC20ToDenomKey, types.ERC20ToDenomKey); err != nil {
		return err
	}
	if err := migrateKeysFromKeys(store, PastEthSignatureCheckpointKey, types.PastEthSignatureCheckpointKey); err != nil {
		return err
	}

	migrateKey(store, LastObservedEventNonceKey, types.LastObservedEventNonceKey)
	migrateKey(store, KeyLastTXPoolID, types.KeyLastTXPoolID)
	migrateKey(store, KeyLastOutgoingBatchID, types.KeyLastOutgoingBatchID)
	migrateKey(store, LastObservedEthereumBlockHeightKey, types.LastObservedEthereumBlockHeightKey)
	migrateKey(store, LastSlashedValsetNonce, types.LastSlashedValsetNonce)
	migrateKey(store, LatestValsetNonce, types.LatestValsetNonce)
	migrateKey(store, LastSlashedBatchBlock, types.LastSlashedBatchBlock)
	migrateKey(store, LastSlashedLogicCallBlock, types.LastSlashedLogicCallBlock)
	migrateKey(store, LastUnBondingBlockHeight, types.LastUnBondingBlockHeight)
	migrateKey(store, LastObservedValsetKey, types.LastObservedValsetKey)

	deleteUnusedKeys(store, OracleClaimKey)
	deleteUnusedKeys(store, DenomiatorPrefix)
	deleteUnusedKeys(store, SecondIndexNonceByClaimKey)

	return nil
}

// key conversion functions
func migrateKeysFromValues(store sdk.KVStore, cdc codec.BinaryCodec, keyPrefix string, getNewKey func([]byte, codec.BinaryCodec) ([]byte, error)) error {
	oldStore := prefix.NewStore(store, []byte(keyPrefix))
	oldStoreIter := oldStore.Iterator(nil, nil)
	defer oldStoreIter.Close()

	for ; oldStoreIter.Valid(); oldStoreIter.Next() {
		// Set new key on store. Values don't change.
		value := oldStoreIter.Value()
		newKey, err := getNewKey(value, cdc)
		if err != nil {
			return err
		}
		store.Set(newKey, value)
		oldStore.Delete(oldStoreIter.Key())
	}
	return nil
}

func migrateKey(store sdk.KVStore, oldKey string, newKey []byte) {
	value := store.Get([]byte(oldKey))
	if len(value) == 0 {
		return
	}
	store.Set(newKey, value)
	store.Delete([]byte(oldKey))
}

func migrateKeysFromKeys(store sdk.KVStore, oldKeyPrefix string, newKeyPrefix []byte) error {
	oldStore := prefix.NewStore(store, []byte(oldKeyPrefix))
	oldStoreIter := oldStore.Iterator(nil, nil)
	defer oldStoreIter.Close()
	for ; oldStoreIter.Valid(); oldStoreIter.Next() {
		newKeyCopy := newKeyPrefix
		newKeyCopy = append(newKeyCopy, oldStoreIter.Key()...)
		// Set new key on store. Values don't change.
		store.Set(newKeyCopy, oldStoreIter.Value())
		oldStore.Delete(oldStoreIter.Key())
	}
	return nil
}

func deleteUnusedKeys(store sdk.KVStore, keyPrefix string) {
	oldStore := prefix.NewStore(store, []byte(keyPrefix))
	oldStoreIter := oldStore.Iterator(nil, nil)
	defer oldStoreIter.Close()

	for ; oldStoreIter.Valid(); oldStoreIter.Next() {
		oldStore.Delete(oldStoreIter.Key())
	}
}

func convertValsetKey(oldValue []byte, cdc codec.BinaryCodec) ([]byte, error) {
	var valset types.Valset
	cdc.MustUnmarshal(oldValue, &valset)

	return types.GetValsetKey(valset.Nonce), nil
}

func convertValsetConfirmKey(oldValue []byte, cdc codec.BinaryCodec) ([]byte, error) {
	var valsetConfirm types.MsgValsetConfirm
	cdc.MustUnmarshal(oldValue, &valsetConfirm)
	orchAddr, err := sdk.AccAddressFromBech32(valsetConfirm.Orchestrator)
	if err != nil {
		return nil, err
	}

	return types.GetValsetConfirmKey(valsetConfirm.Nonce, orchAddr), nil
}

func convertAttestationKey(oldValue []byte, cdc codec.BinaryCodec) ([]byte, error) {
	var att types.Attestation
	cdc.MustUnmarshal(oldValue, &att)
	claim, err := unpackAttestationClaim(&att, cdc)
	if err != nil {
		return nil, err
	}
	hash, err := claim.ClaimHash()
	if err != nil {
		return nil, err
	}

	return types.GetAttestationKey(claim.GetEventNonce(), hash), nil
}

func convertOutgoingTxKey(oldValue []byte, cdc codec.BinaryCodec) ([]byte, error) {
	var tx types.OutgoingTransferTx
	cdc.MustUnmarshal(oldValue, &tx)
	fee, err := tx.Erc20Fee.ToInternal()
	if err != nil {
		return nil, err
	}

	return types.GetOutgoingTxPoolKey(*fee, tx.Id), nil
}

func convertBatchKey(oldValue []byte, cdc codec.BinaryCodec) ([]byte, error) {
	var batch types.OutgoingTxBatch
	cdc.MustUnmarshal(oldValue, &batch)
	tokenAddr, err := types.NewEthAddress(batch.TokenContract)
	if err != nil {
		return nil, err
	}

	return types.GetOutgoingTxBatchKey(*tokenAddr, batch.BatchNonce), nil
}

func convertBatchConfirmKey(oldValue []byte, cdc codec.BinaryCodec) ([]byte, error) {
	var batchConfirm types.MsgConfirmBatch
	cdc.MustUnmarshal(oldValue, &batchConfirm)
	tokenAddr, err := types.NewEthAddress(batchConfirm.TokenContract)
	if err != nil {
		return nil, err
	}
	orchAddr, err := sdk.AccAddressFromBech32(batchConfirm.Orchestrator)
	if err != nil {
		return nil, err
	}

	return types.GetBatchConfirmKey(*tokenAddr, batchConfirm.Nonce, orchAddr), nil
}

func convertLogicCallKey(oldValue []byte, cdc codec.BinaryCodec) ([]byte, error) {
	var outgoingLogicCall types.OutgoingLogicCall
	cdc.MustUnmarshal(oldValue, &outgoingLogicCall)

	return types.GetOutgoingLogicCallKey(outgoingLogicCall.InvalidationId, outgoingLogicCall.InvalidationNonce), nil
}

func convertLogicCallConfirmKey(oldValue []byte, cdc codec.BinaryCodec) ([]byte, error) {
	var outgoingLogicConfirm types.MsgConfirmLogicCall
	cdc.MustUnmarshal(oldValue, &outgoingLogicConfirm)
	invalidationIdBytes, err := hex.DecodeString(outgoingLogicConfirm.InvalidationId)
	if err != nil {
		return nil, err
	}
	orchAddr, err := sdk.AccAddressFromBech32(outgoingLogicConfirm.Orchestrator)
	if err != nil {
		return nil, err
	}

	return types.GetLogicConfirmKey(invalidationIdBytes, outgoingLogicConfirm.InvalidationNonce, orchAddr), nil
}

// helper functions
func unpackAttestationClaim(att *types.Attestation, cdc codec.BinaryCodec) (types.EthereumClaim, error) {
	var msg types.EthereumClaim
	err := cdc.UnpackAny(att.Claim, &msg)
	if err != nil {
		return nil, err
	} else {
		return msg, nil
	}
}
