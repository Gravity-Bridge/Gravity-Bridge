package keeper

import (
	"testing"

	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
	channeltypes "github.com/cosmos/ibc-go/v8/modules/core/04-channel/types"
	"github.com/stretchr/testify/require"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/bech32ibc/types"
)

// mockChannelKeeper is a minimal in-memory stand-in for types.ChannelKeeper, allowing tests to
// control exactly which port/channel pairs are considered to exist.
type mockChannelKeeper struct {
	// openChannels maps "port/channel" -> exists
	openChannels map[string]bool
}

func newMockChannelKeeper(openChannels ...string) *mockChannelKeeper {
	m := &mockChannelKeeper{openChannels: map[string]bool{}}
	for _, c := range openChannels {
		m.openChannels[c] = true
	}
	return m
}

func (m *mockChannelKeeper) GetChannel(_ sdk.Context, srcPort, srcChan string) (channeltypes.Channel, bool) {
	if m.openChannels[srcPort+"/"+srcChan] {
		// nolint: exhaustruct
		return channeltypes.Channel{}, true
	}
	// nolint: exhaustruct
	return channeltypes.Channel{}, false
}

// mockTransferKeeper is a minimal stand-in for types.TransferKeeper with a fixed port.
type mockTransferKeeper struct {
	port string
}

func (m mockTransferKeeper) GetPort(_ sdk.Context) string {
	if m.port == "" {
		return "transfer"
	}
	return m.port
}

// testKeeperEnv bundles the objects needed to exercise the keeper in tests.
type testKeeperEnv struct {
	Keeper        Keeper
	Ctx           sdk.Context
	ChannelKeeper *mockChannelKeeper
}

// setupTestKeeper builds a Keeper backed by an in-memory store, along with a mock channel keeper
// pre-populated with the given "port/channel" strings marked as existing.
func setupTestKeeper(t *testing.T, openChannels ...string) testKeeperEnv {
	t.Helper()

	storeKey := storetypes.NewKVStoreKey(types.StoreKey)
	ctx := testutil.DefaultContext(storeKey, storetypes.NewTransientStoreKey("transient_test"))

	cdc := codec.NewProtoCodec(codectypes.NewInterfaceRegistry())
	channelKeeper := newMockChannelKeeper(openChannels...)
	transferKeeper := mockTransferKeeper{port: "transfer"}

	k := NewKeeper(channelKeeper, cdc, storeKey, transferKeeper)

	return testKeeperEnv{Keeper: *k, Ctx: ctx, ChannelKeeper: channelKeeper}
}

func TestSetGetNativeHrp(t *testing.T) {
	env := setupTestKeeper(t)

	t.Run("get before set returns ErrNoNativeHrp", func(t *testing.T) {
		_, err := env.Keeper.GetNativeHrp(env.Ctx)
		require.ErrorIs(t, err, types.ErrNoNativeHrp)
	})

	t.Run("set and get round trip", func(t *testing.T) {
		require.NoError(t, env.Keeper.SetNativeHrp(env.Ctx, "gravity"))

		hrp, err := env.Keeper.GetNativeHrp(env.Ctx)
		require.NoError(t, err)
		require.Equal(t, "gravity", hrp)
	})

	t.Run("set invalid hrp fails", func(t *testing.T) {
		err := env.Keeper.SetNativeHrp(env.Ctx, "INVALID")
		require.Error(t, err)
	})

	t.Run("set can overwrite an existing native hrp", func(t *testing.T) {
		require.NoError(t, env.Keeper.SetNativeHrp(env.Ctx, "osmo"))

		hrp, err := env.Keeper.GetNativeHrp(env.Ctx)
		require.NoError(t, err)
		require.Equal(t, "osmo", hrp)
	})
}

func TestLogger(t *testing.T) {
	env := setupTestKeeper(t)
	logger := env.Keeper.Logger(env.Ctx)
	require.NotNil(t, logger)
}
