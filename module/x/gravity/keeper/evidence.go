package keeper

import (
	"bytes"
	"encoding/hex"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
	"github.com/cosmos/cosmos-sdk/store/prefix"
)

func (k Keeper) CheckBadSignatureEvidence(
	ctx sdk.Context,
	msg *types.MsgSubmitBadSignatureEvidence) error {
	var subject types.EthereumSigned

	err := k.cdc.UnpackAny(msg.Subject, &subject)

	if err != nil {
		return sdkerrors.Wrap(types.ErrInvalid, fmt.Sprintf("Invalid Any encoded evidence %s", err))
	}

	switch subject := subject.(type) {
	case *types.OutgoingTxBatch, *types.Valset, *types.OutgoingLogicCall:
		return k.checkBadSignatureEvidenceInternal(ctx, msg.EvmChainPrefix, subject, msg.Signature)
	default:
		return sdkerrors.Wrap(types.ErrInvalid, fmt.Sprintf("Bad signature must be over a batch, valset, or logic call got %s", subject))
	}
}

func (k Keeper) checkBadSignatureEvidenceInternal(ctx sdk.Context, evmChainPrefix string, subject types.EthereumSigned, signature string) error {
	// Get checkpoint of the supposed bad signature (fake valset, batch, or logic call submitted to evm)
	gravityID := k.GetGravityID(ctx, evmChainPrefix)
	checkpoint := subject.GetCheckpoint(gravityID)

	// Try to find the checkpoint in the archives. If it exists, we don't slash because
	// this is not a bad signature
	if k.GetPastEthSignatureCheckpoint(ctx, evmChainPrefix, checkpoint) {
		return sdkerrors.Wrap(types.ErrInvalid, "Checkpoint exists, cannot slash")
	}

	// Decode Eth signature to bytes

	// strip 0x prefix if needed
	if signature[:2] == "0x" {
		signature = signature[2:]
	}
	sigBytes, err := hex.DecodeString(signature)
	if err != nil {
		return sdkerrors.Wrap(types.ErrInvalid, fmt.Sprintf("signature decoding %s", signature))
	}

	// Get eth address of the offending validator using the checkpoint and the signature
	evmAddress, err := types.EthAddressFromSignature(checkpoint, sigBytes)
	if err != nil {
		return sdkerrors.Wrap(types.ErrInvalid, fmt.Sprintf("signature to eth address failed with checkpoint %s and signature %s", hex.EncodeToString(checkpoint), signature))
	}

	// Find the offending validator by eth address
	val, found := k.GetValidatorByEvmAddress(ctx, *evmAddress)
	if !found {
		return sdkerrors.Wrap(types.ErrInvalid, fmt.Sprintf("Did not find validator for eth address %s from signature %s with checkpoint %s and GravityID %s", evmAddress.GetAddress().Hex(), signature, hex.EncodeToString(checkpoint), gravityID))
	}

	// Slash the offending validator
	cons, err := val.GetConsAddr()
	if err != nil {
		return sdkerrors.Wrap(err, "Could not get consensus key address for validator")
	}

	params := k.GetParams(ctx)
	if !val.IsJailed() {
		k.StakingKeeper.Jail(ctx, cons)
		k.StakingKeeper.Slash(ctx, cons, ctx.BlockHeight(), val.ConsensusPower(sdk.DefaultPowerReduction), params.SlashFractionBadEthSignature)
	}

	return nil
}

// SetPastEthSignatureCheckpoint puts the checkpoint of a valset, batch, or logic call into a set
// in order to prove later that it existed at one point.
func (k Keeper) SetPastEthSignatureCheckpoint(ctx sdk.Context, evmChainPrefix string, checkpoint []byte) {
	store := ctx.KVStore(k.storeKey)
	store.Set(types.GetPastEvmSignatureCheckpointKey(evmChainPrefix, checkpoint), []byte{0x1})
}

// GetPastEthSignatureCheckpoint tells you whether a given checkpoint has ever existed
func (k Keeper) GetPastEthSignatureCheckpoint(ctx sdk.Context, evmChainPrefix string, checkpoint []byte) (found bool) {
	store := ctx.KVStore(k.storeKey)
	return bytes.Equal(store.Get(types.GetPastEvmSignatureCheckpointKey(evmChainPrefix, checkpoint)), []byte{0x1})
}

func (k Keeper) IteratePastEthSignatureCheckpoints(ctx sdk.Context, evmChainPrefix string, cb func(key []byte, value []byte) (stop bool)) {
	prefixStore := prefix.NewStore(ctx.KVStore(k.storeKey), types.AppendChainPrefix(types.PastEvmSignatureCheckpointKey, evmChainPrefix))
	iter := prefixStore.Iterator(nil, nil)
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		key := iter.Key()
		val := iter.Value()
		if !bytes.Equal(val, []byte{0x1}) {
			panic(fmt.Sprintf("Invalid stored past eth signature checkpoint key=%v: value %v", key, val))
		}

		if cb(key, val) {
			break
		}
	}
}
