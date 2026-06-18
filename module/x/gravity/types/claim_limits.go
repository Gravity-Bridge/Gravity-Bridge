package types

import (
	errorsmod "cosmossdk.io/errors"
)

const (
	// MaxTokenNameLength is the maximum length of a token name in ERC20 deployment claims.
	MaxTokenNameLength = 256
	// MaxTokenSymbolLength is the maximum length of a token symbol in ERC20 deployment claims.
	MaxTokenSymbolLength = 64
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

	case *MsgERC20DeployedClaim:
		if err := validateERC20AddressField(c.TokenContract, "token contract"); err != nil {
			return err
		}
		// Use ValidateStrictDenom for the full structural check (length, ASCII, separator,
		// IBC format, gravity prefix) rather than duplicating the length check here.
		if err := ValidateStrictDenom(c.CosmosDenom); err != nil {
			return errorsmod.Wrapf(ErrInvalidClaim, "invalid cosmos denom: %s", err)
		}
		if len(c.Name) > MaxTokenNameLength {
			return errorsmod.Wrapf(ErrInvalidClaim, "token name too long: %d > %d", len(c.Name), MaxTokenNameLength)
		}
		if len(c.Symbol) > MaxTokenSymbolLength {
			return errorsmod.Wrapf(ErrInvalidClaim, "token symbol too long: %d > %d", len(c.Symbol), MaxTokenSymbolLength)
		}

	case *MsgBatchSendToEthClaim:
		if err := validateERC20AddressField(c.TokenContract, "token contract"); err != nil {
			return err
		}

	case *MsgValsetUpdatedClaim:
		if err := validateERC20AddressField(c.RewardToken, "reward token"); err != nil {
			return err
		}
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
