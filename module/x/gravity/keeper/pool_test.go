package keeper

import (
	"fmt"
	"math/big"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/althea-net/cosmos-gravity-bridge/module/x/gravity/types"
)

// Tests that the pool is populated with the created transactions before any batch is created
func TestAddToOutgoingPool(t *testing.T) {
	input := CreateTestEnv(t)
	ctx := input.Context
	var (
		mySender, _         = sdk.AccAddressFromBech32("cosmos1ahx7f8wyertuus9r20284ej0asrs085case3kn")
		myReceiver          = "0xd041c41EA1bf0F006ADBb6d2c9ef9D425dE5eaD7"
		myTokenContractAddr = "0x429881672B9AE42b8EbA0E26cD9C73711b891Ca5"
	)
	// mint some voucher first
	allVouchers := sdk.Coins{types.NewERC20Token(99999, myTokenContractAddr).GravityCoin()}
	err := input.BankKeeper.MintCoins(ctx, types.ModuleName, allVouchers)
	require.NoError(t, err)

	// set senders balance
	input.AccountKeeper.NewAccountWithAddress(ctx, mySender)
	err = input.BankKeeper.SetBalances(ctx, mySender, allVouchers)
	require.NoError(t, err)

	// when
	for i, v := range []uint64{2, 3, 2, 1} {
		amount := types.NewERC20Token(uint64(i+100), myTokenContractAddr).GravityCoin()
		fee := types.NewERC20Token(v, myTokenContractAddr).GravityCoin()
		r, err := input.GravityKeeper.AddToOutgoingPool(ctx, mySender, myReceiver, amount, fee)
		require.NoError(t, err)
		t.Logf("___ response: %#v", r)
		// Should create:
		// 1: amount 100, fee 2
		// 2: amount 101, fee 3
		// 3: amount 102, fee 2
		// 4: amount 103, fee 1

	}
	// then
	got := input.GravityKeeper.GetUnbatchedTransactionsByContract(ctx, myTokenContractAddr)

	exp := []*types.OutgoingTransferTx{
		{
			Id:          2,
			Erc20Fee:    types.NewERC20Token(3, myTokenContractAddr),
			Sender:      mySender.String(),
			DestAddress: myReceiver,
			Erc20Token:  types.NewERC20Token(101, myTokenContractAddr),
		},
		{
			Id:          3,
			Erc20Fee:    types.NewERC20Token(2, myTokenContractAddr),
			Sender:      mySender.String(),
			DestAddress: myReceiver,
			Erc20Token:  types.NewERC20Token(102, myTokenContractAddr),
		},
		{
			Id:          1,
			Erc20Fee:    types.NewERC20Token(2, myTokenContractAddr),
			Sender:      mySender.String(),
			DestAddress: myReceiver,
			Erc20Token:  types.NewERC20Token(100, myTokenContractAddr),
		},
		{
			Id:          4,
			Erc20Fee:    types.NewERC20Token(1, myTokenContractAddr),
			Sender:      mySender.String(),
			DestAddress: myReceiver,
			Erc20Token:  types.NewERC20Token(103, myTokenContractAddr),
		},
	}
	assert.Equal(t, exp, got)
}

// Checks some common edge cases like invalid inputs, user doesn't have enough tokens, token doesn't exist, inconsistent entry
func TestAddToOutgoingPoolEdgeCases(t *testing.T) {
	input := CreateTestEnv(t)
	ctx := input.Context
	var (
		mySender, _         = sdk.AccAddressFromBech32("cosmos1ahx7f8wyertuus9r20284ej0asrs085case3kn")
		myReceiver          = "0xd041c41EA1bf0F006ADBb6d2c9ef9D425dE5eaD7"
		myTokenContractAddr = "0x429881672B9AE42b8EbA0E26cD9C73711b891Ca5"
	)

	amount := types.NewERC20Token(uint64(100), myTokenContractAddr).GravityCoin()
	fee := types.NewERC20Token(2, myTokenContractAddr).GravityCoin()

	//////// Nonexistant Token ////////
	r, err := input.GravityKeeper.AddToOutgoingPool(ctx, mySender, myReceiver, amount, fee)
	require.Error(t, err)
	require.Zero(t, r)

	// mint some voucher first
	allVouchers := sdk.Coins{types.NewERC20Token(99999, myTokenContractAddr).GravityCoin()}
	err = input.BankKeeper.MintCoins(ctx, types.ModuleName, allVouchers)
	require.NoError(t, err)

	// set senders balance
	input.AccountKeeper.NewAccountWithAddress(ctx, mySender)
	err = input.BankKeeper.SetBalances(ctx, mySender, allVouchers)
	require.NoError(t, err)

	//////// Insufficient Balance from Amount ////////
	badAmount := types.NewERC20Token(uint64(999999), myTokenContractAddr).GravityCoin()
	r, err = input.GravityKeeper.AddToOutgoingPool(ctx, mySender, myReceiver, badAmount, fee)
	require.Error(t, err)
	require.Zero(t, r)

	//////// Insufficient Balance from Fee ////////
	badFee := types.NewERC20Token(uint64(999999), myTokenContractAddr).GravityCoin()
	r, err = input.GravityKeeper.AddToOutgoingPool(ctx, mySender, myReceiver, amount, badFee)
	require.Error(t, err)
	require.Zero(t, r)

	//////// Insufficient Balance from Amount and Fee ////////
	// Amount is 100, fee is the current balance - 99
	badFee = types.NewERC20Token(uint64(99999-99), myTokenContractAddr).GravityCoin()
	r, err = input.GravityKeeper.AddToOutgoingPool(ctx, mySender, myReceiver, amount, badFee)
	require.Error(t, err)
	require.Zero(t, r)

	//////// Zero inputs ////////
	mtCtx := new(sdk.Context)
	mtSend := new(sdk.AccAddress)
	mtRecieve := new(string)
	mtCoin := new(sdk.Coin)
	r, err = input.GravityKeeper.AddToOutgoingPool(*mtCtx, *mtSend, *mtRecieve, *mtCoin, *mtCoin)
	require.Error(t, err)
	require.Zero(t, r)

	//////// Inconsistent Entry ////////
	badFeeContractAddr := "0x429881672B9AE42b8EbA0E26cD9C73711b891Ca6"
	badFee = types.NewERC20Token(100, badFeeContractAddr).GravityCoin()
	r, err = input.GravityKeeper.AddToOutgoingPool(ctx, mySender, myReceiver, amount, badFee)
	require.Error(t, err)
	require.Zero(t, r)
}

func TestTotalBatchFeeInPool(t *testing.T) {
	input := CreateTestEnv(t)
	ctx := input.Context

	// token1
	var (
		mySender, _         = sdk.AccAddressFromBech32("cosmos1ahx7f8wyertuus9r20284ej0asrs085case3kn")
		myReceiver          = "0xd041c41EA1bf0F006ADBb6d2c9ef9D425dE5eaD7"
		myTokenContractAddr = "0x429881672B9AE42b8EbA0E26cD9C73711b891Ca5"
	)
	// mint some voucher first
	allVouchers := sdk.Coins{types.NewERC20Token(99999, myTokenContractAddr).GravityCoin()}
	err := input.BankKeeper.MintCoins(ctx, types.ModuleName, allVouchers)
	require.NoError(t, err)

	// set senders balance
	input.AccountKeeper.NewAccountWithAddress(ctx, mySender)
	err = input.BankKeeper.SetBalances(ctx, mySender, allVouchers)
	require.NoError(t, err)

	// create outgoing pool
	for i, v := range []uint64{2, 3, 2, 1} {
		amount := types.NewERC20Token(uint64(i+100), myTokenContractAddr).GravityCoin()
		fee := types.NewERC20Token(v, myTokenContractAddr).GravityCoin()
		r, err2 := input.GravityKeeper.AddToOutgoingPool(ctx, mySender, myReceiver, amount, fee)
		require.NoError(t, err2)
		t.Logf("___ response: %#v", r)
	}

	// token 2 - Only top 100
	var (
		myToken2ContractAddr = "0x7D1AfA7B718fb893dB30A3aBc0Cfc608AaCfeBB0"
	)
	// mint some voucher first
	allVouchers = sdk.Coins{types.NewERC20Token(18446744073709551615, myToken2ContractAddr).GravityCoin()}
	err = input.BankKeeper.MintCoins(ctx, types.ModuleName, allVouchers)
	require.NoError(t, err)

	// set senders balance
	input.AccountKeeper.NewAccountWithAddress(ctx, mySender)
	err = input.BankKeeper.SetBalances(ctx, mySender, allVouchers)
	require.NoError(t, err)

	// Add

	// create outgoing pool
	for i := 0; i < 110; i++ {
		amount := types.NewERC20Token(uint64(i+100), myToken2ContractAddr).GravityCoin()
		fee := types.NewERC20Token(uint64(5), myToken2ContractAddr).GravityCoin()
		r, err := input.GravityKeeper.AddToOutgoingPool(ctx, mySender, myReceiver, amount, fee)
		require.NoError(t, err)
		t.Logf("___ response: %#v", r)
	}

	batchFees := input.GravityKeeper.GetAllBatchFees(ctx, OutgoingTxBatchSize)
	/*
		tokenFeeMap should be
		map[0x429881672B9AE42b8EbA0E26cD9C73711b891Ca5:8 0x7D1AfA7B718fb893dB30A3aBc0Cfc608AaCfeBB0:500]
		**/
	assert.Equal(t, batchFees[0].TotalFees.BigInt(), big.NewInt(int64(8)))
	assert.Equal(t, batchFees[1].TotalFees.BigInt(), big.NewInt(int64(500)))

}

func TestGetBatchFeeByTokenType(t *testing.T) {
	input := CreateTestEnv(t)
	ctx := input.Context

	// token1
	var (
		mySender1, _                        = sdk.AccAddressFromBech32("cosmos1ahx7f8wyertuus9r20284ej0asrs085case3kn")
		mySender2            sdk.AccAddress = []byte("cosmos1ahx7f8wyertus")
		mySender3            sdk.AccAddress = []byte("cosmos1ahx7f8wyertut")
		myReceiver                          = "0xd041c41EA1bf0F006ADBb6d2c9ef9D425dE5eaD7"
		myTokenContractAddr1                = "0x429881672B9AE42b8EbA0E26cD9C73711b891Ca5"
		myTokenContractAddr2                = "0x429881672B9AE42b8EbA0E26cD9C73711b891Ca6"
		myTokenContractAddr3                = "0x429881672B9AE42b8EbA0E26cD9C73711b891Ca7"
	)
	// mint some vouchers first
	allVouchers1 := sdk.Coins{types.NewERC20Token(99999, myTokenContractAddr1).GravityCoin()}
	allVouchers2 := sdk.Coins{types.NewERC20Token(99999, myTokenContractAddr2).GravityCoin()}
	allVouchers3 := sdk.Coins{types.NewERC20Token(99999, myTokenContractAddr3).GravityCoin()}
	err := input.BankKeeper.MintCoins(ctx, types.ModuleName, allVouchers1)
	require.NoError(t, err)
	err = input.BankKeeper.MintCoins(ctx, types.ModuleName, allVouchers2)
	require.NoError(t, err)
	err = input.BankKeeper.MintCoins(ctx, types.ModuleName, allVouchers3)
	require.NoError(t, err)

	// set senders balance
	input.AccountKeeper.NewAccountWithAddress(ctx, mySender1)
	err = input.BankKeeper.SetBalances(ctx, mySender1, allVouchers1)
	require.NoError(t, err)
	input.AccountKeeper.NewAccountWithAddress(ctx, mySender2)
	err = input.BankKeeper.SetBalances(ctx, mySender2, allVouchers2)
	require.NoError(t, err)
	input.AccountKeeper.NewAccountWithAddress(ctx, mySender3)
	err = input.BankKeeper.SetBalances(ctx, mySender3, allVouchers3)
	require.NoError(t, err)

	totalFee1 := uint64(0)
	totalFee2 := uint64(0)
	totalFee3 := uint64(0)
	// create outgoing pool
	for i := 0; i < 110; i++ {
		amount1 := types.NewERC20Token(uint64(i+100), myTokenContractAddr1).GravityCoin()
		feeAmt1 := uint64(i + 1) // fees can't be 0
		if i >= 10 {
			totalFee1 += feeAmt1
		}
		fee1 := types.NewERC20Token(feeAmt1, myTokenContractAddr1).GravityCoin()
		r, err := input.GravityKeeper.AddToOutgoingPool(ctx, mySender1, myReceiver, amount1, fee1)
		require.NoError(t, err)
		t.Logf("___ response: %d", r)

		amount2 := types.NewERC20Token(uint64(i+100), myTokenContractAddr2).GravityCoin()
		feeAmt2 := uint64(2*i + 1) // fees can't be 0
		if i >= 10 {
			totalFee2 += feeAmt2
		}
		fee2 := types.NewERC20Token(feeAmt2, myTokenContractAddr2).GravityCoin()
		r, err = input.GravityKeeper.AddToOutgoingPool(ctx, mySender2, myReceiver, amount2, fee2)
		require.NoError(t, err)
		t.Logf("___ response: %d", r)

		amount3 := types.NewERC20Token(uint64(i+100), myTokenContractAddr3).GravityCoin()
		feeAmt3 := uint64(3*i + 1) // fees can't be 0
		if i >= 10 {
			totalFee3 += feeAmt3
		}
		fee3 := types.NewERC20Token(feeAmt3, myTokenContractAddr3).GravityCoin()
		r, err = input.GravityKeeper.AddToOutgoingPool(ctx, mySender3, myReceiver, amount3, fee3)
		require.NoError(t, err)
		t.Logf("___ response: %d", r)
	}

	batchFee1 := input.GravityKeeper.GetBatchFeeByTokenType(ctx, myTokenContractAddr1, 100)
	require.Equal(t, batchFee1.Token, myTokenContractAddr1)
	require.Equal(t, batchFee1.TotalFees.Uint64(), totalFee1, fmt.Errorf("expected total fees %d but got %d", batchFee1.TotalFees.Uint64(), totalFee1))
	batchFee2 := input.GravityKeeper.GetBatchFeeByTokenType(ctx, myTokenContractAddr2, 100)
	require.Equal(t, batchFee2.Token, myTokenContractAddr2)
	require.Equal(t, batchFee2.TotalFees.Uint64(), totalFee2, fmt.Errorf("expected total fees %d but got %d", batchFee2.TotalFees.Uint64(), totalFee2))
	batchFee3 := input.GravityKeeper.GetBatchFeeByTokenType(ctx, myTokenContractAddr3, 100)
	require.Equal(t, batchFee3.Token, myTokenContractAddr3)
	require.Equal(t, batchFee3.TotalFees.Uint64(), totalFee3, fmt.Errorf("expected total fees %d but got %d", batchFee3.TotalFees.Uint64(), totalFee3))

}

func TestRemoveFromOutgoingPoolAndRefund(t *testing.T) {
	input := CreateTestEnv(t)
	ctx := input.Context
	var (
		mySender, _         = sdk.AccAddressFromBech32("cosmos1ahx7f8wyertuus9r20284ej0asrs085case3kn")
		myReceiver          = "0xd041c41EA1bf0F006ADBb6d2c9ef9D425dE5eaD7"
		myTokenContractAddr = "0x429881672B9AE42b8EbA0E26cD9C73711b891Ca5"
		myTokenDenom        = "gravity" + myTokenContractAddr
	)
	// mint some voucher first
	originalBal := uint64(99999)
	allVouchers := sdk.Coins{types.NewERC20Token(originalBal, myTokenContractAddr).GravityCoin()}
	err := input.BankKeeper.MintCoins(ctx, types.ModuleName, allVouchers)
	require.NoError(t, err)

	// set senders balance
	input.AccountKeeper.NewAccountWithAddress(ctx, mySender)
	err = input.BankKeeper.SetBalances(ctx, mySender, allVouchers)
	require.NoError(t, err)

	// Create unbatched transactions
	require.Empty(t, input.GravityKeeper.GetUnbatchedTransactions(ctx))
	feesAndAmounts := uint64(0)
	ids := make([]uint64, 4)
	fees := []uint64{2, 3, 2, 1}
	amounts := []uint64{100, 101, 102, 103}
	for i, v := range fees {
		amount := types.NewERC20Token(amounts[i], myTokenContractAddr).GravityCoin()
		fee := types.NewERC20Token(v, myTokenContractAddr).GravityCoin()
		feesAndAmounts += 100 + uint64(i) + v
		r, err := input.GravityKeeper.AddToOutgoingPool(ctx, mySender, myReceiver, amount, fee)
		require.NoError(t, err)
		t.Logf("___ response: %#v", r)
		ids[i] = r
		// Should create:
		// 1: amount 100, fee 2
		// 2: amount 101, fee 3
		// 3: amount 102, fee 2
		// 4: amount 103, fee 1

	}
	// Check balance
	currentBal := input.BankKeeper.GetBalance(ctx, mySender, myTokenDenom).Amount.Uint64()
	require.Equal(t, currentBal, originalBal-feesAndAmounts)

	// Check that removing a transaction refunds the costs and the tx no longer exists in the pool
	checkRemovedTx(t, input, ctx, ids[2], fees[2], amounts[2], &feesAndAmounts, originalBal, mySender, myTokenContractAddr, myTokenDenom)
	checkRemovedTx(t, input, ctx, ids[3], fees[3], amounts[3], &feesAndAmounts, originalBal, mySender, myTokenContractAddr, myTokenDenom)
	checkRemovedTx(t, input, ctx, ids[1], fees[1], amounts[1], &feesAndAmounts, originalBal, mySender, myTokenContractAddr, myTokenDenom)
	checkRemovedTx(t, input, ctx, ids[0], fees[0], amounts[0], &feesAndAmounts, originalBal, mySender, myTokenContractAddr, myTokenDenom)
	require.Empty(t, input.GravityKeeper.GetUnbatchedTransactions(ctx))
}

// Helper method to:
// 1. Remove the transaction specified by `id`, `myTokenContractAddr` and `fee`
// 2. Update the feesAndAmounts tracker by subtracting the refunded `fee` and `amount`
// 3. Require that `mySender` has been refunded the correct amount for the cancelled transaction
// 4. Require that the unbatched transaction pool does not contain the refunded transaction via iterating its elements
func checkRemovedTx(t *testing.T, input TestInput, ctx sdk.Context, id uint64, fee uint64, amount uint64,
	feesAndAmounts *uint64, originalBal uint64, mySender sdk.AccAddress, myTokenContractAddr string, myTokenDenom string) {
	err := input.GravityKeeper.RemoveFromOutgoingPoolAndRefund(ctx, id, mySender)
	require.NoError(t, err)
	*feesAndAmounts -= fee + amount // user should have regained the locked amounts from tx
	currentBal := input.BankKeeper.GetBalance(ctx, mySender, myTokenDenom).Amount.Uint64()
	require.Equal(t, currentBal, originalBal-*feesAndAmounts)
	expectedKey := myTokenContractAddr + fmt.Sprint(fee) + fmt.Sprint(id)
	input.GravityKeeper.IterateUnbatchedTransactions(ctx, types.OutgoingTXPoolKey, func(key []byte, tx *types.OutgoingTransferTx) bool {
		require.NotEqual(t, []byte(expectedKey), key)
		found := id == tx.Id &&
			fee == tx.Erc20Fee.Amount.Uint64() &&
			amount == tx.Erc20Token.Amount.Uint64()
		require.False(t, found)
		return false
	})
}

// ======================== Edge case tests for RemoveFromOutgoingPoolAndRefund() =================================== //

// Checks some common edge cases like invalid inputs, user didn't submit the transaction, tx doesn't exist, inconsistent entry
func TestRefundInconsistentTx(t *testing.T) {
	input := CreateTestEnv(t)
	ctx := input.Context
	var (
		mySender, _         = sdk.AccAddressFromBech32("cosmos1ahx7f8wyertuus9r20284ej0asrs085case3kn")
		myReceiver          = "0xd041c41EA1bf0F006ADBb6d2c9ef9D425dE5eaD7"
		myTokenContractAddr = "0x429881672B9AE42b8EbA0E26cD9C73711b891Ca5"
	)

	//////// Refund an inconsistent tx ////////
	amount := types.NewERC20Token(uint64(100), myTokenContractAddr)
	badTokenContractAddr := "0x429881672B9AE42b8EbA0E26cD9C73711b891Ca6" // different last char
	badFee := types.NewERC20Token(2, badTokenContractAddr)

	// This way should fail
	r, err := input.GravityKeeper.AddToOutgoingPool(ctx, mySender, myReceiver, amount.GravityCoin(), badFee.GravityCoin())
	require.Zero(t, r)
	require.Error(t, err)
	// But this unsafe override won't fail
	err = input.GravityKeeper.addUnbatchedTX(ctx, &types.OutgoingTransferTx{
		Id:          uint64(5),
		Sender:      mySender.String(),
		DestAddress: myReceiver,
		Erc20Token:  amount,
		Erc20Fee:    badFee,
	})
	origBalances := input.BankKeeper.GetAllBalances(ctx, mySender)
	require.NoError(t, err, "someone added validation to addUnbatchedTx")
	err = input.GravityKeeper.RemoveFromOutgoingPoolAndRefund(ctx, uint64(5), mySender)
	require.Error(t, err)
	newBalances := input.BankKeeper.GetAllBalances(ctx, mySender)
	require.Equal(t, origBalances, newBalances)
}

func TestRefundNonexistentTx(t *testing.T) {
	input := CreateTestEnv(t)
	ctx := input.Context
	var (
		mySender, _ = sdk.AccAddressFromBech32("cosmos1ahx7f8wyertuus9r20284ej0asrs085case3kn")
	)

	//////// Refund a tx which never existed ////////
	origBalances := input.BankKeeper.GetAllBalances(ctx, mySender)
	err := input.GravityKeeper.RemoveFromOutgoingPoolAndRefund(ctx, uint64(1), mySender)
	require.Error(t, err)
	newBalances := input.BankKeeper.GetAllBalances(ctx, mySender)
	require.Equal(t, origBalances, newBalances)
}

func TestRefundTwice(t *testing.T) {
	input := CreateTestEnv(t)
	ctx := input.Context
	var (
		mySender, _         = sdk.AccAddressFromBech32("cosmos1ahx7f8wyertuus9r20284ej0asrs085case3kn")
		myReceiver          = "0xd041c41EA1bf0F006ADBb6d2c9ef9D425dE5eaD7"
		myTokenContractAddr = "0x429881672B9AE42b8EbA0E26cD9C73711b891Ca5"
	)

	//////// Refund a tx twice ////////

	// mint some voucher first
	originalBal := uint64(99999)
	allVouchers := sdk.Coins{types.NewERC20Token(originalBal, myTokenContractAddr).GravityCoin()}
	err := input.BankKeeper.MintCoins(ctx, types.ModuleName, allVouchers)
	require.NoError(t, err)

	// set senders balance
	input.AccountKeeper.NewAccountWithAddress(ctx, mySender)
	err = input.BankKeeper.SetBalances(ctx, mySender, allVouchers)
	require.NoError(t, err)

	amount := types.NewERC20Token(uint64(100), myTokenContractAddr)
	fee := types.NewERC20Token(2, myTokenContractAddr)
	origBalances := input.BankKeeper.GetAllBalances(ctx, mySender)

	txId, err := input.GravityKeeper.AddToOutgoingPool(ctx, mySender, myReceiver, amount.GravityCoin(), fee.GravityCoin())
	require.NoError(t, err)
	afterAddBalances := input.BankKeeper.GetAllBalances(ctx, mySender)

	// First refund goes through
	err = input.GravityKeeper.RemoveFromOutgoingPoolAndRefund(ctx, txId, mySender)
	require.NoError(t, err)
	afterRefundBalances := input.BankKeeper.GetAllBalances(ctx, mySender)

	// Second fails
	err = input.GravityKeeper.RemoveFromOutgoingPoolAndRefund(ctx, txId, mySender)
	require.Error(t, err)
	afterSecondRefundBalances := input.BankKeeper.GetAllBalances(ctx, mySender)

	require.NotEqual(t, origBalances, afterAddBalances)
	require.Equal(t, origBalances, afterRefundBalances)
	require.Equal(t, origBalances, afterSecondRefundBalances)
}

// Check the various getter methods for the pool
func TestGetUnbatchedTransactions(t *testing.T) {
	input := CreateTestEnv(t)
	ctx := input.Context

	// token1
	var (
		mySender1, _                        = sdk.AccAddressFromBech32("cosmos1ahx7f8wyertuus9r20284ej0asrs085case3kn")
		mySender2            sdk.AccAddress = []byte("cosmos1ahx7f8wyertus")
		myReceiver                          = "0xd041c41EA1bf0F006ADBb6d2c9ef9D425dE5eaD7"
		myTokenContractAddr1                = "0x429881672B9AE42b8EbA0E26cD9C73711b891Ca5"
		myTokenContractAddr2                = "0x429881672B9AE42b8EbA0E26cD9C73711b891Ca6"
	)
	// mint some vouchers first
	allVouchers1 := sdk.Coins{types.NewERC20Token(99999, myTokenContractAddr1).GravityCoin()}
	allVouchers2 := sdk.Coins{types.NewERC20Token(99999, myTokenContractAddr2).GravityCoin()}
	err := input.BankKeeper.MintCoins(ctx, types.ModuleName, allVouchers1)
	require.NoError(t, err)
	err = input.BankKeeper.MintCoins(ctx, types.ModuleName, allVouchers2)
	require.NoError(t, err)

	// set senders balance
	input.AccountKeeper.NewAccountWithAddress(ctx, mySender1)
	err = input.BankKeeper.SetBalances(ctx, mySender1, allVouchers1)
	require.NoError(t, err)
	input.AccountKeeper.NewAccountWithAddress(ctx, mySender2)
	err = input.BankKeeper.SetBalances(ctx, mySender2, allVouchers2)
	require.NoError(t, err)

	ids1 := make([]uint64, 4)
	ids2 := make([]uint64, 4)
	fees := []uint64{2, 3, 2, 1}
	amounts := []uint64{100, 101, 102, 103}
	idToTxMap := make(map[uint64]*types.OutgoingTransferTx)
	for i, v := range fees {
		amount1 := types.NewERC20Token(amounts[i], myTokenContractAddr1)
		fee1 := types.NewERC20Token(v, myTokenContractAddr1)
		r, err := input.GravityKeeper.AddToOutgoingPool(ctx, mySender1, myReceiver, amount1.GravityCoin(), fee1.GravityCoin())
		require.NoError(t, err)
		ids1[i] = r
		idToTxMap[r] = &types.OutgoingTransferTx{
			Id:          r,
			Sender:      mySender1.String(),
			DestAddress: myReceiver,
			Erc20Token:  amount1,
			Erc20Fee:    fee1,
		}
		amount2 := types.NewERC20Token(amounts[i], myTokenContractAddr2)
		fee2 := types.NewERC20Token(v, myTokenContractAddr2)
		r, err = input.GravityKeeper.AddToOutgoingPool(ctx, mySender2, myReceiver, amount2.GravityCoin(), fee2.GravityCoin())
		require.NoError(t, err)
		ids2[i] = r
		idToTxMap[r] = &types.OutgoingTransferTx{
			Id:          r,
			Sender:      mySender2.String(),
			DestAddress: myReceiver,
			Erc20Token:  amount2,
			Erc20Fee:    fee2,
		}
	}

	// GetUnbatchedTxByFeeAndId
	token1Fee := types.NewERC20Token(fees[0], myTokenContractAddr1)
	token1Amount := types.NewERC20Token(amounts[0], myTokenContractAddr1)
	token1Id := ids1[0]
	tx1, err1 := input.GravityKeeper.GetUnbatchedTxByFeeAndId(ctx, *token1Fee, token1Id)
	require.NoError(t, err1)
	expTx1 := types.OutgoingTransferTx{
		Id:          token1Id,
		Sender:      mySender1.String(),
		DestAddress: myReceiver,
		Erc20Token:  token1Amount,
		Erc20Fee:    token1Fee,
	}
	require.Equal(t, expTx1, *tx1)

	token2Fee := types.NewERC20Token(fees[3], myTokenContractAddr2)
	token2Amount := types.NewERC20Token(amounts[3], myTokenContractAddr2)
	token2Id := ids2[3]
	tx2, err2 := input.GravityKeeper.GetUnbatchedTxByFeeAndId(ctx, *token2Fee, token2Id)
	require.NoError(t, err2)
	expTx2 := types.OutgoingTransferTx{
		Id:          token2Id,
		Sender:      mySender2.String(),
		DestAddress: myReceiver,
		Erc20Token:  token2Amount,
		Erc20Fee:    token2Fee,
	}
	require.Equal(t, expTx2, *tx2)

	// GetUnbatchedTxById
	tx1, err1 = input.GravityKeeper.GetUnbatchedTxById(ctx, token1Id)
	require.NoError(t, err1)
	require.Equal(t, expTx1, *tx1)

	tx2, err2 = input.GravityKeeper.GetUnbatchedTxById(ctx, token2Id)
	require.NoError(t, err2)
	require.Equal(t, expTx2, *tx2)

	// GetUnbatchedTransactionsByContract
	token1Txs := input.GravityKeeper.GetUnbatchedTransactionsByContract(ctx, myTokenContractAddr1)
	for _, v := range token1Txs {
		expTx := idToTxMap[v.Id]
		require.NotNil(t, expTx)
		require.Equal(t, myTokenContractAddr1, v.Erc20Fee.Contract)
		require.Equal(t, myTokenContractAddr1, v.Erc20Token.Contract)
		require.Equal(t, *v, *expTx)
	}
	token2Txs := input.GravityKeeper.GetUnbatchedTransactionsByContract(ctx, myTokenContractAddr2)
	for _, v := range token2Txs {
		expTx := idToTxMap[v.Id]
		require.NotNil(t, expTx)
		require.Equal(t, myTokenContractAddr2, v.Erc20Fee.Contract)
		require.Equal(t, myTokenContractAddr2, v.Erc20Token.Contract)
		require.Equal(t, *v, *expTx)
	}
	// GetUnbatchedTransactions
	allTxs := input.GravityKeeper.GetUnbatchedTransactions(ctx)
	for _, v := range allTxs {
		expTx := idToTxMap[v.Id]
		require.NotNil(t, expTx)
		require.Equal(t, *v, *expTx)
	}
}

// Check the various iteration methods for the pool
func TestIterateUnbatchedTransactions(t *testing.T) {
	input := CreateTestEnv(t)
	ctx := input.Context

	// token1
	var (
		mySender1, _                        = sdk.AccAddressFromBech32("cosmos1ahx7f8wyertuus9r20284ej0asrs085case3kn")
		mySender2            sdk.AccAddress = []byte("cosmos1ahx7f8wyertus")
		myReceiver                          = "0xd041c41EA1bf0F006ADBb6d2c9ef9D425dE5eaD7"
		myTokenContractAddr1                = "0x429881672B9AE42b8EbA0E26cD9C73711b891Ca5"
		myTokenContractAddr2                = "0x429881672B9AE42b8EbA0E26cD9C73711b891Ca6"
	)
	// mint some vouchers first
	allVouchers1 := sdk.Coins{types.NewERC20Token(99999, myTokenContractAddr1).GravityCoin()}
	allVouchers2 := sdk.Coins{types.NewERC20Token(99999, myTokenContractAddr2).GravityCoin()}
	err := input.BankKeeper.MintCoins(ctx, types.ModuleName, allVouchers1)
	require.NoError(t, err)
	err = input.BankKeeper.MintCoins(ctx, types.ModuleName, allVouchers2)
	require.NoError(t, err)

	// set senders balance
	input.AccountKeeper.NewAccountWithAddress(ctx, mySender1)
	err = input.BankKeeper.SetBalances(ctx, mySender1, allVouchers1)
	require.NoError(t, err)
	input.AccountKeeper.NewAccountWithAddress(ctx, mySender2)
	err = input.BankKeeper.SetBalances(ctx, mySender2, allVouchers2)
	require.NoError(t, err)

	ids1 := make([]uint64, 4)
	ids2 := make([]uint64, 4)
	fees := []uint64{2, 3, 2, 1}
	amounts := []uint64{100, 101, 102, 103}
	idToTxMap := make(map[uint64]*types.OutgoingTransferTx)
	for i, v := range fees {
		amount1 := types.NewERC20Token(amounts[i], myTokenContractAddr1)
		fee1 := types.NewERC20Token(v, myTokenContractAddr1)
		r, err := input.GravityKeeper.AddToOutgoingPool(ctx, mySender1, myReceiver, amount1.GravityCoin(), fee1.GravityCoin())
		require.NoError(t, err)
		ids1[i] = r
		idToTxMap[r] = &types.OutgoingTransferTx{
			Id:          r,
			Sender:      mySender1.String(),
			DestAddress: myReceiver,
			Erc20Token:  amount1,
			Erc20Fee:    fee1,
		}
		amount2 := types.NewERC20Token(amounts[i], myTokenContractAddr2)
		fee2 := types.NewERC20Token(v, myTokenContractAddr2)
		r, err = input.GravityKeeper.AddToOutgoingPool(ctx, mySender2, myReceiver, amount2.GravityCoin(), fee2.GravityCoin())
		require.NoError(t, err)
		ids2[i] = r
		idToTxMap[r] = &types.OutgoingTransferTx{
			Id:          r,
			Sender:      mySender2.String(),
			DestAddress: myReceiver,
			Erc20Token:  amount2,
			Erc20Fee:    fee2,
		}
	}
	// IterateUnbatchedTransactionsByContract
	foundMap := make(map[uint64]bool)
	input.GravityKeeper.IterateUnbatchedTransactionsByContract(ctx, myTokenContractAddr1, func(key []byte, tx *types.OutgoingTransferTx) bool {
		require.NotNil(t, tx)
		fTx := idToTxMap[tx.Id]
		require.NotNil(t, fTx)
		require.Equal(t, fTx.Erc20Fee.Contract, myTokenContractAddr1)
		require.Equal(t, fTx.Erc20Token.Contract, myTokenContractAddr1)
		require.Equal(t, *fTx, *tx)
		foundMap[fTx.Id] = true
		return false
	})
	input.GravityKeeper.IterateUnbatchedTransactionsByContract(ctx, myTokenContractAddr2, func(key []byte, tx *types.OutgoingTransferTx) bool {
		require.NotNil(t, tx)
		fTx := idToTxMap[tx.Id]
		require.NotNil(t, fTx)
		require.Equal(t, fTx.Erc20Fee.Contract, myTokenContractAddr2)
		require.Equal(t, fTx.Erc20Token.Contract, myTokenContractAddr2)
		require.Equal(t, *fTx, *tx)
		foundMap[fTx.Id] = true
		return false
	})

	for i := 1; i <= 8; i++ {
		require.True(t, foundMap[uint64(i)])
	}
	// IterateUnbatchedTransactions
	anotherFoundMap := make(map[uint64]bool)
	input.GravityKeeper.IterateUnbatchedTransactions(ctx, types.OutgoingTXPoolKey, func(key []byte, tx *types.OutgoingTransferTx) bool {
		require.NotNil(t, tx)
		fTx := idToTxMap[tx.Id]
		require.NotNil(t, fTx)
		require.Equal(t, *fTx, *tx)
		anotherFoundMap[fTx.Id] = true
		return false
	})

	for i := 1; i <= 8; i++ {
		require.True(t, anotherFoundMap[uint64(i)])
	}
}

// Ensures that any unbatched tx will make its way into the exported data from ExportGenesis
func TestAddToOutgoingPoolExportGenesis(t *testing.T) {
	input := CreateTestEnv(t)
	ctx := input.Context
	k := input.GravityKeeper
	var (
		mySender, _         = sdk.AccAddressFromBech32("cosmos1ahx7f8wyertuus9r20284ej0asrs085case3kn")
		myReceiver          = "0xd041c41EA1bf0F006ADBb6d2c9ef9D425dE5eaD7"
		myTokenContractAddr = "0x429881672B9AE42b8EbA0E26cD9C73711b891Ca5"
	)
	// mint some voucher first
	allVouchers := sdk.Coins{types.NewERC20Token(99999, myTokenContractAddr).GravityCoin()}
	err := input.BankKeeper.MintCoins(ctx, types.ModuleName, allVouchers)
	require.NoError(t, err)

	// set senders balance
	input.AccountKeeper.NewAccountWithAddress(ctx, mySender)
	err = input.BankKeeper.SetBalances(ctx, mySender, allVouchers)
	require.NoError(t, err)

	unbatchedTxMap := make(map[uint64]types.OutgoingTransferTx)
	foundTxsMap := make(map[uint64]bool)
	// when
	for i, v := range []uint64{2, 3, 2, 1} {
		amount := types.NewERC20Token(uint64(i+100), myTokenContractAddr)
		fee := types.NewERC20Token(v, myTokenContractAddr)
		r, err := input.GravityKeeper.AddToOutgoingPool(ctx, mySender, myReceiver, amount.GravityCoin(), fee.GravityCoin())
		require.NoError(t, err)

		unbatchedTxMap[r] = types.OutgoingTransferTx{
			Id:          r,
			Sender:      mySender.String(),
			DestAddress: myReceiver,
			Erc20Token:  amount,
			Erc20Fee:    fee,
		}
		foundTxsMap[r] = false

	}
	// then
	got := ExportGenesis(ctx, k)
	require.NotNil(t, got)

	for _, tx := range got.UnbatchedTransfers {
		cached := unbatchedTxMap[tx.Id]
		require.NotNil(t, cached)
		require.Equal(t, cached, *tx, "cached: %+v\nactual: %+v\n", cached, *tx)
		foundTxsMap[tx.Id] = true
	}

	for _, v := range foundTxsMap {
		require.True(t, v)
	}
}
