package bech32ibc_test

import (
	"testing"

	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
	channeltypes "github.com/cosmos/ibc-go/v8/modules/core/04-channel/types"
	"github.com/stretchr/testify/require"

	bech32ibc "github.com/Gravity-Bridge/Gravity-Bridge/module/x/bech32ibc"
	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/bech32ibc/keeper"
	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/bech32ibc/types"
)

type mockChannelKeeper struct {
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

type mockTransferKeeper struct{}

func (mockTransferKeeper) GetPort(_ sdk.Context) string { return "transfer" }

func setupTestKeeper(t *testing.T, openChannels ...string) (keeper.Keeper, sdk.Context) {
	t.Helper()

	storeKey := storetypes.NewKVStoreKey(types.StoreKey)
	ctx := testutil.DefaultContext(storeKey, storetypes.NewTransientStoreKey("transient_test"))
	cdc := codec.NewProtoCodec(codectypes.NewInterfaceRegistry())

	k := keeper.NewKeeper(newMockChannelKeeper(openChannels...), cdc, storeKey, mockTransferKeeper{})
	return *k, ctx
}

func TestInitExportGenesis(t *testing.T) {
	k, ctx := setupTestKeeper(t, "transfer/channel-0")

	// nolint: exhaustruct
	genState := types.GenesisState{
		NativeHRP: "gravity",
		HrpIBCRecords: []types.HrpIbcRecord{
			// nolint: exhaustruct
			{Hrp: "osmo", SourceChannel: "channel-0"},
		},
	}

	bech32ibc.InitGenesis(ctx, k, genState)

	exported := bech32ibc.ExportGenesis(ctx, k)
	require.Equal(t, genState.NativeHRP, exported.NativeHRP)
	require.Equal(t, genState.HrpIBCRecords, exported.HrpIBCRecords)
}

func TestInitGenesis_PanicsOnInvalidNativeHrp(t *testing.T) {
	k, ctx := setupTestKeeper(t)

	// nolint: exhaustruct
	genState := types.GenesisState{NativeHRP: "INVALID"}

	require.Panics(t, func() {
		bech32ibc.InitGenesis(ctx, k, genState)
	})
}

func TestExportGenesis_PanicsWithoutNativeHrp(t *testing.T) {
	k, ctx := setupTestKeeper(t)

	require.Panics(t, func() {
		bech32ibc.ExportGenesis(ctx, k)
	})
}
