package keeper

import (
	"fmt"

	"github.com/althea-net/cosmos-gravity-bridge/module/x/gravity/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	disttypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
)

// this file contains code related to custom governance proposals

func RegisterProposalTypes() {
	govtypes.RegisterProposalType(types.ProposalTypeUnhaltBridge)
	govtypes.RegisterProposalTypeCodec(&types.UnhaltBridgeProposal{}, "gravity/UnhaltBridge")
	govtypes.RegisterProposalType(types.ProposalTypeAirdrop)
	govtypes.RegisterProposalTypeCodec(&types.AirdropProposal{}, "gravity/Airdrop")
}

func NewGravityProposalHandler(k Keeper) govtypes.Handler {
	return func(ctx sdk.Context, content govtypes.Content) error {
		switch c := content.(type) {
		case *types.UnhaltBridgeProposal:
			return k.HandleUnhaltBridgeProposal(ctx, c)
		case *types.AirdropProposal:
			return k.HandleAirdropProposal(ctx, c)

		default:
			return sdkerrors.Wrapf(sdkerrors.ErrUnknownRequest, "unrecognized Gravity proposal content type: %T", c)
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

	numValidators := len(k.StakingKeeper.GetBondedValidatorsByPower(ctx))
	// void and setMember are necessary for sets to work
	type void struct{}
	var setMember void
	// Initialize a Set of validators
	affectedValidatorsSet := make(map[string]void, numValidators)

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
			panic(sdkerrors.Wrap(err, "invalid validator address affected by bridge reset"))
		}
		valLastNonce := k.GetLastEventNonceByValidator(ctx, val)
		if valLastNonce > nonceCutoff {
			ctx.Logger().Info("Resetting validator's last event nonce due to bridge unhalt", "validator", vote, "lastEventNonce", valLastNonce, "resetNonce", nonceCutoff)
			k.SetLastEventNonceByValidator(ctx, val, nonceCutoff)
		}
	}
}

// In the event the bridge is halted and governance has decided to reset oracle
// history, we roll back oracle history and reset the parameters
func (k Keeper) HandleAirdropProposal(ctx sdk.Context, p *types.AirdropProposal) error {
	ctx.Logger().Info("Gov vote passed: Performing airdrop", "amount", p.Amount)

	validateDenom := sdk.ValidateDenom(p.Amount.Denom)
	if validateDenom != nil {
		ctx.Logger().Info("Airdrop failed to execute invalid denom!")
		return sdkerrors.Wrap(types.ErrInvalid, "Invalid airdrop denom")
	}

	feePool := k.DistKeeper.GetFeePool(ctx)
	decAmount := sdk.NewDecCoinFromCoin(p.Amount)
	feePoolAmount := feePool.CommunityPool.AmountOf(decAmount.Denom)

	// check that we have enough tokens in the community pool to actually execute
	// this airdrop with the provided recipients list
	totalRequired := decAmount.Amount.MulInt64(int64(len(p.Recipients)))
	if totalRequired.GT(feePoolAmount) {
		ctx.Logger().Info("Airdrop failed to excute insufficient tokens in the community pool!")
		return sdkerrors.Wrap(types.ErrInvalid, "Insufficient tokens in community pool")
	}

	parsedRecipients := make([]sdk.AccAddress, len(p.Recipients))
	for i, addr := range p.Recipients {
		parsedAddr, err := sdk.AccAddressFromBech32(addr)
		if err != nil {
			ctx.Logger().Info("invalid address in airdrop! not executing", "address", addr)
			return err
		}
		parsedRecipients[i] = parsedAddr
	}

	// the total amount actually sent in dec coins
	totalSent := sdk.NewDec(0)
	for _, addr := range parsedRecipients {
		err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, disttypes.ModuleName, addr, sdk.NewCoins(p.Amount))
		// if there is no error we add to the total actually sent
		if err == nil {
			totalSent = totalSent.Add(decAmount.Amount)
		} else {
			// return an err to prevent execution from finishing, this will prevent the changes we
			// have made so far from taking effect the governance proposal will instead time out
			ctx.Logger().Info("invalid address in airdrop! not executing", "address", addr)
			return err
		}
	}

	newCoins, InvalidModuleBalance := feePool.CommunityPool.SafeSub(sdk.NewDecCoins(sdk.NewDecCoinFromDec(p.Amount.Denom, totalSent)))
	// this shouldn't ever happen because we check that we have enough before starting
	// but lets be conservative.
	if InvalidModuleBalance {
		panic("Negative community pool coins after airdrop, chain in invalid state")
	}
	feePool.CommunityPool = newCoins
	k.DistKeeper.SetFeePool(ctx, feePool)

	return nil
}
