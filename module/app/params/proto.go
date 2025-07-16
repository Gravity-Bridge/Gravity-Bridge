package params

import (
	"cosmossdk.io/x/tx/signing"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/address"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	authtx "github.com/cosmos/cosmos-sdk/x/auth/tx"
	"github.com/cosmos/gogoproto/proto"

	gravityconfig "github.com/Gravity-Bridge/Gravity-Bridge/module/config"
)

// MakeEncodingConfig creates an EncodingConfig for an amino based test configuration.
func MakeEncodingConfig() EncodingConfig {
	interfaceRegistry, _ := codectypes.NewInterfaceRegistryWithOptions(
		codectypes.InterfaceRegistryOptions{
			ProtoFiles: proto.HybridResolver,
			SigningOptions: signing.Options{
				AddressCodec: address.Bech32Codec{
					Bech32Prefix: gravityconfig.Bech32PrefixAccAddr,
				},
				ValidatorAddressCodec: address.Bech32Codec{
					Bech32Prefix: gravityconfig.Bech32PrefixValAddr,
				},
			},
		},
	)
	appCodec := codec.NewProtoCodec(interfaceRegistry)
	legacyAmino := codec.NewLegacyAmino()
	txConfig := authtx.NewTxConfig(appCodec, authtx.DefaultSignModes)

	return EncodingConfig{
		InterfaceRegistry: interfaceRegistry,
		Marshaler:         appCodec,
		TxConfig:          txConfig,
		Amino:             legacyAmino,
	}
}
