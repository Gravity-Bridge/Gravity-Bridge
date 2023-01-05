package v2

import (
	"fmt"

	"github.com/tendermint/tendermint/crypto/tmhash"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
)

// Hash implements WithdrawBatch.Hash
func MsgBatchSendToEthClaimHash(msg types.MsgBatchSendToEthClaim) ([]byte, error) {
	path := fmt.Sprintf("%s/%d/%d/%s", msg.TokenContract, msg.BatchNonce, msg.EventNonce, msg.TokenContract)
	return tmhash.Sum([]byte(path)), nil
}

func MsgSendToCosmosClaimHash(msg types.MsgSendToCosmosClaim) ([]byte, error) {
	path := fmt.Sprintf("%d/%d/%s/%s/%s/%s", msg.EventNonce, msg.EthBlockHeight, msg.TokenContract, msg.Amount.String(), msg.EthereumSender, msg.CosmosReceiver)
	return tmhash.Sum([]byte(path)), nil
}
