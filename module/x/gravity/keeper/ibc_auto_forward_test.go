package keeper

import (
	"context"
	"fmt"
	"testing"
	"time"

	"cosmossdk.io/math"
	bech32ibctypes "github.com/Gravity-Bridge/Gravity-Bridge/module/x/bech32ibc/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/bech32"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	transfertypes "github.com/cosmos/ibc-go/v8/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v8/modules/core/02-client/types"
	connectiontypes "github.com/cosmos/ibc-go/v8/modules/core/03-connection/types"
	channeltypes "github.com/cosmos/ibc-go/v8/modules/core/04-channel/types"
	commitmenttypes "github.com/cosmos/ibc-go/v8/modules/core/23-commitment/types"
	host "github.com/cosmos/ibc-go/v8/modules/core/24-host"
	"github.com/cosmos/ibc-go/v8/modules/core/exported"
	tendermint "github.com/cosmos/ibc-go/v8/modules/light-clients/07-tendermint"
	"github.com/stretchr/testify/require"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
)

// TestProcessNextPendingIbcAutoForward_TransferAtomicity tests that the IBC auto-forward process correctly handles
// atomicity of transfers, ensuring that funds are either successfully sent via IBC or remain in the fallback account
// in case of failure. It covers both escrow and burn transfer kinds and checks behavior for both active and expired clients.
func TestProcessNextPendingIbcAutoForward_TransferAtomicity(t *testing.T) {
	for _, transferKind := range []string{"escrow", "burn", "ibc-escrow", "missing-trace"} {
		for _, status := range []exported.Status{exported.Expired, exported.Active} {
			t.Run(transferKind+"/"+string(status), func(t *testing.T) {
				input, ctx := SetupFiveValChain(t)
				tendermint.RegisterInterfaces(input.EncodingConfig.InterfaceRegistry)
				ctx = ctx.WithBlockTime(time.Date(2026, 9, 21, 18, 0, 0, 0, time.UTC))
				input.IbcKeeper.ClientKeeper.SetParams(ctx, clienttypes.DefaultParams())
				gravityKeeper := input.GravityKeeper
				const channel = "channel-88"
				const clientID = "07-tendermint-156"
				const connectionID = "connection-150"
				const nonce = uint64(61427)
				foreignReceiver := "canto1wumaxapdm3nlz3pmgr97d4qp6eya8yr26gp2ke"
				fallback, err := types.IBCAddressFromBech32(foreignReceiver)
				require.NoError(t, err)

				// Use the real IBC keepers: an expired client rejects the packet after tokens have moved.
				height := clienttypes.NewHeight(1, 20818936)
				clientState := tendermint.NewClientState("canto_7700-1", tendermint.DefaultTrustLevel,
					14*24*time.Hour, 21*24*time.Hour, 40*time.Second, height, commitmenttypes.GetSDKSpecs(), nil)
				consensusTime := ctx.BlockTime()
				if status == exported.Expired {
					consensusTime = consensusTime.Add(-15 * 24 * time.Hour)
				}
				input.IbcKeeper.ClientKeeper.SetClientState(ctx, clientID, clientState)
				input.IbcKeeper.ClientKeeper.SetClientConsensusState(ctx, clientID, height,
					tendermint.NewConsensusState(consensusTime, commitmenttypes.NewMerkleRoot([]byte("root")), make([]byte, 32)))
				require.Equal(t, status, input.IbcKeeper.ClientKeeper.GetClientStatus(ctx, clientState, clientID))
				input.IbcKeeper.ConnectionKeeper.SetConnection(ctx, connectionID, connectiontypes.NewConnectionEnd(
					connectiontypes.OPEN, clientID, connectiontypes.Counterparty{}, connectiontypes.GetCompatibleVersions(), 0))
				input.IbcKeeper.ChannelKeeper.SetChannel(ctx, transfertypes.PortID, channel, channeltypes.NewChannel(
					channeltypes.OPEN, channeltypes.UNORDERED, channeltypes.NewCounterparty(transfertypes.PortID, "channel-0"),
					[]string{connectionID}, transfertypes.Version))
				input.IbcKeeper.ChannelKeeper.SetNextSequenceSend(ctx, transfertypes.PortID, channel, 1)
				capabilityPath := host.ChannelCapabilityPath(transfertypes.PortID, channel)
				capability, err := input.IbcScope.NewCapability(ctx, capabilityPath)
				require.NoError(t, err)
				require.NoError(t, input.TransferScope.ClaimCapability(ctx, capability, capabilityPath))
				input.IbcTransferKeeper.SetPort(ctx, transfertypes.PortID)
				input.IbcTransferKeeper.SetParams(ctx, transfertypes.DefaultParams())
				gravityKeeper.bech32IbcKeeper.SetHrpIbcRecords(ctx, []bech32ibctypes.HrpIbcRecord{
					{Hrp: "canto", SourceChannel: channel, IcsToHeightOffset: 5000},
				})
				gravityKeeper.setLastObservedEventNonce(ctx, nonce)

				// WETH is locked on Gravity for the outgoing packet. Returning Canto vouchers are burned instead.
				denom := "gravity20xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2"
				if transferKind != "escrow" {
					traceChannel := channel
					if transferKind == "ibc-escrow" {
						traceChannel = "channel-99"
					}
					trace := transfertypes.ParseDenomTrace("transfer/" + traceChannel + "/acanto")
					if transferKind != "missing-trace" {
						input.IbcTransferKeeper.SetDenomTrace(ctx, trace)
					}
					denom = trace.IBCDenom()
				}
				coin := sdk.NewInt64Coin(denom, 6000000000000)
				moduleAddr := input.AccountKeeper.GetModuleAddress(types.ModuleName)
				escrowAddr := transfertypes.GetEscrowAddress(transfertypes.PortID, channel)
				transferAddr := input.AccountKeeper.GetModuleAddress(transfertypes.ModuleName)
				// Existing holdings must survive too; checking only empty accounts could miss an over-refund.
				moduleBefore := coin.Add(sdk.NewInt64Coin(denom, 11))
				fallbackBefore := sdk.NewInt64Coin(denom, 17)
				escrowBefore := sdk.NewInt64Coin(denom, 23)
				transferBefore := sdk.NewInt64Coin(denom, 29)
				supplyBefore := coin.Add(moduleBefore).Add(fallbackBefore).Add(escrowBefore).Add(transferBefore)
				require.NoError(t, input.BankKeeper.MintCoins(ctx, types.ModuleName, sdk.NewCoins(supplyBefore)))
				require.NoError(t, input.BankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, fallback, sdk.NewCoins(fallbackBefore)))
				require.NoError(t, input.BankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, escrowAddr, sdk.NewCoins(escrowBefore)))
				require.NoError(t, input.BankKeeper.SendCoinsFromModuleToModule(ctx, types.ModuleName, transfertypes.ModuleName, sdk.NewCoins(transferBefore)))
				input.IbcTransferKeeper.SetTotalEscrowForDenom(ctx, escrowBefore)
				forward := types.PendingIbcAutoForward{
					ForeignReceiver: foreignReceiver, Token: &coin, IbcChannel: channel, EventNonce: nonce,
				}
				require.NoError(t, gravityKeeper.addPendingIbcAutoForward(ctx, forward, "0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2"))

				success := status == exported.Active && transferKind != "missing-trace"
				t.Run("escrow-address-receiver", func(t *testing.T) {
					testIbcAutoForwardEscrowReceiver(t, input, ctx, forward, success, transferKind == "burn")
				})
				if success {
					faults := []string{"duplicated-credit", "unexpected-supply"}
					if transferKind != "burn" {
						faults = append(faults, "misdirected-escrow")
					}
					for _, fault := range faults {
						t.Run("reject-"+fault, func(t *testing.T) {
							expectedEscrow, expectedSupply := escrowBefore.Add(coin), supplyBefore
							if transferKind == "burn" {
								expectedEscrow, expectedSupply = escrowBefore, supplyBefore.Sub(coin)
							}
							actualFallback, actualEscrow, actualSupply := fallbackBefore, expectedEscrow, expectedSupply
							switch fault {
							case "duplicated-credit":
								actualFallback = fallbackBefore.Add(coin)
							case "unexpected-supply":
								actualSupply = expectedSupply.Add(coin)
							case "misdirected-escrow":
								actualEscrow = escrowBefore
							}
							input.BankKeeper.AppendSendRestriction(func(sendCtx context.Context, from, to sdk.AccAddress, coins sdk.Coins) (sdk.AccAddress, error) {
								if from.Equals(sdk.AccAddress(fallback)) {
									switch fault {
									case "duplicated-credit":
										require.NoError(t, input.BankKeeper.SendCoinsFromModuleToAccount(sendCtx, types.ModuleName, fallback, coins))
									case "unexpected-supply":
										require.NoError(t, input.BankKeeper.MintCoins(sendCtx, types.ModuleName, coins))
									case "misdirected-escrow":
										return moduleAddr, nil
									}
								}
								return to, nil
							})
							t.Cleanup(input.BankKeeper.ClearSendRestriction)
							probeCtx, _ := ctx.CacheContext()
							require.PanicsWithValue(t,
								fmt.Sprintf("IBC auto-forward balance invariant violated for nonce %d: local %s (expected %s), escrow %s (expected %s), supply %s (expected %s)",
									nonce, actualFallback, fallbackBefore, actualEscrow, expectedEscrow, actualSupply, expectedSupply),
								func() { _, _ = gravityKeeper.ProcessNextPendingIbcAutoForward(probeCtx) })
						})
					}
				} else if transferKind != "missing-trace" {
					t.Run("reject-failed-rollback", func(t *testing.T) {
						probeCtx, _ := ctx.CacheContext()
						injected := false
						input.BankKeeper.AppendSendRestriction(func(sendCtx context.Context, from, to sdk.AccAddress, coins sdk.Coins) (sdk.AccAddress, error) {
							if from.Equals(sdk.AccAddress(fallback)) && !injected {
								injected = true
								require.NoError(t, input.BankKeeper.SendCoins(probeCtx, from, to, coins))
							}
							return to, nil
						})
						t.Cleanup(input.BankKeeper.ClearSendRestriction)
						actualEscrow := escrowBefore.Add(coin)
						if transferKind == "burn" {
							actualEscrow = escrowBefore
						}
						require.PanicsWithValue(t,
							fmt.Sprintf("IBC auto-forward balance invariant violated for nonce %d: local %s (expected %s), escrow %s (expected %s), supply %s (expected %s)",
								nonce, fallbackBefore, fallbackBefore.Add(coin), actualEscrow, escrowBefore, supplyBefore, supplyBefore),
							func() { _, _ = gravityKeeper.ProcessNextPendingIbcAutoForward(probeCtx) })
					})
				}

				eventsBefore := len(ctx.EventManager().Events())
				stop, err := gravityKeeper.ProcessNextPendingIbcAutoForward(ctx)
				require.NoError(t, err)
				require.False(t, stop)
				expectedFallback, expectedEscrow, expectedSupply := fallbackBefore, escrowBefore, supplyBefore
				expectedSequence := uint64(1)
				expectedTransfers, expectedLocal := 0, 1
				expectedBurns := 0
				if !success {
					// Keep the local credit, but discard the attempted IBC debit, supply changes and events.
					expectedFallback = fallbackBefore.Add(coin)
				} else {
					// A successful send must keep both the token movement and exactly one packet's events.
					expectedSequence = 2
					expectedTransfers, expectedLocal = 1, 0
					if transferKind != "burn" {
						expectedEscrow = escrowBefore.Add(coin)
					} else {
						expectedSupply = supplyBefore.Sub(coin)
						expectedBurns = 1
					}
				}
				localEvent, err := sdk.TypedEventToEvent(&types.EventSendToCosmosLocal{})
				require.NoError(t, err)
				successEvent, err := sdk.TypedEventToEvent(&types.EventSendToCosmosExecutedIbcAutoForward{})
				require.NoError(t, err)
				assertOutcome := func() {
					t.Helper()
					require.Nil(t, gravityKeeper.GetNextPendingIbcAutoForward(ctx))
					require.Equal(t, moduleBefore, input.BankKeeper.GetBalance(ctx, moduleAddr, denom))
					require.Equal(t, transferBefore, input.BankKeeper.GetBalance(ctx, transferAddr, denom))
					require.Equal(t, expectedFallback, input.BankKeeper.GetBalance(ctx, fallback, denom))
					require.Equal(t, expectedEscrow, input.BankKeeper.GetBalance(ctx, escrowAddr, denom))
					require.Equal(t, expectedEscrow, input.IbcTransferKeeper.GetTotalEscrowForDenom(ctx, denom))
					require.Equal(t, expectedSupply, input.BankKeeper.GetSupply(ctx, denom))
					sequence, found := input.IbcKeeper.ChannelKeeper.GetNextSequenceSend(ctx, transfertypes.PortID, channel)
					require.True(t, found)
					require.Equal(t, expectedSequence, sequence)
					commitment := input.IbcKeeper.ChannelKeeper.GetPacketCommitment(ctx, transfertypes.PortID, channel, 1)
					require.Equal(t, success, len(commitment) > 0)
					require.Empty(t, input.IbcKeeper.ChannelKeeper.GetPacketCommitment(ctx, transfertypes.PortID, channel, 2))
					eventCounts := make(map[string]int)
					for _, event := range ctx.EventManager().Events()[eventsBefore:] {
						eventCounts[event.Type]++
					}
					require.Equal(t, expectedTransfers, eventCounts[transfertypes.EventTypeTransfer])
					require.Equal(t, expectedTransfers, eventCounts[channeltypes.EventTypeSendPacket])
					require.Equal(t, expectedTransfers, eventCounts[successEvent.Type])
					require.Equal(t, expectedLocal, eventCounts[localEvent.Type])
					// Failed attempts must not leak bank events either; only the initial local credit remains.
					require.Equal(t, 1+expectedTransfers, eventCounts[banktypes.EventTypeTransfer])
					require.Equal(t, 1+expectedTransfers, eventCounts[banktypes.EventTypeCoinReceived])
					require.Equal(t, 1+expectedTransfers+expectedBurns, eventCounts[banktypes.EventTypeCoinSpent])
					require.Equal(t, expectedBurns, eventCounts[banktypes.EventTypeCoinBurn])
					require.Zero(t, eventCounts[banktypes.EventTypeCoinMint])
				}
				assertOutcome()
				params, err := gravityKeeper.GetParams(ctx)
				require.NoError(t, err)
				require.True(t, params.BridgeActive)
				// The forward was consumed even on failure. Trying again must not credit or send anything twice.
				eventsAfter := len(ctx.EventManager().Events())
				stop, err = gravityKeeper.ProcessNextPendingIbcAutoForward(ctx)
				require.NoError(t, err)
				require.True(t, stop)
				assertOutcome()
				require.Len(t, ctx.EventManager().Events(), eventsAfter)
			})
		}
	}
}

func testIbcAutoForwardEscrowReceiver(t *testing.T, input TestInput, ctx sdk.Context, forward types.PendingIbcAutoForward, success, burn bool) {
	t.Helper()
	probeCtx, _ := ctx.CacheContext()
	gravityKeeper := input.GravityKeeper
	coin := *forward.Token
	denom, channel := coin.Denom, forward.IbcChannel
	moduleAddr := input.AccountKeeper.GetModuleAddress(types.ModuleName)
	transferAddr := input.AccountKeeper.GetModuleAddress(transfertypes.ModuleName)
	escrowAddr := transfertypes.GetEscrowAddress(transfertypes.PortID, channel)
	fallback, err := types.IBCAddressFromBech32(forward.ForeignReceiver)
	require.NoError(t, err)
	escrowBefore := input.BankKeeper.GetBalance(probeCtx, escrowAddr, denom)
	transferBefore := input.BankKeeper.GetBalance(probeCtx, transferAddr, denom)
	expectedFallback := input.BankKeeper.GetBalance(probeCtx, fallback, denom)
	expectedModule := input.BankKeeper.GetBalance(probeCtx, moduleAddr, denom)
	expectedSupply := input.BankKeeper.GetSupply(probeCtx, denom)
	expectedEscrowTotal := input.IbcTransferKeeper.GetTotalEscrowForDenom(probeCtx, denom)

	sharedReceiver, err := bech32.ConvertAndEncode("canto", escrowAddr)
	require.NoError(t, err)
	sharedForward := forward
	sharedForward.ForeignReceiver = sharedReceiver
	require.NoError(t, gravityKeeper.deletePendingIbcAutoForward(probeCtx, forward.EventNonce))
	require.NoError(t, gravityKeeper.addPendingIbcAutoForward(probeCtx, sharedForward, ""))
	nextForward := forward
	nextForward.EventNonce++
	gravityKeeper.setLastObservedEventNonce(probeCtx, nextForward.EventNonce)
	require.NoError(t, gravityKeeper.addPendingIbcAutoForward(probeCtx, nextForward, ""))

	expectedShared := escrowBefore.Add(coin)
	if success {
		if burn {
			expectedShared = escrowBefore
			expectedSupply = expectedSupply.Sub(coin)
		} else {
			expectedEscrowTotal = expectedEscrowTotal.Add(coin)
		}
	}
	expectedSequence := uint64(1)
	for attempt := uint64(0); attempt < 2; attempt++ {
		expectedModule = expectedModule.Sub(coin)
		if attempt == 1 {
			switch {
			case !success:
				expectedFallback = expectedFallback.Add(coin)
			case burn:
				expectedSupply = expectedSupply.Sub(coin)
			default:
				expectedShared = expectedShared.Add(coin)
				expectedEscrowTotal = expectedEscrowTotal.Add(coin)
			}
		}
		stop, err := gravityKeeper.ProcessNextPendingIbcAutoForward(probeCtx)
		require.NoError(t, err)
		require.False(t, stop)
		if attempt == 0 {
			require.Equal(t, &nextForward, gravityKeeper.GetNextPendingIbcAutoForward(probeCtx))
		} else {
			require.Nil(t, gravityKeeper.GetNextPendingIbcAutoForward(probeCtx))
		}
		require.Equal(t, expectedModule, input.BankKeeper.GetBalance(probeCtx, moduleAddr, denom))
		require.Equal(t, expectedShared, input.BankKeeper.GetBalance(probeCtx, escrowAddr, denom))
		require.Equal(t, expectedEscrowTotal, input.IbcTransferKeeper.GetTotalEscrowForDenom(probeCtx, denom))
		require.Equal(t, expectedFallback, input.BankKeeper.GetBalance(probeCtx, fallback, denom))
		require.Equal(t, transferBefore, input.BankKeeper.GetBalance(probeCtx, transferAddr, denom))
		require.Equal(t, expectedSupply, input.BankKeeper.GetSupply(probeCtx, denom))
		if success {
			expectedSequence++
		}
		sequence, found := input.IbcKeeper.ChannelKeeper.GetNextSequenceSend(probeCtx, transfertypes.PortID, channel)
		require.True(t, found)
		require.Equal(t, expectedSequence, sequence)
		commitment := input.IbcKeeper.ChannelKeeper.GetPacketCommitment(probeCtx, transfertypes.PortID, channel, attempt+1)
		require.Equal(t, success, len(commitment) > 0)
	}
}

func TestValidatePendingIbcAutoForward_BadDenom(t *testing.T) {
	input, ctx := SetupFiveValChain(t)

	// Register a foreign HRP so the address is valid and foreign.
	foreignHrp := "astro"
	rec := bech32ibctypes.HrpIbcRecord{
		Hrp:               foreignHrp,
		SourceChannel:     "channel-0",
		IcsToHeightOffset: 1000,
		IcsToTimeOffset:   1000,
	}
	input.GravityKeeper.bech32IbcKeeper.SetHrpIbcRecords(ctx, []bech32ibctypes.HrpIbcRecord{rec})

	// Provide module balance so funds check passes.
	coins := sdk.NewCoins(sdk.NewCoin("ugraviton", math.NewInt(1000)))
	require.NoError(t, input.BankKeeper.MintCoins(ctx, types.ModuleName, coins))

	foreignAddr, err := bech32.ConvertAndEncode(foreignHrp, []byte{1})
	require.NoError(t, err)

	fwd := types.PendingIbcAutoForward{
		ForeignReceiver: foreignAddr,
		Token:           &sdk.Coin{Denom: "ibc/gravity0xbad", Amount: math.NewInt(1)},
		EventNonce:      1,
		IbcChannel:      "channel-0",
	}

	err = input.GravityKeeper.ValidatePendingIbcAutoForward(ctx, fwd)
	require.ErrorIs(t, err, types.ErrInvalidDenom)
}

func TestProcessNextPendingIbcAutoForward_BadDenom(t *testing.T) {
	input, ctx := SetupFiveValChain(t)
	// Invariant assertion is skipped because we intentionally persist a bad denom to state.

	foreignHrp := "astro"
	rec := bech32ibctypes.HrpIbcRecord{
		Hrp:               foreignHrp,
		SourceChannel:     "channel-0",
		IcsToHeightOffset: 1000,
		IcsToTimeOffset:   1000,
	}
	input.GravityKeeper.bech32IbcKeeper.SetHrpIbcRecords(ctx, []bech32ibctypes.HrpIbcRecord{rec})

	foreignAddr, err := bech32.ConvertAndEncode(foreignHrp, []byte{1})
	require.NoError(t, err)

	// Manually store a forward with an invalid denom to test the panic path.
	store := ctx.KVStore(input.GravityKeeper.storeKey)
	key := types.GetPendingIbcAutoForwardKey(1)
	fwd := types.PendingIbcAutoForward{
		ForeignReceiver: foreignAddr,
		Token:           &sdk.Coin{Denom: "ibc/gravity0xbad", Amount: math.NewInt(1)},
		EventNonce:      1,
		IbcChannel:      "channel-0",
	}
	store.Set(key, input.GravityKeeper.cdc.MustMarshal(&fwd))

	require.Panics(t, func() {
		_, err = input.GravityKeeper.ProcessNextPendingIbcAutoForward(ctx)
		require.Error(t, err)
	})
}
