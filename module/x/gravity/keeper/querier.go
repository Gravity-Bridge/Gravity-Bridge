package keeper

import (
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	abci "github.com/tendermint/tendermint/abci/types"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
)

const (
	QueryCurrentValset = "currentValset"
	QueryGravityID     = "gravityID"
)

// NewQuerier is the module level router for state queries
func NewQuerier(keeper Keeper) sdk.Querier {
	return func(ctx sdk.Context, path []string, req abci.RequestQuery) (res []byte, err error) {
		switch path[0] {
		// Valsets
		case QueryCurrentValset:
			return queryCurrentValset(ctx, keeper, path[1])
		case QueryGravityID:
			return queryGravityID(ctx, keeper, path[1])
		default:
			return nil, sdkerrors.Wrapf(sdkerrors.ErrUnknownRequest, "unknown %s query endpoint", types.ModuleName)
		}
	}
}

func queryCurrentValset(ctx sdk.Context, keeper Keeper, emvChainPrefix string) ([]byte, error) {
	valset, err := keeper.GetCurrentValset(ctx, emvChainPrefix)
	if err != nil {
		return nil, err
	}
	res, err := codec.MarshalJSONIndent(types.ModuleCdc, valset)
	if err != nil {
		return nil, sdkerrors.Wrap(sdkerrors.ErrJSONMarshal, err.Error())
	}

	return res, nil
}

func queryGravityID(ctx sdk.Context, keeper Keeper, evmChainPrefix string) ([]byte, error) {
	gravityID := keeper.GetGravityID(ctx, evmChainPrefix)
	res, err := codec.MarshalJSONIndent(types.ModuleCdc, gravityID)
	if err != nil {
		return nil, sdkerrors.Wrap(sdkerrors.ErrJSONMarshal, err.Error())
	} else {
		return res, nil
	}
}

type MultiSigUpdateResponse struct {
	Valset     types.Valset `json:"valset"`
	Signatures [][]byte     `json:"signatures,omitempty"`
}
