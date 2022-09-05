package v3

import (
	"encoding/hex"
	"fmt"
	"reflect"

	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/store/prefix"
	storetypes "github.com/cosmos/cosmos-sdk/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/tendermint/tendermint/libs/log"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
)

// MigrateStore performs in-place store migrations from v2 to v3. The migration
// includes:
//
// - Migrate all attestations to their correct new keys
func MigrateStore(ctx sdk.Context, storeKey storetypes.StoreKey, cdc codec.BinaryCodec) error {
	ctx.Logger().Info("Pleiades Upgrade: Beginning the migrations for the gravity module")
	store := ctx.KVStore(storeKey)

	convertAttestationKey := getAttestationConverter(ctx.Logger())
	// Migrate all stored attestations by iterating over everything stored under the OracleAttestationKey
	ctx.Logger().Info("Pleiades Upgrade: Beginning Attestation Upgrade")
	if err := migrateKeysFromValues(store, cdc, types.OracleAttestationKey, convertAttestationKey); err != nil {
		return err
	}

	ctx.Logger().Info("Pleiades Upgrade: Finished the migrations for the gravity module successfully!")
	return nil
}

// Iterates over every value stored under keyPrefix, computes the new key using getNewKey,
// then stores the value in the new key before deleting the old key
func migrateKeysFromValues(store sdk.KVStore, cdc codec.BinaryCodec, keyPrefix []byte, getNewKey func([]byte, codec.BinaryCodec, []byte, []byte) ([]byte, error)) error {
	prefixStore := prefix.NewStore(store, keyPrefix)
	prefixStoreIter := prefixStore.Iterator(nil, nil)
	defer prefixStoreIter.Close()

	for ; prefixStoreIter.Valid(); prefixStoreIter.Next() {
		// Set new key on store. Values don't change.
		oldKey := prefixStoreIter.Key()
		// The old key lacks a prefix, because the PrefixStore omits it on Get and expects no prefix on Set
		oldKeyWithPrefix := types.AppendBytes(keyPrefix, oldKey)
		value := prefixStoreIter.Value()
		newKey, err := getNewKey(value, cdc, oldKeyWithPrefix, keyPrefix)
		if err != nil {
			return err
		}
		if reflect.DeepEqual(oldKey, newKey) {
			// Nothing changed, don't write anything
			continue
		} else {
			// The key has changed
			if prefixStore.Has(newKey) {
				// Collisions are not allowed
				panic(fmt.Sprintf("New key collides with an existing key! %s", hex.EncodeToString(newKey)))
			}

			// Delete the old key and replace it with the new key
			prefixStore.Delete(oldKey)
			prefixStore.Set(newKey, value)
		}
	}
	return nil
}

// Creates a closure with the current logger for the attestation key conversion function
func getAttestationConverter(logger log.Logger) func([]byte, codec.BinaryCodec, []byte, []byte) ([]byte, error) {
	// Unmarshal the old Attestation, unpack its claim, recompute the key using the new ClaimHash
	// Note: The oldKey will contain the implicitPrefix, but the return key should **NOT** have the prefix
	return func(oldValue []byte, cdc codec.BinaryCodec, oldKey []byte, implicitPrefix []byte) ([]byte, error) {
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

		newKey, err := types.GetAttestationKey(claim.GetEventNonce(), hash), nil
		// The new key must be returned without a prefix, since it will be set on a PrefixStore
		newKeyNoPrefix := newKey[len(implicitPrefix):]

		// Get the old ClaimHash off the end of the old key
		oldClaimHash := oldKey[len(oldKey)-len(hash):]

		if claim.GetType() != types.CLAIM_TYPE_BATCH_SEND_TO_ETH {
			// Non-batch attestations should **not** be moved
			if !reflect.DeepEqual(oldKey, newKey) {
				logger.Error("Migrated an old attestaton to a new key!", "event-nonce", claim.GetEventNonce(),
					"eth-block-height", claim.GetEthBlockHeight(), "type", claim.GetType().String(),
					"old-key", hex.EncodeToString(oldKey), "new-key", hex.EncodeToString(newKey),
					"old-claim-hash", hex.EncodeToString(oldClaimHash), "new-claim-hash", hex.EncodeToString(hash),
				)
				panic("Attempted to migrate an attestation which should not have moved!")
			}
		} else {
			// Batch attestations **must** move, unless we have some sort of hash collision
			if reflect.DeepEqual(oldKey, newKey) {
				logger.Error("Failed to migrate an old batch!", "event-nonce", claim.GetEventNonce(),
					"eth-block-height", claim.GetEthBlockHeight(), "type", claim.GetType().String(),
					"old-key", hex.EncodeToString(oldKey), "new-key", hex.EncodeToString(newKey),
					"old-claim-hash", hex.EncodeToString(oldClaimHash), "new-claim-hash", hex.EncodeToString(hash),
				)
				panic("Failed to migrate an old batch!")
			} else {
				// Batch migrated to a new key!
				logger.Info("Successfully moved a batch to a new key!", "event-nonce", claim.GetEventNonce(),
					"eth-block-height", claim.GetEthBlockHeight(), "type", claim.GetType().String(),
					"old-key", hex.EncodeToString(oldKey), "new-key", hex.EncodeToString(newKey),
					"old-claim-hash", hex.EncodeToString(oldClaimHash), "new-claim-hash", hex.EncodeToString(hash),
				)
			}
		}

		// Reminder, the new key should **NOT** contain the prefix
		return newKeyNoPrefix, err
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
