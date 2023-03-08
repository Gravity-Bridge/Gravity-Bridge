package keeper

import (
	"fmt"

	gethcommon "github.com/ethereum/go-ethereum/common"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
	"github.com/cosmos/cosmos-sdk/store/prefix"
)

/////////////////////////////
/////// BATCH CONFIRMS     //
/////////////////////////////

// GetBatchConfirm returns a batch confirmation given its nonce, the token contract, and a validator address
func (k Keeper) GetBatchConfirm(ctx sdk.Context, nonce uint64, tokenContract types.EthAddress, validator sdk.AccAddress) *types.MsgConfirmBatch {
	store := ctx.KVStore(k.storeKey)
	if err := sdk.VerifyAddressFormat(validator); err != nil {
		ctx.Logger().Error("invalid validator address")
		return nil
	}
	entity := store.Get(types.GetBatchConfirmKey(tokenContract, nonce, validator))
	if entity == nil {
		return nil
	}
	confirm := types.MsgConfirmBatch{
		Nonce:         nonce,
		TokenContract: tokenContract.GetAddress().Hex(),
		EthSigner:     "",
		Orchestrator:  "",
		Signature:     "",
	}
	k.cdc.MustUnmarshal(entity, &confirm)
	return &confirm
}

// SetBatchConfirm sets a batch confirmation by a validator
func (k Keeper) SetBatchConfirm(ctx sdk.Context, batch *types.MsgConfirmBatch) []byte {
	store := ctx.KVStore(k.storeKey)
	acc, err := sdk.AccAddressFromBech32(batch.Orchestrator)
	if err != nil {
		panic(sdkerrors.Wrap(err, "invalid Orchestrator address"))
	}
	contract, err := types.NewEthAddress(batch.TokenContract)
	if err != nil {
		panic(sdkerrors.Wrap(err, "invalid TokenContract"))
	}
	key := types.GetBatchConfirmKey(*contract, batch.Nonce, acc)
	store.Set(key, k.cdc.MustMarshal(batch))
	return key
}

// DeleteBatchConfirms deletes confirmations for an outgoing transaction batch
func (k Keeper) DeleteBatchConfirms(ctx sdk.Context, batch types.InternalOutgoingTxBatch) {
	store := ctx.KVStore(k.storeKey)
	for _, confirm := range k.GetBatchConfirmByNonceAndTokenContract(ctx, batch.BatchNonce, batch.TokenContract) {
		orchestrator, err := sdk.AccAddressFromBech32(confirm.Orchestrator)
		if err == nil {
			confirmKey := types.GetBatchConfirmKey(batch.TokenContract, batch.BatchNonce, orchestrator)
			if store.Has(confirmKey) {
				store.Delete(confirmKey)
			}
		}
	}
}

// IterateBatchConfirmByNonceAndTokenContract iterates through all batch confirmations
// MARK finish-batches: this is where the key is iterated in the old (presumed working) code
// TODO: specify which nonce this is
func (k Keeper) IterateBatchConfirmByNonceAndTokenContract(ctx sdk.Context, nonce uint64, tokenContract types.EthAddress, cb func([]byte, types.MsgConfirmBatch) bool) {
	store := ctx.KVStore(k.storeKey)
	prefix := types.GetBatchConfirmNonceContractPrefix(tokenContract, nonce)
	iter := store.Iterator(prefixRange([]byte(prefix)))

	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		confirm := types.MsgConfirmBatch{
			Nonce:         nonce,
			TokenContract: tokenContract.GetAddress().Hex(),
			EthSigner:     "",
			Orchestrator:  "",
			Signature:     "",
		}
		k.cdc.MustUnmarshal(iter.Value(), &confirm)
		// cb returns true to stop early
		if cb(iter.Key(), confirm) {
			break
		}
	}
}

// GetBatchConfirmByNonceAndTokenContract returns the batch confirms
func (k Keeper) GetBatchConfirmByNonceAndTokenContract(ctx sdk.Context, nonce uint64, tokenContract types.EthAddress) (out []types.MsgConfirmBatch) {
	k.IterateBatchConfirmByNonceAndTokenContract(ctx, nonce, tokenContract, func(_ []byte, msg types.MsgConfirmBatch) bool {
		out = append(out, msg)
		return false
	})
	return
}

// IterateBatchConfirms iterates through all batch confirmations
func (k Keeper) IterateBatchConfirms(ctx sdk.Context, cb func([]byte, types.MsgConfirmBatch) (stop bool)) {
	store := ctx.KVStore(k.storeKey)
	prefixStore := prefix.NewStore(store, types.BatchConfirmKey)
	iter := prefixStore.Iterator(nil, nil)

	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		var confirm types.MsgConfirmBatch
		k.cdc.MustUnmarshal(iter.Value(), &confirm)

		// cb returns true to stop early
		if cb(iter.Key(), confirm) {
			break
		}
	}
}

// setMonitoredTokenAddresses populates the list of tokens Orchestrators must monitor the Gravity.sol ERC20 balance of.
// Panics if the collected bytes to store are of unexpected length
// Note: This should ONLY be called on two occassions:
// * via governance proposal when the chain has voted on a new list to monitor, OR
// * via chain migration when updating the list as part of an in-place chain upgrade (complex launch, not recommended)
func (k Keeper) setMonitoredTokenAddresses(ctx sdk.Context, addresses []types.EthAddress) error {
	if err := types.ValidateEthAddresses(addresses); err != nil {
		return err
	}

	var bytesToStore []byte
	for _, address := range addresses {
		bytesToStore = append(bytesToStore, address.GetAddress().Bytes()...)
	}

	expectedLen := len(addresses) * gethcommon.AddressLength
	if len(bytesToStore) != expectedLen {
		errMsg := fmt.Sprintf("something went wrong when updating addresses - unexpected store value length (%v != expected %v)", len(bytesToStore), expectedLen)
		k.logger(ctx).Error(errMsg)
		panic(errMsg)
	}

	key := types.MonitoredTokenAddresses
	store := ctx.KVStore(k.storeKey)
	store.Set(key, bytesToStore)

	return nil
}

// MonitoredTokenAddresses returns the currently stored list of monitored ERC20 tokens as EthAddress-es
func (k Keeper) MonitoredTokenAddresses(ctx sdk.Context) []types.EthAddress {
	key := types.MonitoredTokenAddresses
	store := ctx.KVStore(k.storeKey)

	// Return early if the monitored tokens have not been voted on yet/assigned in an upgrade
	if !store.Has(key) {
		return []types.EthAddress{}
	}

	// Get the bytes and check the length
	value := store.Get(key)
	if len(value)%gethcommon.AddressLength != 0 {
		errMsg := fmt.Sprintf("unable to decode MonitoredTokenAddresses: %v is not a multiple of %v", len(value), gethcommon.AddressLength)
		k.logger(ctx).Error(errMsg)
		panic(errMsg)
	}

	// Decode the stored monitored tokens 20 bytes at a time
	numMonitoredTokens := len(value) / gethcommon.AddressLength
	addresses := make([]types.EthAddress, numMonitoredTokens)
	for i := 0; i < numMonitoredTokens; i++ {
		start, end := i*gethcommon.AddressLength, (i+1)*gethcommon.AddressLength
		addrBz := value[start:end]
		addr, err := types.NewEthAddressFromBytes(addrBz)
		if err != nil {
			errMsg := fmt.Sprintf("invalid address %v in MonitoredTokenAddresses: %v", addrBz, err)
			k.logger(ctx).Error(errMsg)
			panic(errMsg)
		}
		if addr == nil {
			errMsg := fmt.Sprintf("decoded nil address from bytes %v", addrBz)
			k.logger(ctx).Error(errMsg)
			panic(errMsg)
		}

		addresses[i] = *addr
	}

	return addresses
}

// MonitoredTokenStrings fetches the MonitoredTokenAddresses and converts them to string representation
func (k Keeper) MonitoredTokenStrings(ctx sdk.Context) []string {
	addrs := k.MonitoredTokenAddresses(ctx)

	ret := make([]string, len(addrs))
	for i, addr := range addrs {
		ret[i] = addr.GetAddress().String()
	}

	return ret
}
