package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// Wrapper struct
type Hooks struct {
	k Keeper
}

// Create new gravity hooks
func (k Keeper) Hooks() Hooks {
	// if startup is mis-ordered in app.go this hook will halt
	// the chain when called. Keep this check to make such a mistake
	// obvious
	if k.storeKey == nil {
		panic("Hooks initialized before GravityKeeper!")
	}
	return Hooks{k}
}

func (h Hooks) AfterValidatorBeginUnbonding(ctx sdk.Context, _ sdk.ConsAddress, _ sdk.ValAddress) {
	// When Validator starts Unbonding, Persist the block height in the store
	// Later in endblocker, check if there is at least one validator who started unbonding and create a valset request.
	// The reason for creating valset requests in endblock is to create only one valset request per block,
	// if multiple validators starts unbonding at same block.

	// this hook IS called for jailing or unbonding triggered by users but it IS NOT called for jailing triggered
	// in the endblocker therefore we call the keeper function ourselves there.

	h.k.SetLastUnBondingBlockHeight(ctx, uint64(ctx.BlockHeight()))
}

func (h Hooks) BeforeDelegationCreated(_ sdk.Context, _ sdk.AccAddress, _ sdk.ValAddress) {
}
func (h Hooks) AfterValidatorCreated(_ sdk.Context, _ sdk.ValAddress)                   {}
func (h Hooks) BeforeValidatorModified(_ sdk.Context, _ sdk.ValAddress)                 {}
func (h Hooks) AfterValidatorBonded(_ sdk.Context, _ sdk.ConsAddress, _ sdk.ValAddress) {}

func (h Hooks) BeforeDelegationRemoved(_ sdk.Context, _ sdk.AccAddress, _ sdk.ValAddress)      {}
func (h Hooks) AfterValidatorRemoved(_ sdk.Context, _ sdk.ConsAddress, valAddr sdk.ValAddress) {}
func (h Hooks) BeforeValidatorSlashed(_ sdk.Context, _ sdk.ValAddress, _ sdk.Dec)              {}
func (h Hooks) BeforeDelegationSharesModified(_ sdk.Context, _ sdk.AccAddress, _ sdk.ValAddress) {
}

func (h Hooks) AfterDelegationModified(_ sdk.Context, _ sdk.AccAddress, _ sdk.ValAddress) {
}
