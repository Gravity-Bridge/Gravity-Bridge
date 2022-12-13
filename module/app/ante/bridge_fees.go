package ante

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	feegrantkeeper "github.com/cosmos/cosmos-sdk/x/feegrant/keeper"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/keeper"
	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
	sdkante "github.com/cosmos/cosmos-sdk/x/auth/ante"
)

// BasisPointDivisor used in determining if a SendToEth fee meets the governance-controlled minimum
const BasisPointDivisor uint64 = 10000

// BridgeFeeDecorator deducts fees from the first signer of the tx in the same way that cosmos-sdk's DeductFeeDecorator
// will deduct fees, but only for MsgSendToEth. This is NOT meant to replace the DeductFeeDecorator!
// If the first signer does not have the funds to pay for the fees, return with InsufficientFunds error
// Call next AnteHandler if fees successfully deducted
// CONTRACT: Tx must implement FeeTx interface to use BridgeFeeDecorator
type BridgeFeeDecorator struct {
	gravityKeeper  *keeper.Keeper
	accountKeeper  *authkeeper.AccountKeeper
	bankKeeper     *bankkeeper.BaseKeeper
	feegrantKeeper *feegrantkeeper.Keeper // Note that gravity does not use the feegrant module, this should be nil
}

func NewBridgeFeeDecorator(gk *keeper.Keeper, ak *authkeeper.AccountKeeper, bk *bankkeeper.BaseKeeper, fk *feegrantkeeper.Keeper) BridgeFeeDecorator {
	bfd := BridgeFeeDecorator{
		gravityKeeper:  gk,
		accountKeeper:  ak,
		bankKeeper:     bk,
		feegrantKeeper: fk, // Note that gravity does not use the feegrant module, this should be nil
	}
	bfd.ValidateMembers()

	return bfd
}

func (bfd BridgeFeeDecorator) ValidateMembers() {
	if bfd.gravityKeeper == nil {
		panic("Nil gravityKeeper!")
	}
	if bfd.accountKeeper == nil {
		panic("Nil accountKeeper!")
	}
	if bfd.bankKeeper == nil {
		panic("Nil bankKeeper!")
	}
	// Gravity does not use the feegrant module, if it is added then this check should be inverted
	if bfd.feegrantKeeper != nil {
		panic("Non-nil feegrantKeeper!")
	}
}

// AnteHandle will perform validation and deduction of the ChainFee provided on any MsgSendToEth, rejecting Msgs if the
// provided fee does not meet the governance-set minimum and distributing the collected fees to the stakers + validators
//
// Note that this func is heavily inspired by and copies much of the code from the cosmos-sdk DeductFeeDecorator here:
// https://github.com/cosmos/cosmos-sdk/blob/main/x/auth/ante/fee.go
func (bfd BridgeFeeDecorator) AnteHandle(ctx sdk.Context, tx sdk.Tx, simulate bool, next sdk.AnteHandler) (newCtx sdk.Context, err error) {
	feeTx, ok := tx.(sdk.FeeTx)
	if !ok {
		return ctx, sdkerrors.Wrap(sdkerrors.ErrTxDecode, "Tx must be a FeeTx")
	}

	if addr := bfd.accountKeeper.GetModuleAddress(authtypes.FeeCollectorName); addr == nil {
		return ctx, fmt.Errorf("fee collector module account (%s) has not been set", authtypes.FeeCollectorName)
	}

	feePayer := feeTx.FeePayer()
	feeGranter := feeTx.FeeGranter()

	deductFeesFrom := feePayer

	// Tally any SendToEth chain fees to be paid
	var fullChainFees sdk.Coins
	var minFeeBasisPoints int64
	params, err := bfd.gravityKeeper.GetParamsIfSet(ctx)
	if err == nil {
		// The params have been set, get the min send to eth fee
		minFeeBasisPoints = int64(params.MinChainFeeBasisPoints)
	}

	for _, msg := range tx.GetMsgs() {
		switch msg := msg.(type) {
		case *types.MsgSendToEth:
			// The minimum fee must be >= amount * some number of basis points (set by governance)
			// If any provided message is invalid, the whole tx will be thrown out without any fees being collected
			minFee := sdk.NewDecFromInt(msg.Amount.Amount).QuoInt64(int64(BasisPointDivisor)).MulInt64(minFeeBasisPoints).TruncateInt()
			if minFee.GT(sdk.ZeroInt()) { // Ignore fees too low to collect
				minFeeCoin := sdk.NewCoin(msg.Amount.GetDenom(), minFee)
				if msg.ChainFee.IsLT(minFeeCoin) {
					err := sdkerrors.Wrapf(
						sdkerrors.ErrInsufficientFee,
						"chain fee provided [%s] is insufficient, need at least [%s]",
						msg.ChainFee,
						minFeeCoin,
					)
					return ctx, err
				}
				fullChainFees = fullChainFees.Add(msg.ChainFee)
			}
		default:
			continue
		}
	}

	// Acceptable fees have been provided, now they must be deducted
	if !fullChainFees.IsZero() {
		// feeGranter requires the feegrantKeeper to be set
		if feeGranter != nil {
			if bfd.feegrantKeeper == nil {
				return ctx, sdkerrors.Wrap(sdkerrors.ErrInvalidRequest, "fee grants are not enabled")
			} else if !feeGranter.Equals(feePayer) {
				err := bfd.feegrantKeeper.UseGrantedFees(ctx, feeGranter, feePayer, fullChainFees, tx.GetMsgs())
				if err != nil {
					return ctx, sdkerrors.Wrapf(err, "%s not allowed to pay fees from %s", feeGranter, feePayer)
				}
			}

			deductFeesFrom = feeGranter
		}

		// Determinte the account which will be paying fees
		deductFeesFromAcc := bfd.accountKeeper.GetAccount(ctx, deductFeesFrom)
		if deductFeesFromAcc == nil {
			return ctx, sdkerrors.Wrapf(sdkerrors.ErrUnknownAddress, "fee payer address: %s does not exist", deductFeesFrom)
		}
		// deduct the fees
		err = sdkante.DeductFees(*bfd.bankKeeper, ctx, deductFeesFromAcc, fullChainFees)
		if err != nil {
			ctx.Logger().Error("Could not deduct MsgSendToEth fees!", "error", err, "account", deductFeesFromAcc, "fullChainFees", fullChainFees)
			return ctx, err
		}

		events := sdk.Events{
			sdk.NewEvent(
				sdk.EventTypeTx,
				sdk.NewAttribute("send_to_eth_fee", fullChainFees.String()),
				sdk.NewAttribute(sdk.AttributeKeyFeePayer, deductFeesFrom.String()),
			),
		}
		ctx.EventManager().EmitEvents(events)
	}
	return next(ctx, tx, simulate)
}
