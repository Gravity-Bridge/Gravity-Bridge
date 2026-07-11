package v7

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
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

	// Diagnostic tallies so a local upgrade replay can confirm that every in-progress attestation
	// (partial votes, not yet observed) as well as every observed attestation was accounted for.
	claimTypeCounts := map[types.ClaimType]int{}
	totalVotes := 0
	observedCount := 0
	unobservedCount := 0

	k.IterateAttestations(ctx, false, func(key []byte, att types.Attestation) bool {
		claim, err := k.UnpackAttestationClaim(&att)
		if err != nil {
			ctx.Logger().Error("Attestation migration: failed to unpack claim",
				"oldKey", hex.EncodeToString(key), "error", err)
			panic(fmt.Sprintf("Failed to unpack claim during migration: %v", err))
		}

		// Compute the new hash using the updated ClaimHash (with AttestationSeparator)
		newHash, err := claim.ClaimHash()
		if err != nil {
			ctx.Logger().Error("Attestation migration: failed to compute new hash",
				"oldKey", hex.EncodeToString(key), "error", err)
			panic(fmt.Sprintf("Failed to compute new hash during migration: %v", err))
		}

		// Extract eventNonce from the key (GetAttestationKey format: OracleAttestationKey + eventNonce + claimHash)
		// The key structure is: prefix + uint64(eventNonce) + claimHash
		keyPrefix := types.OracleAttestationKey
		eventNonceBytes := key[len(keyPrefix) : len(keyPrefix)+8]
		eventNonce := binary.BigEndian.Uint64(eventNonceBytes)

		claimTypeCounts[claim.GetType()]++
		totalVotes += len(att.Votes)
		if att.Observed {
			observedCount++
		} else {
			unobservedCount++
		}

		attestationsToMigrate = append(attestationsToMigrate, attestationToMigrate{
			oldKey:      key,
			eventNonce:  eventNonce,
			attestation: att,
			claim:       claim,
			newHash:     newHash,
		})

		return false // Continue
	})

	ctx.Logger().Info(fmt.Sprintf("Attestation migration: collected %d attestations to inspect", len(attestationsToMigrate)))
	ctx.Logger().Info("Attestation migration: pre-migration attestation breakdown",
		"total", len(attestationsToMigrate),
		"observed", observedCount,
		"unobserved(inProgress)", unobservedCount,
		"totalVotes", totalVotes,
		"claimTypeCounts", fmt.Sprintf("%v", claimTypeCounts),
	)

	// Now migrate all attestations
	migratedCount := 0
	unchangedCount := 0

	for _, item := range attestationsToMigrate {
		// Only migrate if the new hash differs from the old hash (i.e., if the separator changed the hash)
		oldHashStart := len(types.OracleAttestationKey) + 8
		oldHash := item.oldKey[oldHashStart:]

		if !bytes.Equal(oldHash, item.newHash) {
			ctx.Logger().Debug("Attestation migration: rewriting attestation key",
				"eventNonce", item.eventNonce,
				"oldHash", hex.EncodeToString(oldHash),
				"newHash", hex.EncodeToString(item.newHash),
			)

			// Delete old entry
			k.DeleteRawAttestationKey(ctx, item.oldKey)

			// Store under new key with new hash
			k.SetAttestation(ctx, item.eventNonce, item.newHash, &item.attestation)
		} else {
			unchangedCount++
		}
		migratedCount++
	}

	ctx.Logger().Info(fmt.Sprintf(
		"Attestation migration complete: inspected %d attestations, rewrote %d, left %d unchanged",
		migratedCount, migratedCount-unchangedCount, unchangedCount,
	))

	// Integrity check: re-key must be one-to-one. If two distinct old keys collided onto the same
	// new (eventNonce, hash) key, the second SetAttestation would have overwritten the first and we
	// would have fewer attestations than we collected — silently losing votes for an in-progress
	// attestation. Re-count and compare so a local replay surfaces any loss immediately.
	postCount := 0
	postVotes := 0
	k.IterateAttestations(ctx, false, func(_ []byte, att types.Attestation) bool {
		postCount++
		postVotes += len(att.Votes)
		return false
	})
	if postCount != len(attestationsToMigrate) || postVotes != totalVotes {
		ctx.Logger().Error("Attestation migration: INTEGRITY MISMATCH after re-keying attestations",
			"attestationsBefore", len(attestationsToMigrate), "attestationsAfter", postCount,
			"votesBefore", totalVotes, "votesAfter", postVotes)
		panic(fmt.Sprintf(
			"Attestation migration integrity failure: attestations before=%d after=%d, votes before=%d after=%d",
			len(attestationsToMigrate), postCount, totalVotes, postVotes,
		))
	}
	ctx.Logger().Info("Attestation migration: integrity check passed (attestation and vote counts preserved)",
		"attestations", postCount, "totalVotes", postVotes)
	return nil
}

// MigrateAttestationsToClaimHashComponents migrates all stored attestations to populate
// the new ClaimType and ClaimComponents fields from the stored claim.
func MigrateAttestationsToClaimHashComponents(ctx sdk.Context, k GravKeeper) error {
	ctx.Logger().Info("Begin attestation migration: populating ClaimType and ClaimComponents")

	var migrated int
	var inspected int

	k.IterateAttestations(ctx, false, func(key []byte, att types.Attestation) bool {
		inspected++
		if att.ClaimComponents != nil {
			ctx.Logger().Error("ClaimHashComponents migration: attestation already migrated",
				"oldKey", hex.EncodeToString(key), "attestation", att)
			panic(fmt.Sprintf("Attestation has already been migrated? %v", att))
		}

		claim, err := k.UnpackAttestationClaim(&att)
		if err != nil {
			ctx.Logger().Error("ClaimHashComponents migration: failed to unpack claim",
				"oldKey", hex.EncodeToString(key), "error", err)
			panic(fmt.Sprintf("Failed to unpack claim during ClaimHashComponents migration: %v", err))
		}

		claimType := claim.GetType()
		components, err := types.ExtractClaimHashComponents(claim)
		if err != nil {
			ctx.Logger().Error("ClaimHashComponents migration: failed to extract claim components",
				"eventNonce", claim.GetEventNonce(), "claimType", claimType, "error", err)
			panic(fmt.Sprintf("Failed to extract claim components during migration: %v", err))
		}

		// Sanity check: the hash computed from the components must match the hash in the store key.
		computedHash, err := components.ComputeClaimHash(claimType)
		if err != nil {
			ctx.Logger().Error("ClaimHashComponents migration: failed to compute claim hash from components",
				"eventNonce", claim.GetEventNonce(), "claimType", claimType, "error", err)
			panic(fmt.Sprintf("Failed to compute claim hash from components during migration: %v", err))
		}

		keyPrefix := types.OracleAttestationKey
		hashStart := len(keyPrefix) + 8
		storedHash := key[hashStart:]

		if !bytes.Equal(storedHash, computedHash) {
			ctx.Logger().Error("ClaimHashComponents migration: claim hash mismatch",
				"eventNonce", claim.GetEventNonce(),
				"claimType", claimType,
				"storedHash", hex.EncodeToString(storedHash),
				"computedHash", hex.EncodeToString(computedHash),
			)
			panic(fmt.Sprintf(
				"Claim hash mismatch during ClaimHashComponents migration: stored %x, computed %x",
				storedHash, computedHash,
			))
		}

		att.ClaimType = claimType
		att.ClaimComponents = components

		eventNonce := claim.GetEventNonce()
		k.SetAttestation(ctx, eventNonce, storedHash, &att)

		ctx.Logger().Debug("ClaimHashComponents migration: attestation migrated",
			"eventNonce", eventNonce,
			"claimType", claimType,
			"hash", hex.EncodeToString(storedHash),
		)

		migrated++
		return false
	})

	ctx.Logger().Info(fmt.Sprintf(
		"ClaimHashComponents migration complete: inspected %d attestations, migrated %d",
		inspected, migrated,
	))
	return nil
}
