package types

import (
	"bytes"
	"fmt"

	errorsmod "cosmossdk.io/errors"
	sdkmath "cosmossdk.io/math"

	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/gogoproto/proto"
)

func (m Attestation) ValidateBasic(cdc codec.BinaryCodec) error {
	if m.Observed && len(m.Votes) == 0 {
		return errorsmod.Wrap(ErrInvalidAttestation, "must be voted on to be observed")
	}

	for _, validator := range m.Votes {
		_, err := sdk.ValAddressFromBech32(validator)
		if err != nil {
			return errorsmod.Wrap(ErrInvalidAttestation, "votes must contain bech32 validator addresses")
		}
	}

	err := ClaimValidateBasic(cdc, m.Claim)
	if err != nil {
		return errorsmod.Wrap(ErrInvalidAttestation, err.Error())
	}
	return nil
}

func ClaimValidateBasic(cdc codec.BinaryCodec, claim *codectypes.Any) error {
	var ethClaim EthereumClaim
	err := cdc.UnpackAny(claim, &ethClaim)
	if err != nil {
		return errorsmod.Wrap(ErrInvalidClaim, "unable to unmarshal claim")
	}
	if ethClaim == nil {
		return errorsmod.Wrap(ErrInvalidClaim, "decoded nil claim")
	}

	// Returns nil on no error from ValidateBasic
	err = ethClaim.ValidateBasic()
	if err != nil {
		return errorsmod.Wrap(ErrInvalidClaim, err.Error())
	}
	return nil
}

// ClaimTypeToTypeUrl takes a type of EthereumClaim and returns the associated protobuf Msg TypeUrl
// nolint: exhaustruct
func ClaimTypeToTypeUrl(claimType ClaimType) string {
	var msgName string
	switch claimType {
	case CLAIM_TYPE_UNSPECIFIED:
		return "unspecified"
	case CLAIM_TYPE_SEND_TO_COSMOS:
		msgName = proto.MessageName(&MsgSendToCosmosClaim{})
	case CLAIM_TYPE_BATCH_SEND_TO_ETH:
		msgName = proto.MessageName(&MsgBatchSendToEthClaim{})
	case CLAIM_TYPE_ERC20_DEPLOYED:
		msgName = proto.MessageName(&MsgERC20DeployedClaim{})
	case CLAIM_TYPE_LOGIC_CALL_EXECUTED:
		msgName = proto.MessageName(&MsgLogicCallExecutedClaim{})
	case CLAIM_TYPE_VALSET_UPDATED:
		msgName = proto.MessageName(&MsgValsetUpdatedClaim{})
	}

	return "/" + msgName
}

// ExtractClaimHashComponents takes any EthereumClaim and returns the populated ClaimHashComponents.
// It extracts exactly the fields used in ClaimHash() computation for each claim type.
func ExtractClaimHashComponents(claim EthereumClaim) (*ClaimHashComponents, error) {
	switch c := claim.(type) {
	case *MsgSendToCosmosClaim:
		return &ClaimHashComponents{
			Components: &ClaimHashComponents_SendToCosmos{
				SendToCosmos: &SendToCosmosClaimComponents{
					EventNonce:     c.EventNonce,
					EthBlockHeight: c.EthBlockHeight,
					TokenContract:  c.TokenContract,
					Amount:         c.Amount.String(),
					EthereumSender: c.EthereumSender,
					CosmosReceiver: c.CosmosReceiver,
				},
			},
		}, nil
	case *MsgBatchSendToEthClaim:
		return &ClaimHashComponents{
			Components: &ClaimHashComponents_BatchSendToEth{
				BatchSendToEth: &BatchSendToEthClaimComponents{
					EventNonce:     c.EventNonce,
					EthBlockHeight: c.EthBlockHeight,
					BatchNonce:     c.BatchNonce,
					TokenContract:  c.TokenContract,
				},
			},
		}, nil
	case *MsgERC20DeployedClaim:
		return &ClaimHashComponents{
			Components: &ClaimHashComponents_Erc20Deployed{
				Erc20Deployed: &ERC20DeployedClaimComponents{
					EventNonce:     c.EventNonce,
					EthBlockHeight: c.EthBlockHeight,
					CosmosDenom:    c.CosmosDenom,
					TokenContract:  c.TokenContract,
					Name:           c.Name,
					Symbol:         c.Symbol,
					Decimals:       c.Decimals,
				},
			},
		}, nil
	case *MsgLogicCallExecutedClaim:
		return &ClaimHashComponents{
			Components: &ClaimHashComponents_LogicCallExecuted{
				LogicCallExecuted: &LogicCallExecutedClaimComponents{
					EventNonce:        c.EventNonce,
					EthBlockHeight:    c.EthBlockHeight,
					InvalidationId:    c.InvalidationId,
					InvalidationNonce: c.InvalidationNonce,
				},
			},
		}, nil
	case *MsgValsetUpdatedClaim:
		// Replicate the exact sort-and-externalize sequence used in ClaimHash()
		members := BridgeValidators(c.Members)
		internalMembers, err := members.ToInternal()
		if err != nil {
			return nil, errorsmod.Wrap(err, "invalid valset members")
		}
		internalMembers.Sort()
		return &ClaimHashComponents{
			Components: &ClaimHashComponents_ValsetUpdated{
				ValsetUpdated: &ValsetUpdatedClaimComponents{
					EventNonce:     c.EventNonce,
					ValsetNonce:    c.ValsetNonce,
					EthBlockHeight: c.EthBlockHeight,
					Members:        internalMembers.ToExternal(),
					RewardAmount:   c.RewardAmount.String(),
					RewardToken:    c.RewardToken,
				},
			},
		}, nil
	default:
		return nil, errorsmod.Wrap(ErrInvalidClaim, fmt.Sprintf("unknown claim type %T", claim))
	}
}

// ReconstructClaim takes the stored ClaimHashComponents + a claim type and builds a full claim message.
// The reconstructed claim can be used to recompute the ClaimHash for verification.
func ReconstructClaim(claimType ClaimType, components *ClaimHashComponents) (EthereumClaim, error) {
	if components == nil {
		return nil, errorsmod.Wrap(ErrInvalidAttestation, "nil claim components")
	}

	switch c := components.Components.(type) {
	case *ClaimHashComponents_SendToCosmos:
		comp := c.SendToCosmos
		if comp == nil {
			return nil, errorsmod.Wrap(ErrInvalidAttestation, "nil SendToCosmos components")
		}
		amount, ok := sdkmath.NewIntFromString(comp.Amount)
		if !ok {
			return nil, errorsmod.Wrapf(ErrInvalidAttestation, "invalid amount %s", comp.Amount)
		}
		return &MsgSendToCosmosClaim{
			EventNonce:     comp.EventNonce,
			EthBlockHeight: comp.EthBlockHeight,
			TokenContract:  comp.TokenContract,
			Amount:         amount,
			EthereumSender: comp.EthereumSender,
			CosmosReceiver: comp.CosmosReceiver,
			Orchestrator:   "",
		}, nil

	case *ClaimHashComponents_BatchSendToEth:
		comp := c.BatchSendToEth
		if comp == nil {
			return nil, errorsmod.Wrap(ErrInvalidAttestation, "nil BatchSendToEth components")
		}
		return &MsgBatchSendToEthClaim{
			EventNonce:     comp.EventNonce,
			EthBlockHeight: comp.EthBlockHeight,
			BatchNonce:     comp.BatchNonce,
			TokenContract:  comp.TokenContract,
			Orchestrator:   "",
		}, nil

	case *ClaimHashComponents_Erc20Deployed:
		comp := c.Erc20Deployed
		if comp == nil {
			return nil, errorsmod.Wrap(ErrInvalidAttestation, "nil ERC20Deployed components")
		}
		return &MsgERC20DeployedClaim{
			EventNonce:     comp.EventNonce,
			EthBlockHeight: comp.EthBlockHeight,
			CosmosDenom:    comp.CosmosDenom,
			TokenContract:  comp.TokenContract,
			Name:           comp.Name,
			Symbol:         comp.Symbol,
			Decimals:       comp.Decimals,
			Orchestrator:   "",
		}, nil

	case *ClaimHashComponents_LogicCallExecuted:
		comp := c.LogicCallExecuted
		if comp == nil {
			return nil, errorsmod.Wrap(ErrInvalidAttestation, "nil LogicCallExecuted components")
		}
		return &MsgLogicCallExecutedClaim{
			EventNonce:        comp.EventNonce,
			EthBlockHeight:    comp.EthBlockHeight,
			InvalidationId:    comp.InvalidationId,
			InvalidationNonce: comp.InvalidationNonce,
			Orchestrator:      "",
		}, nil

	case *ClaimHashComponents_ValsetUpdated:
		comp := c.ValsetUpdated
		if comp == nil {
			return nil, errorsmod.Wrap(ErrInvalidAttestation, "nil ValsetUpdated components")
		}
		amount, ok := sdkmath.NewIntFromString(comp.RewardAmount)
		if !ok {
			return nil, errorsmod.Wrapf(ErrInvalidAttestation, "invalid reward_amount %s", comp.RewardAmount)
		}
		return &MsgValsetUpdatedClaim{
			EventNonce:     comp.EventNonce,
			ValsetNonce:    comp.ValsetNonce,
			EthBlockHeight: comp.EthBlockHeight,
			Members:        comp.Members,
			RewardAmount:   amount,
			RewardToken:    comp.RewardToken,
			Orchestrator:   "",
		}, nil

	default:
		return nil, errorsmod.Wrap(ErrInvalidClaim, fmt.Sprintf("unknown claim components type %T", components.Components))
	}
}

// ComputeClaimHash reconstructs the claim from components and calls ClaimHash() on it.
func (c *ClaimHashComponents) ComputeClaimHash(claimType ClaimType) ([]byte, error) {
	claim, err := ReconstructClaim(claimType, c)
	if err != nil {
		return nil, errorsmod.Wrap(err, "reconstruct claim")
	}
	return claim.ClaimHash()
}

// VerifyClaimHash verifies that the hash computed from the stored Any claim matches
// the hash computed from the individually stored claim components.
func (m Attestation) VerifyClaimHash(cdc codec.BinaryCodec) error {
	if m.ClaimComponents == nil {
		return errorsmod.Wrap(ErrInvalidAttestation, "nil claim components")
	}
	if m.Claim == nil {
		return errorsmod.Wrap(ErrInvalidAttestation, "nil claim")
	}

	var ethClaim EthereumClaim
	err := cdc.UnpackAny(m.Claim, &ethClaim)
	if err != nil {
		return errorsmod.Wrap(err, "unable to unmarshal stored claim")
	}
	if ethClaim == nil {
		return errorsmod.Wrap(ErrInvalidClaim, "decoded nil stored claim")
	}

	hashFromClaim, err := ethClaim.ClaimHash()
	if err != nil {
		return errorsmod.Wrap(err, "compute hash from stored claim")
	}

	hashFromComponents, err := m.ClaimComponents.ComputeClaimHash(m.ClaimType)
	if err != nil {
		return errorsmod.Wrap(err, "compute hash from stored components")
	}

	// Compare the hash from the claim and the components, they must match or tampering has occurred.
	if !bytes.Equal(hashFromClaim, hashFromComponents) {
		return errorsmod.Wrap(ErrInvalidAttestation, "claim hash from components does not match hash from stored claim")
	}

	return nil
}
