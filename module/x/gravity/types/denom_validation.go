package types

import (
	"strings"

	errorsmod "cosmossdk.io/errors"
)

// ibcHashLen is the number of uppercase hex characters in a SHA-256 IBC denom hash.
const ibcHashLen = 64

// ibcDenomLen is the exact byte length of a well-formed IBC denom: len("ibc/") + ibcHashLen.
const ibcDenomLen = len("ibc/") + ibcHashLen

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
		// A valid IBC denom is always "ibc/" + 64 uppercase hex chars (SHA-256). Enforce the
		// exact length so that any padding or truncation is caught early.
		if len(denom) != ibcDenomLen {
			return errorsmod.Wrapf(ErrInvalidDenom,
				"IBC denom must be exactly %d bytes (ibc/ + 64 hex chars), got %d",
				ibcDenomLen, len(denom))
		}
		// The hash must be uppercase hex only (A-F0-9); lowercase is not produced by the IBC
		// transfer module and could be used to sneak in unexpected characters.
		hash := denom[len("ibc/"):]
		if !isUpperHex(hash) {
			return errorsmod.Wrap(ErrInvalidDenom, "IBC denom hash must be uppercase hex")
		}
		// NOTE: these substrings cannot appear in a valid IBC denom (uppercase hex only),
		if strings.Contains(denom, "gravity") ||
			strings.Contains(denom, "graviton") ||
			strings.Contains(denom, "0x") {
			return errorsmod.Wrap(ErrInvalidDenom, "IBC denom contains forbidden substring")
		}
	} else if strings.HasPrefix(denom, Gravity2DenomPrefix) {
		// gravity2-prefixed denoms represent remapped Ethereum-originated tokens. They must be
		// exactly Gravity2DenomLen bytes and carry a well-formed Ethereum address.
		if _, err := Gravity2DenomToERC20(denom); err != nil {
			return errorsmod.Wrapf(ErrInvalidDenom, "invalid gravity2 denom: %s", err)
		}
		// match against gravity0x to avoid matching gravity2 prefix
	} else if strings.HasPrefix(denom, GravityDenomPrefix+"0x") {
		// gravity-prefixed denoms represent Ethereum-originated tokens bridged into Cosmos.
		// They must be exactly GravityDenomLen bytes and carry a well-formed Ethereum address.
		if _, err := GravityDenomToERC20(denom); err != nil {
			return errorsmod.Wrapf(ErrInvalidDenom, "invalid gravity denom: %s", err)
		}
	} else if strings.HasPrefix(denom, GravityDenomPrefix) {
		// No legitimate Cosmos denom starts with "gravity" other than the bridge denoms
		// validated above (gravity0x... and gravity20x...). The native token is "ugraviton"
		// which starts with "graviton", not "gravity". Reject anything else to prevent spoofing.
		return errorsmod.Wrap(ErrInvalidDenom, "denom has gravity prefix but is not a valid bridge denom")
	} else if strings.Contains(denom, "/") {
		return errorsmod.Wrap(ErrInvalidDenom, "non-IBC denom contains slash")
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

// isUpperHex reports whether every byte of s is an uppercase hex digit (0-9 or A-F).
func isUpperHex(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !((c >= '0' && c <= '9') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}
