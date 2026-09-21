package types

import (
	"strings"

	errorsmod "cosmossdk.io/errors"
)

const (
	// MaxTokenNameLength is the maximum length of a token name in ERC20 deployment claims.
	MaxTokenNameLength = 256
	// MaxTokenSymbolLength is the maximum length of a token symbol in ERC20 deployment claims.
	MaxTokenSymbolLength = 64
	// MaxCosmosReceiverLength is the maximum length of a SendToCosmos receiver. The
	// orchestrator applies the same bound and substitutes an empty receiver when it is
	// exceeded, so an oversized destination reaches the community pool rather than making the
	// claim impossible to submit and halting the oracle.
	MaxCosmosReceiverLength = 256
	// MaxInvalidationIdLength is the maximum length of a logic call invalidation id in Ethereum claims.
	MaxInvalidationIdLength = 32
)

// ValidateClaimFieldLengths enforces maximum lengths on oracle claim string fields
// to bound input size before any state changes are made.
func ValidateClaimFieldLengths(claim EthereumClaim) error {
	switch c := claim.(type) {
	case *MsgSendToCosmosClaim:
		if err := validateERC20AddressField(c.TokenContract, "token contract"); err != nil {
			return err
		}
		if err := validateERC20AddressField(c.EthereumSender, "ethereum sender"); err != nil {
			return err
		}
		if err := validateClaimTextField(c.CosmosReceiver, "cosmos receiver", MaxCosmosReceiverLength); err != nil {
			return err
		}

	case *MsgERC20DeployedClaim:
		if err := validateERC20AddressField(c.TokenContract, "token contract"); err != nil {
			return err
		}
		// Use ValidateStrictDenom for the full structural check (length, ASCII, separator,
		// IBC format, gravity prefix) rather than duplicating the length check here.
		if err := ValidateStrictDenom(c.CosmosDenom); err != nil {
			return errorsmod.Wrapf(ErrInvalidClaim, "invalid cosmos denom: %s", err)
		}
		if err := validateClaimTextField(c.Name, "token name", MaxTokenNameLength); err != nil {
			return err
		}
		if err := validateClaimTextField(c.Symbol, "token symbol", MaxTokenSymbolLength); err != nil {
			return err
		}

	case *MsgBatchSendToEthClaim:
		if err := validateERC20AddressField(c.TokenContract, "token contract"); err != nil {
			return err
		}

	case *MsgValsetUpdatedClaim:
		if err := validateERC20AddressField(c.RewardToken, "reward token"); err != nil {
			return err
		}

	case *MsgLogicCallExecutedClaim:
		if len(c.InvalidationId) > MaxInvalidationIdLength {
			return errorsmod.Wrapf(ErrInvalidClaim, "invalidation id too long: %d > %d", len(c.InvalidationId), MaxInvalidationIdLength)
		}

	default:
		return errorsmod.Wrapf(ErrInvalidClaim, "unrecognized claim type %T", claim)
	}
	return nil
}

// validateClaimTextField bounds a free-form claim string and rejects the AttestationSeparator,
// which would otherwise let the field forge a boundary in the ClaimHash pre-image.
func validateClaimTextField(value, fieldName string, maxLen int) error {
	if len(value) > maxLen {
		return errorsmod.Wrapf(ErrInvalidClaim, "%s too long: %d > %d", fieldName, len(value), maxLen)
	}
	if strings.Contains(value, AttestationSeparator) {
		return errorsmod.Wrapf(ErrInvalidClaim, "%s contains forbidden separator", fieldName)
	}
	return nil
}

// validateERC20AddressField checks that an ERC20 address string is within the allowed
// length and has valid hex encoding with a 0x prefix.
func validateERC20AddressField(addr, fieldName string) error {
	if len(addr) > ETHContractAddressLen {
		return errorsmod.Wrapf(ErrInvalidClaim, "%s too long: %d > %d", fieldName, len(addr), ETHContractAddressLen)
	}
	if err := ValidateEthAddress(addr); err != nil {
		return errorsmod.Wrapf(ErrInvalidClaim, "invalid %s: %s", fieldName, err)
	}
	return nil
}
