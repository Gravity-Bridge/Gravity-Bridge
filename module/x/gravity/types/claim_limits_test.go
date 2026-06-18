package types

import (
	"strings"
	"testing"

	sdkmath "cosmossdk.io/math"

	"github.com/stretchr/testify/require"
)

func validERC20DeployedClaim() MsgERC20DeployedClaim {
	return MsgERC20DeployedClaim{
		EventNonce:    1,
		CosmosDenom:   "uatom",
		TokenContract: NonemptyEthAddress(),
		Name:          "Cosmos Hub Atom",
		Symbol:        "ATOM",
		Decimals:      6,
		Orchestrator:  NonemptySdkAccAddress().String(),
	}
}

func validSendToCosmosClaim() MsgSendToCosmosClaim {
	return MsgSendToCosmosClaim{
		EventNonce:     1,
		TokenContract:  NonemptyEthAddress(),
		Amount:         sdkmath.NewInt(100),
		EthereumSender: NonemptyEthAddress(),
		CosmosReceiver: NonemptySdkAccAddress().String(),
		Orchestrator:   NonemptySdkAccAddress().String(),
	}
}

func TestValidateClaimFieldLengths_ERC20Deployed_ValidClaim(t *testing.T) {
	claim := validERC20DeployedClaim()
	require.NoError(t, ValidateClaimFieldLengths(&claim))
}

func TestValidateClaimFieldLengths_ERC20Deployed_OversizedDenom(t *testing.T) {
	claim := validERC20DeployedClaim()
	claim.CosmosDenom = strings.Repeat("a", MaxDenomLength+1)
	require.ErrorIs(t, ValidateClaimFieldLengths(&claim), ErrInvalidClaim)
}

func TestValidateClaimFieldLengths_ERC20Deployed_InvalidDenomStructure(t *testing.T) {
	// A structurally invalid IBC denom (wrong length) should be caught by
	// ValidateStrictDenom inside ValidateClaimFieldLengths.
	claim := validERC20DeployedClaim()
	claim.CosmosDenom = "ibc/notahash" // wrong IBC format (length != 68)
	require.ErrorIs(t, ValidateClaimFieldLengths(&claim), ErrInvalidClaim)
}

func TestValidateClaimFieldLengths_ERC20Deployed_MalformedGravityDenom(t *testing.T) {
	// A gravity-prefixed denom that is too short should be caught by
	// ValidateStrictDenom's delegation to GravityDenomToERC20.
	claim := validERC20DeployedClaim()
	claim.CosmosDenom = "gravity0xbadaddr" // too short for gravity denom
	require.ErrorIs(t, ValidateClaimFieldLengths(&claim), ErrInvalidClaim)
}

func TestValidateClaimFieldLengths_ERC20Deployed_OversizedName(t *testing.T) {
	claim := validERC20DeployedClaim()
	claim.Name = strings.Repeat("a", MaxTokenNameLength+1)
	require.ErrorIs(t, ValidateClaimFieldLengths(&claim), ErrInvalidClaim)
}

func TestValidateClaimFieldLengths_ERC20Deployed_OversizedSymbol(t *testing.T) {
	claim := validERC20DeployedClaim()
	claim.Symbol = strings.Repeat("a", MaxTokenSymbolLength+1)
	require.ErrorIs(t, ValidateClaimFieldLengths(&claim), ErrInvalidClaim)
}

func TestValidateClaimFieldLengths_ERC20Deployed_OversizedTokenContract(t *testing.T) {
	claim := validERC20DeployedClaim()
	claim.TokenContract = "0x" + strings.Repeat("a", ETHContractAddressLen) // 42 + 2 chars → too long
	require.ErrorIs(t, ValidateClaimFieldLengths(&claim), ErrInvalidClaim)
}

func TestValidateClaimFieldLengths_ERC20Deployed_BadHexTokenContract(t *testing.T) {
	claim := validERC20DeployedClaim()
	claim.TokenContract = "0x" + strings.Repeat("zz", 20) // 42 chars but invalid hex
	require.ErrorIs(t, ValidateClaimFieldLengths(&claim), ErrInvalidClaim)
}

func TestValidateClaimFieldLengths_SendToCosmos_Valid(t *testing.T) {
	claim := validSendToCosmosClaim()
	require.NoError(t, ValidateClaimFieldLengths(&claim))
}

func TestValidateClaimFieldLengths_SendToCosmos_OversizedTokenContract(t *testing.T) {
	claim := validSendToCosmosClaim()
	claim.TokenContract = "0x" + strings.Repeat("a", ETHContractAddressLen)
	require.ErrorIs(t, ValidateClaimFieldLengths(&claim), ErrInvalidClaim)
}

func TestValidateClaimFieldLengths_SendToCosmos_OversizedEthereumSender(t *testing.T) {
	claim := validSendToCosmosClaim()
	claim.EthereumSender = "0x" + strings.Repeat("a", ETHContractAddressLen)
	require.ErrorIs(t, ValidateClaimFieldLengths(&claim), ErrInvalidClaim)
}

func TestValidateClaimFieldLengths_BatchSendToEth_Valid(t *testing.T) {
	claim := MsgBatchSendToEthClaim{
		EventNonce:    1,
		TokenContract: NonemptyEthAddress(),
		Orchestrator:  NonemptySdkAccAddress().String(),
	}
	require.NoError(t, ValidateClaimFieldLengths(&claim))
}

func TestValidateClaimFieldLengths_BatchSendToEth_OversizedTokenContract(t *testing.T) {
	claim := MsgBatchSendToEthClaim{
		EventNonce:    1,
		TokenContract: "0x" + strings.Repeat("a", ETHContractAddressLen),
		Orchestrator:  NonemptySdkAccAddress().String(),
	}
	require.ErrorIs(t, ValidateClaimFieldLengths(&claim), ErrInvalidClaim)
}

func TestValidateClaimFieldLengths_ValsetUpdated_Valid(t *testing.T) {
	claim := MsgValsetUpdatedClaim{
		EventNonce:  1,
		ValsetNonce: 1,
		RewardToken: NonemptyEthAddress(),
		Orchestrator: NonemptySdkAccAddress().String(),
	}
	require.NoError(t, ValidateClaimFieldLengths(&claim))
}

func TestValidateClaimFieldLengths_ValsetUpdated_OversizedRewardToken(t *testing.T) {
	claim := MsgValsetUpdatedClaim{
		EventNonce:  1,
		ValsetNonce: 1,
		RewardToken: "0x" + strings.Repeat("a", ETHContractAddressLen),
		Orchestrator: NonemptySdkAccAddress().String(),
	}
	require.ErrorIs(t, ValidateClaimFieldLengths(&claim), ErrInvalidClaim)
}

func TestValidateClaimFieldLengths_ValsetUpdated_BadHexRewardToken(t *testing.T) {
	claim := MsgValsetUpdatedClaim{
		EventNonce:  1,
		ValsetNonce: 1,
		RewardToken: "0x" + strings.Repeat("zz", 20), // 42 chars but invalid hex
		Orchestrator: NonemptySdkAccAddress().String(),
	}
	require.ErrorIs(t, ValidateClaimFieldLengths(&claim), ErrInvalidClaim)
}
