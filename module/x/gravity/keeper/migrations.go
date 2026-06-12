package keeper

import (
	"bytes"
	"encoding/binary"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	paramstypes "github.com/cosmos/cosmos-sdk/x/params/types"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
)

// Migrator is a struct for handling in-place store migrations.
type Migrator struct {
	keeper           Keeper
	legacyParamSpace paramstypes.Subspace
}

// NewMigrator returns a new Migrator.
func NewMigrator(keeper Keeper, legacyParamSpace paramstypes.Subspace) Migrator {
	return Migrator{keeper: keeper, legacyParamSpace: legacyParamSpace}
}

// The following migrations are commented out because they rely on an old paramSpace field
// which no longer exists and will cause trouble to keep trying to use.
// They are kept here for reference for future migrations.

// // Migrate3to4 migrates from consensus version 3 to 4.
// func (m Migrator) Migrate3to4(ctx sdk.Context) error {
// 	ctx.Logger().Info("Pleiades Upgrade part 2: Enter Migrate3to4()")
// 	newParams := v4.MigrateParams(ctx, m.keeper.paramSpace)
// 	m.keeper.SetParams(ctx, newParams)
// 	ctx.Logger().Info("Pleiades Upgrade part 2: Gravity module migration is complete!")
// 	return nil
// }

// // Migrate4to5 migrates from consensus version 4 to 5.
// func (m Migrator) Migrate4to5(ctx sdk.Context) error {
// 	ctx.Logger().Info("Begin Gravity v4 -> v5 migration")
// 	newParams := v5.MigrateParams(ctx, m.keeper.paramSpace)
// 	m.keeper.SetParams(ctx, newParams)
// 	ctx.Logger().Info("Gravity migration finished!")
// 	return nil
// }

func (m Migrator) Migrate5to6(ctx sdk.Context) error {
	ctx.Logger().Info("Begin Gravity v5 -> v6 migration")

	// nolint: exhaustruct
	params := types.Params{}

	m.legacyParamSpace.GetParamSet(ctx, &params)

	err := m.keeper.SetParams(ctx, params)
	if err != nil {
		return err
	}

	ctx.Logger().Info("Gravity v6 migration finished!")
	return nil
}

// MigrateAttestationsToAttestationSeparator migrates all stored attestations from "/" separator to AttestationSeparator in hash keys.
// It iterates through all existing attestations, extracts the claim, recomputes the hash with the new separator,
// and stores it under the new key while deleting the old key.
func (m Migrator) MigrateAttestationsToAttestationSeparator(ctx sdk.Context) error {
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

	m.keeper.IterateAttestations(ctx, false, func(key []byte, att types.Attestation) bool {
		claim, err := m.keeper.UnpackAttestationClaim(&att)
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
	store := ctx.KVStore(m.keeper.storeKey)
	migratedCount := 0

	for _, item := range attestationsToMigrate {
		// Only migrate if the new hash differs from the old hash (i.e., if the separator changed the hash)
		oldHashStart := len(types.OracleAttestationKey) + 8
		oldHash := item.oldKey[oldHashStart:]

		if !bytes.Equal(oldHash, item.newHash) {
			// Delete old entry
			store.Delete(item.oldKey)

			// Store under new key with new hash
			m.keeper.SetAttestation(ctx, item.eventNonce, item.newHash, &item.attestation)
		}
		migratedCount++
	}

	ctx.Logger().Info(fmt.Sprintf("Attestation migration complete: migrated %d attestations", migratedCount))
	return nil
}
