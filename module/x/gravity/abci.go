package gravity

import (
	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/keeper"
	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// EndBlocker is called at the end of every block
func EndBlocker(ctx sdk.Context, k keeper.Keeper) {
	params := k.GetParams(ctx)
	evmChains := k.GetEvmChains(ctx)

	for _, evmChain := range evmChains {
		slashing(ctx, k, params, evmChain.EvmChainPrefix)
		attestationTally(ctx, k, evmChain.EvmChainPrefix)
		cleanupTimedOutBatches(ctx, k, evmChain.EvmChainPrefix)
		cleanupTimedOutLogicCalls(ctx, k, evmChain.EvmChainPrefix)
		createValsets(ctx, k, evmChain.EvmChainPrefix)
		pruneValsets(ctx, k, params, evmChain.EvmChainPrefix)
		pruneAttestations(ctx, k, evmChain.EvmChainPrefix)
	}
}

func createValsets(ctx sdk.Context, k keeper.Keeper, evmChainPrefix string) {
	// Auto ValsetRequest Creation.
	// WARNING: do not use k.GetLastObservedValset in this function, it *will* result in losing control of the bridge
	// 1. If there are no valset requests, create a new one.
	// 2. If there is at least one validator who started unbonding in current block. (we persist last unbonded block height in hooks.go)
	// This will make sure the unbonding validator has to provide an attestation to a new Valset
	// that excludes him before he completely Unbonds.  Otherwise he will be slashed
	// 3. If power change between validators of CurrentValset and latest valset request is > 5%

	// get the last valsets to compare against
	latestValset := k.GetLatestValset(ctx, evmChainPrefix)
	lastUnbondingHeight := k.GetLastUnBondingBlockHeight(ctx)

	significantPowerDiff := false
	if latestValset != nil {
		vs, err := k.GetCurrentValset(ctx, evmChainPrefix)
		if err != nil {
			// this condition should only occur in the simulator
			// ref : https://github.com/Gravity-Bridge/Gravity-Bridge/issues/35
			if err == types.ErrNoValidators {
				ctx.Logger().Error("no bonded validators",
					"cause", err.Error(),
				)
				return
			}
			panic(err)
		}
		intCurrMembers, err := types.BridgeValidators(vs.Members).ToInternal()
		if err != nil {
			panic(sdkerrors.Wrap(err, "invalid current valset members"))
		}
		intLatestMembers, err := types.BridgeValidators(latestValset.Members).ToInternal()
		if err != nil {
			panic(sdkerrors.Wrap(err, "invalid latest valset members"))
		}

		significantPowerDiff = intCurrMembers.PowerDiff(*intLatestMembers).GT(sdk.NewDecWithPrec(5, 2))
	}

	if (latestValset == nil) || (lastUnbondingHeight == uint64(ctx.BlockHeight())) || significantPowerDiff {
		// if the conditions are true, put in a new validator set request to be signed and submitted to evm chain
		k.SetValsetRequest(ctx, evmChainPrefix)
	}
}

func pruneValsets(ctx sdk.Context, k keeper.Keeper, params types.Params, evmChainPrefix string) {
	// Validator set pruning
	// prune all validator sets with a nonce less than the
	// last observed nonce, they can't be submitted any longer
	// Only prune valsets after the signed valsets window has passed
	// so that slashing can occur the block before we remove them
	lastObserved := k.GetLastObservedValset(ctx, evmChainPrefix)
	currentBlock := uint64(ctx.BlockHeight())
	tooEarly := currentBlock < params.SignedValsetsWindow
	if lastObserved != nil && !tooEarly {
		earliestToPrune := currentBlock - params.GetSignedValsetsWindow()
		sets := k.GetValsets(ctx, evmChainPrefix)
		for _, set := range sets {
			if set.Nonce < lastObserved.Nonce && set.Height < earliestToPrune {
				k.DeleteValset(ctx, evmChainPrefix, set.Nonce)
				k.DeleteValsetConfirms(ctx, evmChainPrefix, set.Nonce)
			}
		}
	}
}

func slashing(ctx sdk.Context, k keeper.Keeper, params types.Params, evmChainPrefix string) {
	// Slash validator for not confirming valset requests, batch requests, logic call requests
	valsetSlashing(ctx, k, params, evmChainPrefix)
	batchSlashing(ctx, k, params, evmChainPrefix)
	logicCallSlashing(ctx, k, params, evmChainPrefix)
}

// Iterate over all attestations currently being voted on in order of nonce and
// "Observe" those who have passed the threshold. Break the loop once we see
// an attestation that has not passed the threshold
func attestationTally(ctx sdk.Context, k keeper.Keeper, evmChainPrefix string) {
	params := k.GetParams(ctx)

	evmChainParams := params.EvmChain(evmChainPrefix)

	if evmChainParams == nil {
		return
	}

	// bridge is currently disabled, do not process attestations from evm chain
	if !evmChainParams.BridgeActive {
		return
	}

	attmap, keys := k.GetAttestationMapping(ctx, evmChainPrefix)

	// This iterates over all keys (event nonces) in the attestation mapping. Each value contains
	// a slice with one or more attestations at that event nonce. There can be multiple attestations
	// at one event nonce when validators disagree about what event happened at that nonce.
	for _, nonce := range keys {
		// This iterates over all attestations at a particular event nonce.
		// They are ordered by when the first attestation at the event nonce was received.
		// This order is not important.
		for _, att := range attmap[nonce] {
			// We check if the event nonce is exactly 1 higher than the last attestation that was
			// observed. If it is not, we just move on to the next nonce. This will skip over all
			// attestations that have already been observed.
			//
			// Once we hit an event nonce that is one higher than the last observed event, we stop
			// skipping over this conditional and start calling tryAttestation (counting votes)
			// Once an attestation at a given event nonce has enough votes and becomes observed,
			// every other attestation at that nonce will be skipped, since the lastObservedEventNonce
			// will be incremented.
			//
			// Then we go to the next event nonce in the attestation mapping, if there is one. This
			// nonce will once again be one higher than the lastObservedEventNonce.
			// If there is an attestation at this event nonce which has enough votes to be observed,
			// we skip the other attestations and move on to the next nonce again.
			// If no attestation becomes observed, when we get to the next nonce, every attestation in
			// it will be skipped. The same will happen for every nonce after that.
			if nonce == uint64(k.GetLastObservedEventNonce(ctx, evmChainPrefix))+1 {
				k.TryAttestation(ctx, &att)
			}
		}
	}
}

// cleanupTimedOutBatches deletes batches that have passed their expiration on evm chain
// keep in mind several things when modifying this function
// A) unlike nonces timeouts are not monotonically increasing, meaning batch 5 can have a later timeout than batch 6
//    this means that we MUST only cleanup a single batch at a time
// B) it is possible for evm chain height to be zero if no events have ever occurred, make sure your code accounts for this
// C) When we compute the timeout we do our best to estimate the evm chain block height at that very second. But what we work with
//    here is the evm chain block height at the time of the last Deposit or Withdraw to be observed. It's very important we do not
//    project, if we do a slowdown on evm chain could cause a double spend. Instead timeouts will *only* occur after the timeout period
//    AND any deposit or withdraw has occurred to update the evm chain block height.
func cleanupTimedOutBatches(ctx sdk.Context, k keeper.Keeper, evmChainPrefix string) {
	evmChainHeight := k.GetLastObservedEvmChainBlockHeight(ctx, evmChainPrefix).EthereumBlockHeight
	batches := k.GetOutgoingTxBatches(ctx, evmChainPrefix)
	for _, batch := range batches {
		if batch.BatchTimeout < evmChainHeight {
			err := k.CancelOutgoingTxBatch(ctx, evmChainPrefix, batch.TokenContract, batch.BatchNonce)
			if err != nil {
				panic("Failed to cancel outgoing txbatch!")
			}
		}
	}
}

// cleanupTimedOutBatches deletes logic calls that have passed their expiration on evm chain
// keep in mind several things when modifying this function
// A) unlike nonces timeouts are not monotonically increasing, meaning call 5 can have a later timeout than batch 6
//    this means that we MUST only cleanup a single call at a time
// B) it is possible for  evm chain height to be zero if no events have ever occurred, make sure your code accounts for this
// C) When we compute the timeout we do our best to estimate the evm chain block height at that very second. But what we work with
//    here is the evm chain block height at the time of the last Deposit or Withdraw to be observed. It's very important we do not
//    project, if we do a slowdown on evm chain could cause a double spend. Instead timeouts will *only* occur after the timeout period
//    AND any deposit or withdraw has occurred to update the evm chain block height.
func cleanupTimedOutLogicCalls(ctx sdk.Context, k keeper.Keeper, evmChainPrefix string) {
	evmChainHeight := k.GetLastObservedEvmChainBlockHeight(ctx, evmChainPrefix).EthereumBlockHeight
	calls := k.GetOutgoingLogicCalls(ctx, evmChainPrefix)
	for _, call := range calls {
		if call.Timeout < evmChainHeight {
			err := k.CancelOutgoingLogicCall(ctx, evmChainPrefix, call.InvalidationId, call.InvalidationNonce)
			if err != nil {
				panic("Failed to cancel outgoing logic call!")
			}
		}
	}
}

// prepValsetConfirms loads all confirmations into a hashmap indexed by validatorAddr
// reducing the lookup time dramatically and separating out the task of looking up
// the orchestrator for each validator
func prepValsetConfirms(ctx sdk.Context, k keeper.Keeper, evmChainPrefix string, nonce uint64) map[string]types.MsgValsetConfirm {
	confirms := k.GetValsetConfirms(ctx, evmChainPrefix, nonce)
	// bytes are incomparable in go, so we convert the sdk.ValAddr bytes to a string
	ret := make(map[string]types.MsgValsetConfirm)
	for _, confirm := range confirms {
		// TODO this presents problems for delegate key rotation see issue #344
		confVal, err := sdk.AccAddressFromBech32(confirm.Orchestrator)
		if err != nil {
			panic("Invalid confirm in store")
		}
		val, foundValidator := k.GetOrchestratorValidatorAddr(ctx, confVal)
		if !foundValidator {
			// This means that the validator never sent a SetOrchestratorAddress message.
			panic("Confirm from validator we can't identify?")
		}
		ret[val.String()] = confirm
	}
	return ret
}

// valsetSlashing slashes validators who have not signed validator sets during the signing window
func valsetSlashing(ctx sdk.Context, k keeper.Keeper, params types.Params, evmChainPrefix string) {
	// don't slash in the beginning before there aren't even SignedValsetsWindow blocks yet
	if uint64(ctx.BlockHeight()) <= params.SignedValsetsWindow {
		return
	}
	unslashedValsets := k.GetUnSlashedValsets(ctx, evmChainPrefix, params.SignedValsetsWindow)

	currentBondedSet := k.StakingKeeper.GetBondedValidatorsByPower(ctx)
	unbondingValidators := getUnbondingValidators(ctx, k)

	for _, vs := range unslashedValsets {
		confirms := prepValsetConfirms(ctx, k, evmChainPrefix, vs.Nonce)

		// SLASH BONDED VALIDTORS who didn't attest valset request

		for _, val := range currentBondedSet {
			consAddr, err := val.GetConsAddr()
			if err != nil {
				panic("Failed to get validator consensus addr")
			}
			valSigningInfo, exist := k.SlashingKeeper.GetValidatorSigningInfo(ctx, consAddr)

			// Slash validator ONLY if they joined before valset was created
			startedBeforeValsetCreated := valSigningInfo.StartHeight < int64(vs.Height)

			if exist && startedBeforeValsetCreated {
				// Check if validator has confirmed valset or not
				_, found := confirms[val.GetOperator().String()]
				// slash validators for not confirming valsets
				if !found {
					// refresh validator before slashing/jailing
					val = updateValidator(ctx, k, val.GetOperator())
					if !val.IsJailed() {
						k.StakingKeeper.Slash(ctx, consAddr, ctx.BlockHeight(), val.ConsensusPower(sdk.DefaultPowerReduction), params.SlashFractionValset)
						ctx.EventManager().EmitTypedEvent(
							&types.EventSignatureSlashing{
								Type:    types.AttributeKeyValsetSignatureSlashing,
								Address: consAddr.String(),
							},
						)

						k.StakingKeeper.Jail(ctx, consAddr)
					}

				}
			}
		}

		// SLASH UNBONDING VALIDATORS who didn't attest valset request

		for _, valAddr := range unbondingValidators {
			addr, err := sdk.ValAddressFromBech32(valAddr)
			if err != nil {
				panic(err)
			}
			validator, found := k.StakingKeeper.GetValidator(ctx, sdk.ValAddress(addr))
			if !found {
				panic("Unable to find validator!")
			}
			valConsAddr, err := validator.GetConsAddr()
			if err != nil {
				panic(err)
			}
			valSigningInfo, exist := k.SlashingKeeper.GetValidatorSigningInfo(ctx, valConsAddr)

			// Only slash validators who joined after valset is created and they are unbonding and UNBOND_SLASHING_WINDOW hasn't passed
			startedBeforeValsetCreated := valSigningInfo.StartHeight < int64(vs.Height)
			unbondingPeriodEndsAfterSlashingPeriod := vs.Height < uint64(validator.UnbondingHeight)+params.UnbondSlashingValsetsWindow

			if exist && startedBeforeValsetCreated && validator.IsUnbonding() && unbondingPeriodEndsAfterSlashingPeriod {
				// Check if validator has confirmed valset or not
				_, found := confirms[validator.GetOperator().String()]

				// slash validators for not confirming valsets
				if !found {
					// refresh validator before slashing/jailing
					validator = updateValidator(ctx, k, validator.GetOperator())
					if !validator.IsJailed() {
						k.StakingKeeper.Slash(ctx, valConsAddr, ctx.BlockHeight(), validator.ConsensusPower(sdk.DefaultPowerReduction), params.SlashFractionValset)
						ctx.EventManager().EmitTypedEvent(
							&types.EventSignatureSlashing{
								Type:    types.AttributeKeyValsetSignatureSlashing,
								Address: valConsAddr.String(),
							},
						)
						k.StakingKeeper.Jail(ctx, valConsAddr)
					}
				}
			}
		}
		// then we set the latest slashed valset  nonce
		k.SetLastSlashedValsetNonce(ctx, evmChainPrefix, vs.Nonce)
	}
}

// updateValidator is a very specific utility function, used to update the validator object during
// slashing loops. This allows us to load the validators list at the start of our slashing and only
// pull in individual validators as needed to check that we are not jailing them twice, or slashing
// them improperly
func updateValidator(ctx sdk.Context, k keeper.Keeper, val sdk.ValAddress) stakingtypes.Validator {
	valObj, found := k.StakingKeeper.GetValidator(ctx, val)
	if !found {
		// this should be impossible, we haven't even progressed a single block since we got the list
		panic("Validator exited set during endblocker?")
	}
	return valObj
}

// getUnbondingValidators gets all currently unbonding validators in groups based on
// the block at which they will finish validating.
func getUnbondingValidators(ctx sdk.Context, k keeper.Keeper) (addresses []string) {
	blockTime := ctx.BlockTime().Add(k.StakingKeeper.GetParams(ctx).UnbondingTime)
	blockHeight := ctx.BlockHeight()
	unbondingValIterator := k.StakingKeeper.ValidatorQueueIterator(ctx, blockTime, blockHeight)
	defer unbondingValIterator.Close()

	// All unbonding validators
	for ; unbondingValIterator.Valid(); unbondingValIterator.Next() {
		unbondingValidators := k.DeserializeValidatorIterator(unbondingValIterator.Value())
		addresses = append(addresses, unbondingValidators.Addresses...)
	}
	return addresses
}

// prepBatchConfirms loads all confirmations into a hashmap indexed by validatorAddr
// reducing the lookup time dramatically and separating out the task of looking up
// the orchestrator for each validator
func prepBatchConfirms(ctx sdk.Context, k keeper.Keeper, evmChainPrefix string, batch types.InternalOutgoingTxBatch) map[string]types.MsgConfirmBatch {
	confirms := k.GetBatchConfirmByNonceAndTokenContract(ctx, evmChainPrefix, batch.BatchNonce, batch.TokenContract)
	// bytes are incomparable in go, so we convert the sdk.ValAddr bytes to a string (note this is NOT bech32)
	ret := make(map[string]types.MsgConfirmBatch)
	for _, confirm := range confirms {
		// TODO this presents problems for delegate key rotation see issue #344
		confVal, err := sdk.AccAddressFromBech32(confirm.Orchestrator)
		if err != nil {
			panic(err)
		}
		val, foundValidator := k.GetOrchestratorValidatorAddr(ctx, confVal)
		if !foundValidator {
			// This means that the validator never sent a SetOrchestratorAddress message.
			panic("Confirm from validator we can't identify?")
		}
		ret[val.String()] = confirm
	}
	return ret
}

// batchSlashing slashes currently bonded validators who have not submitted batch
// signatures. This is distinct from validator sets, which includes unbonding validators
// because validator set updates must succeed as validators leave the set, batches will just be re-created
func batchSlashing(ctx sdk.Context, k keeper.Keeper, params types.Params, evmChainPrefix string) {
	// We look through the full bonded set (the active set)
	// and we slash users who haven't signed a batch confirmation that is >15hrs in blocks old
	var maxHeight uint64

	// don't slash in the beginning before there aren't even SignedBatchesWindow blocks yet
	if uint64(ctx.BlockHeight()) > params.SignedBatchesWindow {
		maxHeight = uint64(ctx.BlockHeight()) - params.SignedBatchesWindow
	} else {
		// we can't slash anyone if this window has not yet passed
		return
	}

	currentBondedSet := k.StakingKeeper.GetBondedValidatorsByPower(ctx)
	unslashedBatches := k.GetUnSlashedBatches(ctx, evmChainPrefix, maxHeight)
	for _, batch := range unslashedBatches {
		// SLASH BONDED VALIDTORS who didn't attest batch requests
		confirms := prepBatchConfirms(ctx, k, evmChainPrefix, batch)
		for _, val := range currentBondedSet {
			consAddr, err := val.GetConsAddr()
			if err != nil {
				panic(err)
			}
			valSigningInfo, exist := k.SlashingKeeper.GetValidatorSigningInfo(ctx, consAddr)

			// Don't slash validators who joined after batch is created
			startedBeforeBatchCreated := valSigningInfo.StartHeight < int64(batch.CosmosBlockCreated)
			if exist && startedBeforeBatchCreated {
				// check if validator confirmed the batch
				_, found := confirms[val.GetOperator().String()]
				// slashing for not confirming the batch
				if !found {
					// refresh validator before slashing/jailing
					val = updateValidator(ctx, k, val.GetOperator())
					if !val.IsJailed() {
						k.StakingKeeper.Slash(ctx, consAddr, ctx.BlockHeight(), val.ConsensusPower(sdk.DefaultPowerReduction), params.SlashFractionBatch)
						ctx.EventManager().EmitTypedEvent(
							&types.EventSignatureSlashing{
								Type:    types.AttributeKeyBatchSignatureSlashing,
								Address: consAddr.String(),
							},
						)
						k.StakingKeeper.Jail(ctx, consAddr)
					}
				}
			}
		}
		// then we set the latest slashed batch block
		k.SetLastSlashedBatchBlock(ctx, evmChainPrefix, batch.CosmosBlockCreated)
	}
}

// prepLogicCallConfirms loads all confirmations into a hashmap indexed by validatorAddr
// reducing the lookup time dramatically and separating out the task of looking up
// the orchestrator for each validator
func prepLogicCallConfirms(ctx sdk.Context, k keeper.Keeper, evmChainPrefix string, call types.OutgoingLogicCall) map[string]*types.MsgConfirmLogicCall {
	confirms := k.GetLogicConfirmsByInvalidationIDAndNonce(ctx, evmChainPrefix, call.InvalidationId, call.InvalidationNonce)
	// bytes are incomparable in go, so we convert the sdk.ValAddr bytes to a string (note this is NOT bech32)
	ret := make(map[string]*types.MsgConfirmLogicCall)
	for _, confirm := range confirms {
		// TODO this presents problems for delegate key rotation see issue #344
		confVal, err := sdk.AccAddressFromBech32(confirm.Orchestrator)
		if err != nil {
			panic(err)
		}
		val, foundValidator := k.GetOrchestratorValidatorAddr(ctx, confVal)
		if !foundValidator {
			// This means that the validator never sent a SetOrchestratorAddress message.
			panic("Confirm from validator we can't identify?")
		}
		ret[val.String()] = &confirm
	}
	return ret
}

// logicCallSlashing slashes currently bonded validators who have not submitted logicCall
// signatures. This is distinct from validator sets, which includes unbonding validators
// because validator set updates must succeed as validators leave the set, logicCalls will just be re-created
func logicCallSlashing(ctx sdk.Context, k keeper.Keeper, params types.Params, evmChainPrefix string) {
	// We look through the full bonded set (the active set)
	// and we slash users who haven't signed a batch confirmation that is >15hrs in blocks old
	var maxHeight uint64

	// don't slash in the beginning before there aren't even SignedBatchesWindow blocks yet
	if uint64(ctx.BlockHeight()) > params.SignedLogicCallsWindow {
		maxHeight = uint64(ctx.BlockHeight()) - params.SignedLogicCallsWindow
	} else {
		// we can't slash anyone if this window has not yet passed
		return
	}

	currentBondedSet := k.StakingKeeper.GetBondedValidatorsByPower(ctx)
	unslashedLogicCalls := k.GetUnSlashedLogicCalls(ctx, evmChainPrefix, maxHeight)
	for _, call := range unslashedLogicCalls {

		// SLASH BONDED VALIDTORS who didn't attest batch requests
		confirms := prepLogicCallConfirms(ctx, k, evmChainPrefix, call)
		for _, val := range currentBondedSet {
			// Don't slash validators who joined after batch is created
			consAddr, err := val.GetConsAddr()
			if err != nil {
				panic(err)
			}
			valSigningInfo, exist := k.SlashingKeeper.GetValidatorSigningInfo(ctx, consAddr)
			startedBeforeCallCreated := valSigningInfo.StartHeight < int64(call.CosmosBlockCreated)
			if exist && startedBeforeCallCreated {
				// check that the validator confirmed the logic call
				_, found := confirms[val.GetOperator().String()]
				if !found {
					// refresh validator before slashing/jailing
					val = updateValidator(ctx, k, val.GetOperator())
					if !val.IsJailed() {
						k.StakingKeeper.Slash(ctx, consAddr, ctx.BlockHeight(), val.ConsensusPower(sdk.DefaultPowerReduction), params.SlashFractionLogicCall)
						ctx.EventManager().EmitTypedEvent(
							&types.EventSignatureSlashing{
								Type:    types.AttributeKeyLogicCallSignatureSlashing,
								Address: consAddr.String(),
							},
						)
						k.StakingKeeper.Jail(ctx, consAddr)
					}
				}
			}
		}
		// then we set the latest slashed logic call block
		k.SetLastSlashedLogicCallBlock(ctx, evmChainPrefix, call.CosmosBlockCreated)
	}
}

// Iterate over all attestations currently being voted on in order of nonce
// and prune those that are older than the current nonce and no longer have any
// use. This could be combined with create attestation and save some computation
// but (A) pruning keeps the iteration small in the first place and (B) there is
// already enough nuance in the other handler that it's best not to complicate it further
func pruneAttestations(ctx sdk.Context, k keeper.Keeper, evmChainPrefix string) {
	attmap, keys := k.GetAttestationMapping(ctx, evmChainPrefix)

	// we delete all attestations earlier than the current event nonce
	// minus some buffer value. This buffer value is purely to allow
	// frontends and other UI components to view recent oracle history
	const eventsToKeep = 1000
	lastNonce := uint64(k.GetLastObservedEventNonce(ctx, evmChainPrefix))
	var cutoff uint64
	if lastNonce <= eventsToKeep {
		return
	} else {
		cutoff = lastNonce - eventsToKeep
	}

	// This iterates over all keys (event nonces) in the attestation mapping. Each value contains
	// a slice with one or more attestations at that event nonce. There can be multiple attestations
	// at one event nonce when validators disagree about what event happened at that nonce.
	for _, nonce := range keys {
		// This iterates over all attestations at a particular event nonce.
		// They are ordered by when the first attestation at the event nonce was received.
		// This order is not important.
		for _, att := range attmap[nonce] {
			// delete all before the cutoff
			if nonce < cutoff {
				k.DeleteAttestation(ctx, att)
			}
		}
	}
}
