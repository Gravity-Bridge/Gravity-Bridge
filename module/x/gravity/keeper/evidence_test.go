package keeper

import (
	"encoding/hex"
	"testing"
	"time"

	"bytes"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
)

// nolint: exhaustruct
func TestSubmitBadSignatureEvidenceBatchExists(t *testing.T) {
	input := CreateTestEnv(t)
	defer func() { input.Context.Logger().Info("Asserting invariants at test end"); input.AssertInvariants() }()

	ctx := input.Context

	var (
		now                 = time.Now().UTC()
		mySender, _         = sdk.AccAddressFromBech32("gravity1ahx7f8wyertuus9r20284ej0asrs085ceqtfnm")
		myReceiver          = "0xd041c41EA1bf0F006ADBb6d2c9ef9D425dE5eaD7"
		myTokenContractAddr = "0x429881672B9AE42b8EbA0E26cD9C73711b891Ca5" // Pickle
		token, err          = types.NewInternalERC20Token(sdk.NewInt(99999), myTokenContractAddr)
		allVouchers         = sdk.NewCoins(token.GravityCoin(EthChainPrefix))
		evmChain            = input.GravityKeeper.GetEvmChainData(ctx, EthChainPrefix)
	)
	require.NoError(t, err)
	receiver, err := types.NewEthAddress(myReceiver)
	require.NoError(t, err)
	tokenContract, err := types.NewEthAddress(myTokenContractAddr)
	require.NoError(t, err)

	// mint some voucher first
	require.NoError(t, input.BankKeeper.MintCoins(ctx, types.ModuleName, allVouchers))
	// set senders balance
	input.AccountKeeper.NewAccountWithAddress(ctx, mySender)
	require.NoError(t, input.BankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, mySender, allVouchers))

	// CREATE BATCH

	// add some TX to the pool
	for i, v := range []uint64{2, 3, 2, 1} {
		amountToken, err := types.NewInternalERC20Token(sdk.NewInt(int64(i+100)), myTokenContractAddr)
		require.NoError(t, err)
		amount := amountToken.GravityCoin(evmChain.EvmChainPrefix)
		feeToken, err := types.NewInternalERC20Token(sdk.NewIntFromUint64(v), myTokenContractAddr)
		require.NoError(t, err)
		fee := feeToken.GravityCoin(evmChain.EvmChainPrefix)

		_, err = input.GravityKeeper.AddToOutgoingPool(ctx, evmChain.EvmChainPrefix, mySender, *receiver, amount, fee)
		require.NoError(t, err)
	}

	// when
	ctx = ctx.WithBlockTime(now)

	goodBatch, err := input.GravityKeeper.BuildOutgoingTxBatch(ctx, evmChain.EvmChainPrefix, *tokenContract, 2)
	goodBatchExternal := goodBatch.ToExternal()
	require.NoError(t, err)

	any, _ := codectypes.NewAnyWithValue(&goodBatchExternal)

	msg := types.MsgSubmitBadSignatureEvidence{
		Subject:        any,
		Signature:      "foo",
		EvmChainPrefix: evmChain.EvmChainPrefix,
	}

	err = input.GravityKeeper.CheckBadSignatureEvidence(ctx, &msg)
	require.EqualError(t, err, "Checkpoint exists, cannot slash: invalid")
}

// nolint: exhaustruct
func TestSubmitBadSignatureEvidenceValsetExists(t *testing.T) {
	// input := CreateTestEnv(t)
	input, ctx := SetupFiveValChain(t)
	evmChain := input.GravityKeeper.GetEvmChainData(ctx, EthChainPrefix)
	defer func() { input.Context.Logger().Info("Asserting invariants at test end"); input.AssertInvariants() }()

	// ctx := input.Context

	valset := input.GravityKeeper.SetValsetRequest(ctx, evmChain.EvmChainPrefix)

	any, _ := codectypes.NewAnyWithValue(&valset)

	msg := types.MsgSubmitBadSignatureEvidence{
		Subject:        any,
		Signature:      "foo",
		EvmChainPrefix: evmChain.EvmChainPrefix,
	}

	err := input.GravityKeeper.CheckBadSignatureEvidence(ctx, &msg)
	require.EqualError(t, err, "Checkpoint exists, cannot slash: invalid")
}

// nolint: exhaustruct
func TestSubmitBadSignatureEvidenceLogicCallExists(t *testing.T) {
	input := CreateTestEnv(t)
	defer func() { input.Context.Logger().Info("Asserting invariants at test end"); input.AssertInvariants() }()

	ctx := input.Context
	evmChain := input.GravityKeeper.GetEvmChainData(ctx, EthChainPrefix)

	contract := common.BytesToAddress(bytes.Repeat([]byte{0x1}, 20)).String()
	logicCall := types.OutgoingLogicCall{
		Transfers:            []types.ERC20Token{},
		Fees:                 []types.ERC20Token{},
		LogicContractAddress: contract,
		Payload:              []byte{},
		Timeout:              420,
		InvalidationId:       []byte{},
		InvalidationNonce:    0,
		CosmosBlockCreated:   0,
	}

	input.GravityKeeper.SetOutgoingLogicCall(ctx, evmChain.EvmChainPrefix, logicCall)

	any, _ := codectypes.NewAnyWithValue(&logicCall)

	msg := types.MsgSubmitBadSignatureEvidence{
		Subject:        any,
		Signature:      "foo",
		EvmChainPrefix: evmChain.EvmChainPrefix,
	}

	err := input.GravityKeeper.CheckBadSignatureEvidence(ctx, &msg)
	require.EqualError(t, err, "Checkpoint exists, cannot slash: invalid")
}

// nolint: exhaustruct
func TestSubmitBadSignatureEvidenceSlash(t *testing.T) {
	input, ctx := SetupFiveValChain(t)
	defer func() { input.Context.Logger().Info("Asserting invariants at test end"); input.AssertInvariants() }()

	evmChain := input.GravityKeeper.GetEvmChainData(ctx, EthChainPrefix)

	batch := types.OutgoingTxBatch{
		TokenContract: "0xd041c41EA1bf0F006ADBb6d2c9ef9D425dE5eaD7",
		BatchTimeout:  420,
	}

	checkpoint := batch.GetCheckpoint(input.GravityKeeper.GetGravityID(ctx))

	any, err := codectypes.NewAnyWithValue(&batch)
	require.NoError(t, err)

	privKey, err := crypto.GenerateKey()
	require.NoError(t, err)

	ethAddress, err := types.NewEthAddress(crypto.PubkeyToAddress(privKey.PublicKey).String())
	require.NoError(t, err)

	input.GravityKeeper.SetEvmAddressForValidator(ctx, ValAddrs[0], *ethAddress)

	ethSignature, err := types.NewEthereumSignature(checkpoint, privKey)
	require.NoError(t, err)

	msg := types.MsgSubmitBadSignatureEvidence{
		Subject:        any,
		Signature:      hex.EncodeToString(ethSignature),
		EvmChainPrefix: evmChain.EvmChainPrefix,
	}

	err = input.GravityKeeper.CheckBadSignatureEvidence(ctx, &msg)
	require.NoError(t, err)

	val := input.StakingKeeper.Validator(ctx, ValAddrs[0])
	require.True(t, val.IsJailed())
}
