package app

import (
	"github.com/cosmos/cosmos-sdk/std"

	ethermintcryptocodec "github.com/evmos/ethermint/crypto/codec"
	ethermintcodec "github.com/evmos/ethermint/encoding/codec"
	etherminttypes "github.com/evmos/ethermint/types"

	gravityparams "github.com/Gravity-Bridge/Gravity-Bridge/module/app/params"
)

// MakeEncodingConfig creates an EncodingConfig for gravity.
func MakeEncodingConfig() gravityparams.EncodingConfig {
	encodingConfig := gravityparams.MakeEncodingConfig()
	ethermintcodec.RegisterLegacyAminoCodec(encodingConfig.Amino)
	ethermintcryptocodec.RegisterInterfaces(encodingConfig.InterfaceRegistry)
	std.RegisterInterfaces(encodingConfig.InterfaceRegistry)
	ModuleBasics.RegisterLegacyAminoCodec(encodingConfig.Amino)
	ModuleBasics.RegisterInterfaces(encodingConfig.InterfaceRegistry)

	// The EIP-712 signature extension option must be registered separately, as we do not want ethermint.v1.EthAccount to be registered
	// (Gravity only supports SDK x/auth accounts since it is not an Ethermint chain)
	// nolint: exhaustruct
	encodingConfig.InterfaceRegistry.RegisterInterface(
		"ethermint.v1.ExtensionOptionsWeb3Tx",
		(*etherminttypes.ExtensionOptionsWeb3TxI)(nil),
		&etherminttypes.ExtensionOptionsWeb3Tx{},
	)

	return encodingConfig
}
