package keeper

import (
	"fmt"

	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/store/prefix"
	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/bech32ibc/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/gogoproto/proto"
)

// GetFeeToken returns the fee token record for a specific denom
func (k Keeper) GetNativeHrp(ctx sdk.Context) (hrp string, err error) {
	store := ctx.KVStore(k.storeKey)

	if !store.Has(types.NativeHrpKey) {
		return "", types.ErrNoNativeHrp
	}

	bz := store.Get(types.NativeHrpKey)

	return string(bz), nil
}

// SetNativeHrp sets the native prefix for the chain. Should only be used once.
func (k Keeper) SetNativeHrp(ctx sdk.Context, hrp string) error {
	store := ctx.KVStore(k.storeKey)

	err := types.ValidateHrp(hrp)
	if err != nil {
		return err
	}

	store.Set(types.NativeHrpKey, []byte(hrp))
	return nil
}

// ValidateHrpIbcRecord validates that an HRP IBC record is valid at runtime.
// It checks:
// - The HRP is valid
// - The HRP is not for the chain's native prefix
// - The IBC channel and port are real (live chain state query)
func (k Keeper) ValidateHrpIbcRecord(ctx sdk.Context, record types.HrpIbcRecord) error {
	err := types.ValidateHrp(record.Hrp)
	if err != nil {
		return err
	}

	nativeHrp, err := k.GetNativeHrp(ctx)
	if err != nil {
		return err
	}

	if record.Hrp == nativeHrp {
		return errorsmod.Wrap(types.ErrInvalidHRP, "cannot set a record for the chain's native prefix")
	}

	_, found := k.channelKeeper.GetChannel(ctx, k.tk.GetPort(ctx), record.SourceChannel)
	if !found {
		return errorsmod.Wrap(types.ErrInvalidIBCData, fmt.Sprintf("channel not found: %s", record.SourceChannel))
	}

	return nil
}

// GetHrpIbcRecord returns the hrp ibc record for a specific hrp
func (k Keeper) GetHrpSourceChannel(ctx sdk.Context, hrp string) (string, error) {
	record, err := k.GetHrpIbcRecord(ctx, hrp)
	if err != nil {
		return "", err
	}

	return record.SourceChannel, nil
}

// GetHrpIbcRecord returns the hrp ibc record for a specific hrp
func (k Keeper) GetHrpIbcRecord(ctx sdk.Context, hrp string) (types.HrpIbcRecord, error) {
	store := ctx.KVStore(k.storeKey)
	prefixStore := prefix.NewStore(store, types.HrpIBCRecordStorePrefix)
	if !prefixStore.Has([]byte(hrp)) {
		// nolint: exhaustruct
		return types.HrpIbcRecord{}, errorsmod.Wrap(types.ErrRecordNotFound, fmt.Sprintf("hrp record not found for %s", hrp))
	}
	bz := prefixStore.Get([]byte(hrp))

	// nolint: exhaustruct
	record := types.HrpIbcRecord{}
	err := proto.Unmarshal(bz, &record)
	if err != nil {
		// nolint: exhaustruct
		return types.HrpIbcRecord{}, err
	}

	return record, nil
}

// setHrpIbcRecord sets a new hrp ibc record for a specific denom.
// WARNING: If SourceChannel is empty, the existing record for the HRP is deleted.
func (k Keeper) setHrpIbcRecord(ctx sdk.Context, hrpIbcRecord types.HrpIbcRecord) error {
	store := ctx.KVStore(k.storeKey)
	prefixStore := prefix.NewStore(store, types.HrpIBCRecordStorePrefix)

	if hrpIbcRecord.SourceChannel == "" {
		if prefixStore.Has([]byte(hrpIbcRecord.Hrp)) {
			prefixStore.Delete([]byte(hrpIbcRecord.Hrp))
			k.Logger(ctx).Info("HRP IBC record deleted", "hrp", hrpIbcRecord.Hrp)
			ctx.EventManager().EmitEvent(sdk.NewEvent(
				types.EventTypeHrpIbcRecordDelete,
				sdk.NewAttribute(types.AttributeKeyHrp, hrpIbcRecord.Hrp),
			))
		}
		return nil
	}

	bz, err := proto.Marshal(&hrpIbcRecord)
	if err != nil {
		return err
	}

	prefixStore.Set([]byte(hrpIbcRecord.Hrp), bz)
	k.Logger(ctx).Info("HRP IBC record set", "hrp", hrpIbcRecord.Hrp, "channel", hrpIbcRecord.SourceChannel)
	ctx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventTypeHrpIbcRecordSet,
		sdk.NewAttribute(types.AttributeKeyHrp, hrpIbcRecord.Hrp),
		sdk.NewAttribute(types.AttributeKeyChannel, hrpIbcRecord.SourceChannel),
	))
	return nil
}

func (k Keeper) GetHrpIbcRecords(ctx sdk.Context) (hrpIbcRecords []types.HrpIbcRecord) {
	store := ctx.KVStore(k.storeKey)
	prefixStore := prefix.NewStore(store, types.HrpIBCRecordStorePrefix)

	iterator := prefixStore.Iterator(nil, nil)
	defer iterator.Close()

	records := []types.HrpIbcRecord{}

	for ; iterator.Valid(); iterator.Next() {

		// nolint: exhaustruct
		record := types.HrpIbcRecord{}

		err := proto.Unmarshal(iterator.Value(), &record)
		if err != nil {
			panic(err)
		}

		records = append(records, record)
	}
	return records
}

func (k Keeper) SetHrpIbcRecords(ctx sdk.Context, hrpIbcRecords []types.HrpIbcRecord) {
	nativePrefix, err := k.GetNativeHrp(ctx)
	if err != nil {
		panic(fmt.Sprintf("native HRP not set: %v", err))
	}

	//nolint: exhaustruct
	gs := types.GenesisState{NativeHRP: nativePrefix, HrpIBCRecords: hrpIbcRecords}
	if err := gs.Validate(); err != nil {
		panic(fmt.Sprintf("invalid hrp ibc records: %v", err))
	}
	for _, record := range hrpIbcRecords {
		err := k.setHrpIbcRecord(ctx, record)
		if err != nil {
			panic(fmt.Sprintf("failed to set hrp ibc record: %v", err))
		}
	}
}
