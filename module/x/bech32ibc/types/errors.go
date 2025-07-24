package types

// DONTCOVER

import (
	errorsmod "cosmossdk.io/errors"
)

// x/bech32ibc module sentinel errors
// ibc-go registers error codes under 10
var (
	ErrInvalidHRP                 = errorsmod.Register(ModuleName, 11, "Invalid HRP")
	ErrInvalidIBCData             = errorsmod.Register(ModuleName, 12, "Invalid IBC Data")
	ErrRecordNotFound             = errorsmod.Register(ModuleName, 13, "No record found for requested HRP")
	ErrNoNativeHrp                = errorsmod.Register(ModuleName, 14, "No native prefix was set")
	ErrInvalidOffsetHeightTimeout = errorsmod.Register(ModuleName, 15, "At least one of offset height or offset timeout should be set")
)
