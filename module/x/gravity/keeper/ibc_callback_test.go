package keeper

import (
	"crypto/sha256"
	"fmt"
	"testing"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	transfertypes "github.com/cosmos/ibc-go/v3/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v3/modules/core/02-client/types"
	channeltypes "github.com/cosmos/ibc-go/v3/modules/core/04-channel/types"
	ibcmock "github.com/cosmos/ibc-go/v3/testing/mock"
	"github.com/stretchr/testify/require"
)

func TestOnRecvPacket(t *testing.T) {

	input := CreateTestEnv(t)
	defer func() { input.Context.Logger().Info("Asserting invariants at test end"); input.AssertInvariants() }()

	ctx := input.Context

	var (
		// Setup Oraichain <=> Orai Bridge IBC relayer
		sourceChannel     = "channel-0"
		oraibChannel      = "channel-1"
		tokenContractAddr = "0x429881672B9AE42b8EbA0E26cD9C73711b891Ca5"
		ethDestAddr       = "0xd041c41EA1bf0F006ADBb6d2c9ef9D425dE5eaD7"
		// if not create Claim from Deploy Erc20 contract, then denom = prefix + contract
		myTokenDenom = "bsc" + tokenContractAddr
		ibcDenom     = fmt.Sprintf("ibc/%X", sha256.Sum256([]byte("transfer/"+oraibChannel+"/"+myTokenDenom)))
		evmChain     = input.GravityKeeper.GetEvmChainData(ctx, EthChainPrefix)
	)

	tokenAddr, err := types.NewEthAddress(tokenContractAddr)
	require.NoError(t, err)

	// secp256k1 account for oraichain
	secpPk := secp256k1.GenPrivKey()
	gravityAddr := sdk.AccAddress(secpPk.PubKey().Address())
	oraiAddr := sdk.MustBech32ifyAddressBytes("orai", gravityAddr)

	path := fmt.Sprintf("%s/%s", transfertypes.PortID, oraibChannel)

	timeoutHeight := clienttypes.NewHeight(0, 100)
	expAck := ibcmock.MockAcknowledgement
	params := input.GravityKeeper.GetParams(ctx)

	// add it to the ERC20 registry
	// because this is one way from Oraichain to Gravity Bridge so just use the ibc token as default native token and mint some
	for _, evmChain := range input.GravityKeeper.GetEvmChains(ctx) {
		input.GravityKeeper.setCosmosOriginatedDenomToERC20(ctx, evmChain.EvmChainPrefix, ibcDenom, *tokenAddr)
		isCosmosOriginated, addr, err := input.GravityKeeper.DenomToERC20Lookup(ctx, evmChain.EvmChainPrefix, ibcDenom)
		require.True(t, isCosmosOriginated)
		require.NoError(t, err)
		require.Equal(t, tokenAddr.GetAddress().Hex(), tokenContractAddr)
		require.Equal(t, tokenAddr, addr)
	}

	require.NoError(t, input.BankKeeper.MintCoins(input.Context, types.ModuleName, sdk.NewCoins(
		sdk.NewCoin(ibcDenom, sdk.NewInt(1000)), // some IBC coin with a registered token pair
	)))

	testCases := []struct {
		name        string
		getPacket   func() channeltypes.Packet
		ackSuccess  bool
		expectedRes types.QueryPendingSendToEthResponse
	}{
		{
			name: "ibc conversion - auto forward to evm chain",
			getPacket: func() channeltypes.Packet {
				// Send bsc from Oraichain to OraiBridge in SendPacket method, the denom is extracted by calling DenomPathFromHash()
				transfer := transfertypes.NewFungibleTokenPacketData(myTokenDenom, "100", oraiAddr, gravityAddr.String())
				// set destination in memo
				transfer.Memo = evmChain.EvmChainPrefix + ethDestAddr

				bz := transfertypes.ModuleCdc.MustMarshalJSON(&transfer)
				return channeltypes.NewPacket(bz, 1, transfertypes.PortID, sourceChannel, transfertypes.PortID, oraibChannel, timeoutHeight, 0)
			},

			ackSuccess: true,
			expectedRes: types.QueryPendingSendToEthResponse{
				TransfersInBatches: []types.OutgoingTransferTx{
					{
						Id:          1,
						Sender:      gravityAddr.String(),
						DestAddress: ethDestAddr,
						Erc20Token: types.ERC20Token{
							Contract: tokenContractAddr,
							Amount:   sdk.NewInt(100),
						},
						Erc20Fee: types.ERC20Token{
							Contract: tokenContractAddr,
							Amount:   sdk.NewInt(int64(params.MinChainFeeBasisPoints)),
						},
					},
				},

				UnbatchedTransfers: []types.OutgoingTransferTx{},
			},
		},
	}

	for _, tc := range testCases {

		packet := tc.getPacket()

		// Set Denom Trace
		denomTrace := transfertypes.DenomTrace{
			Path:      path,
			BaseDenom: myTokenDenom,
		}

		input.IbcTransferKeeper.SetDenomTrace(ctx, denomTrace)

		// Set Cosmos Channel
		channel := channeltypes.Channel{
			State:          channeltypes.INIT,
			Ordering:       channeltypes.UNORDERED,
			Counterparty:   channeltypes.NewCounterparty(transfertypes.PortID, sourceChannel),
			ConnectionHops: []string{sourceChannel},
		}

		input.IBCKeeper.ChannelKeeper.SetChannel(ctx, transfertypes.PortID, oraibChannel, channel)

		// Set Next Sequence Send
		input.IBCKeeper.ChannelKeeper.SetNextSequenceSend(ctx, transfertypes.PortID, oraibChannel, 1)

		// Perform IBC callback, simulate app.OnRecvPacket by sending coin to receiver
		input.BankKeeper.SendCoinsFromModuleToAccount(
			input.Context,
			types.ModuleName,
			gravityAddr,
			sdk.NewCoins(sdk.NewCoin(ibcDenom, sdk.NewInt(102))))
		ack := input.GravityKeeper.OnRecvPacket(ctx, packet, expAck)

		// Check acknowledgement
		if tc.ackSuccess {
			require.True(t, ack.Success(), string(ack.Acknowledgement()))
			require.Equal(t, expAck, ack)
		} else {
			require.False(t, ack.Success(), string(ack.Acknowledgement()))
		}

		input.GravityKeeper.BuildOutgoingTxBatch(ctx, evmChain.EvmChainPrefix, *tokenAddr, 1)

		context := sdk.WrapSDKContext(input.Context)
		response, err := input.GravityKeeper.GetPendingSendToEth(context, &types.QueryPendingSendToEth{SenderAddress: gravityAddr.String()})
		require.NoError(t, err)

		require.Equal(t, tc.expectedRes, *response)

	}

}
