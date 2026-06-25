package v7

import (
	"bytes"
	"encoding/binary"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
)

// AttestationMigrationKeeper defines the keeper methods required by
// MigrateAttestationsToAttestationSeparator. It is satisfied by the gravity module's Keeper type.
type AttestationMigrationKeeper interface {
	IterateAttestations(ctx sdk.Context, reverse bool, cb func(key []byte, att types.Attestation) (stop bool))
	UnpackAttestationClaim(att *types.Attestation) (types.EthereumClaim, error)
	SetAttestation(ctx sdk.Context, eventNonce uint64, claimHash []byte, att *types.Attestation)
	DeleteRawAttestationKey(ctx sdk.Context, key []byte)
}

// MigrateAttestationsToAttestationSeparator migrates all stored attestations from "/" separator to AttestationSeparator in hash keys.
// It iterates through all existing attestations, extracts the claim, recomputes the hash with the new separator,
// and stores it under the new key while deleting the old key.
func MigrateAttestationsToAttestationSeparator(ctx sdk.Context, k AttestationMigrationKeeper) error {
	ctx.Logger().Info("Begin attestation migration: updating all claim hashes")

	// Collect all attestations with their old keys and claims
	type attestationToMigrate struct {
		oldKey      []byte
		eventNonce  uint64
		attestation types.Attestation
		claim       types.EthereumClaim
		newHash     []byte
	}

	var attestationsToMigrate []attestationToMigrate

	k.IterateAttestations(ctx, false, func(key []byte, att types.Attestation) bool {
		claim, err := k.UnpackAttestationClaim(&att)
		if err != nil {
			panic(fmt.Sprintf("Failed to unpack claim during migration: %v", err))
		}

		// Compute the new hash using the updated ClaimHash (with AttestationSeparator)
		newHash, err := claim.ClaimHash()
		if err != nil {
			panic(fmt.Sprintf("Failed to compute new hash during migration: %v", err))
		}

		// Extract eventNonce from the key (GetAttestationKey format: OracleAttestationKey + eventNonce + claimHash)
		// The key structure is: prefix + uint64(eventNonce) + claimHash
		keyPrefix := types.OracleAttestationKey
		eventNonceBytes := key[len(keyPrefix) : len(keyPrefix)+8]
		eventNonce := binary.BigEndian.Uint64(eventNonceBytes)

		attestationsToMigrate = append(attestationsToMigrate, attestationToMigrate{
			oldKey:      key,
			eventNonce:  eventNonce,
			attestation: att,
			claim:       claim,
			newHash:     newHash,
		})

		return false // Continue
	})

	// Now migrate all attestations
	migratedCount := 0

	for _, item := range attestationsToMigrate {
		// Only migrate if the new hash differs from the old hash (i.e., if the separator changed the hash)
		oldHashStart := len(types.OracleAttestationKey) + 8
		oldHash := item.oldKey[oldHashStart:]

		if !bytes.Equal(oldHash, item.newHash) {
			// Delete old entry
			k.DeleteRawAttestationKey(ctx, item.oldKey)

			// Store under new key with new hash
			k.SetAttestation(ctx, item.eventNonce, item.newHash, &item.attestation)
		}
		migratedCount++
	}

	ctx.Logger().Info(fmt.Sprintf("Attestation migration complete: migrated %d attestations", migratedCount))
	return nil
}
