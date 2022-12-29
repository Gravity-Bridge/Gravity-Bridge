package keeper

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/crypto/tmhash"

	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdktypes "github.com/cosmos/cosmos-sdk/types"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
)

func TestGetAndDeleteAttestation(t *testing.T) {
	input := CreateTestEnv(t)
	k := input.GravityKeeper
	ctx := input.Context

	length := 10
	for _, evmChain := range k.GetEvmChains(ctx) {
		_, _, hashes := createAttestations(t, ctx, k, length, evmChain.EvmChainPrefix)

		// Get created attestations
		for i := 0; i < length; i++ {
			nonce := uint64(1 + i)
			att := k.GetAttestation(ctx, evmChain.EvmChainPrefix, nonce, hashes[i])
			require.NotNil(t, att)
		}
		recentAttestations := k.GetMostRecentAttestations(ctx, evmChain.EvmChainPrefix, uint64(length))
		require.True(t, len(recentAttestations) == length)

		// Delete last 3 attestations
		var nilAtt *types.Attestation
		for i := 7; i < length; i++ {
			nonce := uint64(1 + i)
			att := k.GetAttestation(ctx, evmChain.EvmChainPrefix, nonce, hashes[i])
			k.DeleteAttestation(ctx, *att)

			att = k.GetAttestation(ctx, evmChain.EvmChainPrefix, nonce, hashes[i])
			require.Equal(t, nilAtt, att)
		}
		recentAttestations = k.GetMostRecentAttestations(ctx, evmChain.EvmChainPrefix, uint64(10))
		require.True(t, len(recentAttestations) == 7)

		// Check all attestations again
		for i := 0; i < 7; i++ {
			nonce := uint64(1 + i)
			att := k.GetAttestation(ctx, evmChain.EvmChainPrefix, nonce, hashes[i])
			require.NotNil(t, att)
		}
		for i := 7; i < length; i++ {
			nonce := uint64(1 + i)
			att := k.GetAttestation(ctx, evmChain.EvmChainPrefix, nonce, hashes[i])
			require.Equal(t, nilAtt, att)
		}
	}
}

// Sets up 10 attestations and checks that they are returned in the correct order
func TestGetMostRecentAttestations(t *testing.T) {
	input := CreateTestEnv(t)
	defer func() { input.Context.Logger().Info("Asserting invariants at test end"); input.AssertInvariants() }()

	k := input.GravityKeeper
	ctx := input.Context

	length := 10
	for _, evmChain := range k.GetEvmChains(ctx) {
		msgs, anys, _ := createAttestations(t, ctx, k, length, evmChain.EvmChainPrefix)

		recentAttestations := k.GetMostRecentAttestations(ctx, evmChain.EvmChainPrefix, uint64(length))
		require.True(t, len(recentAttestations) == length,
			"recentAttestations should have len %v but instead has %v", length, len(recentAttestations))
		for n, attest := range recentAttestations {
			require.Equal(t, attest.Claim.GetCachedValue(), anys[n].GetCachedValue(),
				"The %vth claim does not match our message: claim %v\n message %v", n, attest.Claim, msgs[n])
		}
	}
}

func createAttestations(t *testing.T, ctx sdktypes.Context, k Keeper, length int, evmChainPrefix string) ([]types.MsgSendToCosmosClaim, []codectypes.Any, [][]byte) {
	msgs := make([]types.MsgSendToCosmosClaim, 0, length)
	anys := make([]codectypes.Any, 0, length)
	hashes := make([][]byte, 0, length)
	for i := 0; i < length; i++ {
		nonce := uint64(1 + i)

		contract := common.BytesToAddress(bytes.Repeat([]byte{0x1}, 20)).String()
		sender := common.BytesToAddress(bytes.Repeat([]byte{0x2}, 20)).String()
		orch := sdktypes.AccAddress(bytes.Repeat([]byte{0x3}, 20)).String()
		receiver := sdktypes.AccAddress(bytes.Repeat([]byte{0x4}, 20)).String()
		msg := types.MsgSendToCosmosClaim{
			EventNonce:     nonce,
			EthBlockHeight: 1,
			TokenContract:  contract,
			Amount:         sdktypes.NewInt(10000000000 + int64(i)),
			EthereumSender: sender,
			CosmosReceiver: receiver,
			Orchestrator:   orch,
			EvmChainPrefix: evmChainPrefix,
		}
		msgs = append(msgs, msg)

		any, _ := codectypes.NewAnyWithValue(&msg)
		anys = append(anys, *any)
		att := &types.Attestation{
			Observed: false,
			Height:   uint64(ctx.BlockHeight()),
			Claim:    any,
		}
		unpackedClaim, err := k.UnpackAttestationClaim(att)
		if err != nil {
			panic(fmt.Sprintf("Bad new attestation: %s", err.Error()))
		}
		err = unpackedClaim.ValidateBasic()
		if err != nil {
			panic(fmt.Sprintf("Bad claim discovered: %s", err.Error()))
		}
		hash, err := msg.ClaimHash()
		hashes = append(hashes, hash)
		require.NoError(t, err)
		k.SetAttestation(ctx, evmChainPrefix, nonce, hash, att)
	}

	return msgs, anys, hashes
}

func TestGetSetLastObservedEvmChainBlockHeight(t *testing.T) {
	input := CreateTestEnv(t)
	k := input.GravityKeeper
	ctx := input.Context

	ethereumHeight := uint64(7654321)

	for _, evmChain := range k.GetEvmChains(ctx) {
		require.NotPanics(t, func() { k.SetLastObservedEvmChainBlockHeight(ctx, evmChain.EvmChainPrefix, ethereumHeight) })

		ethHeight := k.GetLastObservedEvmChainBlockHeight(ctx, evmChain.EvmChainPrefix)
		require.Equal(t, uint64(ctx.BlockHeight()), ethHeight.CosmosBlockHeight)
		require.Equal(t, ethereumHeight, ethHeight.EthereumBlockHeight)
	}
}

func TestGetSetLastObservedValset(t *testing.T) {
	input := CreateTestEnv(t)
	k := input.GravityKeeper
	ctx := input.Context

	setValset := types.Valset{
		Nonce:  1,
		Height: 1,
		Members: []types.BridgeValidator{
			{
				Power:           999999999,
				EthereumAddress: "0x0000000000000001",
			},
			{
				Power:           999999999,
				EthereumAddress: "0x0000000000000002",
			},
			{
				Power:           999999999,
				EthereumAddress: "0x0000000000000003",
			},
		},
		RewardAmount: sdktypes.NewInt(1000000000),
		RewardToken:  "footoken",
	}

	for _, evmChain := range k.GetEvmChains(ctx) {
		require.NotPanics(t, func() { k.SetLastObservedValset(ctx, evmChain.EvmChainPrefix, setValset) })

		getValset := k.GetLastObservedValset(ctx, evmChain.EvmChainPrefix)
		require.EqualValues(t, setValset, *getValset)
	}
}

func TestGetSetLastEventNonceByValidator(t *testing.T) {
	input := CreateTestEnv(t)
	k := input.GravityKeeper
	ctx := input.Context

	valAddrString := "gravity1ahx7f8wyertuus9r20284ej0asrs085ceqtfnm"
	valAccAddress, _ := sdktypes.AccAddressFromBech32(valAddrString)
	valAccount := k.accountKeeper.NewAccountWithAddress(ctx, valAccAddress)
	require.NotNil(t, valAccount)

	nonce := uint64(1234)
	addrInBytes := valAccount.GetAddress().Bytes()

	for _, evmChain := range k.GetEvmChains(ctx) {
		// In case this is first time validator is submiting claim, nonce is expected to be LastObservedNonce-1
		k.setLastObservedEventNonce(ctx, evmChain.EvmChainPrefix, nonce)
		getEventNonce := k.GetLastEventNonceByValidator(ctx, evmChain.EvmChainPrefix, addrInBytes)
		require.Equal(t, nonce-1, getEventNonce)

		require.NotPanics(t, func() { k.SetLastEventNonceByValidator(ctx, evmChain.EvmChainPrefix, addrInBytes, nonce) })

		getEventNonce = k.GetLastEventNonceByValidator(ctx, evmChain.EvmChainPrefix, addrInBytes)
		require.Equal(t, nonce, getEventNonce)
	}
}

func TestInvalidHeight(t *testing.T) {
	input, ctx := SetupFiveValChain(t)
	defer func() { input.Context.Logger().Info("Asserting invariants at test end"); input.AssertInvariants() }()
	pk := input.GravityKeeper
	msgServer := NewMsgServerImpl(pk)
	log := ctx.Logger()
	evmChain := pk.GetEvmChainData(ctx, EthChainPrefix)

	val0 := ValAddrs[0]
	orch0 := OrchAddrs[0]
	sender := AccAddrs[0]
	receiver := EthAddrs[0]
	lastNonce := pk.GetLastObservedEventNonce(ctx, evmChain.EvmChainPrefix)
	lastEthHeight := pk.GetLastObservedEvmChainBlockHeight(ctx, evmChain.EvmChainPrefix).EthereumBlockHeight
	lastBatchNonce := 0
	tokenContract := "0x2260FAC5E5542a773Aa44fBCfeDf7C193bc2C599"
	goodHeight := lastEthHeight + 1
	batchTimeout := lastEthHeight + 100
	badHeight := batchTimeout

	// Setup a batch with a timeout
	batch := types.OutgoingTxBatch{
		BatchNonce:   uint64(lastBatchNonce + 1),
		BatchTimeout: batchTimeout,
		Transactions: []types.OutgoingTransferTx{{
			Id:          0,
			Sender:      sender.String(),
			DestAddress: receiver.String(),
			Erc20Token: types.ERC20Token{
				Contract: tokenContract,
				Amount:   sdktypes.NewInt(1),
			},
			Erc20Fee: types.ERC20Token{
				Contract: tokenContract,
				Amount:   sdktypes.NewInt(1),
			},
		}},
		TokenContract:      tokenContract,
		CosmosBlockCreated: 0,
	}
	b, err := batch.ToInternal()
	require.NoError(t, err)
	pk.StoreBatch(ctx, evmChain.EvmChainPrefix, *b)

	// Submit a bad claim with BlockHeight >= timeout

	bad := types.MsgBatchSendToEthClaim{
		EventNonce:     lastNonce + 1,
		EthBlockHeight: badHeight,
		BatchNonce:     uint64(lastBatchNonce + 1),
		TokenContract:  tokenContract,
		Orchestrator:   orch0.String(),
		EvmChainPrefix: evmChain.EvmChainPrefix,
	}
	err = bad.ValidateBasic()
	require.NoError(t, err)

	context := sdktypes.WrapSDKContext(ctx)
	log.Info("Submitting bad eth claim from orchestrator 0", "orch", orch0.String(), "val", val0.String())

	// BatchSendToEthClaim is supposed to panic and fail the message execution, set up a defer recover to catch it
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("recovered from panic:", r)
		} else {
			panic("Expected to find a panic coming from BatchSendToEthClaim()!")
		}
	}()
	_, err = msgServer.BatchSendToEthClaim(context, &bad)
	require.NoError(t, err)

	// Assert that there is no attestation since the above panicked
	badHash, err := bad.ClaimHash()
	require.NoError(t, err)
	att := pk.GetAttestation(ctx, EthChainPrefix, bad.GetEventNonce(), badHash)
	require.Nil(t, att)

	// Attest the actual batch, and assert the votes are correct
	for i, orch := range OrchAddrs[1:] {
		log.Info("Submitting good eth claim from orchestrators", "orch", orch.String())
		good := types.MsgBatchSendToEthClaim{
			EventNonce:     lastNonce + 1,
			EthBlockHeight: goodHeight,
			BatchNonce:     uint64(lastBatchNonce + 1),
			TokenContract:  tokenContract,
			Orchestrator:   orch.String(),
			EvmChainPrefix: evmChain.EvmChainPrefix,
		}
		_, err := msgServer.BatchSendToEthClaim(context, &good)
		require.NoError(t, err)

		goodHash, err := good.ClaimHash()
		require.NoError(t, err)
		require.Equal(t, badHash, goodHash) // The hash should be the same, even though that's wrong

		att := pk.GetAttestation(ctx, EthChainPrefix, good.GetEventNonce(), goodHash)
		require.NotNil(t, att)
		log.Info("Asserting that the bad attestation only has one claimer", "attVotes", att.Votes)
		require.Equal(t, len(att.Votes), i+1) // Only these good orchestrators votes should be counted
	}

}

func TestDiffAttestationsWithDiffEvmChainPrefixClaimHash(t *testing.T) {
	input := CreateTestEnv(t)
	k := input.GravityKeeper
	ctx := input.Context

	ethEvmPrefix := "ethereum"
	bscEvmPrefix := "bsc"

	nonce := uint64(1)
	evmPrefixMsg := types.MsgSendToCosmosClaim{
		EventNonce:     nonce,
		EthBlockHeight: 1,
		TokenContract:  "0x00000000000000000001",
		Amount:         sdktypes.NewInt(10000000000 + int64(1)),
		EthereumSender: "0x00000000000000000002",
		CosmosReceiver: "0x00000000000000000003",
		Orchestrator:   "0x00000000000000000004",
		EvmChainPrefix: ethEvmPrefix,
	}

	bscPrefixMsg := evmPrefixMsg
	bscPrefixMsg.EvmChainPrefix = bscEvmPrefix

	any, err := codectypes.NewAnyWithValue(&evmPrefixMsg)
	require.NoError(t, err)

	att := &types.Attestation{
		Observed: false,
		Height:   uint64(ctx.BlockHeight()),
		Claim:    any,
	}

	hash, err := evmPrefixMsg.ClaimHash()
	require.NoError(t, err)
	require.Equal(t, tmhash.Sum([]byte(fmt.Sprintf("%s/%d/%d/%s/%s/%s/%s", evmPrefixMsg.EvmChainPrefix, evmPrefixMsg.EventNonce, evmPrefixMsg.EthBlockHeight, evmPrefixMsg.TokenContract, evmPrefixMsg.Amount.String(), evmPrefixMsg.EthereumSender, evmPrefixMsg.CosmosReceiver))), hash)

	k.SetAttestation(ctx, ethEvmPrefix, nonce, hash, att)

	any, err = codectypes.NewAnyWithValue(&bscPrefixMsg)
	require.NoError(t, err)

	att.Claim = any

	bscHash, err := bscPrefixMsg.ClaimHash()
	require.NoError(t, err)
	require.Equal(t, tmhash.Sum([]byte(fmt.Sprintf("%s/%d/%d/%s/%s/%s/%s", bscPrefixMsg.EvmChainPrefix, bscPrefixMsg.EventNonce, bscPrefixMsg.EthBlockHeight, bscPrefixMsg.TokenContract, bscPrefixMsg.Amount.String(), bscPrefixMsg.EthereumSender, bscPrefixMsg.CosmosReceiver))), bscHash)

	k.SetAttestation(ctx, ethEvmPrefix, nonce, bscHash, att)

	// when we get attestation, two hashes should return two different attestations
	evmAtt := k.GetAttestation(ctx, ethEvmPrefix, nonce, hash)
	bscAtt := k.GetAttestation(ctx, ethEvmPrefix, nonce, bscHash)

	require.NotEqual(t, evmAtt, bscAtt)
}
