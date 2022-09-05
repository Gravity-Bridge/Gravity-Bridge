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
