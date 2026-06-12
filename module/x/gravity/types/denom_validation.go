package types

import (
	"strings"

	errorsmod "cosmossdk.io/errors"
)

// AttestationSeparator is a reserved string that denoms must never contain,
// used for separating fields in the ClaimHash from validator claims
const AttestationSeparator = "G\u0304\u0310\u030f\u030d\u0304\u0313\u0303\u0308\u030e\u0322\u0308\u030e\u0322\u0300\u0308\u030e\u0322\u0301\u0352\u0357\u0320\u0300\u0357\u0340\u0309\u0321\u0357\u0330\u0350\u0308\u0301"

// MaxDenomLength is the maximum allowed length for a denom string.
const MaxDenomLength = 256

func ValidateStrictDenom(denom string) error {
	if len(denom) == 0 {
		return errorsmod.Wrap(ErrInvalidDenom, "denom is empty")
	}
	if len(denom) > MaxDenomLength {
		return errorsmod.Wrapf(ErrInvalidDenom, "denom exceeds maximum length of %d", MaxDenomLength)
	}

	if !isASCII(denom) {
		return errorsmod.Wrap(ErrInvalidDenom, "denom contains non-ASCII characters")
	}

	if strings.Contains(denom, AttestationSeparator) {
		return errorsmod.Wrapf(ErrInvalidDenom, "denom contains forbidden separator %s", AttestationSeparator)
	}

	if strings.Contains(denom, `\`) {
		return errorsmod.Wrap(ErrInvalidDenom, "denom contains forbidden backslash")
	}

	isIbc := strings.HasPrefix(denom, "ibc/")
	if isIbc {
		if len(denom) <= len("ibc/") {
			return errorsmod.Wrap(ErrInvalidDenom, "IBC denom too short")
		}
		// The hash portion (after "ibc/") must not contain any further slashes
		if strings.Contains(denom[len("ibc/"):], "/") {
			return errorsmod.Wrap(ErrInvalidDenom, "IBC denom contains extra slash in hash")
		}
		if strings.Contains(denom, "gravity") ||
			strings.Contains(denom, "graviton") ||
			strings.Contains(denom, "0x") {
			return errorsmod.Wrap(ErrInvalidDenom, "IBC denom contains forbidden substring")
		}
	} else {
		if strings.Contains(denom, "/") {
			return errorsmod.Wrap(ErrInvalidDenom, "non-IBC denom contains slash")
		}
	}

	return nil
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] > 127 {
			return false
		}
	}
	return true
}
