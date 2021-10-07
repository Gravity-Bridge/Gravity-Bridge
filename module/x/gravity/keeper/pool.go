package keeper

import (
	"encoding/binary"
	"fmt"
	"sort"
	"strconv"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"

	"github.com/althea-net/cosmos-gravity-bridge/module/x/gravity/types"
)

// AddToOutgoingPool creates a transaction and adds it to the pool, returns the id of the unbatched transaction
// - checks a counterpart denominator exists for the given voucher type
// - burns the voucher for transfer amount and fees
// - persists an OutgoingTx
// - adds the TX to the `available` TX pool
func (k Keeper) AddToOutgoingPool(
	ctx sdk.Context,
	sender sdk.AccAddress,
	counterpartReceiver types.EthAddress,
	amount sdk.Coin,
	fee sdk.Coin,
) (uint64, error) {
	if ctx.IsZero() || sender.Empty() || counterpartReceiver.ValidateBasic() != nil ||
		!amount.IsValid() || !fee.IsValid() || fee.Denom != amount.Denom {
		return 0, sdkerrors.Wrap(types.ErrInvalid, "arguments")
	}
	totalAmount := amount.Add(fee)
	totalInVouchers := sdk.Coins{totalAmount}

	// If the coin is a gravity voucher, burn the coins. If not, check if there is a deployed ERC20 contract representing it.
	// If there is, lock the coins.

	isCosmosOriginated, tokenContract, err := k.DenomToERC20Lookup(ctx, totalAmount.Denom)
	if err != nil {
		return 0, err
	}

	// If it is a cosmos-originated asset we lock it
	if isCosmosOriginated {
		// lock coins in module
		if err := k.bankKeeper.SendCoinsFromAccountToModule(ctx, sender, types.ModuleName, totalInVouchers); err != nil {
			return 0, err
		}
	} else {
		// If it is an ethereum-originated asset we burn it
		// send coins to module in prep for burn
		if err := k.bankKeeper.SendCoinsFromAccountToModule(ctx, sender, types.ModuleName, totalInVouchers); err != nil {
			return 0, err
		}

		// burn vouchers to send them back to ETH
		if err := k.bankKeeper.BurnCoins(ctx, types.ModuleName, totalInVouchers); err != nil {
			panic(err)
		}
	}

	// get next tx id from keeper
	nextID := k.autoIncrementID(ctx, []byte(types.KeyLastTXPoolID))

	erc20Fee, err := types.NewInternalERC20Token(fee.Amount, tokenContract.GetAddress())
	if err != nil {
		return 0, sdkerrors.Wrapf(err, "invalid Erc20Fee from amount %d and contract %v",
			fee.Amount, tokenContract)
	}
	erc20Token, err := types.NewInternalERC20Token(amount.Amount, tokenContract.GetAddress())
	if err != nil {
		return 0, sdkerrors.Wrapf(err, "invalid ERC20Token from amount %d and contract %v",
			amount.Amount, tokenContract)
	}
	// construct outgoing tx, as part of this process we represent
	// the token as an ERC20 token since it is preparing to go to ETH
	// rather than the denom that is the input to this function.
	outgoing, err := types.OutgoingTransferTx{
		Id:          nextID,
		Sender:      sender.String(),
		DestAddress: counterpartReceiver.GetAddress(),
		Erc20Token:  erc20Token.ToExternal(),
		Erc20Fee:    erc20Fee.ToExternal(),
	}.ToInternal()
	if err != nil { // This should never happen since all the components are validated
		panic(sdkerrors.Wrap(err, "unable to create InternalOutgoingTransferTx"))
	}

	// add a second index with the fee
	err = k.addUnbatchedTX(ctx, outgoing)
	if err != nil {
		panic(err)
	}

	// todo: add second index for sender so that we can easily query: give pending Tx by sender
	// todo: what about a second index for receiver?

	poolEvent := sdk.NewEvent(
		types.EventTypeBridgeWithdrawalReceived,
		sdk.NewAttribute(sdk.AttributeKeyModule, types.ModuleName),
		sdk.NewAttribute(types.AttributeKeyContract, k.GetBridgeContractAddress(ctx).GetAddress()),
		sdk.NewAttribute(types.AttributeKeyBridgeChainID, strconv.Itoa(int(k.GetBridgeChainID(ctx)))),
		sdk.NewAttribute(types.AttributeKeyOutgoingTXID, strconv.Itoa(int(nextID))),
		sdk.NewAttribute(types.AttributeKeyNonce, fmt.Sprint(nextID)),
	)
	ctx.EventManager().EmitEvent(poolEvent)

	return nextID, nil
}

// RemoveFromOutgoingPoolAndRefund
// - checks that the provided tx actually exists
// - deletes the unbatched tx from the pool
// - issues the tokens back to the sender
func (k Keeper) RemoveFromOutgoingPoolAndRefund(ctx sdk.Context, txId uint64, sender sdk.AccAddress) error {
	if ctx.IsZero() || txId < 1 || sender.Empty() {
		return sdkerrors.Wrap(types.ErrInvalid, "arguments")
	}
	// check that we actually have a tx with that id and what it's details are
	tx, err := k.GetUnbatchedTxById(ctx, txId)
	if err != nil {
		return sdkerrors.Wrapf(err, "unknown transaction with id %d from sender %s", txId, sender.String())
	}

	// Check that this user actually sent the transaction, this prevents someone from refunding someone
	// elses transaction to themselves.
	if !tx.Sender.Equals(sender) {
		return sdkerrors.Wrapf(types.ErrInvalid, "Sender %s did not send Id %d", sender, txId)
	}

	// An inconsistent entry should never enter the store, but this is the ideal place to exploit
	// it such a bug if it did ever occur, so we should double check to be really sure
	if tx.Erc20Fee.Contract != tx.Erc20Token.Contract {
		return sdkerrors.Wrapf(types.ErrInvalid, "Inconsistent tokens to cancel!: %s %s", tx.Erc20Fee.Contract, tx.Erc20Token.Contract)
	}

	// delete this tx from the pool
	err = k.removeUnbatchedTX(ctx, *tx.Erc20Fee, txId)
	if err != nil {
		return sdkerrors.Wrapf(types.ErrInvalid, "txId %d not in unbatched index! Must be in a batch!", txId)
	}
	// Make sure the tx was removed
	oldTx, oldTxErr := k.GetUnbatchedTxByFeeAndId(ctx, *tx.Erc20Fee, tx.Id)
	if oldTx != nil || oldTxErr == nil {
		return sdkerrors.Wrapf(types.ErrInvalid, "tx with id %d was not fully removed from the pool, a duplicate must exist", txId)
	}

	// reissue the amount and the fee
	totalToRefund := tx.Erc20Token.GravityCoin()
	totalToRefund.Amount = totalToRefund.Amount.Add(tx.Erc20Fee.Amount)
	totalToRefundCoins := sdk.NewCoins(totalToRefund)

	isCosmosOriginated, _ := k.ERC20ToDenomLookup(ctx, tx.Erc20Token.Contract)

	// If it is a cosmos-originated the coins are in the module (see AddToOutgoingPool) so we can just take them out
	if isCosmosOriginated {
		if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, sender, totalToRefundCoins); err != nil {
			return err
		}
	} else {
		// If it is an ethereum-originated asset we have to mint it (see Handle in attestation_handler.go)
		// mint coins in module for prep to send
		if err := k.bankKeeper.MintCoins(ctx, types.ModuleName, totalToRefundCoins); err != nil {
			return sdkerrors.Wrapf(err, "mint vouchers coins: %s", totalToRefundCoins)
		}
		if err = k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, sender, totalToRefundCoins); err != nil {
			return sdkerrors.Wrap(err, "transfer vouchers")
		}
	}

	poolEvent := sdk.NewEvent(
		types.EventTypeBridgeWithdrawCanceled,
		sdk.NewAttribute(sdk.AttributeKeyModule, types.ModuleName),
		sdk.NewAttribute(types.AttributeKeyContract, k.GetBridgeContractAddress(ctx).GetAddress()),
		sdk.NewAttribute(types.AttributeKeyBridgeChainID, strconv.Itoa(int(k.GetBridgeChainID(ctx)))),
	)
	ctx.EventManager().EmitEvent(poolEvent)

	return nil
}

// addUnbatchedTx creates a new transaction in the pool
// WARNING: Do not make this function public
func (k Keeper) addUnbatchedTX(ctx sdk.Context, val *types.InternalOutgoingTransferTx) error {
	store := ctx.KVStore(k.storeKey)
	idxKey := []byte(types.GetOutgoingTxPoolKey(*val.Erc20Fee, val.Id))
	if store.Has(idxKey) {
		return sdkerrors.Wrap(types.ErrDuplicate, "transaction already in pool")
	}

	extVal := val.ToExternal()

	bz, err := k.cdc.MarshalBinaryBare(extVal)
	if err != nil {
		return err
	}

	store.Set(idxKey, bz)
	return err
}

// removeUnbatchedTXIndex removes the tx from the pool
// WARNING: Do not make this function public
func (k Keeper) removeUnbatchedTX(ctx sdk.Context, fee types.InternalERC20Token, txID uint64) error {
	store := ctx.KVStore(k.storeKey)
	idxKey := []byte(types.GetOutgoingTxPoolKey(fee, txID))
	if !store.Has(idxKey) {
		return sdkerrors.Wrap(types.ErrUnknown, "pool transaction")
	}
	store.Delete(idxKey)
	return nil
}

// GetUnbatchedTxByFeeAndId grabs a tx from the pool given its fee and txID
func (k Keeper) GetUnbatchedTxByFeeAndId(ctx sdk.Context, fee types.InternalERC20Token, txID uint64) (*types.InternalOutgoingTransferTx, error) {
	store := ctx.KVStore(k.storeKey)
	bz := store.Get([]byte(types.GetOutgoingTxPoolKey(fee, txID)))
	if bz == nil {
		return nil, sdkerrors.Wrap(types.ErrUnknown, "pool transaction")
	}
	var r types.OutgoingTransferTx
	k.cdc.UnmarshalBinaryBare(bz, &r)
	intR, err := r.ToInternal()
	if err != nil {
		panic(sdkerrors.Wrapf(err, "invalid unbatched tx in store: %v", r))
	}
	return intR, nil
}

// GetUnbatchedTxById grabs a tx from the pool given only the txID
// note that due to the way unbatched txs are indexed, the GetUnbatchedTxByFeeAndId method is much faster
func (k Keeper) GetUnbatchedTxById(ctx sdk.Context, txID uint64) (*types.InternalOutgoingTransferTx, error) {
	var r *types.InternalOutgoingTransferTx = nil
	k.IterateUnbatchedTransactions(ctx, []byte(types.OutgoingTXPoolKey), func(_ []byte, tx *types.InternalOutgoingTransferTx) bool {
		if tx.Id == txID {
			r = tx
			return true
		}
		return false // iterating DESC, exit early
	})

	if r == nil {
		// We have no return tx, it was either batched or never existed
		return nil, sdkerrors.Wrap(types.ErrUnknown, "pool transaction")
	}
	return r, nil
}

// GetUnbatchedTransactionsByContract, grabs all unbatched transactions from the tx pool for the given contract
// unbatched transactions are sorted by fee amount in DESC order
func (k Keeper) GetUnbatchedTransactionsByContract(ctx sdk.Context, contractAddress types.EthAddress) []*types.InternalOutgoingTransferTx {
	return k.collectUnbatchedTransactions(ctx, []byte(types.GetOutgoingTxPoolContractPrefix(contractAddress)))
}

// GetPoolTransactions, grabs all transactions from the tx pool, useful for queries or genesis save/load
func (k Keeper) GetUnbatchedTransactions(ctx sdk.Context) []*types.InternalOutgoingTransferTx {
	return k.collectUnbatchedTransactions(ctx, []byte(types.OutgoingTXPoolKey))
}

// Aggregates all unbatched transactions in the store with a given prefix
func (k Keeper) collectUnbatchedTransactions(ctx sdk.Context, prefixKey []byte) (out []*types.InternalOutgoingTransferTx) {
	k.IterateUnbatchedTransactions(ctx, prefixKey, func(_ []byte, tx *types.InternalOutgoingTransferTx) bool {
		out = append(out, tx)
		return false
	})
	return
}

// IterateUnbatchedTransactionsByContract, iterates through unbatched transactions from the tx pool for the given contract
// unbatched transactions are sorted by fee amount in DESC order
func (k Keeper) IterateUnbatchedTransactionsByContract(ctx sdk.Context, contractAddress types.EthAddress, cb func(key []byte, tx *types.InternalOutgoingTransferTx) bool) {
	k.IterateUnbatchedTransactions(ctx, []byte(types.GetOutgoingTxPoolContractPrefix(contractAddress)), cb)
}

// IterateUnbatchedTransactions iterates through all unbatched transactions whose keys begin with prefixKey in DESC order
func (k Keeper) IterateUnbatchedTransactions(ctx sdk.Context, prefixKey []byte, cb func(key []byte, tx *types.InternalOutgoingTransferTx) bool) {
	prefixStore := ctx.KVStore(k.storeKey)
	iter := prefixStore.ReverseIterator(prefixRange(prefixKey))
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		var transact types.OutgoingTransferTx
		k.cdc.MustUnmarshalBinaryBare(iter.Value(), &transact)
		intTx, err := transact.ToInternal()
		if err != nil {
			panic(sdkerrors.Wrapf(err, "invalid unbatched transaction in store: %v", transact))
		}
		// cb returns true to stop early
		if cb(iter.Key(), intTx) {
			break
		}
	}
}

// GetBatchFeeByTokenType gets the fee the next batch of a given token type would
// have if created right now. This info is both presented to relayers for the purpose of determining
// when to request batches and also used by the batch creation process to decide not to create
// a new batch (fees must be increasing)
func (k Keeper) GetBatchFeeByTokenType(ctx sdk.Context, tokenContractAddr types.EthAddress, maxElements uint) *types.BatchFees {
	batchFee := types.BatchFees{Token: tokenContractAddr.GetAddress(), TotalFees: sdk.NewInt(0)}
	txCount := 0

	k.IterateUnbatchedTransactions(ctx, []byte(types.GetOutgoingTxPoolContractPrefix(tokenContractAddr)), func(_ []byte, tx *types.InternalOutgoingTransferTx) bool {
		fee := tx.Erc20Fee
		if fee.Contract.GetAddress() != tokenContractAddr.GetAddress() {
			panic(fmt.Errorf("unexpected fee contract %s when getting batch fees for contract %s", fee.Contract, tokenContractAddr))
		}
		batchFee.TotalFees = batchFee.TotalFees.Add(fee.Amount)
		txCount += 1
		return txCount == int(maxElements)
	})
	return &batchFee
}

// GetAllBatchFees creates a fee entry for every batch type currently in the store
// this can be used by relayers to determine what batch types are desireable to request
func (k Keeper) GetAllBatchFees(ctx sdk.Context, maxElements uint) (batchFees []*types.BatchFees) {
	batchFeesMap := k.createBatchFees(ctx, maxElements)
	// create array of batchFees
	for _, batchFee := range batchFeesMap {
		batchFees = append(batchFees, batchFee)
	}

	// quick sort by token to make this function safe for use
	// in consensus computations
	sort.Slice(batchFees, func(i, j int) bool {
		return batchFees[i].Token < batchFees[j].Token
	})

	return batchFees
}

// createBatchFees iterates over the unbatched transaction pool and creates batch token fee map
// Implicitly creates batches with the highest potential fee because the transaction keys enforce an order which goes
// fee contract address -> fee amount -> transaction nonce
func (k Keeper) createBatchFees(ctx sdk.Context, maxElements uint) map[string]*types.BatchFees {
	batchFeesMap := make(map[string]*types.BatchFees)
	txCountMap := make(map[string]int)

	k.IterateUnbatchedTransactions(ctx, []byte(types.OutgoingTXPoolKey), func(_ []byte, tx *types.InternalOutgoingTransferTx) bool {
		if txCountMap[tx.Erc20Fee.Contract.GetAddress()] < int(maxElements) {
			addFeeToMap(tx.Erc20Fee, batchFeesMap, txCountMap)
		}
		return false
	})

	return batchFeesMap
}

// Helper method for creating batch fees
func addFeeToMap(fee *types.InternalERC20Token, batchFeesMap map[string]*types.BatchFees, txCountMap map[string]int) {
	feeAddrStr := fee.Contract.GetAddress()
	txCountMap[feeAddrStr] = txCountMap[feeAddrStr] + 1

	// add fee amount
	if _, ok := batchFeesMap[feeAddrStr]; ok {
		batchFeesMap[feeAddrStr].TotalFees = batchFeesMap[feeAddrStr].TotalFees.Add(fee.Amount)
	} else {
		batchFeesMap[feeAddrStr] = &types.BatchFees{
			Token:     feeAddrStr,
			TotalFees: fee.Amount}
	}
}

func (k Keeper) autoIncrementID(ctx sdk.Context, idKey []byte) uint64 {
	store := ctx.KVStore(k.storeKey)
	bz := store.Get(idKey)
	var id uint64 = 1
	if bz != nil {
		id = binary.BigEndian.Uint64(bz)
	}
	bz = sdk.Uint64ToBigEndian(id + 1)
	store.Set(idKey, bz)
	return id
}
