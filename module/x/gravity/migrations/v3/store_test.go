package v3

import (
	"testing"

	"github.com/stretchr/testify/require"

	storetypes "cosmossdk.io/store/types"
	"cosmossdk.io/x/evidence"
	"cosmossdk.io/x/tx/signing"
	"cosmossdk.io/x/upgrade"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/address"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/std"
	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/cosmos/cosmos-sdk/x/auth"
	"github.com/cosmos/cosmos-sdk/x/auth/vesting"
	"github.com/cosmos/cosmos-sdk/x/bank"
	"github.com/cosmos/cosmos-sdk/x/crisis"
	"github.com/cosmos/cosmos-sdk/x/distribution"
	"github.com/cosmos/cosmos-sdk/x/genutil"
	"github.com/cosmos/cosmos-sdk/x/gov"
	govclient "github.com/cosmos/cosmos-sdk/x/gov/client"
	"github.com/cosmos/cosmos-sdk/x/mint"
	"github.com/cosmos/cosmos-sdk/x/params"
	paramsclient "github.com/cosmos/cosmos-sdk/x/params/client"
	"github.com/cosmos/cosmos-sdk/x/slashing"
	"github.com/cosmos/cosmos-sdk/x/staking"
	proto "github.com/cosmos/gogoproto/proto"
	"github.com/cosmos/ibc-go/modules/capability"

	v2 "github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/migrations/v2"
	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
)

func TestMigrateAttestation(t *testing.T) {
	// create old prefixes KV store
	gravityKey := storetypes.NewKVStoreKey("gravity")
	ctx := testutil.DefaultContext(gravityKey, storetypes.NewTransientStoreKey("transient-test"))
	store := ctx.KVStore(gravityKey)
	marshaler := MakeTestMarshaler()

	nonce := uint64(1)

	msg := types.MsgBatchSendToEthClaim{
		EventNonce:     nonce,
		EthBlockHeight: 1,
		BatchNonce:     nonce,
		TokenContract:  "0x00000000000000000001",
		Orchestrator:   "0x00000000000000000004",
	}
	msgAny, err := codectypes.NewAnyWithValue(&msg)
	require.NoError(t, err)

	_, err = msg.ClaimHash()
	require.NoError(t, err)

	dummyAttestation := &types.Attestation{
		Observed: false,
		Votes:    []string{},
		Height:   uint64(1),
		Claim:    msgAny,
	}
	oldClaimHash, err := v2.MsgBatchSendToEthClaimHash(msg)
	require.NoError(t, err)
	newClaimHash, err := msg.ClaimHash()
	require.NoError(t, err)
	attestationOldKey := v2.GetAttestationKey(nonce, oldClaimHash)

	store.Set(attestationOldKey, marshaler.MustMarshal(dummyAttestation))

	// Run migrations
	err = MigrateStore(ctx, gravityKey, marshaler)
	require.NoError(t, err)

	oldKeyEntry := store.Get(attestationOldKey)
	newKeyEntry := store.Get(types.GetAttestationKey(nonce, newClaimHash))
	// Check migration results:
	require.Empty(t, oldKeyEntry)
	require.NotEqual(t, oldKeyEntry, newKeyEntry)
	require.NotEqual(t, newKeyEntry, []byte(""))
	require.NotEmpty(t, newKeyEntry)
}

// Need to duplicate these because of cyclical imports
// ModuleBasics is a mock module basic manager for testing
var (
	ModuleBasics = module.NewBasicManager(
		auth.AppModuleBasic{},
		genutil.AppModuleBasic{},
		bank.AppModuleBasic{},
		capability.AppModuleBasic{},
		staking.AppModuleBasic{},
		mint.AppModuleBasic{},
		distribution.AppModuleBasic{},
		gov.NewAppModuleBasic(
			[]govclient.ProposalHandler{paramsclient.ProposalHandler},
		),
		params.AppModuleBasic{},
		crisis.AppModuleBasic{},
		slashing.AppModuleBasic{},
		upgrade.AppModuleBasic{},
		evidence.AppModuleBasic{},
		vesting.AppModuleBasic{},
	)
)

// MakeTestMarshaler creates a proto codec for use in testing
func MakeTestMarshaler() codec.Codec {
	signingOptions := signing.Options{
		AddressCodec: address.Bech32Codec{
			Bech32Prefix: sdk.GetConfig().GetBech32AccountAddrPrefix(),
		},
		ValidatorAddressCodec: address.Bech32Codec{
			Bech32Prefix: sdk.GetConfig().GetBech32ValidatorAddrPrefix(),
		},
	}
	interfaceRegistry, _ := codectypes.NewInterfaceRegistryWithOptions(codectypes.InterfaceRegistryOptions{
		ProtoFiles:     proto.HybridResolver,
		SigningOptions: signingOptions,
	})
	std.RegisterInterfaces(interfaceRegistry)
	ModuleBasics.RegisterInterfaces(interfaceRegistry)
	types.RegisterInterfaces(interfaceRegistry)
	return codec.NewProtoCodec(interfaceRegistry)
}
