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
	_, _, hashes := createAttestations(t, length, k, ctx)

	// Get created attestations
	for i := 0; i < length; i++ {
		nonce := uint64(1 + i)
		att := k.GetAttestation(ctx, nonce, hashes[i])
		require.NotNil(t, att)
	}
	recentAttestations := k.GetMostRecentAttestations(ctx, uint64(length))
	require.True(t, len(recentAttestations) == length)

	// Delete last 3 attestations
	var nilAtt *types.Attestation
	for i := 7; i < length; i++ {
		nonce := uint64(1 + i)
		att := k.GetAttestation(ctx, nonce, hashes[i])
		k.DeleteAttestation(ctx, *att)

		att = k.GetAttestation(ctx, nonce, hashes[i])
		require.Equal(t, nilAtt, att)
	}
	recentAttestations = k.GetMostRecentAttestations(ctx, uint64(10))
	require.True(t, len(recentAttestations) == 7)

	// Check all attestations again
	for i := 0; i < 7; i++ {
		nonce := uint64(1 + i)
		att := k.GetAttestation(ctx, nonce, hashes[i])
		require.NotNil(t, att)
	}
	for i := 7; i < length; i++ {
		nonce := uint64(1 + i)
		att := k.GetAttestation(ctx, nonce, hashes[i])
		require.Equal(t, nilAtt, att)
	}
}

// Sets up 10 attestations and checks that they are returned in the correct order
func TestGetMostRecentAttestations(t *testing.T) {
	input := CreateTestEnv(t)
	defer func() { input.Context.Logger().Info("Asserting invariants at test end"); input.AssertInvariants() }()

	k := input.GravityKeeper
	ctx := input.Context

	length := 10
	msgs, anys, _ := createAttestations(t, length, k, ctx)

	recentAttestations := k.GetMostRecentAttestations(ctx, uint64(length))
	require.True(t, len(recentAttestations) == length,
		"recentAttestations should have len %v but instead has %v", length, len(recentAttestations))
	for n, attest := range recentAttestations {
		require.Equal(t, attest.Claim.GetCachedValue(), anys[n].GetCachedValue(),
			"The %vth claim does not match our message: claim %v\n message %v", n, attest.Claim, msgs[n])
	}
}

func createAttestations(t *testing.T, length int, k Keeper, ctx sdktypes.Context) ([]types.MsgSendToCosmosClaim, []codectypes.Any, [][]byte) {
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
		k.SetAttestation(ctx, nonce, hash, att)
	}

	return msgs, anys, hashes
}

func TestGetSetLastObservedEthereumBlockHeight(t *testing.T) {
	input := CreateTestEnv(t)
	k := input.GravityKeeper
	ctx := input.Context

	ethereumHeight := uint64(7654321)

	require.NotPanics(t, func() { k.SetLastObservedEthereumBlockHeight(ctx, ethereumHeight) })

	ethHeight := k.GetLastObservedEthereumBlockHeight(ctx)
	require.Equal(t, uint64(ctx.BlockHeight()), ethHeight.CosmosBlockHeight)
	require.Equal(t, ethereumHeight, ethHeight.EthereumBlockHeight)
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

	require.NotPanics(t, func() { k.SetLastObservedValset(ctx, setValset) })

	getValset := k.GetLastObservedValset(ctx)
	require.EqualValues(t, setValset, *getValset)
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

	// In case this is first time validator is submiting claim, nonce is expected to be LastObservedNonce-1
	k.setLastObservedEventNonce(ctx, nonce)
	getEventNonce := k.GetLastEventNonceByValidator(ctx, addrInBytes)
	require.Equal(t, nonce-1, getEventNonce)

	require.NotPanics(t, func() { k.SetLastEventNonceByValidator(ctx, addrInBytes, nonce) })

	getEventNonce = k.GetLastEventNonceByValidator(ctx, addrInBytes)
	require.Equal(t, nonce, getEventNonce)
}

func TestInvalidHeight(t *testing.T) {
	input, ctx := SetupFiveValChain(t)
	defer func() { input.Context.Logger().Info("Asserting invariants at test end"); input.AssertInvariants() }()
	pk := input.GravityKeeper
	log := ctx.Logger()

	msgServer := NewMsgServerImpl(pk)

	// Submit a bad claim:
	val0 := ValAddrs[0]
	orch0 := OrchAddrs[0]
	lastNonce := pk.GetLastObservedEventNonce(ctx)
	lastEthHeight := pk.GetLastObservedEthereumBlockHeight(ctx)
	lastBatchNonce := 0
	tokenContract := "0x2260FAC5E5542a773Aa44fBCfeDf7C193bc2C599"
	goodHeight := lastEthHeight.EthereumBlockHeight + 1
	badHeight := lastEthHeight.EthereumBlockHeight + 2

	bad := types.MsgBatchSendToEthClaim{
		EventNonce:    lastNonce + 1,
		BlockHeight:   badHeight,
		BatchNonce:    uint64(lastBatchNonce + 1),
		TokenContract: tokenContract,
		Orchestrator:  orch0.String(),
	}
	context := sdktypes.WrapSDKContext(ctx)
	log.Info("Submitting bad eth claim from orchestrator 0", "orch", orch0.String(), "val", val0.String())
	_, err := msgServer.BatchSendToEthClaim(context, &bad)
	require.NoError(t, err)

	badHash, err := bad.ClaimHash()
	require.NoError(t, err)

	for _, orch := range OrchAddrs[1:] {
		log.Info("Submitting good eth claim from orchestrators", "orch", orch.String())
		good := types.MsgBatchSendToEthClaim{
			EventNonce:    lastNonce + 1,
			BlockHeight:   goodHeight,
			BatchNonce:    uint64(lastBatchNonce + 1),
			TokenContract: tokenContract,
			Orchestrator:  orch.String(),
		}
		_, err := msgServer.BatchSendToEthClaim(context, &good)
		require.Error(t, err)

		goodHash, err := good.ClaimHash()
		require.NoError(t, err)
		require.Equal(t, badHash, goodHash) // The hash should be the same, even though that's wrong

		att := pk.GetAttestation(ctx, good.GetEventNonce(), goodHash)
		require.NotNil(t, att)
		log.Info("Asserting that the bad attestation only has one claimer", "attVotes", att.Votes)
		require.Equal(t, len(att.Votes), 1) // No votes on this attestation
	}

}
