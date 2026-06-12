package keeper

import (
	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
)

// CancelAllOutgoingTxsForContract cancels and refunds all pending outgoing bridge
// transactions (both unconfirmed batches and unbatched pool entries) for the given
// ERC20 contract address.
//
// This must be called while the contract's cosmos-originated denom mapping is still
// intact (before DeleteCosmosOriginatedDenomToERC20 and SetRemappedERC20) so that
// ERC20ToDenomLookup returns the old gravity0x denom and RemoveFromOutgoingPoolAndRefund
// refunds users in the correct pre-remap denom.
//
// Note on fees: the chain fee (MsgSendToEth.ChainFee) is paid to stakers/auction before
// the transaction ever enters the pool and is therefore not returned.  Only the send
// amount and bridge fee are escrowed in the module account and will be refunded.
func (k Keeper) CancelAllOutgoingTxsForContract(ctx sdk.Context, tokenContract types.EthAddress) error {
	// Cancel all unconfirmed batches for this contract.
	// CancelOutgoingTXBatch moves each batch's transactions back into the unbatched pool.
	var batchNonces []uint64
	k.IterateOutgoingTxBatches(ctx, func(_ []byte, batch types.InternalOutgoingTxBatch) bool {
		if batch.TokenContract.GetAddress() == tokenContract.GetAddress() {
			batchNonces = append(batchNonces, batch.BatchNonce)
		}
		return false
	})
	for _, nonce := range batchNonces {
		if err := k.CancelOutgoingTXBatch(ctx, tokenContract, nonce); err != nil {
			return errorsmod.Wrapf(err, "recovery: failed to cancel batch with nonce %d for contract %s",
				nonce, tokenContract.GetAddress().Hex())
		}
	}

	// Refund all unbatched pool entries for this contract.
	type txEntry struct {
		id     uint64
		sender sdk.AccAddress
	}
	var pending []txEntry
	k.IterateUnbatchedTransactionsByContract(ctx, tokenContract, func(_ []byte, tx *types.InternalOutgoingTransferTx) bool {
		pending = append(pending, txEntry{id: tx.Id, sender: tx.Sender})
		return false
	})
	for _, entry := range pending {
		if err := k.RemoveFromOutgoingPoolAndRefund(ctx, entry.id, entry.sender); err != nil {
			return errorsmod.Wrapf(err, "recovery: failed to refund tx %d for contract %s",
				entry.id, tokenContract.GetAddress().Hex())
		}
	}
	return nil
}
