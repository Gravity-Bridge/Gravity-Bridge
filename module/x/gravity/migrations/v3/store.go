package v3

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"reflect"

	v2 "github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/migrations/v2"
	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/store/prefix"
	storetypes "github.com/cosmos/cosmos-sdk/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// change this to your current evm mainnet, by default it is your prev GravityDenomPrefix
const EthereumChainPrefix string = types.GravityDenomPrefix
const BridgeEthereumAddress string = "0xb40C364e70bbD98E8aaab707A41a52A2eAF5733f"

// MigrateStore performs in-place store migrations from v2 to v3. The migration
// includes:
//
// - Moving currently existing chain specific data to use the new keys that include chain prefix.
func MigrateStore(ctx sdk.Context, storeKey storetypes.StoreKey, cdc codec.BinaryCodec) error {

	ctx.Logger().Info("Pleiades Upgrade: Beginning the migrations for the gravity module")
	store := ctx.KVStore(storeKey)

	// single key with chain
	updateKeyPrefixToEvm(store, v2.KeyLastOutgoingBatchID, types.KeyLastOutgoingBatchID)
	updateKeyPrefixToEvm(store, v2.LastObservedEventNonceKey, types.LastObservedEventNonceKey)
	updateKeyPrefixToEvm(store, v2.LastObservedEthereumBlockHeightKey, types.LastObservedEvmBlockHeightKey)
	updateKeyPrefixToEvm(store, v2.KeyLastTXPoolID, types.KeyLastTXPoolID)
	updateKeyPrefixToEvm(store, v2.LastSlashedValsetNonce, types.LastSlashedValsetNonce)
	updateKeyPrefixToEvm(store, v2.LatestValsetNonce, types.LatestValsetNonce)
	updateKeyPrefixToEvm(store, v2.LastSlashedBatchBlock, types.LastSlashedBatchBlock)
	updateKeyPrefixToEvm(store, v2.LastSlashedLogicCallBlock, types.LastSlashedLogicCallBlock)
	updateKeyPrefixToEvm(store, v2.LastObservedValsetKey, types.LastObservedValsetKey)

	// multi key with chain
	updateKeysPrefixToEvm(store, v2.ValsetRequestKey, types.ValsetRequestKey)
	updateKeysPrefixToEvm(store, v2.ValsetConfirmKey, types.ValsetConfirmKey)
	updateKeysPrefixToEvm(store, v2.OracleAttestationKey, types.OracleAttestationKey)
	updateKeysPrefixToEvm(store, v2.OutgoingTXPoolKey, types.OutgoingTXPoolKey)
	updateKeysPrefixToEvm(store, v2.OutgoingTxBatchKey, types.OutgoingTxBatchKey)
	updateKeysPrefixToEvm(store, v2.BatchConfirmKey, types.BatchConfirmKey)
	updateKeysPrefixToEvm(store, v2.LastEventNonceByValidatorKey, types.LastEventNonceByValidatorKey)
	updateKeysPrefixToEvm(store, v2.KeyOutgoingLogicCall, types.KeyOutgoingLogicCall)
	updateKeysPrefixToEvm(store, v2.KeyOutgoingLogicConfirm, types.KeyOutgoingLogicConfirm)
	updateKeysPrefixToEvm(store, v2.DenomToERC20Key, types.DenomToERC20Key)
	updateKeysPrefixToEvm(store, v2.ERC20ToDenomKey, types.ERC20ToDenomKey)
	updateKeysPrefixToEvm(store, v2.PastEthSignatureCheckpointKey, types.PastEvmSignatureCheckpointKey)
	// PendingIbcAutoForwards is only existed in v3
	updateKeysPrefixToEvm(store, types.PendingIbcAutoForwards, types.PendingIbcAutoForwards)

	// single key no chain
	updateKeyPrefixToEvmWithoutChain(store, v2.LastUnBondingBlockHeight, types.LastUnBondingBlockHeight)

	// multi key no chain
	updateKeysPrefixToEvmWithoutChain(store, v2.ValidatorByEthAddressKey, types.ValidatorByEthAddressKey)
	updateKeysPrefixToEvmWithoutChain(store, v2.EthAddressByValidatorKey, types.EthAddressByValidatorKey)

	// attestion convert
	convertAttestationKeyValue := getAttestationConverter(ctx, store)
	// Migrate all stored attestations by iterating over everything stored under the OracleAttestationKey
	ctx.Logger().Info("Pleiades Upgrade: Beginning Attestation Upgrade")
	if err := migrateKeysFromValues(store, cdc, types.OracleAttestationKey, convertAttestationKeyValue); err != nil {
		return err
	}

	ctx.Logger().Info("Pleiades Upgrade: Finished the migrations for the gravity module successfully!")
	return nil
}

// Iterates over every value stored under keyPrefix, computes the new key using getNewKey,
// then stores the value in the new key before deleting the old key
func migrateKeysFromValues(store sdk.KVStore, cdc codec.BinaryCodec, keyPrefix []byte, getNewKeyValue func([]byte, codec.BinaryCodec, []byte, []byte) ([]byte, []byte, error)) error {
	prefixStore := prefix.NewStore(store, keyPrefix)
	prefixStoreIter := prefixStore.Iterator(nil, nil)
	defer prefixStoreIter.Close()

	for ; prefixStoreIter.Valid(); prefixStoreIter.Next() {
		// Set new key on store. Values don't change.
		oldKey := prefixStoreIter.Key()
		// The old key lacks a prefix, because the PrefixStore omits it on Get and expects no prefix on Set
		oldKeyWithPrefix := types.AppendBytes(keyPrefix, oldKey)
		value := prefixStoreIter.Value()
		newKey, newValue, err := getNewKeyValue(value, cdc, oldKeyWithPrefix, keyPrefix)
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(oldKey, newKey) {
			// The key has changed
			if prefixStore.Has(newKey) {
				// Collisions are not allowed
				panic(fmt.Sprintf("New key collides with an existing key! %s", hex.EncodeToString(newKey)))
			}

			// Delete the old key and replace it with the new key
			prefixStore.Delete(oldKey)
		}

		// update new value
		prefixStore.Set(newKey, newValue)
	}
	return nil
}

// Creates a closure with the current logger for the attestation key conversion function
func getAttestationConverter(ctx sdk.Context, store sdk.KVStore) func([]byte, codec.BinaryCodec, []byte, []byte) ([]byte, []byte, error) {
	// Unmarshal the old Attestation, unpack its claim, recompute the key using the new ClaimHash
	// Note: The oldKey will contain the implicitPrefix, but the return key should **NOT** have the prefix
	return func(oldValue []byte, cdc codec.BinaryCodec, oldKey []byte, implicitPrefix []byte) ([]byte, []byte, error) {

		var att types.Attestation
		cdc.MustUnmarshal(oldValue, &att)
		claim, err := unpackAttestationClaim(&att, cdc)
		if err != nil {
			return nil, nil, err
		}

		// migrate last observed block height if needed because of the export genesis bug
		migrateLastObservedEvmBlockHeight(ctx, store, cdc, att.Observed, claim.GetEthBlockHeight())

		hash, err := claim.ClaimHash()
		if err != nil {
			return nil, nil, err
		}

		newKey := types.GetAttestationKey(EthereumChainPrefix, claim.GetEventNonce(), hash)
		// The new key must be returned without a prefix, since it will be set on a PrefixStore
		newKeyNoPrefix := newKey[len(implicitPrefix):]

		// Get the old ClaimHash off the end of the old key
		oldClaimHash := oldKey[len(oldKey)-len(hash):]

		if claim.GetType() != types.CLAIM_TYPE_BATCH_SEND_TO_ETH {
			// Non-batch attestations should **not** be moved
			if !reflect.DeepEqual(oldKey, newKey) {
				ctx.Logger().Error("Migrated an old attestaton to a new key!", "event-nonce", claim.GetEventNonce(),
					"eth-block-height", claim.GetEthBlockHeight(), "type", claim.GetType().String(),
					"old-key", hex.EncodeToString(oldKey), "new-key", hex.EncodeToString(newKey),
					"old-claim-hash", hex.EncodeToString(oldClaimHash), "new-claim-hash", hex.EncodeToString(hash),
				)
				panic("Attempted to migrate an attestation which should not have moved!")
			}
		} else {
			// Batch attestations **must** move, unless we have some sort of hash collision
			if reflect.DeepEqual(oldKey, newKey) {
				ctx.Logger().Error("Failed to migrate an old batch!", "event-nonce", claim.GetEventNonce(),
					"eth-block-height", claim.GetEthBlockHeight(), "type", claim.GetType().String(),
					"old-key", hex.EncodeToString(oldKey), "new-key", hex.EncodeToString(newKey),
					"old-claim-hash", hex.EncodeToString(oldClaimHash), "new-claim-hash", hex.EncodeToString(hash),
				)
				panic("Failed to migrate an old batch!")
			} else {
				// Batch migrated to a new key!
				ctx.Logger().Info("Successfully moved a batch to a new key!", "event-nonce", claim.GetEventNonce(),
					"eth-block-height", claim.GetEthBlockHeight(), "type", claim.GetType().String(),
					"old-key", hex.EncodeToString(oldKey), "new-key", hex.EncodeToString(newKey),
					"old-claim-hash", hex.EncodeToString(oldClaimHash), "new-claim-hash", hex.EncodeToString(hash),
				)
			}
		}

		// update evm chain prefix for the value
		claim.SetEvmChainPrefix(EthereumChainPrefix)
		att.Claim = codectypes.UnsafePackAny(claim)
		newValue := cdc.MustMarshal(&att)

		// Reminder, the new key should **NOT** contain the prefix
		return newKeyNoPrefix, newValue, nil
	}
}

// Unpacks the contained EthereumClaim
func unpackAttestationClaim(att *types.Attestation, cdc codec.BinaryCodec) (types.EthereumClaim, error) {
	var msg types.EthereumClaim
	err := cdc.UnpackAny(att.Claim, &msg)
	if err != nil {
		return nil, err
	} else {
		return msg, nil
	}
}

func updateKeysPrefixToEvm(store storetypes.KVStore, oldKeyPrefix, newKeyPrefix []byte) {
	updateKeysPrefixToEvmWithoutChain(store, oldKeyPrefix, types.AppendChainPrefix(newKeyPrefix, EthereumChainPrefix))
}

func updateKeysPrefixToEvmWithoutChain(store storetypes.KVStore, oldKeyPrefix, newKeyPrefix []byte) {
	// nothing change
	if bytes.Equal(oldKeyPrefix, newKeyPrefix) {
		return
	}
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
	updateKeyPrefixToEvmWithoutChain(store, oldKey, types.AppendChainPrefix(newKey, EthereumChainPrefix))
}

func updateKeyPrefixToEvmWithoutChain(store sdk.KVStore, oldKey, newKey []byte) {
	// nothing change
	if bytes.Equal(oldKey, newKey) {
		return
	}
	value := store.Get(oldKey)
	if len(value) == 0 {
		store.Set(newKey, []byte{})
	} else {
		store.Set(newKey, value)
	}
	store.Delete(oldKey)

}

func migrateLastObservedEvmBlockHeight(ctx sdk.Context, store sdk.KVStore, cdc codec.BinaryCodec, observed bool, claimHeight uint64) {
	key := types.AppendChainPrefix(types.LastObservedEvmBlockHeightKey, EthereumChainPrefix)
	bytes := store.Get(key)
	height := types.LastObservedEthereumBlockHeight{
		CosmosBlockHeight:   0,
		EthereumBlockHeight: 0,
	}

	if len(bytes) > 0 {
		cdc.MustUnmarshal(bytes, &height)
	}
	if observed && claimHeight > height.EthereumBlockHeight {
		height = types.LastObservedEthereumBlockHeight{
			EthereumBlockHeight: claimHeight,
			CosmosBlockHeight:   uint64(ctx.BlockHeight()),
		}
		store.Set(key, cdc.MustMarshal(&height))
	}
}
