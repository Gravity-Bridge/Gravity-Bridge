package keeper

import (
	"fmt"
	"strings"

	errorsmod "cosmossdk.io/errors"
	sdkmath "cosmossdk.io/math"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	disttypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	govv1beta1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1beta1"
)

// this file contains code related to custom governance proposals

func RegisterProposalTypes() {
	// use of prefix stripping to prevent a typo between the proposal we check
	// and the one we register, any issues with the registration string will prevent
	// the proposal from working. We must check for double registration so that cli commands
	// submitting these proposals work.
	// For some reason the cli code is run during app.go startup, but of course app.go is not
	// run during operation of one off tx commands, so we need to run this 'twice'
	prefix := "gravity/"
	unhalt := "gravity/UnhaltBridge"
	if !govv1beta1.IsValidProposalType(strings.TrimPrefix(unhalt, prefix)) {
		govv1beta1.RegisterProposalType(types.ProposalTypeUnhaltBridge)
	}
	airdrop := "gravity/Airdrop"
	if !govv1beta1.IsValidProposalType(strings.TrimPrefix(airdrop, prefix)) {
		govv1beta1.RegisterProposalType(types.ProposalTypeAirdrop)
	}
	cosmosBridgeableTokens := "gravity/CosmosBridgeableTokens"
	if !govv1beta1.IsValidProposalType(strings.TrimPrefix(cosmosBridgeableTokens, prefix)) {
		govv1beta1.RegisterProposalType(types.ProposalTypeCosmosBridgeableTokens)
	}
}

func NewGravityProposalHandler(k Keeper) govv1beta1.Handler {
	return func(ctx sdk.Context, content govv1beta1.Content) error {
		switch c := content.(type) {
		case *types.UnhaltBridgeProposal:
			return k.HandleUnhaltBridgeProposal(ctx, c)
		case *types.AirdropProposal:
			return k.HandleAirdropProposal(ctx, c)
		case *types.CosmosBridgeableTokensProposal:
			return k.HandleCosmosBridgeableTokensProposal(ctx, c)

		default:
			return errorsmod.Wrapf(sdkerrors.ErrUnknownRequest, "unrecognized Gravity proposal content type: %T", c)
		}
	}
}

// Unhalt Bridge specific functions

// In the event the bridge is halted and governance has decided to reset oracle
// history, we roll back oracle history and reset the parameters
func (k Keeper) HandleUnhaltBridgeProposal(ctx sdk.Context, p *types.UnhaltBridgeProposal) error {
	ctx.Logger().Info("Gov vote passed: Resetting oracle history", "nonce", p.TargetNonce)
	pruneAttestationsAfterNonce(ctx, k, p.TargetNonce)
	return nil
}

// Iterate over all attestations currently being voted on in order of nonce
// and prune those that are older than nonceCutoff
func pruneAttestationsAfterNonce(ctx sdk.Context, k Keeper, nonceCutoff uint64) {
	// Decide on the most recent nonce we can actually roll back to
	lastObserved := k.GetLastObservedEventNonce(ctx)
	if nonceCutoff < lastObserved || nonceCutoff == 0 {
		ctx.Logger().Error("Attempted to reset to a nonce before the last \"observed\" event, which is not allowed", "lastObserved", lastObserved, "nonce", nonceCutoff)
		return
	}

	// Get relevant event nonces
	attmap, keys := k.GetAttestationMapping(ctx)

	// Discover all affected validators whose LastEventNonce must be reset to nonceCutoff

	validators, err := k.StakingKeeper.GetBondedValidatorsByPower(ctx)
	if err != nil {
		return
	}
	// void and setMember are necessary for sets to work
	type void struct{}
	var setMember void
	// Initialize a Set of validators
	affectedValidatorsSet := make(map[string]void, len(validators))

	// Delete all reverted attestations, keeping track of the validators who attested to any of them
	for _, nonce := range keys {
		for _, att := range attmap[nonce] {
			// we delete all attestations earlier than the cutoff event nonce
			if nonce > nonceCutoff {
				ctx.Logger().Info(fmt.Sprintf("Deleting attestation at height %v", att.Height))
				for _, vote := range att.Votes {
					if _, ok := affectedValidatorsSet[vote]; !ok { // if set does not contain vote
						affectedValidatorsSet[vote] = setMember // add key to set
					}
				}

				k.DeleteAttestation(ctx, att)
			}
		}
	}

	// Reset the last event nonce for all validators affected by history deletion
	for vote := range affectedValidatorsSet {
		val, err := sdk.ValAddressFromBech32(vote)
		if err != nil {
			panic(errorsmod.Wrap(err, "invalid validator address affected by bridge reset"))
		}
		valLastNonce := k.GetLastEventNonceByValidator(ctx, val)
		if valLastNonce > nonceCutoff {
			ctx.Logger().Info("Resetting validator's last event nonce due to bridge unhalt", "validator", vote, "lastEventNonce", valLastNonce, "resetNonce", nonceCutoff)
			k.SetLastEventNonceByValidator(ctx, val, nonceCutoff)
		}
	}
}

// Allows governance to deploy an airdrop to a provided list of addresses
func (k Keeper) HandleAirdropProposal(ctx sdk.Context, p *types.AirdropProposal) error {
	ctx.Logger().Info("Gov vote passed: Performing airdrop")

	// Perform additional validation on the denom
	if err := types.ValidateStrictDenom(p.Denom); err != nil {
		ctx.Logger().Info("Airdrop failed to execute invalid denom!")
		return errorsmod.Wrap(err, "invalid airdrop denom")
	}

	startingSupply := k.bankKeeper.GetSupply(ctx, p.Denom)

	validateDenom := sdk.ValidateDenom(p.Denom)
	if validateDenom != nil {
		ctx.Logger().Info("Airdrop failed to execute invalid denom!")
		return errorsmod.Wrap(types.ErrInvalid, "Invalid airdrop denom")
	}

	feePool, err := k.DistKeeper.FeePool.Get(ctx)
	if err != nil {
		ctx.Logger().Info("Airdrop failed to execute error getting fee pool!")
		return errorsmod.Wrap(err, "Error getting fee pool")
	}
	feePoolAmount := feePool.CommunityPool.AmountOf(p.Denom)

	airdropTotal := sdkmath.NewInt(0)
	for _, v := range p.Amounts {
		airdropTotal = airdropTotal.Add(sdkmath.NewIntFromUint64(v))
	}

	totalRequiredDecCoin := sdk.NewDecCoinFromCoin(sdk.NewCoin(p.Denom, airdropTotal))

	// check that we have enough tokens in the community pool to actually execute
	// this airdrop with the provided recipients list
	totalRequiredDec := totalRequiredDecCoin.Amount
	if totalRequiredDec.GT(feePoolAmount) {
		ctx.Logger().Info("Airdrop failed to execute insufficient tokens in the community pool!")
		return errorsmod.Wrap(types.ErrInvalid, "Insufficient tokens in community pool")
	}

	// we're packing addresses as 20 bytes rather than valid bech32 in order to maximize participants
	// so if the recipients list is not a multiple of 20 it must be invalid
	numRecipients := len(p.Recipients) / 20
	if len(p.Recipients)%20 != 0 || numRecipients != len(p.Amounts) {
		ctx.Logger().Info("Airdrop failed to execute invalid recipients")
		return errorsmod.Wrap(types.ErrInvalid, "Invalid recipients")
	}

	parsedRecipients := make([]sdk.AccAddress, len(p.Recipients)/20)
	for i := 0; i < numRecipients; i++ {
		indexStart := i * 20
		indexEnd := indexStart + 20
		addr := p.Recipients[indexStart:indexEnd]
		parsedRecipients[i] = addr
	}

	// check again, just in case the above modulo math is somehow wrong or spoofed
	if len(parsedRecipients) != len(p.Amounts) {
		ctx.Logger().Info("Airdrop failed to execute invalid recipients")
		return errorsmod.Wrap(types.ErrInvalid, "Invalid recipients")
	}

	// the total amount actually sent in dec coins
	totalSent := sdkmath.LegacyNewDec(0)
	for i, addr := range parsedRecipients {
		usersAmount := p.Amounts[i]
		usersIntAmount := sdkmath.NewIntFromUint64(usersAmount)
		usersDecAmount := sdkmath.LegacyNewDecFromInt(usersIntAmount)
		err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, disttypes.ModuleName, addr, sdk.NewCoins(sdk.NewCoin(p.Denom, usersIntAmount)))
		// if there is no error we add to the total actually sent
		if err == nil {
			totalSent = totalSent.Add(usersDecAmount)
		} else {
			// return an err to prevent execution from finishing, this will prevent the changes we
			// have made so far from taking effect the governance proposal will instead time out
			ctx.Logger().Info("invalid address in airdrop! not executing", "address", addr)
			return err
		}
	}

	if !totalRequiredDecCoin.Amount.Equal(totalSent) {
		ctx.Logger().Info("Airdrop failed to execute Invalid amount sent", "sent", totalRequiredDecCoin.Amount, "expected", totalSent)
		return errorsmod.Wrap(types.ErrInvalid, "Invalid amount sent")
	}

	newCoins, InvalidModuleBalance := feePool.CommunityPool.SafeSub(sdk.NewDecCoins(totalRequiredDecCoin))
	// this shouldn't ever happen because we check that we have enough before starting
	// but lets be conservative.
	if InvalidModuleBalance {
		return errorsmod.Wrap(types.ErrInvalid, "internal error!")
	}
	feePool.CommunityPool = newCoins
	err = k.DistKeeper.FeePool.Set(ctx, feePool)
	if err != nil {
		return errorsmod.Wrap(err, "failed to set fee pool")
	}

	endingSupply := k.bankKeeper.GetSupply(ctx, p.Denom)
	if !startingSupply.Equal(endingSupply) {
		return errorsmod.Wrap(types.ErrInvalid, "total chain supply has changed!")
	}

	return nil
}

// Handles updates to the CosmosBridgeableTokens allowlist, either adding/overwriting entries (SET) or removing entries (REMOVE).
//
// On SET, after validation, the bank metadata for each token is overwritten
// On REMOVE only the CosmosBridgeableTokens list is modified, bank is not changed
func (k Keeper) HandleCosmosBridgeableTokensProposal(ctx sdk.Context, p *types.CosmosBridgeableTokensProposal) error {
	ctx.Logger().Info("Gov vote passed: Updating CosmosBridgeableTokens", "operation", p.Operation, "count", len(p.Metadatas))

	// Reject duplicate base denoms within the proposal itself (defense in depth beyond ValidateBasic)
	seen := make(map[string]struct{}, len(p.Metadatas))
	for _, metadata := range p.Metadatas {
		if _, dup := seen[metadata.Base]; dup {
			return errorsmod.Wrapf(types.ErrDuplicate, "CosmosBridgeableTokensProposal contains duplicate base denom: %s", metadata.Base)
		}
		seen[metadata.Base] = struct{}{}
	}

	switch p.Operation {
	case types.CosmosBridgeableTokensOperation_COSMOS_BRIDGEABLE_TOKENS_OPERATION_SET:
		for _, metadata := range p.Metadatas {
			if err := metadata.Validate(); err != nil {
				return errorsmod.Wrapf(err, "invalid metadata in CosmosBridgeableTokensProposal: %s", metadata.Base)
			}
			if err := types.ValidateStrictDenom(metadata.Base); err != nil {
				return errorsmod.Wrapf(err, "invalid denom in CosmosBridgeableTokensProposal: %s", metadata.Base)
			}
			if strings.HasPrefix(metadata.Base, types.GravityDenomPrefix) {
				return errorsmod.Wrapf(types.ErrInvalid,
					"CosmosBridgeableTokens must not contain ethereum-originated (gravity-prefixed) denoms: %s", metadata.Base)
			}

			// Overwrite the bank metadata in case proposer needs the token info to change on chain
			k.bankKeeper.SetDenomMetaData(ctx, metadata)
			k.SetCosmosBridgeableToken(ctx, metadata)
		}
	case types.CosmosBridgeableTokensOperation_COSMOS_BRIDGEABLE_TOKENS_OPERATION_REMOVE:
		for _, metadata := range p.Metadatas {
			existing, found := k.GetCosmosBridgeableToken(ctx, metadata.Base)
			if !found {
				return errorsmod.Wrapf(types.ErrInvalid, "CosmosBridgeableTokens remove denom not found in current list: %s", metadata.Base)
			}
			if !metadataEqual(metadata, existing) {
				return errorsmod.Wrapf(types.ErrInvalid, "CosmosBridgeableTokens remove metadata does not match existing metadata for denom: %s", metadata.Base)
			}
			k.DeleteCosmosBridgeableToken(ctx, metadata.Base)
		}
	default:
		return errorsmod.Wrapf(sdkerrors.ErrInvalidRequest, "CosmosBridgeableTokensProposal requires an explicit operation (SET or REMOVE)")
	}

	return nil
}
