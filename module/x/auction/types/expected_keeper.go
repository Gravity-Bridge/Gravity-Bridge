package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// AccountKeeper defines the expected account keeper used for simulations (noalias)
// Methods imported from account should be defined here
type AccountKeeper interface {
	NewAccount(sdk.Context, sdk.AccountI) sdk.AccountI
	SetAccount(ctx sdk.Context, acc sdk.AccountI)
	GetAccount(ctx sdk.Context, addr sdk.AccAddress) sdk.AccountI
	GetModuleAccount(ctx sdk.Context, moduleName string) sdk.ModuleAccountI
}

// BankKeeper defines the expected interface needed to retrieve account balances and send token.
// BankKeeper interface: https://github.com/cosmos/cosmos-sdk/blob/main/x/bank/keeper/keeper.go
// Methods imported from bank should be defined here
type BankKeeper interface {
	GetBalance(ctx sdk.Context, addr sdk.AccAddress, denom string) sdk.Coin
	SendCoinsFromModuleToAccount(ctx sdk.Context, senderModule string, recipientAddr sdk.AccAddress, amt sdk.Coins) error
	SendCoinsFromAccountToModule(ctx sdk.Context, senderAddr sdk.AccAddress, recipientModule string, amt sdk.Coins) error
	GetAllBalances(ctx sdk.Context, addr sdk.AccAddress) sdk.Coins
	SendCoinsFromModuleToModule(ctx sdk.Context, senderModule string, recipientModule string, amt sdk.Coins) error
}
