package keeper

import (
	"fmt"
	"github.com/althea-net/cosmos-gravity-bridge/module/x/gravity/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
	"sort"
	"testing"
	"time"
)

// Tests that batches and transactions are preserved during chain restart
func TestBatchAndTxImportExport(t *testing.T) {
	// SETUP ENV + DATA
	// ==================
	input := CreateTestEnv(t)
	ctx   := input.Context
	batchSize    := 100
	accAddresses := []string{ // Warning: this must match the length of ctrAddresses

		"cosmos1dg55rtevlfxh46w88yjpdd08sqhh5cc3xhkcej",
		"cosmos164knshrzuuurf05qxf3q5ewpfnwzl4gj4m4dfy",
		"cosmos193fw83ynn76328pty4yl7473vg9x86alq2cft7",
		"cosmos1ahx7f8wyertuus9r20284ej0asrs085case3kn",
		"cosmos1ees2tqhhhm9ahlhceh2zdguww9lqn2ckukn86l",
	}
	ethAddresses := []string{
		"0xd041c41EA1bf0F006ADBb6d2c9ef9D425dE5eaD7",
		"0xd041c41EA1bf0F006ADBb6d2c9ef9D425dE5eaD8",
		"0xd041c41EA1bf0F006ADBb6d2c9ef9D425dE5eaD9",
		"0xd041c41EA1bf0F006ADBb6d2c9ef9D425dE5eaD0",
		"0xd041c41EA1bf0F006ADBb6d2c9ef9D425dE5eaD1",
		"0xd041c41EA1bf0F006ADBb6d2c9ef9D425dE5eaD2",
		"0xd041c41EA1bf0F006ADBb6d2c9ef9D425dE5eaD3",
	}
	ctrAddresses := []string{ // Warning: this must match the length of accAddresses
		"0x429881672B9AE42b8EbA0E26cD9C73711b891Ca5",
		"0x429881672B9AE42b8EbA0E26cD9C73711b891Ca6",
		"0x429881672B9AE42b8EbA0E26cD9C73711b891Ca7",
		"0x429881672B9AE42b8EbA0E26cD9C73711b891Ca8",
		"0x429881672B9AE42b8EbA0E26cD9C73711b891Ca9",
	}

	// SETUP ACCOUNTS
	// ==================
	senders := make([]*sdk.AccAddress, len(accAddresses))
	for i, _ := range senders {
		sender, err := sdk.AccAddressFromBech32(accAddresses[i])
		require.NoError(t, err)
		senders[i]  = &sender
	}
	receivers := make([]*types.EthAddress, len(ethAddresses))
	for i, _ := range receivers {
		receiver, err := types.NewEthAddress(ethAddresses[i])
		require.NoError(t, err)
		receivers[i]  = receiver
	}
	contracts := make([]*types.EthAddress, len(ctrAddresses))
	for i, _ := range contracts {
		contract, err := types.NewEthAddress(ctrAddresses[i])
		require.NoError(t, err)
		contracts[i]  = contract
	}
	tokens := make([]*types.InternalERC20Token, len(contracts))
	vouchers := make([]*sdk.Coins, len(contracts))
	for i, v := range contracts {
		token, err  := types.NewInternalERC20Token(sdk.NewInt(99999999), v.GetAddress())
		tokens[i] 	= token
		allVouchers := sdk.NewCoins(token.GravityCoin())
		vouchers[i] = &allVouchers
		require.NoError(t, err)

		// Mint the vouchers
		require.NoError(t, input.BankKeeper.MintCoins(ctx, types.ModuleName, allVouchers))
	}

	// give sender i a balance of token i
	for i, v := range senders {
		input.AccountKeeper.NewAccountWithAddress(ctx, *v)
		require.NoError(t, input.BankKeeper.SetBalances(ctx, *v, *vouchers[i]))
	}

	// CREATE TRANSACTIONS
	// ==================
	numTxs := 5000 // should end up with 1000 txs per contract
	txs := make([]*types.InternalOutgoingTransferTx, numTxs)
	fees := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}
	amounts := []int{51, 52, 53, 54, 55, 56, 57, 58, 59, 60}
	for i := 0; i < numTxs; i++ {
		// Pick fee, amount, sender, receiver, and contract for the ith transaction
		// Sender and contract will always match up (they must since sender i controls the whole balance of the ith token)
		// Receivers should get a balance of many token types since i % len(receivers) is usually different than i % len(contracts)
		fee := fees[i % len(fees)] // fee for this transaction
		amount := amounts[i % len(amounts)]
		sender := senders[i % len(senders)]
		receiver := receivers[i % len(receivers)]
		contract := contracts[i % len(contracts)]
		amountToken, err := types.NewInternalERC20Token(sdk.NewInt(int64(amount)), contract.GetAddress())
		require.NoError(t, err)
		feeToken, err := types.NewInternalERC20Token(sdk.NewInt(int64(fee)), contract.GetAddress())
		require.NoError(t, err)

		// add transaction to the pool
		id, err := input.GravityKeeper.AddToOutgoingPool(ctx, *sender, *receiver, amountToken.GravityCoin(), feeToken.GravityCoin())
		require.NoError(t, err)
		ctx.Logger().Info(fmt.Sprintf("Created transaction %v with amount %v and fee %v of contract %v from %v to %v", i, amount, fee, contract, sender, receiver))

		// Record the transaction for later testing
		tx, err := types.NewInternalOutgoingTransferTx(id, sender.String(), receiver.GetAddress(), *amountToken.ToExternal(), *feeToken.ToExternal())
		require.NoError(t, err)
		txs[i] = tx
	}

	// when

	now := time.Now().UTC()
	ctx = ctx.WithBlockTime(now)

	// CREATE BATCHES
	// ==================
	// Want to create batches for half of the transactions for each contract
	// with 100 tx in each batch, 1000 txs per contract, we want 5 batches per contract to batch 500 txs per contract
	batches := make([]*types.InternalOutgoingTxBatch, 5 * len(contracts))
	for i, v := range contracts {
		batch, err := input.GravityKeeper.BuildOutgoingTXBatch(ctx, *v, uint(batchSize))
		require.NoError(t, err)
		batches[i] = batch
		ctx.Logger().Info(fmt.Sprintf("Created batch %v for contract %v with %v transactions", i, v.GetAddress(), batchSize))
	}

	checkAllTransactionsExist(t, input.GravityKeeper, ctx, txs)
	exportImport(t, &input)
	checkAllTransactionsExist(t, input.GravityKeeper, ctx, txs)
}

// Requires that all transactions in txs exist in keeper
func checkAllTransactionsExist(t *testing.T, keeper Keeper, ctx sdk.Context, txs []*types.InternalOutgoingTransferTx) {
	unbatched := keeper.GetUnbatchedTransactions(ctx)
	batches := keeper.GetOutgoingTxBatches(ctx)
	// Collect all txs into an array
	var gotTxs []*types.InternalOutgoingTransferTx
	gotTxs = append(gotTxs, unbatched...)
	for _, batch := range batches {
		gotTxs = append(gotTxs, batch.Transactions...)
	}
	require.Equal(t, len(txs), len(gotTxs))
	// Sort both arrays for simple searching
	sort.Slice(gotTxs, func(i, j int) bool {
		return gotTxs[i].Id < gotTxs[j].Id
	})
	sort.Slice(txs, func(i, j int) bool {
		return txs[i].Id < txs[j].Id
	})
	// Actually check that the txs all exist, iterate on txs in case some got lost in the import/export step
	for i, exp := range txs {
		require.Equal(t, exp.Id, gotTxs[i].Id)
		require.Equal(t, exp.Erc20Fee, gotTxs[i].Erc20Fee)
		require.Equal(t, exp.Erc20Token, gotTxs[i].Erc20Token)
		require.Equal(t, exp.DestAddress.GetAddress(), gotTxs[i].DestAddress.GetAddress())
		require.Equal(t, exp.Sender.String(), gotTxs[i].Sender.String())
	}
}

// Exports and then imports all bridge state, overwrites the `input` test environment to simulate chain restart
func exportImport(t *testing.T, input *TestInput) {
	genesisState := ExportGenesis(input.Context, input.GravityKeeper)
	newEnv := CreateTestEnv(t)
	input = &newEnv
	unbatched := input.GravityKeeper.GetUnbatchedTransactions(input.Context)
	require.Empty(t, unbatched)
	batches := input.GravityKeeper.GetOutgoingTxBatches(input.Context)
	require.Empty(t, batches)
	InitGenesis(input.Context, input.GravityKeeper, genesisState)
}
