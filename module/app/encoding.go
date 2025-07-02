package app

import (
	simappparams "cosmossdk.io/simapp/params"
	"cosmossdk.io/x/tx/signing"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/address"
	"github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/tx"
	authtx "github.com/cosmos/cosmos-sdk/x/auth/tx"
	"github.com/cosmos/gogoproto/proto"

	etherminttypes "github.com/evmos/ethermint/types"
)

// MakeEncodingConfig creates an EncodingConfig for gravity.
func MakeEncodingConfig() simappparams.EncodingConfig {
	legacyAmino := codec.NewLegacyAmino()
	signingOptions := signing.Options{
		AddressCodec: address.Bech32Codec{
			Bech32Prefix: sdk.GetConfig().GetBech32AccountAddrPrefix(),
		},
		ValidatorAddressCodec: address.Bech32Codec{
			Bech32Prefix: sdk.GetConfig().GetBech32ValidatorAddrPrefix(),
		},
	}
	interfaceRegistry, _ := types.NewInterfaceRegistryWithOptions(types.InterfaceRegistryOptions{
		ProtoFiles:     proto.HybridResolver,
		SigningOptions: signingOptions,
	})
	appCodec := codec.NewProtoCodec(interfaceRegistry)
	txConfig := authtx.NewTxConfig(appCodec, authtx.DefaultSignModes)

	encodingConfig := simappparams.EncodingConfig{
		InterfaceRegistry: interfaceRegistry,
		Codec:             appCodec,
		TxConfig:          txConfig,
		Amino:             legacyAmino,
	}
	// The EIP-712 signature extension option must be registered separately, as we do not want ethermint.v1.EthAccount to be registered
	// (Gravity only supports SDK x/auth accounts since it is not an Ethermint chain)
	encodingConfig.InterfaceRegistry.RegisterImplementations(
		(*tx.TxExtensionOptionI)(nil),
		// nolint: exhaustruct
		&etherminttypes.ExtensionOptionsWeb3Tx{},
	)

	return encodingConfig
}
