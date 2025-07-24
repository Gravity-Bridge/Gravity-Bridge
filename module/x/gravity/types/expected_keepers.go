package types

import (
	context "context"
	"time"

	sdkmath "cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	bank "github.com/cosmos/cosmos-sdk/x/bank/types"
	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// StakingKeeper defines the expected staking keeper methods
type StakingKeeper interface {
	GetBondedValidatorsByPower(ctx context.Context) ([]stakingtypes.Validator, error)
	GetLastValidatorPower(ctx context.Context, operator sdk.ValAddress) (int64, error)
	GetLastTotalPower(ctx context.Context) (power sdkmath.Int, err error)
	IterateValidators(context.Context, func(index int64, validator stakingtypes.ValidatorI) (stop bool)) error
	ValidatorQueueIterator(ctx context.Context, endTime time.Time, endHeight int64) (storetypes.Iterator, error)
	GetParams(ctx context.Context) (stakingtypes.Params, error)
	GetValidator(ctx context.Context, addr sdk.ValAddress) (validator stakingtypes.Validator, err error)
	IterateBondedValidatorsByPower(context.Context, func(index int64, validator stakingtypes.ValidatorI) (stop bool)) error
	IterateLastValidators(context.Context, func(index int64, validator stakingtypes.ValidatorI) (stop bool)) error
	Validator(context.Context, sdk.ValAddress) (stakingtypes.ValidatorI, error)
	ValidatorByConsAddr(context.Context, sdk.ConsAddress) (stakingtypes.ValidatorI, error)
	Slash(context.Context, sdk.ConsAddress, int64, int64, sdkmath.LegacyDec) (sdkmath.Int, error)
	Jail(context.Context, sdk.ConsAddress) error
}

// BankKeeper defines the expected bank keeper methods
type BankKeeper interface {
	SendCoinsFromModuleToAccount(ctx context.Context, senderModule string, recipientAddr sdk.AccAddress, amt sdk.Coins) error
	SendCoinsFromAccountToModule(ctx context.Context, senderAddr sdk.AccAddress, recipientModule string, amt sdk.Coins) error
	SendCoinsFromModuleToModule(ctx context.Context, senderModule string, recipientModule string, amt sdk.Coins) error
	MintCoins(ctx context.Context, name string, amt sdk.Coins) error
	BurnCoins(ctx context.Context, name string, amt sdk.Coins) error
	GetAllBalances(ctx context.Context, addr sdk.AccAddress) sdk.Coins
	GetDenomMetaData(ctx context.Context, denom string) bank.Metadata
}

type SlashingKeeper interface {
	GetValidatorSigningInfo(ctx context.Context, address sdk.ConsAddress) (info slashingtypes.ValidatorSigningInfo, err error)
}

type DistributionKeeper interface {
	FundCommunityPool(ctx context.Context, amount sdk.Coins, sender sdk.AccAddress) error
}
