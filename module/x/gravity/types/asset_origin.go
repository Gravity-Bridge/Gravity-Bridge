package types

import "fmt"

// AssetOrigin captures the origin of an asset and is returned by ClassifyERC20 or ClassifyDenom.
// Exactly one of IsCosmosOriginated and IsEthOriginated will be true.
// IsRemapped indicates an eth-originated asset has been remapped to the gravity2 prefix, and so must only be true if IsEthOriginated is true
//
// AssetOrigin was added to reduce the complexity of the Cosmos Coin <> Ethereum ERC20 relation logic
// For any Cosmos Coin use ClassifyDenom or for any ERC20 use ClassifyERC20 to get information about the asset's origin
type AssetOrigin struct {
	IsCosmosOriginated bool
	IsEthOriginated    bool
	IsRemapped         bool // only set when IsEthOriginated is true; indicates a gravity2-prefixed denom
	Denom              string
	ERC20              *EthAddress
}

// AssertValid panics if the AssetOrigin invalid:
// - only one of IsCosmosOriginated and IsEthOriginated may be true
// - IsRemapped may only be true if IsEthOriginated is true
// - ERC20 must not be nil
// - Denom must not be empty
func (a AssetOrigin) AssertValid() {
	if a.IsCosmosOriginated == a.IsEthOriginated {
		panic(fmt.Sprintf(
			"AssetOrigin: exactly one of IsCosmosOriginated/IsEthOriginated must be set (cosmos=%v eth=%v denom=%q)",
			a.IsCosmosOriginated, a.IsEthOriginated, a.Denom,
		))
	}
	if a.IsRemapped && !a.IsEthOriginated {
		panic(fmt.Sprintf("AssetOrigin: IsRemapped is true but IsEthOriginated is false (denom=%q)", a.Denom))
	}
	if a.ERC20 == nil {
		panic(fmt.Sprintf("AssetOrigin: ERC20 is nil (denom=%q)", a.Denom))
	}
	if a.Denom == "" {
		panic("AssetOrigin: Denom is empty")
	}
}

// ContainsEthAddress reports whether denom contains a substring that looks like an Ethereum
// address — "0x" followed by exactly 40 hexadecimal characters. Used to detect cosmos-originated
// denom collision attempts where a denom embeds an eth contract address.
func ContainsEthAddress(denom string) bool {
	for i := 0; i < len(denom)-1; i++ {
		if denom[i] == '0' && (denom[i+1] == 'x' || denom[i+1] == 'X') {
			if i+42 <= len(denom) {
				if ValidateEthAddress(denom[i:i+42]) == nil {
					return true
				}
			}
		}
	}
	return false
}
