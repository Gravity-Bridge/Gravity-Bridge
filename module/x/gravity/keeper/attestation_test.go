package keeper

import (
	"testing"

	"github.com/althea-net/cosmos-gravity-bridge/module/x/gravity/types"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdktypes "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
)

// Sets up 10 attestations and checks that they are returned in the correct order
func TestGetMostRecentAttestations(t *testing.T) {
	input := CreateTestEnv(t)
	k := input.GravityKeeper
	ctx := input.Context

	lenth := 10
	msgs := make([]types.MsgSendToCosmosClaim, 0, lenth)
	anys := make([]codectypes.Any, 0, lenth)
	for i := 0; i < lenth; i++ {
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
		require.NoError(t, err)
		k.SetAttestation(ctx, nonce, hash, att)
	}

	recentAttestations := k.GetMostRecentAttestations(ctx, uint64(10))
	require.True(t, len(recentAttestations) == lenth,
		"recentAttestations should have len %v but instead has %v", lenth, len(recentAttestations))
	for n, attest := range recentAttestations {
		require.Equal(t, attest.Claim.GetCachedValue(), anys[n].GetCachedValue(),
			"The %vth claim does not match our message: claim %v\n message %v", n, attest.Claim, msgs[n])
	}
}
