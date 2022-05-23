package keeper

import (
	"testing"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdktypes "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
)

func TestGetAndDeleteAttestation(t *testing.T) {
	input := CreateTestEnv(t)
	k := input.GravityKeeper
	ctx := input.Context

	length := 10
	for _, cd := range k.GetEvmChains(ctx) {
		_, _, hashes := createAttestations(t, ctx, k, length, cd.EvmChainPrefix)

		// Get created attestations
		for i := 0; i < length; i++ {
			nonce := uint64(1 + i)
			att := k.GetAttestation(ctx, cd.EvmChainPrefix, nonce, hashes[i])
			require.NotNil(t, att)
		}
		recentAttestations := k.GetMostRecentAttestations(ctx, cd.EvmChainPrefix, uint64(length))
		require.True(t, len(recentAttestations) == length)

		// Delete last 3 attestations
		var nilAtt *types.Attestation
		for i := 7; i < length; i++ {
			nonce := uint64(1 + i)
			att := k.GetAttestation(ctx, cd.EvmChainPrefix, nonce, hashes[i])
			k.DeleteAttestation(ctx, cd.EvmChainPrefix, *att)

			att = k.GetAttestation(ctx, cd.EvmChainPrefix, nonce, hashes[i])
			require.Equal(t, nilAtt, att)
		}
		recentAttestations = k.GetMostRecentAttestations(ctx, cd.EvmChainPrefix, uint64(10))
		require.True(t, len(recentAttestations) == 7)

		// Check all attestations again
		for i := 0; i < 7; i++ {
			nonce := uint64(1 + i)
			att := k.GetAttestation(ctx, cd.EvmChainPrefix, nonce, hashes[i])
			require.NotNil(t, att)
		}
		for i := 7; i < length; i++ {
			nonce := uint64(1 + i)
			att := k.GetAttestation(ctx, cd.EvmChainPrefix, nonce, hashes[i])
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
	for _, cd := range k.GetEvmChains(ctx) {
		msgs, anys, _ := createAttestations(t, ctx, k, length, cd.EvmChainPrefix)

		recentAttestations := k.GetMostRecentAttestations(ctx, cd.EvmChainPrefix, uint64(length))
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
		msg := types.MsgSendToCosmosClaim{
			EventNonce:     nonce,
			BlockHeight:    1,
			TokenContract:  "0x00000000000000000001",
			Amount:         sdktypes.NewInt(10000000000 + int64(i)),
			EthereumSender: "0x00000000000000000002",
			CosmosReceiver: "0x00000000000000000003",
			Orchestrator:   "0x00000000000000000004",
		}
		msgs = append(msgs, msg)

		any, _ := codectypes.NewAnyWithValue(&msg)
		anys = append(anys, *any)
		att := &types.Attestation{
			Observed: false,
			Height:   uint64(ctx.BlockHeight()),
			Claim:    any,
		}
		hash, err := msg.ClaimHash()
		hashes = append(hashes, hash)
		require.NoError(t, err)
		k.SetAttestation(ctx, evmChainPrefix, nonce, hash, att)
	}

	return msgs, anys, hashes
}

func TestGetSetLastObservedEthereumBlockHeight(t *testing.T) {
	input := CreateTestEnv(t)
	k := input.GravityKeeper
	ctx := input.Context

	ethereumHeight := uint64(7654321)

	for _, cd := range k.GetEvmChains(ctx) {
		require.NotPanics(t, func() { k.SetLastObservedEvmChainBlockHeight(ctx, cd.EvmChainPrefix, ethereumHeight) })

		ethHeight := k.GetLastObservedEvmChainBlockHeight(ctx, cd.EvmChainPrefix)
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

	for _, cd := range k.GetEvmChains(ctx) {
		require.NotPanics(t, func() { k.SetLastObservedValset(ctx, cd.EvmChainPrefix, setValset) })

		getValset := k.GetLastObservedValset(ctx, cd.EvmChainPrefix)
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

	for _, cd := range k.GetEvmChains(ctx) {
		// In case this is first time validator is submiting claim, nonce is expected to be LastObservedNonce-1
		k.setLastObservedEventNonce(ctx, cd.EvmChainPrefix, nonce)
		getEventNonce := k.GetLastEventNonceByValidator(ctx, cd.EvmChainPrefix, addrInBytes)
		require.Equal(t, nonce-1, getEventNonce)

		require.NotPanics(t, func() { k.SetLastEventNonceByValidator(ctx, cd.EvmChainPrefix, addrInBytes, nonce) })

		getEventNonce = k.GetLastEventNonceByValidator(ctx, cd.EvmChainPrefix, addrInBytes)
		require.Equal(t, nonce, getEventNonce)
	}
}
