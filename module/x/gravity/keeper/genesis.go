package keeper

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
)

func initBridgeDataFromGenesis(ctx sdk.Context, k Keeper, cd types.EvmChainData) {
	// reset valsets in state
	chainPrefix := cd.EvmChain.EvmChainPrefix
	highest := uint64(0)
	for _, vs := range cd.Valsets {
		if vs.Nonce > highest {
			highest = vs.Nonce
		}
		k.StoreValset(ctx, chainPrefix, vs)
	}
	k.SetLatestValsetNonce(ctx, chainPrefix, highest)

	// reset valset confirmations in state
	for _, conf := range cd.ValsetConfirms {
		k.SetValsetConfirm(ctx, chainPrefix, conf)
	}

	// reset batches in state
	for _, batch := range cd.Batches {
		// TODO: block height?
		intBatch, err := batch.ToInternal()
		if err != nil {
			panic(sdkerrors.Wrapf(err, "unable to make batch internal: %v", batch))
		}
		k.StoreBatch(ctx, chainPrefix, *intBatch)
	}

	// reset batch confirmations in state
	for _, conf := range cd.BatchConfirms {
		conf := conf
		k.SetBatchConfirm(ctx, chainPrefix, &conf)
	}

	// reset logic calls in state
	for _, call := range cd.LogicCalls {
		k.SetOutgoingLogicCall(ctx, chainPrefix, call)
	}

	// reset logic call confirmations in state
	for _, conf := range cd.LogicCallConfirms {
		conf := conf
		k.SetLogicCallConfirm(ctx, chainPrefix, &conf)
	}
}

// InitGenesis starts a chain from a genesis state
func InitGenesis(ctx sdk.Context, k Keeper, data types.GenesisState) {
	k.SetParams(ctx, *data.Params)

	for _, cd := range data.EvmChains {
		// restore various nonces, this MUST match GravityNonces in genesis
		chainPrefix := cd.EvmChain.EvmChainPrefix
		k.SetLatestValsetNonce(ctx, chainPrefix, cd.GravityNonces.LatestValsetNonce)
		k.setLastObservedEventNonce(ctx, chainPrefix, cd.GravityNonces.LastObservedNonce)
		k.SetLastSlashedValsetNonce(ctx, chainPrefix, cd.GravityNonces.LastSlashedValsetNonce)
		k.SetLastSlashedBatchBlock(ctx, chainPrefix, cd.GravityNonces.LastSlashedBatchBlock)
		k.SetLastSlashedLogicCallBlock(ctx, chainPrefix, cd.GravityNonces.LastSlashedLogicCallBlock)
		k.setID(ctx, cd.GravityNonces.LastTxPoolId, types.AppendChainPrefix(types.KeyLastTXPoolID, chainPrefix))
		k.setID(ctx, cd.GravityNonces.LastBatchId, types.AppendChainPrefix(types.KeyLastOutgoingBatchID, chainPrefix))
		k.SetEvmChainData(ctx, cd.EvmChain)

		initBridgeDataFromGenesis(ctx, k, cd)

		// reset pool transactions in state
		for _, tx := range cd.UnbatchedTransfers {
			intTx, err := tx.ToInternal()
			if err != nil {
				panic(sdkerrors.Wrapf(err, "invalid unbatched tx: %v", tx))
			}
			if err := k.addUnbatchedTX(ctx, chainPrefix, intTx); err != nil {
				panic(err)
			}
		}

		// reset attestations in state
		for _, att := range cd.Attestations {
			att := att
			claim, err := k.UnpackAttestationClaim(&att)
			if err != nil {
				panic("couldn't cast to claim")
			}

			// TODO: block height?
			hash, err := claim.ClaimHash()
			if err != nil {
				panic(fmt.Errorf("error when computing ClaimHash for %v", hash))
			}
			k.SetAttestation(ctx, chainPrefix, claim.GetEventNonce(), hash, &att)
		}

		// reset attestation state of specific validators
		// this must be done after the above to be correct
		for _, att := range cd.Attestations {
			att := att
			claim, err := k.UnpackAttestationClaim(&att)
			if err != nil {
				panic("couldn't cast to claim")
			}
			// reconstruct the latest event nonce for every validator
			// if somehow this genesis state is saved when all attestations
			// have been cleaned up GetLastEventNonceByValidator handles that case
			//
			// if we were to save and load the last event nonce for every validator
			// then we would need to carry that state forever across all chain restarts
			// but since we've already had to handle the edge case of new validators joining
			// while all attestations have already been cleaned up we can do this instead and
			// not carry around every validators event nonce counter forever.
			for _, vote := range att.Votes {
				val, err := sdk.ValAddressFromBech32(vote)
				if err != nil {
					panic(err)
				}
				last := k.GetLastEventNonceByValidator(ctx, chainPrefix, val)
				if claim.GetEventNonce() > last {
					k.SetLastEventNonceByValidator(ctx, chainPrefix, val, claim.GetEventNonce())
				}
			}
		}

		// reset delegate keys in state
		if hasDuplicates(cd.DelegateKeys) {
			panic("Duplicate delegate key found in Genesis!")
		}
		for _, keys := range cd.DelegateKeys {
			err := keys.ValidateBasic()
			if err != nil {
				panic("Invalid delegate key in Genesis!")
			}
			val, err := sdk.ValAddressFromBech32(keys.Validator)
			if err != nil {
				panic(err)
			}
			evmAddr, err := types.NewEthAddress(keys.EthAddress)
			if err != nil {
				panic(err)
			}

			orch, err := sdk.AccAddressFromBech32(keys.Orchestrator)
			if err != nil {
				panic(err)
			}

			// set the orchestrator address
			k.SetOrchestratorValidator(ctx, val, orch)
			// set the ethereum address
			k.SetEvmAddressForValidator(ctx, val, *evmAddr)
		}

		// populate state with cosmos originated denom-erc20 mapping
		for i, item := range cd.Erc20ToDenoms {
			ethAddr, err := types.NewEthAddress(item.Erc20)
			if err != nil {
				panic(fmt.Errorf("invalid erc20 address in Erc20ToDenoms for item %d: %s", i, item.Erc20))
			}
			k.setCosmosOriginatedDenomToERC20(ctx, chainPrefix, item.Denom, *ethAddr)
		}

		// now that we have the denom-erc20 mapping we need to validate
		// that the valset reward is possible and cosmos originated remove
		// this if you want a non-cosmos originated reward
		valsetReward := k.GetParams(ctx).ValsetReward
		if valsetReward.IsValid() && !valsetReward.IsZero() {
			_, exists := k.GetCosmosOriginatedERC20(ctx, chainPrefix, valsetReward.Denom)
			if !exists {
				panic("Invalid Cosmos originated denom for valset reward")
			}
		}
	}
}

func hasDuplicates(d []types.MsgSetOrchestratorAddress) bool {
	ethMap := make(map[string]struct{}, len(d))
	orchMap := make(map[string]struct{}, len(d))
	// creates a hashmap then ensures that the hashmap and the array
	// have the same length, this acts as an O(n) duplicates check
	for i := range d {
		ethMap[d[i].EthAddress] = struct{}{}
		orchMap[d[i].Orchestrator] = struct{}{}
	}
	return len(ethMap) != len(d) || len(orchMap) != len(d)
}

// ExportGenesis exports all the state needed to restart the chain
// from the current state of the chain
func ExportGenesis(ctx sdk.Context, k Keeper) types.GenesisState {
	p := k.GetParams(ctx)

	chains := k.GetEvmChains(ctx)
	evmChains := make([]types.EvmChainData, len(chains))

	for ci, cd := range chains {
		calls := k.GetOutgoingLogicCalls(ctx, cd.EvmChainPrefix)
		batches := k.GetOutgoingTxBatches(ctx, cd.EvmChainPrefix)
		valsets := k.GetValsets(ctx, cd.EvmChainPrefix)
		attmap, attKeys := k.GetAttestationMapping(ctx, cd.EvmChainPrefix)
		vsconfs := []types.MsgValsetConfirm{}
		batchconfs := []types.MsgConfirmBatch{}
		callconfs := []types.MsgConfirmLogicCall{}
		attestations := []types.Attestation{}
		delegates := k.GetDelegateKeys(ctx)
		erc20ToDenoms := []types.ERC20ToDenom{}
		unbatchedTransfers := k.GetUnbatchedTransactions(ctx, cd.EvmChainPrefix)

		// export valset confirmations from state
		for _, vs := range valsets {
			// TODO: set height = 0?
			vsconfs = append(vsconfs, k.GetValsetConfirms(ctx, cd.EvmChainPrefix, vs.Nonce)...)
		}

		// export batch confirmations from state
		extBatches := make([]types.OutgoingTxBatch, len(batches))
		for i, batch := range batches {
			// TODO: set height = 0?
			batchconfs = append(batchconfs,
				k.GetBatchConfirmByNonceAndTokenContract(ctx, cd.EvmChainPrefix, batch.BatchNonce, batch.TokenContract)...)
			extBatches[i] = batch.ToExternal()
		}

		// export logic call confirmations from state
		for _, call := range calls {
			// TODO: set height = 0?
			callconfs = append(callconfs,
				k.GetLogicConfirmsByInvalidationIDAndNonce(ctx, cd.EvmChainPrefix, call.InvalidationId, call.InvalidationNonce)...)
		}

		// export attestations from state
		for _, key := range attKeys {
			// TODO: set height = 0?
			attestations = append(attestations, attmap[key]...)
		}

		// export erc20 to denom relations
		k.IterateERC20ToDenom(ctx, func(key []byte, erc20ToDenom *types.ERC20ToDenom) bool {
			erc20ToDenoms = append(erc20ToDenoms, *erc20ToDenom)
			return false
		})

		unbatchedTxs := make([]types.OutgoingTransferTx, len(unbatchedTransfers))
		for i, v := range unbatchedTransfers {
			unbatchedTxs[i] = v.ToExternal()
		}

		evmChains[ci] = types.EvmChainData{
			EvmChain: types.EvmChain{
				EvmChainPrefix: cd.EvmChainPrefix,
				EvmChainName:   cd.EvmChainName,
			},
			GravityNonces: types.GravityNonces{
				LatestValsetNonce:         k.GetLatestValsetNonce(ctx, cd.EvmChainPrefix),
				LastObservedNonce:         k.GetLastObservedEventNonce(ctx, cd.EvmChainPrefix),
				LastSlashedValsetNonce:    k.GetLastSlashedValsetNonce(ctx, cd.EvmChainPrefix),
				LastSlashedBatchBlock:     k.GetLastSlashedBatchBlock(ctx, cd.EvmChainPrefix),
				LastSlashedLogicCallBlock: k.GetLastSlashedLogicCallBlock(ctx, cd.EvmChainPrefix),
				LastTxPoolId:              k.getID(ctx, types.AppendChainPrefix(types.KeyLastTXPoolID, cd.EvmChainPrefix)),
				LastBatchId:               k.getID(ctx, types.AppendChainPrefix(types.KeyLastOutgoingBatchID, cd.EvmChainPrefix)),
			},
			Valsets:            valsets,
			ValsetConfirms:     vsconfs,
			Batches:            extBatches,
			BatchConfirms:      batchconfs,
			LogicCalls:         calls,
			LogicCallConfirms:  callconfs,
			Attestations:       attestations,
			DelegateKeys:       delegates,
			Erc20ToDenoms:      erc20ToDenoms,
			UnbatchedTransfers: unbatchedTxs,
		}
	}

	return types.GenesisState{
		Params:    &p,
		EvmChains: evmChains,
	}
}
