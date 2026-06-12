package keeper

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"

	sdkmath "cosmossdk.io/math"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"

	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
)

const (
	testTokenContract = "0x2260FAC5E5542a773Aa44fBCfeDf7C193bc2C599"
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

func createAttestations(t *testing.T, length int, k Keeper, ctx sdk.Context) ([]types.MsgSendToCosmosClaim, []codectypes.Any, [][]byte) {
	msgs := make([]types.MsgSendToCosmosClaim, 0, length)
	anys := make([]codectypes.Any, 0, length)
	hashes := make([][]byte, 0, length)
	for i := 0; i < length; i++ {
		nonce := uint64(1 + i)

		contract := common.BytesToAddress(bytes.Repeat([]byte{0x1}, 20)).String()
		sender := common.BytesToAddress(bytes.Repeat([]byte{0x2}, 20)).String()
		orch := sdk.AccAddress(bytes.Repeat([]byte{0x3}, 20)).String()
		receiver := sdk.AccAddress(bytes.Repeat([]byte{0x4}, 20)).String()
		msg := types.MsgSendToCosmosClaim{
			EventNonce:     nonce,
			EthBlockHeight: 1,
			TokenContract:  contract,
			Amount:         sdkmath.NewInt(10000000000 + int64(i)),
			EthereumSender: sender,
			CosmosReceiver: receiver,
			Orchestrator:   orch,
		}
		msgs = append(msgs, msg)

		any, err := codectypes.NewAnyWithValue(&msg)
		require.NoError(t, err)
		anys = append(anys, *any)
		att := &types.Attestation{
			Observed: false,
			Votes:    []string{},
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
		RewardAmount: sdkmath.NewInt(1000000000),
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
	valAccAddress, err := sdk.AccAddressFromBech32(valAddrString)
	require.NoError(t, err)
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
	msgServer := NewMsgServerImpl(pk)
	log := ctx.Logger()

	val0 := ValAddrs[0]
	orch0 := OrchAddrs[0]
	sender := AccAddrs[0]
	receiver := EthAddrs[0]
	lastNonce := pk.GetLastObservedEventNonce(ctx)
	lastEthHeight := pk.GetLastObservedEthereumBlockHeight(ctx).EthereumBlockHeight
	lastBatchNonce := 0
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
				Contract: testTokenContract,
				Amount:   sdkmath.NewInt(1),
			},
			Erc20Fee: types.ERC20Token{
				Contract: testTokenContract,
				Amount:   sdkmath.NewInt(1),
			},
		}},
		TokenContract:      testTokenContract,
		CosmosBlockCreated: 0,
	}
	b, err := batch.ToInternal()
	require.NoError(t, err)
	pk.StoreBatch(ctx, *b)

	// Submit a bad claim with EthBlockHeight >= timeout

	tokenContract := testTokenContract
	bad := types.MsgBatchSendToEthClaim{
		EventNonce:     lastNonce + 1,
		EthBlockHeight: badHeight,
		BatchNonce:     uint64(lastBatchNonce + 1),
		TokenContract:  tokenContract,
		Orchestrator:   orch0.String(),
	}
	log.Info("Submitting bad eth claim from orchestrator 0", "orch", orch0.String(), "val", val0.String())

	// BatchSendToEthClaim is supposed to panic and fail the message execution, set up a defer recover to catch it
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("recovered from panic:", r)
		} else {
			panic("Expected to find a panic coming from BatchSendToEthClaim()!")
		}
	}()
	_, err = msgServer.BatchSendToEthClaim(ctx, &bad)
	require.NoError(t, err)

	// Assert that there is no attestation since the above panicked
	badHash, err := bad.ClaimHash()
	require.NoError(t, err)
	att := pk.GetAttestation(ctx, bad.GetEventNonce(), badHash)
	require.Nil(t, att)

	// Attest the actual batch, and assert the votes are correct
	for i, orch := range OrchAddrs[1:] {
		log.Info("Submitting good eth claim from orchestrators", "orch", orch.String())
		tokenContract := testTokenContract
		good := types.MsgBatchSendToEthClaim{
			EventNonce:     lastNonce + 1,
			EthBlockHeight: goodHeight,
			BatchNonce:     uint64(lastBatchNonce + 1),
			TokenContract:  tokenContract,
			Orchestrator:   orch.String(),
		}
		_, err := msgServer.BatchSendToEthClaim(ctx, &good)
		require.NoError(t, err)

		goodHash, err := good.ClaimHash()
		require.NoError(t, err)
		require.Equal(t, badHash, goodHash) // The hash should be the same, even though that's wrong

		att := pk.GetAttestation(ctx, good.GetEventNonce(), goodHash)
		require.NotNil(t, att)
		log.Info("Asserting that the bad attestation only has one claimer", "attVotes", att.Votes)
		require.Equal(t, len(att.Votes), i+1) // Only these good orchestrators votes should be counted
	}

}

// TestAttestStoresComponents verifies that when a new attestation is created via Attest(),
// the corresponding ClaimType and ClaimComponents are stored in state.
func TestAttestStoresComponents(t *testing.T) {
	input, ctx := SetupFiveValChain(t)
	defer func() { input.Context.Logger().Info("Asserting invariants at test end"); input.AssertInvariants() }()
	k := input.GravityKeeper

	lastNonce := k.GetLastObservedEventNonce(ctx)
	tokenContract := "0x2260FAC5E5542a773Aa44fBCfeDf7C193bc2C599"
	orch0 := OrchAddrs[0]

	claim := types.MsgBatchSendToEthClaim{
		EventNonce:     lastNonce + 1,
		EthBlockHeight: 1,
		BatchNonce:     1,
		TokenContract:  tokenContract,
		Orchestrator:   orch0.String(),
	}
	claimAny, err := codectypes.NewAnyWithValue(&claim)
	require.NoError(t, err)

	_, err = k.Attest(ctx, &claim, claimAny)
	require.NoError(t, err)

	hash, err := claim.ClaimHash()
	require.NoError(t, err)
	att := k.GetAttestation(ctx, claim.GetEventNonce(), hash)
	require.NotNil(t, att)
	require.False(t, att.Observed)
	require.Equal(t, types.CLAIM_TYPE_BATCH_SEND_TO_ETH, att.ClaimType)
	require.NotNil(t, att.ClaimComponents)
	require.NotNil(t, att.ClaimComponents.GetBatchSendToEth())
	storedComp := att.ClaimComponents.GetBatchSendToEth()
	require.Equal(t, claim.BatchNonce, storedComp.BatchNonce)
	require.Equal(t, claim.TokenContract, storedComp.TokenContract)
	require.Equal(t, claim.EthBlockHeight, storedComp.EthBlockHeight)
}

// TestAttestDuplicateClaimMatchingComponents verifies that a second attestation of an
// identical claim is accepted when the stored ClaimHashComponents hash matches the
// incoming claim hash.
func TestAttestDuplicateClaimMatchingComponents(t *testing.T) {
	input, ctx := SetupFiveValChain(t)
	defer func() { input.Context.Logger().Info("Asserting invariants at test end"); input.AssertInvariants() }()
	k := input.GravityKeeper

	lastNonce := k.GetLastObservedEventNonce(ctx)
	tokenContract := "0x2260FAC5E5542a773Aa44fBCfeDf7C193bc2C599"
	orch0 := OrchAddrs[0]
	orch1 := OrchAddrs[1]

	claim := types.MsgBatchSendToEthClaim{
		EventNonce:     lastNonce + 1,
		EthBlockHeight: 1,
		BatchNonce:     1,
		TokenContract:  tokenContract,
		Orchestrator:   orch0.String(),
	}
	claimAny, err := codectypes.NewAnyWithValue(&claim)
	require.NoError(t, err)

	_, err = k.Attest(ctx, &claim, claimAny)
	require.NoError(t, err)

	hash, err := claim.ClaimHash()
	require.NoError(t, err)
	att := k.GetAttestation(ctx, claim.GetEventNonce(), hash)
	require.NotNil(t, att)
	require.Equal(t, 1, len(att.Votes)) // first vote

	// Duplicate claim from another orchestrator with the same fields
	claim2 := types.MsgBatchSendToEthClaim{
		EventNonce:     lastNonce + 1,
		EthBlockHeight: 1,
		BatchNonce:     1,
		TokenContract:  tokenContract,
		Orchestrator:   orch1.String(),
	}
	claim2Any, err := codectypes.NewAnyWithValue(&claim2)
	require.NoError(t, err)

	_, err = k.Attest(ctx, &claim2, claim2Any)
	require.NoError(t, err)

	att = k.GetAttestation(ctx, claim.GetEventNonce(), hash)
	require.NotNil(t, att)
	require.Equal(t, 2, len(att.Votes)) // both votes
}

// TestAttestMismatchingComponents verifies that Attest rejects a claim when the
// stored ClaimHashComponents have been tampered with (simulating data corruption).
func TestAttestMismatchingComponents(t *testing.T) {
	input, ctx := SetupFiveValChain(t)
	defer func() { input.Context.Logger().Info("Skipping invariants check due to intentional state tampering") }()
	k := input.GravityKeeper

	lastNonce := k.GetLastObservedEventNonce(ctx)
	tokenContract := "0x2260FAC5E5542a773Aa44fBCfeDf7C193bc2C599"
	orch0 := OrchAddrs[0]
	orch1 := OrchAddrs[1]

	claim := types.MsgBatchSendToEthClaim{
		EventNonce:     lastNonce + 1,
		EthBlockHeight: 1,
		BatchNonce:     1,
		TokenContract:  tokenContract,
		Orchestrator:   orch0.String(),
	}
	claimAny, err := codectypes.NewAnyWithValue(&claim)
	require.NoError(t, err)

	_, err = k.Attest(ctx, &claim, claimAny)
	require.NoError(t, err)

	hash, err := claim.ClaimHash()
	require.NoError(t, err)
	att := k.GetAttestation(ctx, claim.GetEventNonce(), hash)
	require.NotNil(t, att)

	// Tamper with stored components directly: change BatchNonce
	require.NotNil(t, att.ClaimComponents)
	att.ClaimComponents.GetBatchSendToEth().BatchNonce = 9999
	k.SetAttestation(ctx, claim.GetEventNonce(), hash, att)

	// Second validator attests to the same claim (same fields)
	claim2 := types.MsgBatchSendToEthClaim{
		EventNonce:     lastNonce + 1,
		EthBlockHeight: 1,
		BatchNonce:     1,
		TokenContract:  tokenContract,
		Orchestrator:   orch1.String(),
	}
	claim2Any, err := codectypes.NewAnyWithValue(&claim2)
	require.NoError(t, err)

	_, err = k.Attest(ctx, &claim2, claim2Any)
	require.Error(t, err)
	require.Contains(t, err.Error(), "incoming claim does not match stored attestation components")
}

// TestERC20DeployedClaimHashCollision attempts to engineer a collision in the ClaimHash of an ERC20DeployedClaim
// by replicating the IBC Transfer module's behavior of adding bank denom metadata for a malicious IBC token.
func TestERC20DeployedClaimHashCollision(t *testing.T) {
	input, ctx := SetupFiveValChain(t)
	defer func() { input.Context.Logger().Info("Asserting invariants at test end"); input.AssertInvariants() }()
	k := input.GravityKeeper

	lastNonce := k.GetLastObservedEventNonce(ctx)
	eventNonce := lastNonce + 1

	const (
		gravityDestPort    = "transfer"
		gravityDestChannel = "channel-10"
		// footokenAddr is the Ethereum-originating asset the attacker wants to hijack.
		footokenAddr = "0x0412C7c846bb6b7DC462CF6B453f76D8440b2609"
		// newERC20Addr is the ERC20 Gravity.sol deployed to represent the IBC token.
		newERC20Addr = "0xAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	)

	// Simulate creation of a malicious token on the foreign chain
	// The attacker's token denom is footokenAddr + "/token", IBC Transfer produces metadata like
	//   Path:      "transfer/channel-10"
	//   BaseDenom: "0x0412C7c846bb6b7DC462CF6B453f76D8440b2609/token"
	// This places footokenAddr at a potentially vulnerable position in the IBC denom
	const denomSuffix = "token"
	baseDenom := footokenAddr + "/" + denomSuffix
	fullDenomPath := gravityDestPort + "/" + gravityDestChannel + "/" + baseDenom
	t.Logf("attacker's foreign denom : %s", baseDenom)
	t.Logf("full IBC denom path: %s", fullDenomPath)
	rawHash := sha256.Sum256([]byte(fullDenomPath))
	ibcDenom := "ibc/" + strings.ToUpper(hex.EncodeToString(rawHash[:]))

	// Create bank denom metadata for the token in the way IBC Transfer will do so
	ibcTokenName := fmt.Sprintf("%s IBC token", fullDenomPath)
	ibcTokenSymbol := strings.ToUpper(baseDenom)
	t.Logf("bank metadata.Name  : %s", ibcTokenName)
	t.Logf("bank metadata.Symbol: %s", ibcTokenSymbol)
	metadata := banktypes.Metadata{
		Description: fmt.Sprintf("IBC token from %s", fullDenomPath),
		DenomUnits: []*banktypes.DenomUnit{
			{Denom: baseDenom, Exponent: 0},
		},
		Base:    ibcDenom,
		Display: fullDenomPath,
		Name:    ibcTokenName,
		Symbol:  ibcTokenSymbol,
	}
	input.BankKeeper.SetDenomMetaData(ctx, metadata)
	t.Logf("bank DenomMetaData stored for %s", ibcDenom)

	// Simulate an erc20 deploy event
	honestClaim := types.MsgERC20DeployedClaim{
		EventNonce:     eventNonce,
		EthBlockHeight: 1,
		CosmosDenom:    ibcDenom,
		TokenContract:  newERC20Addr,
		Name:           ibcTokenName,
		Symbol:         ibcTokenSymbol,
		Decimals:       0,
		Orchestrator:   OrchAddrs[0].String(),
	}

	// Attempt to create a collision with a forged claim (must be submitted by a validator)
	forgedCosmosDenom := ibcDenom + "/" + newERC20Addr + "/" + gravityDestPort + "/" + gravityDestChannel
	forgedName := denomSuffix + " IBC token"
	forgedClaim := types.MsgERC20DeployedClaim{
		EventNonce:     eventNonce,
		EthBlockHeight: 1,
		CosmosDenom:    forgedCosmosDenom,
		TokenContract:  footokenAddr,
		Name:           forgedName,
		Symbol:         ibcTokenSymbol,
		Decimals:       0,
		Orchestrator:   OrchAddrs[1].String(),
	}
	// Confirm the collision is PREVENTED by AttestationSeparator
	honestHash, err := honestClaim.ClaimHash()
	require.NoError(t, err)
	forgedHash, err := forgedClaim.ClaimHash()
	require.NoError(t, err)
	t.Logf("honest ClaimHash = %x", honestHash)
	t.Logf("forged ClaimHash = %x", forgedHash)
	require.NotEqual(t, honestHash, forgedHash,
		"AttestationSeparator prevents the hash collision - hashes must differ")
	require.NotEqual(t, honestClaim.CosmosDenom, forgedClaim.CosmosDenom)
	require.NotEqual(t, honestClaim.TokenContract, forgedClaim.TokenContract)

	// Confirm ValidateBasic rejects the invalid claim
	err = forgedClaim.ValidateBasic()
	require.Error(t, err)

	// Confirm honest claim is accepted by the chain
	honestAny, err := codectypes.NewAnyWithValue(&honestClaim)
	require.NoError(t, err)
	t.Logf("submitting honest claim for %s", honestClaim.TokenContract)
	_, err = k.Attest(ctx, &honestClaim, honestAny)
	require.NoError(t, err)

	// Verify the honest attestation was stored with the correct components.
	honestAtt := k.GetAttestation(ctx, eventNonce, honestHash)
	require.NotNil(t, honestAtt)
	require.NotNil(t, honestAtt.ClaimComponents)
	honestComp := honestAtt.ClaimComponents.GetErc20Deployed()
	require.NotNil(t, honestComp)
	require.Equal(t, newERC20Addr, honestComp.TokenContract)
	require.Equal(t, 1, len(honestAtt.Votes))
}
