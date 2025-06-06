package gravity

import (
	"fmt"

	errorsmod "cosmossdk.io/errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/keeper"
	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
	typesv2 "github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types/v2"
)

// NewHandler returns a handler for "Gravity" type messages.
func NewHandler(k keeper.Keeper) sdk.Handler {
	msgServer := keeper.NewMsgServerImpl(k)
	msgServerV2 := msgServer.(typesv2.MsgServer)

	return func(ctx sdk.Context, msg sdk.Msg) (*sdk.Result, error) {
		ctx = ctx.WithEventManager(sdk.NewEventManager())
		switch msg := msg.(type) {
		case *types.MsgSetOrchestratorAddress:
			res, err := msgServer.SetOrchestratorAddress(sdk.WrapSDKContext(ctx), msg)
			return sdk.WrapServiceResult(ctx, res, err)
		case *types.MsgValsetConfirm:
			res, err := msgServer.ValsetConfirm(sdk.WrapSDKContext(ctx), msg)
			return sdk.WrapServiceResult(ctx, res, err)
		case *types.MsgSendToEth:
			res, err := msgServer.SendToEth(sdk.WrapSDKContext(ctx), msg)
			return sdk.WrapServiceResult(ctx, res, err)
		case *types.MsgRequestBatch:
			res, err := msgServer.RequestBatch(sdk.WrapSDKContext(ctx), msg)
			return sdk.WrapServiceResult(ctx, res, err)
		case *types.MsgConfirmBatch:
			res, err := msgServer.ConfirmBatch(sdk.WrapSDKContext(ctx), msg)
			return sdk.WrapServiceResult(ctx, res, err)
		case *types.MsgConfirmLogicCall:
			res, err := msgServer.ConfirmLogicCall(sdk.WrapSDKContext(ctx), msg)
			return sdk.WrapServiceResult(ctx, res, err)
		case *types.MsgSendToCosmosClaim:
			res, err := msgServer.SendToCosmosClaim(sdk.WrapSDKContext(ctx), msg)
			return sdk.WrapServiceResult(ctx, res, err)
		case *types.MsgExecuteIbcAutoForwards:
			res, err := msgServer.ExecuteIbcAutoForwards(sdk.WrapSDKContext(ctx), msg)
			return sdk.WrapServiceResult(ctx, res, err)
		case *types.MsgBatchSendToEthClaim:
			res, err := msgServer.BatchSendToEthClaim(sdk.WrapSDKContext(ctx), msg)
			return sdk.WrapServiceResult(ctx, res, err)
		case *types.MsgERC20DeployedClaim:
			res, err := msgServer.ERC20DeployedClaim(sdk.WrapSDKContext(ctx), msg)
			return sdk.WrapServiceResult(ctx, res, err)
		case *types.MsgLogicCallExecutedClaim:
			res, err := msgServer.LogicCallExecutedClaim(sdk.WrapSDKContext(ctx), msg)
			return sdk.WrapServiceResult(ctx, res, err)
		case *types.MsgCancelSendToEth:
			res, err := msgServer.CancelSendToEth(sdk.WrapSDKContext(ctx), msg)
			return sdk.WrapServiceResult(ctx, res, err)
		case *types.MsgValsetUpdatedClaim:
			res, err := msgServer.ValsetUpdateClaim(sdk.WrapSDKContext(ctx), msg)
			return sdk.WrapServiceResult(ctx, res, err)
		case *types.MsgSubmitBadSignatureEvidence:
			res, err := msgServer.SubmitBadSignatureEvidence(sdk.WrapSDKContext(ctx), msg)
			return sdk.WrapServiceResult(ctx, res, err)
		case *typesv2.MsgAirdropProposal:
			res, err := msgServerV2.AirdropProposal(sdk.WrapSDKContext(ctx), msg)
			return sdk.WrapServiceResult(ctx, res, err)
		case *typesv2.MsgIBCMetadataProposal:
			res, err := msgServerV2.IBCMetadataProposal(sdk.WrapSDKContext(ctx), msg)
			return sdk.WrapServiceResult(ctx, res, err)
		case *typesv2.MsgUnhaltBridgeProposal:
			res, err := msgServerV2.UnhaltBridgeProposal(sdk.WrapSDKContext(ctx), msg)
			return sdk.WrapServiceResult(ctx, res, err)

		default:
			return nil, errorsmod.Wrap(sdkerrors.ErrUnknownRequest, fmt.Sprintf("Unrecognized Gravity Msg type: %v", sdk.MsgTypeURL(msg)))
		}
	}
}
