package v7

import (
	"bytes"
	"encoding/binary"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
)

// GravKeeper defines the expected interface needed to perform the v7 migration
type GravKeeper interface {
	IterateAttestations(ctx sdk.Context, reverse bool, cb func(key []byte, att types.Attestation) (stop bool))
	UnpackAttestationClaim(att *types.Attestation) (types.EthereumClaim, error)
	SetAttestation(ctx sdk.Context, eventNonce uint64, claimHash []byte, att *types.Attestation)
	DeleteRawAttestationKey(ctx sdk.Context, key []byte)
}

// MigrateAttestationsToAttestationSeparator migrates all stored attestations from "/" separator to AttestationSeparator in hash keys.
// It iterates through all existing attestations, extracts the claim, recomputes the hash with the new separator,
// and stores it under the new key while deleting the old key.
func MigrateAttestationsToAttestationSeparator(ctx sdk.Context, k GravKeeper) error {
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

// MigrateAttestationsToClaimHashComponents migrates all stored attestations to populate
// the new ClaimType and ClaimComponents fields from the stored claim.
func MigrateAttestationsToClaimHashComponents(ctx sdk.Context, k GravKeeper) error {
	ctx.Logger().Info("Begin attestation migration: populating ClaimType and ClaimComponents")

	var migrated int

	k.IterateAttestations(ctx, false, func(key []byte, att types.Attestation) bool {
		if att.ClaimComponents != nil {
			panic(fmt.Sprintf("Attestation has already been migrated? %v", att))
		}

		claim, err := k.UnpackAttestationClaim(&att)
		if err != nil {
			panic(fmt.Sprintf("Failed to unpack claim during ClaimHashComponents migration: %v", err))
		}

		claimType := claim.GetType()
		components, err := types.ExtractClaimHashComponents(claim)
		if err != nil {
			panic(fmt.Sprintf("Failed to extract claim components during migration: %v", err))
		}

		// Sanity check: the hash computed from the components must match the hash in the store key.
		computedHash, err := components.ComputeClaimHash(claimType)
		if err != nil {
			panic(fmt.Sprintf("Failed to compute claim hash from components during migration: %v", err))
		}

		keyPrefix := types.OracleAttestationKey
		hashStart := len(keyPrefix) + 8
		storedHash := key[hashStart:]

		if !bytes.Equal(storedHash, computedHash) {
			panic(fmt.Sprintf(
				"Claim hash mismatch during ClaimHashComponents migration: stored %x, computed %x",
				storedHash, computedHash,
			))
		}

		att.ClaimType = claimType
		att.ClaimComponents = components

		eventNonce := claim.GetEventNonce()
		k.SetAttestation(ctx, eventNonce, storedHash, &att)

		migrated++
		return false
	})

	ctx.Logger().Info(fmt.Sprintf("ClaimHashComponents migration complete: migrated %d attestations", migrated))
	return nil
}
