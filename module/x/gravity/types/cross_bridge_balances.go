package types

import "fmt"

// NewBridgeBalanceSnapshot constructs a BridgeBalanceSnapshot conveniently from the Cosmos block height,
// an attestation with Ethereum block height, and a list of monitored tokens with their balances
func NewBridgeBalanceSnapshot(
	cosmosHeight uint64, ethBlockHeight uint64, monitoredBalances []*ERC20Token, eventNonce uint64,
) BridgeBalanceSnapshot {
	return BridgeBalanceSnapshot{
		CosmosBlockHeight:   cosmosHeight,
		EthereumBlockHeight: ethBlockHeight,
		Balances:            monitoredBalances,
		EventNonce:          eventNonce,
	}
}

func (m BridgeBalanceSnapshot) ValidateBasic() error {
	if m.EventNonce == 0 {
		return fmt.Errorf("bridge balance snapshot has a zero event nonce")
	}
	if m.CosmosBlockHeight == 0 {
		return fmt.Errorf("bridge balance snapshot has a zero cosmos height")
	}
	if m.EthereumBlockHeight == 0 {
		return fmt.Errorf("bridge balance snapshot has a zero ethereum height")
	}
	if len(m.Balances) == 0 {
		return fmt.Errorf("bridge balance snapshot has no balances")
	}
	for _, b := range m.Balances {
		_, err := b.ToInternal()
		if err != nil {
			return fmt.Errorf("bridge balance snapshot contains invalid balance (%v): %v", b, err)
		}
	}
	return nil
}

func (b BridgeBalanceSnapshot) IsEmpty() bool {
	return len(b.Balances) == 0
}
