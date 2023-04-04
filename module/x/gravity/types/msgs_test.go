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

func TestParseReceiverRaw(t *testing.T) {
	// cosmos channel
	// args=2. src channel = args[0] = channel-0, destination=args[1] = channel-15/cosmos14n3tx8s5ftzhlxvq0w5962v60vd82h30sythlz:atom
	msgSendToCosmos := MsgSendToCosmosClaim{
		CosmosReceiver: "channel-0/orai14n3tx8s5ftzhlxvq0w5962v60vd82h30rha573:channel-15/cosmos14n3tx8s5ftzhlxvq0w5962v60vd82h30sythlz:atom",
	}

	sourceChannel, cosmosReceiver, destination := msgSendToCosmos.ParseReceiverRaw()
	assert.Equal(t, "channel-0", sourceChannel)
	assert.Equal(t, "orai14n3tx8s5ftzhlxvq0w5962v60vd82h30rha573", cosmosReceiver)
	assert.Equal(t, "channel-15/cosmos14n3tx8s5ftzhlxvq0w5962v60vd82h30sythlz:atom", destination)

	// args=1, no /
	msgSendToCosmos = MsgSendToCosmosClaim{
		CosmosReceiver: "cosmos14n3tx8s5ftzhlxvq0w5962v60vd82h30sythlz",
	}

	sourceChannel, cosmosReceiver, destination = msgSendToCosmos.ParseReceiverRaw()
	assert.Equal(t, "", sourceChannel)
	assert.Equal(t, "cosmos14n3tx8s5ftzhlxvq0w5962v60vd82h30sythlz", destination)
	assert.Equal(t, "cosmos14n3tx8s5ftzhlxvq0w5962v60vd82h30sythlz", cosmosReceiver)

	// args=1, empty
	msgSendToCosmos = MsgSendToCosmosClaim{
		CosmosReceiver: "",
	}

	sourceChannel, cosmosReceiver, destination = msgSendToCosmos.ParseReceiverRaw()
	assert.Equal(t, "", sourceChannel)
	assert.Equal(t, "", destination)
	assert.Equal(t, "", cosmosReceiver)

	//args=1, has /
	msgSendToCosmos = MsgSendToCosmosClaim{
		CosmosReceiver: "channel-15/cosmos14n3tx8s5ftzhlxvq0w5962v60vd82h30sythlz",
	}

	sourceChannel, cosmosReceiver, destination = msgSendToCosmos.ParseReceiverRaw()
	assert.Equal(t, "channel-15", sourceChannel)
	assert.Equal(t, "cosmos14n3tx8s5ftzhlxvq0w5962v60vd82h30sythlz", destination)
	assert.Equal(t, "cosmos14n3tx8s5ftzhlxvq0w5962v60vd82h30sythlz", cosmosReceiver)

	//args=1, has /
	msgSendToCosmos = MsgSendToCosmosClaim{
		CosmosReceiver: "channel-15/cosmos14n3tx8s5ftzhlxvq0w5962v60vd82h30sythlz:eth-mainnet0xdc05090A39650026E6AFe89b2e795fd57a3cfEC7:usdt",
	}

	sourceChannel, cosmosReceiver, destination = msgSendToCosmos.ParseReceiverRaw()
	assert.Equal(t, "channel-15", sourceChannel)
	assert.Equal(t, "cosmos14n3tx8s5ftzhlxvq0w5962v60vd82h30sythlz", cosmosReceiver)
	assert.Equal(t, "eth-mainnet0xdc05090A39650026E6AFe89b2e795fd57a3cfEC7:usdt", destination)
}
