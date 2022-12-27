package keeper

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	transfertypes "github.com/cosmos/ibc-go/v3/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v3/modules/core/02-client/types"
	channeltypes "github.com/cosmos/ibc-go/v3/modules/core/04-channel/types"
	ibcgotesting "github.com/cosmos/ibc-go/v3/testing"
	ibcmock "github.com/cosmos/ibc-go/v3/testing/mock"
	"github.com/stretchr/testify/require"
)

var erc20Denom = "erc20/0xdac17f958d2ee523a2206206994597c13d831ec7"

func TestOnRecvPacket(t *testing.T) {

	input := CreateTestEnv(t)
	defer func() { input.Context.Logger().Info("Asserting invariants at test end"); input.AssertInvariants() }()

	ctx := input.Context

	var (
		mySender, _         = sdk.AccAddressFromBech32("gravity1ahx7f8wyertuus9r20284ej0asrs085ceqtfnm")
		myReceiver          = "0xd041c41EA1bf0F006ADBb6d2c9ef9D425dE5eaD7"
		myTokenContractAddr = "0x429881672B9AE42b8EbA0E26cD9C73711b891Ca5"
		ibcBase             = "ibc/46B44899322F3CD854D2D46DEEF881958467CDD4B3B10086DA49296BBED94BED/grav"
		evmChain            = input.GravityKeeper.GetEvmChainData(ctx, EthChainPrefix)
		// if not create Claim from Deploy Erc20 contract, then denom = prefix + contract
		myTokenDenom = "bsc"
	)
	receiver, err := types.NewEthAddress(myReceiver)
	require.NoError(t, err)
	println(receiver.GetAddress().String(), mySender.String())

	tokenAddr, err := types.NewEthAddress(myTokenContractAddr)
	require.NoError(t, err)

	// add it to the ERC20 registry
	input.GravityKeeper.setCosmosOriginatedDenomToERC20(ctx, evmChain.EvmChainPrefix, myTokenDenom, *tokenAddr)

	isCosmosOriginated, addr, err := input.GravityKeeper.DenomToERC20Lookup(ctx, evmChain.EvmChainPrefix, myTokenDenom)
	println(addr.GetAddress().String())
	require.True(t, isCosmosOriginated)
	require.NoError(t, err)
	require.Equal(t, tokenAddr.GetAddress().Hex(), myTokenContractAddr)
	require.Equal(t, tokenAddr, addr)

	// secp256k1 account for oraichain
	secpPk := secp256k1.GenPrivKey()
	secpAddr := sdk.AccAddress(secpPk.PubKey().Address())
	secpAddrEvm := secpAddr.String()
	secpAddrCosmos := sdk.MustBech32ifyAddressBytes(sdk.Bech32MainPrefix, secpAddr)

	// Setup Oraichain <=> Orai Bridge IBC relayer
	sourceChannel := "channel-292"
	oraibChannel := "channel-293"
	path := fmt.Sprintf("%s/%s", transfertypes.PortID, oraibChannel)

	timeoutHeight := clienttypes.NewHeight(0, 100)
	disabledTimeoutTimestamp := uint64(0)
	mockPacket := channeltypes.NewPacket(ibcgotesting.MockPacketData, 1, transfertypes.PortID, "channel-0", transfertypes.PortID, "channel-0", timeoutHeight, disabledTimeoutTimestamp)
	packet := mockPacket
	expAck := ibcmock.MockAcknowledgement

	registeredDenom := myTokenDenom
	coins := sdk.NewCoins(
		sdk.NewCoin(registeredDenom, sdk.NewInt(1000)), // some ERC20 token
		sdk.NewCoin(ibcBase, sdk.NewInt(1000)),         // some IBC coin with a registered token pair
	)

	testCases := []struct {
		name             string
		malleate         func()
		ackSuccess       bool
		receiver         sdk.AccAddress
		expErc20s        *big.Int
		expCoins         sdk.Coins
		checkBalances    bool
		disableERC20     bool
		disableTokenPair bool
	}{
		{
			name: "ibc conversion - convert denom to MsgSendToEth",
			malleate: func() {
				transfer := transfertypes.NewFungibleTokenPacketData(registeredDenom, "100", secpAddrEvm, secpAddrCosmos)
				bz := transfertypes.ModuleCdc.MustMarshalJSON(&transfer)
				packet = channeltypes.NewPacket(bz, 1, transfertypes.PortID, sourceChannel, transfertypes.PortID, oraibChannel, timeoutHeight, 0)
			},
			receiver:      secpAddr,
			ackSuccess:    true,
			checkBalances: false,
			expErc20s:     big.NewInt(0),
			expCoins:      coins,
		},
	}

	for _, tc := range testCases {

		tc.malleate()

		// Set Denom Trace
		denomTrace := transfertypes.DenomTrace{
			Path:      path,
			BaseDenom: registeredDenom,
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

		// Perform IBC callback
		ack := input.GravityKeeper.OnRecvPacket(ctx, packet, expAck)

		// Check acknowledgement
		if tc.ackSuccess {
			require.True(t, ack.Success(), string(ack.Acknowledgement()))
			require.Equal(t, expAck, ack)
		} else {
			require.False(t, ack.Success(), string(ack.Acknowledgement()))
		}

		if tc.checkBalances {

			// Check ERC20 balances

		}
	}

}
