package types

import (
	"bytes"
	"fmt"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/assert"
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
	msgSendToCosmosNormal := MsgSendToCosmosClaim{
		CosmosReceiver: "oraifoobar",
	}

	channel, receiver := msgSendToCosmosNormal.GetSourceChannelAndReceiver()

	assert.Equal(t, "", channel)
	assert.Equal(t, "oraifoobar", receiver)

	// has channel case
	msgSendToCosmosHasChannel := MsgSendToCosmosClaim{
		CosmosReceiver: "channel-1/oraifoobar",
	}

	channel, receiver = msgSendToCosmosHasChannel.GetSourceChannelAndReceiver()

	assert.Equal(t, "channel-1", channel)
	assert.Equal(t, "oraifoobar", receiver)

	// special case with many / to test split
	msgSendToCosmosWithSlashes := MsgSendToCosmosClaim{
		CosmosReceiver: "channel-1///oraifoobar",
	}

	channel, receiver = msgSendToCosmosWithSlashes.GetSourceChannelAndReceiver()

	assert.Equal(t, "channel-1", channel)
	assert.Equal(t, "//oraifoobar", receiver)
}
