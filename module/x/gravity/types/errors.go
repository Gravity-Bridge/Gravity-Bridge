package types

import (
	errorsmod "cosmossdk.io/errors"
)

var (
	ErrInternal                 = errorsmod.Register(ModuleName, 1, "internal")
	ErrDuplicate                = errorsmod.Register(ModuleName, 2, "duplicate")
	ErrInvalid                  = errorsmod.Register(ModuleName, 3, "invalid")
	ErrTimeout                  = errorsmod.Register(ModuleName, 4, "timeout")
	ErrUnknown                  = errorsmod.Register(ModuleName, 5, "unknown")
	ErrEmpty                    = errorsmod.Register(ModuleName, 6, "empty")
	ErrOutdated                 = errorsmod.Register(ModuleName, 7, "outdated")
	ErrUnsupported              = errorsmod.Register(ModuleName, 8, "unsupported")
	ErrNonContiguousEventNonce  = errorsmod.Register(ModuleName, 9, "non contiguous event nonce, expected: %v received: %v")
	ErrResetDelegateKeys        = errorsmod.Register(ModuleName, 10, "can not set orchestrator addresses more than once")
	ErrMismatched               = errorsmod.Register(ModuleName, 11, "mismatched")
	ErrNoValidators             = errorsmod.Register(ModuleName, 12, "no bonded validators in active set")
	ErrInvalidValAddress        = errorsmod.Register(ModuleName, 13, "invalid validator address in current valset %v")
	ErrInvalidEthAddress        = errorsmod.Register(ModuleName, 14, "discovered invalid eth address stored for validator %v")
	ErrInvalidValset            = errorsmod.Register(ModuleName, 15, "generated invalid valset")
	ErrDuplicateEthereumKey     = errorsmod.Register(ModuleName, 16, "duplicate ethereum key")
	ErrDuplicateOrchestratorKey = errorsmod.Register(ModuleName, 17, "duplicate orchestrator key")
	ErrInvalidAttestation       = errorsmod.Register(ModuleName, 18, "invalid attestation submitted")
	ErrInvalidClaim             = errorsmod.Register(ModuleName, 19, "invalid claim submitted")
	ErrInvalidLogicCall         = errorsmod.Register(ModuleName, 20, "invalid logic call submitted")
)
