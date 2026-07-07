package types

import "fmt"

// AssetOriginChain identifies whether an asset (a Cosmos denom or an Ethereum ERC20) originated
// on some Cosmos chain or on Ethereum.
type AssetOriginChain int32

const (
	// AssetOriginUnspecified is the zero value and is never valid for a populated AssetOrigin.
	AssetOriginUnspecified AssetOriginChain = iota
	// AssetOriginCosmos indicates the asset originated on some Cosmos chain (a native coin or an IBC coin).
	AssetOriginCosmos
	// AssetOriginEthereum indicates the asset originated on Ethereum (a native ERC20).
	AssetOriginEthereum
)

// String implements fmt.Stringer for AssetOriginChain.
func (k AssetOriginChain) String() string {
	switch k {
	case AssetOriginCosmos:
		return "Cosmos"
	case AssetOriginEthereum:
		return "Ethereum"
	default:
		return "Unspecified"
	}
}

// AssetOrigin captures the origin of an asset and is returned by ClassifyERC20 or ClassifyDenom.
// Origin indicates whether the asset originated on Cosmos or on Ethereum.
// IsRemapped indicates an eth-originated asset has been remapped to the gravity2 prefix, and so must only be true if Origin is AssetOriginEthereum
//
// AssetOrigin was added to reduce the complexity of the Cosmos Coin <> Ethereum ERC20 relation logic
// For any Cosmos Coin use ClassifyDenom or for any ERC20 use ClassifyERC20 to get information about the asset's origin
type AssetOrigin struct {
	Origin     AssetOriginChain
	IsRemapped bool // only set when Origin is AssetOriginEthereum; indicates a gravity2-prefixed denom
	Denom      string
	ERC20      *EthAddress
}

// AssertValid panics if the AssetOrigin invalid:
// - Origin must be either AssetOriginCosmos or AssetOriginEthereum
// - IsRemapped may only be true if Origin is AssetOriginEthereum
// - ERC20 must not be nil
// - Denom must not be empty
func (a AssetOrigin) AssertValid() {
	if a.Origin != AssetOriginCosmos && a.Origin != AssetOriginEthereum {
		panic(fmt.Sprintf(
			"AssetOrigin: Origin must be AssetOriginCosmos or AssetOriginEthereum (origin=%v denom=%q)",
			a.Origin, a.Denom,
		))
	}
	if a.IsRemapped && a.Origin != AssetOriginEthereum {
		panic(fmt.Sprintf("AssetOrigin: IsRemapped is true but Origin is not AssetOriginEthereum (denom=%q)", a.Denom))
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
