package types

import (
	"bytes"
	"fmt"
	"testing"

	"cosmossdk.io/math"
	"cosmossdk.io/x/tx/signing"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/address"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	proto "github.com/cosmos/gogoproto/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/slices"
)

func TestValidateMsgSetOrchestratorAddress(t *testing.T) {
	var (
		ethAddress                   = "0xb462864E395d88d6bc7C5dd5F3F5eb4cc2599255"
		cosmosAddress sdk.AccAddress = bytes.Repeat([]byte{0x1}, 20)
		valAddress    sdk.AccAddress = bytes.Repeat([]byte{0x1}, 20)
	)
	specs := map[string]struct {
		srcCosmosAddr sdk.AccAddress
		srcValAddr    sdk.AccAddress
		srcETHAddr    string
		expErr        bool
	}{
		"all good": {
			srcCosmosAddr: cosmosAddress,
			srcValAddr:    valAddress,
			srcETHAddr:    ethAddress,
			expErr:        false,
		},
		"empty validator address": {
			srcETHAddr:    ethAddress,
			srcValAddr:    []byte{},
			srcCosmosAddr: cosmosAddress,
			expErr:        true,
		},
		"short validator address": {
			srcValAddr:    []byte{0x1},
			srcCosmosAddr: cosmosAddress,
			srcETHAddr:    ethAddress,
			expErr:        false,
		},
		"empty cosmos address": {
			srcCosmosAddr: []byte{},
			srcValAddr:    valAddress,
			srcETHAddr:    ethAddress,
			expErr:        true,
		},
		"short cosmos address": {
			srcCosmosAddr: []byte{0x1},
			srcValAddr:    valAddress,
			srcETHAddr:    ethAddress,
			expErr:        false,
		},
	}
	for msg, spec := range specs {
		t.Run(msg, func(t *testing.T) {
			println(fmt.Sprintf("Spec is %v", msg))
			ethAddr, err := NewEthAddress(spec.srcETHAddr)
			assert.NoError(t, err)
			msg := NewMsgSetOrchestratorAddress(spec.srcValAddr, spec.srcCosmosAddr, *ethAddr)
			// when
			err = msg.ValidateBasic()
			if spec.expErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
		})
	}

}

// Gets the ClaimHash() output from every claims member and casts it to a string, panicing on any errors
func getClaimHashStrings(t *testing.T, claims ...EthereumClaim) (hashes []string) {
	for _, claim := range claims {
		hash, e := claim.ClaimHash()
		require.NoError(t, e)
		hashes = append(hashes, string(hash))
	}
	return
}

// Calls SetOrchestrator on every claims member, passing orch as the value
func setOrchestratorOnClaims(orch sdk.AccAddress, claims ...EthereumClaim) (ret []EthereumClaim) {
	for _, claim := range claims {
		clam := claim
		clam.SetOrchestrator(orch)
		ret = append(ret, clam)
	}
	return
}

// Ensures that ClaimHash changes when members of MsgSendToCosmosClaim change
// The only field which MUST NOT affect ClaimHash is Orchestrator
func TestMsgSendToCosmosClaimHash(t *testing.T) {
	base := MsgSendToCosmosClaim{
		EventNonce:     0,
		EthBlockHeight: 0,
		TokenContract:  "",
		Amount:         math.Int{},
		EthereumSender: "",
		CosmosReceiver: "",
		Orchestrator:   "",
	}

	// Copy and populate base with values, saving orchestrator for a special check
	orchestrator := NonemptySdkAccAddress()
	mNonce := base
	mNonce.EventNonce = NonzeroUint64()
	mBlock := base
	mBlock.EthBlockHeight = NonzeroUint64()
	mCtr := base
	mCtr.TokenContract = NonemptyEthAddress()
	mAmt := base
	mAmt.Amount = NonzeroSdkInt()
	mSend := base
	mSend.EthereumSender = NonemptyEthAddress()
	mRecv := base
	mRecv.CosmosReceiver = NonemptySdkAccAddress().String()

	hashes := getClaimHashStrings(t, &base, &mNonce, &mBlock, &mCtr, &mAmt, &mSend, &mRecv)
	baseH := hashes[0]
	rest := hashes[1:]
	// Assert that the base claim hash differs from all the rest
	require.False(t, slices.Contains(rest, baseH))

	newClaims := setOrchestratorOnClaims(orchestrator, &base, &mNonce, &mBlock, &mCtr, &mAmt, &mSend, &mRecv)
	newHashes := getClaimHashStrings(t, newClaims...)
	// Assert that the claims with orchestrator set do not change the hashes
	require.Equal(t, hashes, newHashes)
}

// Ensures that ClaimHash changes when members of MsgBatchSendToEth change
// The only field which MUST NOT affect ClaimHash is Orchestrator
func TestMsgBatchSendToEthClaimHash(t *testing.T) {
	//nolint: exhaustruct
	base := MsgBatchSendToEthClaim{
		EventNonce:     0,
		EthBlockHeight: 0,
		BatchNonce:     0,
		TokenContract:  "",
		Orchestrator:   "",
	}

	orchestrator := NonemptySdkAccAddress()
	mNonce := base
	mNonce.EventNonce = NonzeroUint64()
	mBlock := base
	mBlock.EthBlockHeight = NonzeroUint64()
	mBatch := base
	mBatch.BatchNonce = NonzeroUint64()
	mCtr := base
	mCtr.TokenContract = NonemptyEthAddress()

	hashes := getClaimHashStrings(t, &base, &mNonce, &mBlock, &mBatch, &mCtr)
	baseH := hashes[0]
	rest := hashes[1:]
	// Assert that the base claim hash differs from all the rest
	require.False(t, slices.Contains(rest, baseH))

	newClaims := setOrchestratorOnClaims(orchestrator, &base, &mNonce, &mBlock, &mBatch, &mCtr)
	newHashes := getClaimHashStrings(t, newClaims...)
	// Assert that the claims with orchestrator set do not change the hashes
	require.Equal(t, hashes, newHashes)
}

// Ensures that ClaimHash changes when members of MsgERC20DeployedClaim change
// The only field which MUST NOT affect ClaimHash is Orchestrator
func TestMsgERC20DeployedClaimHash(t *testing.T) {
	//nolint: exhaustruct
	base := MsgERC20DeployedClaim{
		EventNonce:     0,
		EthBlockHeight: 0,
		CosmosDenom:    "",
		TokenContract:  "",
		Name:           "",
		Symbol:         "",
		Decimals:       0,
		Orchestrator:   "",
	}

	orchestrator := NonemptySdkAccAddress()
	mNonce := base
	mNonce.EventNonce = NonzeroUint64()
	mBlock := base
	mBlock.EthBlockHeight = NonzeroUint64()
	mDenom := base
	mDenom.CosmosDenom = NonemptyEthAddress()
	mCtr := base
	mCtr.TokenContract = NonemptyEthAddress()
	mName := base
	mName.Name = NonemptyEthAddress()
	mSymb := base
	mSymb.Symbol = NonemptyEthAddress()
	mDecim := base
	mDecim.Decimals = NonzeroUint64()

	hashes := getClaimHashStrings(t, &base, &mNonce, &mBlock, &mDenom, &mName, &mSymb, &mDecim)
	baseH := hashes[0]
	rest := hashes[1:]
	// Assert that the base claim hash differs from all the rest
	require.False(t, slices.Contains(rest, baseH))

	newClaims := setOrchestratorOnClaims(orchestrator, &base, &mNonce, &mBlock, &mDenom, &mName, &mSymb, &mDecim)
	newHashes := getClaimHashStrings(t, newClaims...)
	// Assert that the claims with orchestrator set do not change the hashes
	require.Equal(t, hashes, newHashes)
}

// Ensures that ClaimHash changes when members of MsgLogicCallExecutedClaim change
// The only field which MUST NOT affect ClaimHash is Orchestrator
func TestMsgLogicCallExecutedClaimHash(t *testing.T) {
	base := MsgLogicCallExecutedClaim{
		EventNonce:        0,
		EthBlockHeight:    0,
		InvalidationId:    []byte{},
		InvalidationNonce: 0,
		Orchestrator:      "",
	}

	orchestrator := NonemptySdkAccAddress()
	mNonce := base
	mNonce.EventNonce = NonzeroUint64()
	mBlock := base
	mBlock.EthBlockHeight = NonzeroUint64()
	mInvId := base
	mInvId.InvalidationId = NonemptySdkAccAddress().Bytes()
	mInvNo := base
	mInvNo.InvalidationNonce = NonzeroUint64()

	hashes := getClaimHashStrings(t, &base, &mNonce, &mBlock, &mInvId, &mInvNo)
	baseH := hashes[0]
	rest := hashes[1:]
	// Assert that the base claim hash differs from all the rest
	require.False(t, slices.Contains(rest, baseH))

	newClaims := setOrchestratorOnClaims(orchestrator, &base, &mNonce, &mBlock, &mInvId, &mInvNo)
	newHashes := getClaimHashStrings(t, newClaims...)
	// Assert that the claims with orchestrator set do not change the hashes
	require.Equal(t, hashes, newHashes)
}

func TestMsgSendToEth_ValidateBasic_Denom(t *testing.T) {
	base := MsgSendToEth{
		Sender:    sdk.AccAddress([]byte{1, 2, 3}).String(),
		EthDest:   "0xb462864E395d88d6bc7C5dd5F3F5eb4cc2599255",
		Amount:    sdk.NewCoin("footoken", math.NewInt(100)),
		BridgeFee: sdk.NewCoin("footoken", math.NewInt(1)),
		ChainFee:  sdk.NewCoin("footoken", math.NewInt(1)),
	}

	badAmount := base
	badAmount.Amount = sdk.NewCoin("ibc/gravity0xbad", math.NewInt(100))

	badBridgeFee := base
	badBridgeFee.BridgeFee = sdk.NewCoin("ibc/gravity0xbad", math.NewInt(1))

	badChainFee := base
	badChainFee.ChainFee = sdk.NewCoin("ibc/gravity0xbad", math.NewInt(1))

	badAll := base
	badAll.Amount = sdk.NewCoin("ibc/gravity0xbad", math.NewInt(100))
	badAll.BridgeFee = sdk.NewCoin("ibc/gravity0xbad", math.NewInt(1))
	badAll.ChainFee = sdk.NewCoin("ibc/gravity0xbad", math.NewInt(1))

	tests := []struct {
		name    string
		msg     MsgSendToEth
		wantErr bool
	}{
		{"valid", base, false},
		{"bad amount denom", badAmount, true},
		{"bad bridge fee denom", badBridgeFee, true},
		{"bad chain fee denom", badChainFee, true},
		{"all bad denoms", badAll, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.msg.ValidateBasic()
			if tt.wantErr {
				require.Error(t, err)
				require.ErrorIs(t, err, ErrInvalidDenom)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestMsgRequestBatch_ValidateBasic_Denom(t *testing.T) {
	valid := MsgRequestBatch{
		Sender: sdk.AccAddress([]byte{1, 2, 3}).String(),
		Denom:  "footoken",
	}
	invalid := MsgRequestBatch{
		Sender: sdk.AccAddress([]byte{1, 2, 3}).String(),
		Denom:  "ibc/gravity0xbad",
	}

	tests := []struct {
		name    string
		msg     MsgRequestBatch
		wantErr bool
	}{
		{"valid", valid, false},
		{"bad denom with forbidden substring", invalid, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.msg.ValidateBasic()
			if tt.wantErr {
				require.Error(t, err)
				require.ErrorIs(t, err, ErrInvalidDenom)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestMsgERC20DeployedClaim_ValidateBasic_Denom(t *testing.T) {
	//nolint: exhaustruct
	valid := MsgERC20DeployedClaim{
		Orchestrator:  sdk.AccAddress([]byte{1, 2, 3}).String(),
		CosmosDenom:   "ugraviton",
		TokenContract: "0xb462864E395d88d6bc7C5dd5F3F5eb4cc2599255",
		EventNonce:    1,
	}
	//nolint: exhaustruct
	invalid := MsgERC20DeployedClaim{
		Orchestrator:  sdk.AccAddress([]byte{1, 2, 3}).String(),
		CosmosDenom:   "ibc/gravity0xbad",
		TokenContract: "0xb462864E395d88d6bc7C5dd5F3F5eb4cc2599255",
		EventNonce:    1,
	}

	tests := []struct {
		name    string
		msg     MsgERC20DeployedClaim
		wantErr bool
	}{
		{"valid cosmos denom", valid, false},
		{"bad cosmos denom with forbidden substring", invalid, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.msg.ValidateBasic()
			if tt.wantErr {
				require.Error(t, err)
				require.ErrorIs(t, err, ErrInvalidDenom)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// Reconstruction tests: verify that a claim reconstructed from individually stored
// ClaimHashComponents produces the same ClaimHash as the original.
func TestMsgSendToCosmosClaimReconstruction(t *testing.T) {
	original := MsgSendToCosmosClaim{
		EventNonce:     NonzeroUint64(),
		EthBlockHeight: NonzeroUint64(),
		TokenContract:  NonemptyEthAddress(),
		Amount:         NonzeroSdkInt(),
		EthereumSender: NonemptyEthAddress(),
		CosmosReceiver: NonemptySdkAccAddress().String(),
		Orchestrator:   NonemptySdkAccAddress().String(),
	}
	originalHash, err := original.ClaimHash()
	require.NoError(t, err)

	components, err := ExtractClaimHashComponents(&original)
	require.NoError(t, err)
	require.NotNil(t, components)

	reconstructed, err := ReconstructClaim(CLAIM_TYPE_SEND_TO_COSMOS, components)
	require.NoError(t, err)
	reconstructedHash, err := reconstructed.ClaimHash()
	require.NoError(t, err)

	require.Equal(t, originalHash, reconstructedHash)
}

func TestMsgBatchSendToEthClaimReconstruction(t *testing.T) {
	//nolint: exhaustruct
	original := MsgBatchSendToEthClaim{
		EventNonce:     NonzeroUint64(),
		EthBlockHeight: NonzeroUint64(),
		BatchNonce:     NonzeroUint64(),
		TokenContract:  NonemptyEthAddress(),
		Orchestrator:   NonemptySdkAccAddress().String(),
	}
	originalHash, err := original.ClaimHash()
	require.NoError(t, err)

	components, err := ExtractClaimHashComponents(&original)
	require.NoError(t, err)
	require.NotNil(t, components)

	reconstructed, err := ReconstructClaim(CLAIM_TYPE_BATCH_SEND_TO_ETH, components)
	require.NoError(t, err)
	reconstructedHash, err := reconstructed.ClaimHash()
	require.NoError(t, err)

	require.Equal(t, originalHash, reconstructedHash)
}

func TestMsgERC20DeployedClaimReconstruction(t *testing.T) {
	//nolint: exhaustruct
	original := MsgERC20DeployedClaim{
		EventNonce:     NonzeroUint64(),
		EthBlockHeight: NonzeroUint64(),
		CosmosDenom:    "ugravity",
		TokenContract:  NonemptyEthAddress(),
		Name:           "TestToken",
		Symbol:         "TTK",
		Decimals:       18,
		Orchestrator:   NonemptySdkAccAddress().String(),
	}
	originalHash, err := original.ClaimHash()
	require.NoError(t, err)

	components, err := ExtractClaimHashComponents(&original)
	require.NoError(t, err)
	require.NotNil(t, components)

	reconstructed, err := ReconstructClaim(CLAIM_TYPE_ERC20_DEPLOYED, components)
	require.NoError(t, err)
	reconstructedHash, err := reconstructed.ClaimHash()
	require.NoError(t, err)

	require.Equal(t, originalHash, reconstructedHash)
}

func TestMsgLogicCallExecutedClaimReconstruction(t *testing.T) {
	original := MsgLogicCallExecutedClaim{
		EventNonce:        NonzeroUint64(),
		EthBlockHeight:    NonzeroUint64(),
		InvalidationId:    NonemptySdkAccAddress().Bytes(),
		InvalidationNonce: NonzeroUint64(),
		Orchestrator:      NonemptySdkAccAddress().String(),
	}
	originalHash, err := original.ClaimHash()
	require.NoError(t, err)

	components, err := ExtractClaimHashComponents(&original)
	require.NoError(t, err)
	require.NotNil(t, components)

	reconstructed, err := ReconstructClaim(CLAIM_TYPE_LOGIC_CALL_EXECUTED, components)
	require.NoError(t, err)
	reconstructedHash, err := reconstructed.ClaimHash()
	require.NoError(t, err)

	require.Equal(t, originalHash, reconstructedHash)
}

func TestMsgValsetUpdatedClaimReconstructionSorted(t *testing.T) {
	//nolint: exhaustruct
	original := MsgValsetUpdatedClaim{
		EventNonce:     NonzeroUint64(),
		ValsetNonce:    NonzeroUint64(),
		EthBlockHeight: NonzeroUint64(),
		Members: []BridgeValidator{
			{Power: 300, EthereumAddress: "0x3333333333333333333333333333333333333333"},
			{Power: 100, EthereumAddress: "0x1111111111111111111111111111111111111111"},
			{Power: 200, EthereumAddress: "0x2222222222222222222222222222222222222222"},
		},
		RewardAmount: NonzeroSdkInt(),
		RewardToken:  "0x9999999999999999999999999999999999999999",
		Orchestrator: NonemptySdkAccAddress().String(),
	}
	originalHash, err := original.ClaimHash()
	require.NoError(t, err)

	components, err := ExtractClaimHashComponents(&original)
	require.NoError(t, err)
	require.NotNil(t, components)

	// Verify that stored members are sorted (descending power, then ascending address)
	vuc := components.GetValsetUpdated()
	require.NotNil(t, vuc)
	require.Len(t, vuc.Members, 3)
	require.Equal(t, uint64(300), vuc.Members[0].Power)
	require.Equal(t, "0x3333333333333333333333333333333333333333", vuc.Members[0].EthereumAddress)
	require.Equal(t, uint64(200), vuc.Members[1].Power)
	require.Equal(t, "0x2222222222222222222222222222222222222222", vuc.Members[1].EthereumAddress)
	require.Equal(t, uint64(100), vuc.Members[2].Power)
	require.Equal(t, "0x1111111111111111111111111111111111111111", vuc.Members[2].EthereumAddress)

	reconstructed, err := ReconstructClaim(CLAIM_TYPE_VALSET_UPDATED, components)
	require.NoError(t, err)
	reconstructedHash, err := reconstructed.ClaimHash()
	require.NoError(t, err)

	require.Equal(t, originalHash, reconstructedHash)
}

// VerifyClaimHash verifies that the hash computed from the stored Any claim matches
// the hash computed from the individually stored claim components.
func TestVerifyClaimHash(t *testing.T) {
	//nolint: exhaustruct
	original := MsgBatchSendToEthClaim{
		EventNonce:     NonzeroUint64(),
		EthBlockHeight: NonzeroUint64(),
		BatchNonce:     NonzeroUint64(),
		TokenContract:  NonemptyEthAddress(),
		Orchestrator:   NonemptySdkAccAddress().String(),
	}
	components, err := ExtractClaimHashComponents(&original)
	require.NoError(t, err)

	anyClaim, err := codectypes.NewAnyWithValue(&original)
	require.NoError(t, err)

	//nolint: exhaustruct
	att := Attestation{
		Claim:           anyClaim,
		ClaimType:       CLAIM_TYPE_BATCH_SEND_TO_ETH,
		ClaimComponents: components,
	}

	require.NoError(t, att.VerifyClaimHash(createTestCodec()))

	// Tamper with components and verify it fails
	att.ClaimComponents.GetBatchSendToEth().BatchNonce = original.BatchNonce + 1
	require.Error(t, att.VerifyClaimHash(createTestCodec()))
	require.Contains(t, att.VerifyClaimHash(createTestCodec()).Error(), "claim hash from components does not match")
}

// Compatibility: legacy attestation without ClaimComponents should be accepted
// by genesis import (handled in genesis.go), and VerifyClaimHash should fail gracefully.
func TestVerifyClaimHashLegacyNilComponents(t *testing.T) {
	//nolint: exhaustruct
	original := MsgBatchSendToEthClaim{
		EventNonce:     NonzeroUint64(),
		EthBlockHeight: NonzeroUint64(),
		BatchNonce:     NonzeroUint64(),
		TokenContract:  NonemptyEthAddress(),
		Orchestrator:   NonemptySdkAccAddress().String(),
	}
	anyClaim, err := codectypes.NewAnyWithValue(&original)
	require.NoError(t, err)

	//nolint: exhaustruct
	att := Attestation{
		Claim:     anyClaim,
		ClaimType: CLAIM_TYPE_BATCH_SEND_TO_ETH,
		// ClaimComponents is nil (legacy)
	}

	require.Error(t, att.VerifyClaimHash(createTestCodec()))
	require.Contains(t, att.VerifyClaimHash(createTestCodec()).Error(), "nil claim components")
}

func createTestCodec() codec.BinaryCodec {
	// nolint: exhaustruct
	signOptions := signing.Options{
		AddressCodec: address.Bech32Codec{
			Bech32Prefix: sdk.GetConfig().GetBech32AccountAddrPrefix(),
		},
		ValidatorAddressCodec: address.Bech32Codec{
			Bech32Prefix: sdk.GetConfig().GetBech32ValidatorAddrPrefix(),
		},
	}
	interfaceRegistry, err := codectypes.NewInterfaceRegistryWithOptions(codectypes.InterfaceRegistryOptions{
		ProtoFiles:     proto.HybridResolver,
		SigningOptions: signOptions,
	})
	if err != nil {
		panic(fmt.Errorf("failed to create interface registry: %w", err))
	}
	RegisterInterfaces(interfaceRegistry)
	return codec.NewProtoCodec(interfaceRegistry)
}
