package types

import (
	"bytes"
	"fmt"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/bech32"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateMsgSetOrchestratorAddress(t *testing.T) {
	var (
		ethAddress                   = "0xb462864E395d88d6bc7C5dd5F3F5eb4cc2599255"
		cosmosAddress sdk.AccAddress = bytes.Repeat([]byte{0x1}, 20)
		valAddress    sdk.ValAddress = bytes.Repeat([]byte{0x1}, 20)
	)
	specs := map[string]struct {
		srcCosmosAddr sdk.AccAddress
		srcValAddr    sdk.ValAddress
		srcETHAddr    string
		expErr        bool
	}{
		"all good": {
			srcCosmosAddr: cosmosAddress,
			srcValAddr:    valAddress,
			srcETHAddr:    ethAddress,
		},
		"empty validator address": {
			srcETHAddr:    ethAddress,
			srcCosmosAddr: cosmosAddress,
			expErr:        true,
		},
		"short validator address": {
			srcValAddr:    []byte{0x1},
			srcCosmosAddr: cosmosAddress,
			srcETHAddr:    ethAddress,
			expErr:        false,
		},
		"empty cosmos address": {
			srcValAddr: valAddress,
			srcETHAddr: ethAddress,
			expErr:     true,
		},
		"short cosmos address": {
			srcCosmosAddr: []byte{0x1},
			srcValAddr:    valAddress,
			srcETHAddr:    ethAddress,
			expErr:        false,
		},
	}
	for msg, spec := range specs {
		t.Run(msg, func(t *testing.T) {
			println(fmt.Sprintf("Spec is %v", msg))
			ethAddr, err := NewEthAddress(spec.srcETHAddr)
			assert.NoError(t, err)
			msg := NewMsgSetOrchestratorAddress(spec.srcValAddr, spec.srcCosmosAddr, *ethAddr)
			// when
			err = msg.ValidateBasic()
			if spec.expErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
		})
	}

}

func TestGetSourceChannelAndReceiver(t *testing.T) {
	// cosmos channel
	msgSendToCosmos := MsgSendToCosmosClaim{
		CosmosReceiver: "channel-0:channel-15/cosmos14n3tx8s5ftzhlxvq0w5962v60vd82h30sythlz:atom",
	}

	receiver, sourceChannel, channel, denom, hrp, err := msgSendToCosmos.ParseReceiver()
	receiverAddr, _ := bech32.ConvertAndEncode(hrp, receiver)
	assert.Equal(t, "channel-15", channel)
	assert.Equal(t, "channel-0", sourceChannel)
	assert.Equal(t, "cosmos14n3tx8s5ftzhlxvq0w5962v60vd82h30sythlz", receiverAddr)
	assert.Equal(t, "atom", denom)
	require.NoError(t, err)

	// evm channel
	msgSendToCosmos = MsgSendToCosmosClaim{
		CosmosReceiver: "channel-1:trx-mainnet0x73Ddc880916021EFC4754Cb42B53db6EAB1f9D64:usdt",
	}

	receiver, sourceChannel, channel, denom, hrp, err = msgSendToCosmos.ParseReceiver()
	ethAddr, _ := NewEthAddressFromBytes(receiver)
	assert.Equal(t, "trx-mainnet", channel)
	assert.Equal(t, "0x73Ddc880916021EFC4754Cb42B53db6EAB1f9D64", ethAddr.GetAddress().String())
	assert.Equal(t, "usdt", denom)
	require.NoError(t, err)

	// no channel, cosmos address
	msgSendToCosmos = MsgSendToCosmosClaim{
		CosmosReceiver: "orai14n3tx8s5ftzhlxvq0w5962v60vd82h30rha573",
	}

	receiver, sourceChannel, channel, denom, hrp, err = msgSendToCosmos.ParseReceiver()
	receiverAddr, _ = bech32.ConvertAndEncode(hrp, receiver)
	assert.Equal(t, "", channel)
	assert.Equal(t, "orai14n3tx8s5ftzhlxvq0w5962v60vd82h30rha573", receiverAddr)
	assert.Equal(t, "", denom)
	require.NoError(t, err)

	// cosmos channel with invalid address
	msgSendToCosmos = MsgSendToCosmosClaim{
		CosmosReceiver: "channel-1///oraifoobar",
	}

	receiver, sourceChannel, channel, denom, hrp, err = msgSendToCosmos.ParseReceiver()
	assert.Equal(t, msgSendToCosmos.GetDestination(sourceChannel), "//oraifoobar")
	require.Error(t, err)

}
