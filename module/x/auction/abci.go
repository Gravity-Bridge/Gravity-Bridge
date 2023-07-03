package auction

import (
	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/auction/keeper"
	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/auction/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
)

// TODO: ADD BeginBlocker function to check for if the auction periods has started or not
// TODO: ADD EndBlocker function to check for if the auction periods has ended or not,
func BeginBlocker(ctx sdk.Context, k keeper.Keeper, bk types.BankKeeper, ak types.AccountKeeper) {
	params := 
	//Send fund from community pool to auction module 
	err := bk.SendCoinsFromModuleToModule(ctx,  , types.ModuleName, sdk.New)
}
