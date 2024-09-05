package ante

import (
	errorsmod "cosmossdk.io/errors"

	"github.com/cosmos/cosmos-sdk/codec"
	sdkcodec "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	sdkante "github.com/cosmos/cosmos-sdk/x/auth/ante"

	ethermint "github.com/evmos/ethermint/types"
)

// nolint: exhaustruct
var _ sdk.AnteDecorator = GravityRejectExtensionsDecorator{}

// ExtensionOptionsWeb3Tx does not have this constant stored on it anywhere, so we copy the value here
// See https://github.com/evmos/ethermint/blob/v0.19.3/app/ante/ante.go for the original typeURL switch statement
const WEB3_EXTENSION_TYPE_URL = "/ethermint.types.v1.ExtensionOptionsWeb3Tx"

// This Ante-Decorator performs the same EIP-712 signing support that Ethermint's does, but without
// requiring the use of two chains of Ante-Decorators and switching at runtime.
//
// To enable EIP-712 signing support, Gravity conditionally rejects extension options
// in the following cases:
// EIP-712: If the transaction has EXACTLY ONE ExtensionOptionsWeb3Tx on it, reject all others
// DEFAULT: Otherwise, reject all extension options (SDK default behavior)
type GravityRejectExtensionsDecorator struct {
	cdc codec.Codec
}

// See GravityRejectExtensionsDecorator for more info
func NewGravityRejectExtensionsDecorator(cdc codec.Codec) GravityRejectExtensionsDecorator {
	return GravityRejectExtensionsDecorator{cdc}
}

// See GravityRejectExtensionsDecorator for more info
func (red GravityRejectExtensionsDecorator) AnteHandle(
	ctx sdk.Context, tx sdk.Tx, simulate bool, next sdk.AnteHandler,
) (newCtx sdk.Context, err error) {
	_, er := IsWeb3Tx(red.cdc, tx)

	if er == nil {
		return next(ctx, tx, simulate)
	} else {
		return ctx, errorsmod.Wrap(er, "invalid transaction extension options")
	}
}

// Indicates if the given tx has any extensions, and returns those extensions
// Explicitly returns false for a tx with 0 extensions in case type casting behaves strangely
func GetTxExtensionOptions(tx sdk.Tx) (hasExtensions bool, extensions []*sdkcodec.Any) {
	if hasExtOptionsTx, ok := tx.(sdkante.HasExtensionOptionsTx); ok {
		// Has extension options
		extensions := hasExtOptionsTx.GetExtensionOptions()
		if len(extensions) == 0 {
			return false, nil
		} else {
			return true, extensions
		}
	} else {
		// No extension options
		return false, nil
	}
}

// Indicates if the given extension is a ExtensionOptionsWeb3Tx option, and returns the
// unpacked option as well.
// Otherwise returns (false, nil)
func IsWeb3TxOption(cdc codec.Codec, extension *sdkcodec.Any) (eip712Signed bool, web3Extension *ethermint.ExtensionOptionsWeb3Tx) {
	if extension == nil { // invalid input
		return false, nil
	}
	if extension.GetTypeUrl() != WEB3_EXTENSION_TYPE_URL {
		return false, nil
	}
	extOpt, ok := extension.GetCachedValue().(*ethermint.ExtensionOptionsWeb3Tx)
	if !ok {
		return false, nil
	}

	return true, extOpt
}

// Determines if the input tx is EIP-712 signed by looking at the extension options, signatures are verified in GravitySigVerificationDecorator
func IsWeb3Tx(cdc codec.Codec, tx sdk.Tx) (bool, error) {
	hasExtensions, extensions := GetTxExtensionOptions(tx)
	// No extensions, move on
	if !hasExtensions {
		return false, nil
	}

	// At least 1 extension has been found
	if len(extensions) > 1 {
		// Gravity only supports a single ExtensionOptionsWeb3Tx for EIP-712 signed messages
		return false, sdkerrors.ErrUnknownExtensionOptions
	}

	// Exactly 1 extension
	ok, _ := IsWeb3TxOption(cdc, extensions[0])
	if !ok {
		return false, sdkerrors.ErrUnknownExtensionOptions
	}

	return true, nil
}
