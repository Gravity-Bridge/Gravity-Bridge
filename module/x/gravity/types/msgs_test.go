package types

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"strings"
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

func TestClaimHashLegacyEncoding(t *testing.T) {
	const contract = "0x1111111111111111111111111111111111111111"
	const sender = "0x2222222222222222222222222222222222222222"
	members := []BridgeValidator{
		{Power: 10, EthereumAddress: sender},
		{Power: 20, EthereumAddress: contract},
	}
	sortedMembers := []BridgeValidator{members[1], members[0]}
	cases := []struct {
		name   string
		claim  EthereumClaim
		fields []string
		digest string
	}{
		{"deposit", &MsgSendToCosmosClaim{EventNonce: 1, EthBlockHeight: 2, TokenContract: contract,
			Amount: math.NewInt(3), EthereumSender: sender, CosmosReceiver: "receiver"},
			[]string{"1", "2", contract, "3", sender, "receiver"},
			"5dbb381c520d10d6494da6b3fb92c2fd1e673f40f71a652f3f3a926cef14185d"},
		{"batch", &MsgBatchSendToEthClaim{EventNonce: 1, EthBlockHeight: 2, BatchNonce: 3, TokenContract: contract},
			[]string{"1", "2", "3", contract},
			"0a4d440dd4ea6c3ff0fba05187a425177f0832a9f6f2b2910435cfc5e7af4038"},
		{"deployment", &MsgERC20DeployedClaim{EventNonce: 1, EthBlockHeight: 2, CosmosDenom: "uatom",
			TokenContract: contract, Name: "Atom", Symbol: "ATOM", Decimals: 6},
			[]string{"1", "2", "uatom", contract, "Atom", "ATOM", "6"},
			"5162fbd0dade38e05fa37bf235393883311c548123f4a593b46233f57e633dba"},
		{"logic", &MsgLogicCallExecutedClaim{EventNonce: 1, EthBlockHeight: 2, InvalidationId: []byte{0, 255, 47}, InvalidationNonce: 3},
			[]string{"1", "2", string([]byte{0, 255, 47}), "3"},
			"48e9dabe2fcf5a98e8152e3abe5b8585d5c2aecf6ba59615ad960bfafffe0202"},
		{"valset", &MsgValsetUpdatedClaim{EventNonce: 1, ValsetNonce: 3, EthBlockHeight: 2,
			Members: members, RewardAmount: math.NewInt(4), RewardToken: contract},
			[]string{"1", "3", "2", fmt.Sprintf("%x", sortedMembers), "4", contract},
			"6444dbc90cdd59bb89cc7d90e87de8fcb1c6fb2c3b91f9c349c5f5b4d8362c1a"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			preimage := []byte(strings.Join(test.fields, AttestationSeparator))
			expected := sha256.Sum256(preimage)
			require.Equal(t, test.digest, fmt.Sprintf("%x", expected))
			components, err := ExtractClaimHashComponents(test.claim)
			require.NoError(t, err)
			hash, err := test.claim.ClaimHash()
			require.NoError(t, err)
			require.Equal(t, expected[:], hash)
			componentHash, err := components.ComputeClaimHash(test.claim.GetType())
			require.NoError(t, err)
			require.Equal(t, expected[:], componentHash)
		})
	}
	require.Equal(t, sortedMembers[1], members[0])
	require.Equal(t, sortedMembers[0], members[1])
}

func TestClaimComponentAmountNormalization(t *testing.T) {
	for _, claim := range []EthereumClaim{
		&MsgSendToCosmosClaim{Amount: math.NewInt(3)},
		&MsgValsetUpdatedClaim{RewardAmount: math.NewInt(3)},
	} {
		expected, err := claim.ClaimHash()
		require.NoError(t, err)
		components, err := ExtractClaimHashComponents(claim)
		require.NoError(t, err)
		switch component := components.Components.(type) {
		case *ClaimHashComponents_SendToCosmos:
			component.SendToCosmos.Amount = "+03"
		case *ClaimHashComponents_ValsetUpdated:
			component.ValsetUpdated.RewardAmount = "+03"
		}
		actual, err := components.ComputeClaimHash(claim.GetType())
		require.NoError(t, err)
		require.Equal(t, expected, actual)
	}
}

func TestHistoricalClaimSeparatorHashing(t *testing.T) {
	const contract = "0x1111111111111111111111111111111111111111"
	claims := []EthereumClaim{
		&MsgSendToCosmosClaim{TokenContract: contract, EthereumSender: contract,
			Amount: math.NewInt(1), CosmosReceiver: AttestationSeparator},
		&MsgBatchSendToEthClaim{TokenContract: AttestationSeparator},
		&MsgERC20DeployedClaim{CosmosDenom: "uatom", TokenContract: contract, Name: AttestationSeparator},
		&MsgLogicCallExecutedClaim{InvalidationId: []byte(AttestationSeparator)},
		&MsgValsetUpdatedClaim{RewardAmount: math.ZeroInt(), RewardToken: AttestationSeparator},
	}
	for _, claim := range claims {
		t.Run(claim.GetType().String(), func(t *testing.T) {
			require.ErrorIs(t, ValidateClaimFieldLengths(claim), ErrInvalidClaim)
			hash, err := claim.ClaimHash()
			require.NoError(t, err)
			components, err := ExtractClaimHashComponents(claim)
			require.NoError(t, err)
			componentHash, err := components.ComputeClaimHash(claim.GetType())
			require.NoError(t, err)
			require.Equal(t, hash, componentHash)
		})
	}
}

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

	hashes := getClaimHashStrings(t, &base, &mNonce, &mBlock, &mDenom, &mCtr, &mName, &mSymb, &mDecim)
	baseH := hashes[0]
	rest := hashes[1:]
	// Assert that the base claim hash differs from all the rest
	require.False(t, slices.Contains(rest, baseH))

	newClaims := setOrchestratorOnClaims(orchestrator, &base, &mNonce, &mBlock, &mDenom, &mCtr, &mName, &mSymb, &mDecim)
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

// TestReconstructClaimTypeMismatch covers an attestation whose stored ClaimType disagrees with its
// stored components, which only a hand-written genesis file can produce.
func TestReconstructClaimTypeMismatch(t *testing.T) {
	//nolint: exhaustruct
	claims := []EthereumClaim{
		&MsgSendToCosmosClaim{Amount: NonzeroSdkInt()},
		&MsgBatchSendToEthClaim{},
		&MsgERC20DeployedClaim{},
		&MsgLogicCallExecutedClaim{},
		&MsgValsetUpdatedClaim{RewardAmount: NonzeroSdkInt()},
	}

	for _, claim := range claims {
		components, err := ExtractClaimHashComponents(claim)
		require.NoError(t, err)

		reconstructed, err := ReconstructClaim(claim.GetType(), components)
		require.NoError(t, err)
		require.Equal(t, claim.GetType(), reconstructed.GetType())

		for _, other := range claims {
			if other.GetType() == claim.GetType() {
				continue
			}
			_, err := ReconstructClaim(other.GetType(), components)
			require.ErrorIs(t, err, ErrInvalidAttestation)
			require.Contains(t, err.Error(), "does not match stored components")
		}

		_, err = ReconstructClaim(CLAIM_TYPE_UNSPECIFIED, components)
		require.ErrorIs(t, err, ErrInvalidAttestation)
	}
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
