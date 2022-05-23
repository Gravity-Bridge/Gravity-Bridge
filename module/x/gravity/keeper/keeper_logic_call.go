package keeper

import (
	"encoding/hex"
	"fmt"

	"github.com/cosmos/cosmos-sdk/store/prefix"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
)

/////////////////////////////
//       LOGICCALLS        //
/////////////////////////////

// GetOutgoingLogicCall gets an outgoing logic call
func (k Keeper) GetOutgoingLogicCall(ctx sdk.Context, evmChainPrefix string, invalidationID []byte, invalidationNonce uint64) *types.OutgoingLogicCall {
	store := ctx.KVStore(k.storeKey)
	call := types.OutgoingLogicCall{
		Transfers:            []types.ERC20Token{},
		Fees:                 []types.ERC20Token{},
		LogicContractAddress: "",
		Payload:              []byte{},
		Timeout:              0,
		InvalidationId:       invalidationID,
		InvalidationNonce:    invalidationNonce,
		Block:                0,
	}
	k.cdc.MustUnmarshal(store.Get(types.GetOutgoingLogicCallKey(evmChainPrefix, invalidationID, invalidationNonce)), &call)
	return &call
}

// SetOutogingLogicCall sets an outgoing logic call, panics if one already exists at this
// index, since we collect signatures over logic calls no mutation can be valid
func (k Keeper) SetOutgoingLogicCall(ctx sdk.Context, evmChainPrefix string, call types.OutgoingLogicCall) {
	store := ctx.KVStore(k.storeKey)

	// Store checkpoint to prove that this logic call actually happened
	checkpoint := call.GetCheckpoint(k.GetGravityID(ctx))
	k.SetPastEthSignatureCheckpoint(ctx, evmChainPrefix, checkpoint)
	key := types.GetOutgoingLogicCallKey(evmChainPrefix, call.InvalidationId, call.InvalidationNonce)
	if store.Has(key) {
		panic("Can not overwrite logic call")
	}
	store.Set(key,
		k.cdc.MustMarshal(&call))
}

// DeleteOutgoingLogicCall deletes outgoing logic calls
func (k Keeper) DeleteOutgoingLogicCall(ctx sdk.Context, evmChainPrefix string, invalidationID []byte, invalidationNonce uint64) {
	ctx.KVStore(k.storeKey).Delete(types.GetOutgoingLogicCallKey(evmChainPrefix, invalidationID, invalidationNonce))
}

// IterateOutgoingLogicCalls iterates over outgoing logic calls
func (k Keeper) IterateOutgoingLogicCalls(ctx sdk.Context, evmChainPrefix string, cb func([]byte, types.OutgoingLogicCall) bool) {
	prefixStore := prefix.NewStore(ctx.KVStore(k.storeKey), types.AppendChainPrefix(types.KeyOutgoingLogicCall, evmChainPrefix))
	iter := prefixStore.Iterator(nil, nil)
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		var call types.OutgoingLogicCall
		k.cdc.MustUnmarshal(iter.Value(), &call)
		// cb returns true to stop early
		if cb(iter.Key(), call) {
			break
		}
	}
}

// GetOutgoingLogicCalls returns the outgoing logic calls
func (k Keeper) GetOutgoingLogicCalls(ctx sdk.Context, evmChainPrefix string) (out []types.OutgoingLogicCall) {
	k.IterateOutgoingLogicCalls(ctx, evmChainPrefix, func(_ []byte, call types.OutgoingLogicCall) bool {
		out = append(out, call)
		return false
	})
	return
}

// CancelOutgoingLogicCalls releases all TX in the batch and deletes the batch
func (k Keeper) CancelOutgoingLogicCall(ctx sdk.Context, evmChainPrefix string, invalidationId []byte, invalidationNonce uint64) error {
	call := k.GetOutgoingLogicCall(ctx, evmChainPrefix, invalidationId, invalidationNonce)
	if call == nil {
		return types.ErrUnknown
	}
	// Delete batch since it is finished
	k.DeleteOutgoingLogicCall(ctx, evmChainPrefix, call.InvalidationId, call.InvalidationNonce)

	// a consuming application will have to watch for this event and act on it
	ctx.EventManager().EmitTypedEvent(
		&types.EventOutgoingLogicCallCanceled{
			LogicCallInvalidationId:    fmt.Sprint(call.InvalidationId),
			LogicCallInvalidationNonce: fmt.Sprint(call.InvalidationNonce),
		},
	)

	return nil
}

/////////////////////////////
//       LOGICCONFIRMS     //
/////////////////////////////

// SetLogicCallConfirm sets a logic confirm in the store
func (k Keeper) SetLogicCallConfirm(ctx sdk.Context, evmChainPrefix string, msg *types.MsgConfirmLogicCall) {
	bytes, err := hex.DecodeString(msg.InvalidationId)
	if err != nil {
		panic(err)
	}

	acc, err := sdk.AccAddressFromBech32(msg.Orchestrator)
	if err != nil {
		panic(err)
	}

	ctx.KVStore(k.storeKey).
		Set(types.GetLogicConfirmKey(evmChainPrefix, bytes, msg.InvalidationNonce, acc), k.cdc.MustMarshal(msg))
}

// GetLogicCallConfirm gets a logic confirm from the store
func (k Keeper) GetLogicCallConfirm(ctx sdk.Context, evmChainPrefix string, invalidationId []byte, invalidationNonce uint64, val sdk.AccAddress) *types.MsgConfirmLogicCall {
	if err := sdk.VerifyAddressFormat(val); err != nil {
		ctx.Logger().Error("invalid val address")
		return nil
	}
	store := ctx.KVStore(k.storeKey)
	data := store.Get(types.GetLogicConfirmKey(evmChainPrefix, invalidationId, invalidationNonce, val))
	if data == nil {
		return nil
	}
	out := types.MsgConfirmLogicCall{
		InvalidationId:    "",
		InvalidationNonce: invalidationNonce,
		EthSigner:         "",
		Orchestrator:      "",
		Signature:         "",
	}
	k.cdc.MustUnmarshal(data, &out)
	return &out
}

// DeleteLogicCallConfirm deletes a logic confirm from the store
func (k Keeper) DeleteLogicCallConfirm(
	ctx sdk.Context,
	evmChainPrefix string,
	invalidationID []byte,
	invalidationNonce uint64,
	val sdk.AccAddress) {
	ctx.KVStore(k.storeKey).Delete(types.GetLogicConfirmKey(evmChainPrefix, invalidationID, invalidationNonce, val))
}

// IterateLogicConfirmByInvalidationIDAndNonce iterates over all logic confirms stored by nonce
func (k Keeper) IterateLogicConfirmByInvalidationIDAndNonce(
	ctx sdk.Context,
	evmChainPrefix string,
	invalidationID []byte,
	invalidationNonce uint64,
	cb func([]byte, *types.MsgConfirmLogicCall) bool) {
	store := ctx.KVStore(k.storeKey)
	prefix := types.GetLogicConfirmNonceInvalidationIdPrefix(evmChainPrefix, invalidationID, invalidationNonce)
	iter := store.Iterator(prefixRange([]byte(prefix)))

	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		confirm := types.MsgConfirmLogicCall{
			InvalidationId:    "",
			InvalidationNonce: invalidationNonce,
			EthSigner:         "",
			Orchestrator:      "",
			Signature:         "",
		}
		k.cdc.MustUnmarshal(iter.Value(), &confirm)
		// cb returns true to stop early
		if cb(iter.Key(), &confirm) {
			break
		}
	}
}

// GetLogicConfirmsByInvalidationIdAndNonce returns the logic call confirms
func (k Keeper) GetLogicConfirmByInvalidationIDAndNonce(ctx sdk.Context, evmChainPrefix string, invalidationId []byte, invalidationNonce uint64) (out []types.MsgConfirmLogicCall) {
	k.IterateLogicConfirmByInvalidationIDAndNonce(ctx, evmChainPrefix, invalidationId, invalidationNonce, func(_ []byte, msg *types.MsgConfirmLogicCall) bool {
		out = append(out, *msg)
		return false
	})
	return
}
