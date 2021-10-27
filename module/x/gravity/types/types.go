package types

import (
	"encoding/binary"
	"fmt"
	math "math"
	"math/big"
	"sort"
	"strconv"
	"strings"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/bech32"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/ethereum/go-ethereum/accounts/abi"
	gethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// UInt64FromBytes create uint from binary big endian representation
func UInt64FromBytes(s []byte) uint64 {
	return binary.BigEndian.Uint64(s)
}

// UInt64Bytes uses the SDK byte marshaling to encode a uint64
func UInt64Bytes(n uint64) []byte {
	return sdk.Uint64ToBigEndian(n)
}

// UInt64FromString to parse out a uint64 for a nonce
func UInt64FromString(s string) (uint64, error) {
	return strconv.ParseUint(s, 10, 64)
}

// IBCAddressFromBech32 decodes an IBC-compatible Address from a Bech32
// encoded string
// TODO: This is very similar to the sdk's GetFromBech32 method, but makes no
// assertions about the bech32 prefix (aka "human readable part"), when Gravity
// IBC Forwarding is completed, this function should return invalid prefix errors
func IBCAddressFromBech32(bech32str string) ([]byte, error) {
	if len(bech32str) == 0 {
		return nil, ErrEmpty
	}

	_, bz, err := bech32.DecodeAndConvert(bech32str)
	if err != nil {
		return nil, err
	}

	err = sdk.VerifyAddressFormat(bz)
	if err != nil {
		return nil, err
	}

	return bz, nil
}

// GetPrefixFromBech32 returns the human readable part of a bech32 string (excluding the 1 byte)
// Returns an error on too short input or when the 1 byte cannot be found
// Note: This is an excerpt from the Decode function for bech32 strings
func GetPrefixFromBech32(bech32str string) (string, error) {
	if len(bech32str) < 8 {
		return "", fmt.Errorf("invalid bech32 string length %d",
			len(bech32str))
	}
	one := strings.LastIndexByte(bech32str, '1')
	if one < 1 || one+7 > len(bech32str) {
		return "", fmt.Errorf("invalid index of 1")
	}

	return bech32str[:one], nil
}

// GetNativePrefixedAccAddressString treats the input as an AccAddress and re-prefixes the string
// with this chain's configured Bech32AccountAddrPrefix
// Returns an error when input is not a bech32 string or the original string it is already natively prefixed
func GetNativePrefixedAccAddressString(foreignStr string) (string, error) {
	prefix, err := GetPrefixFromBech32(foreignStr)
	if err != nil {
		return "", sdkerrors.Wrap(err, "invalid bech32 string")
	}
	nativePrefix := sdk.GetConfig().GetBech32AccountAddrPrefix()
	if prefix == nativePrefix {
		return foreignStr, nil
	}

	return nativePrefix + foreignStr[len(prefix):], nil
}

// GetNativePrefixedAccAddress re-prefixes the input AccAddress with this chain's configured Bech32AccountAddrPrefix
func GetNativePrefixedAccAddress(foreignAddr sdk.AccAddress) (sdk.AccAddress, error) {
	nativeStr, err := GetNativePrefixedAccAddressString(foreignAddr.String())
	if err != nil {
		return nil, err
	}
	return sdk.AccAddressFromBech32(nativeStr)
}

//////////////////////////////////////
//      BRIDGE VALIDATOR(S)         //
//////////////////////////////////////

// ToInternal transforms a BridgeValidator into its fully validated internal type
func (b BridgeValidator) ToInternal() (*InternalBridgeValidator, error) {
	return NewInternalBridgeValidator(b)
}

// BridgeValidators is the sorted set of validator data for Ethereum bridge MultiSig set
type BridgeValidators []BridgeValidator

func (b BridgeValidators) ToInternal() (*InternalBridgeValidators, error) {
	ret := make(InternalBridgeValidators, len(b))
	for i := range b {
		ibv, err := NewInternalBridgeValidator(b[i])
		if err != nil {
			return nil, sdkerrors.Wrapf(err, "member %d", i)
		}
		ret[i] = ibv
	}
	return &ret, nil
}

// Bridge Validator but with validated EthereumAddress
type InternalBridgeValidator struct {
	Power           uint64
	EthereumAddress EthAddress
}

func NewInternalBridgeValidator(bridgeValidator BridgeValidator) (*InternalBridgeValidator, error) {
	i := &InternalBridgeValidator{
		Power:           bridgeValidator.Power,
		EthereumAddress: EthAddress{bridgeValidator.EthereumAddress},
	}
	if err := i.ValidateBasic(); err != nil {
		return nil, sdkerrors.Wrap(err, "invalid bridge validator")
	}
	return i, nil
}

func (i InternalBridgeValidator) ValidateBasic() error {
	if i.Power == 0 {
		return sdkerrors.Wrap(ErrEmpty, "power")
	}
	if err := i.EthereumAddress.ValidateBasic(); err != nil {
		return sdkerrors.Wrap(err, "ethereum address")
	}
	return nil
}

func (i InternalBridgeValidator) ToExternal() BridgeValidator {
	return BridgeValidator{
		Power:           i.Power,
		EthereumAddress: i.EthereumAddress.GetAddress(),
	}
}

// InternalBridgeValidators is the sorted set of validator data for Ethereum bridge MultiSig set
type InternalBridgeValidators []*InternalBridgeValidator

func (i InternalBridgeValidators) ToExternal() BridgeValidators {
	bridgeValidators := make([]BridgeValidator, len(i))
	for b := range bridgeValidators {
		bridgeValidators[b] = i[b].ToExternal()
	}

	return BridgeValidators(bridgeValidators)
}

// Sort sorts the validators by power
func (b InternalBridgeValidators) Sort() {
	sort.Slice(b, func(i, j int) bool {
		if b[i].Power == b[j].Power {
			// Secondary sort on eth address in case powers are equal
			return EthAddrLessThan(b[i].EthereumAddress, b[j].EthereumAddress)
		}
		return b[i].Power > b[j].Power
	})
}

// PowerDiff returns the difference in power between two bridge validator sets
// note this is Gravity bridge power *not* Cosmos voting power. Cosmos voting
// power is based on the absolute number of tokens in the staking pool at any given
// time Gravity bridge power is normalized using the equation.
//
// validators cosmos voting power / total cosmos voting power in this block = gravity bridge power / u32_max
//
// As an example if someone has 52% of the Cosmos voting power when a validator set is created their Gravity
// bridge voting power is u32_max * .52
//
// Normalized voting power dramatically reduces how often we have to produce new validator set updates. For example
// if the total on chain voting power increases by 1% due to inflation, we shouldn't have to generate a new validator
// set, after all the validators retained their relative percentages during inflation and normalized Gravity bridge power
// shows no difference.
func (b InternalBridgeValidators) PowerDiff(c InternalBridgeValidators) float64 {
	powers := map[string]int64{}
	// loop over b and initialize the map with their powers
	for _, bv := range b {
		powers[bv.EthereumAddress.GetAddress()] = int64(bv.Power)
	}

	// subtract c powers from powers in the map, initializing
	// uninitialized keys with negative numbers
	for _, bv := range c {
		if val, ok := powers[bv.EthereumAddress.GetAddress()]; ok {
			powers[bv.EthereumAddress.GetAddress()] = val - int64(bv.Power)
		} else {
			powers[bv.EthereumAddress.GetAddress()] = -int64(bv.Power)
		}
	}

	var delta float64
	for _, v := range powers {
		// NOTE: we care about the absolute value of the changes
		delta += math.Abs(float64(v))
	}

	return math.Abs(delta / float64(math.MaxUint32))
}

// TotalPower returns the total power in the bridge validator set
func (b InternalBridgeValidators) TotalPower() (out uint64) {
	for _, v := range b {
		out += v.Power
	}
	return
}

// HasDuplicates returns true if there are duplicates in the set
func (b InternalBridgeValidators) HasDuplicates() bool {
	m := make(map[string]struct{}, len(b))
	for i := range b {
		m[b[i].EthereumAddress.GetAddress()] = struct{}{}
	}
	return len(m) != len(b)
}

// GetPowers returns only the power values for all members
func (b InternalBridgeValidators) GetPowers() []uint64 {
	r := make([]uint64, len(b))
	for i := range b {
		r[i] = b[i].Power
	}
	return r
}

// ValidateBasic performs stateless checks
func (b InternalBridgeValidators) ValidateBasic() error {
	// TODO: check if the set is sorted here?
	if len(b) == 0 {
		return ErrEmpty
	}
	for i := range b {
		if err := b[i].ValidateBasic(); err != nil {
			return sdkerrors.Wrapf(err, "member %d", i)
		}
	}
	if b.HasDuplicates() {
		return sdkerrors.Wrap(ErrDuplicate, "addresses")
	}
	return nil
}

// NewValset returns a new valset
func NewValset(nonce, height uint64, members InternalBridgeValidators, rewardAmount sdk.Int, rewardToken EthAddress) (*Valset, error) {
	if err := members.ValidateBasic(); err != nil {
		return nil, sdkerrors.Wrap(err, "invalid members")
	}
	members.Sort()
	var mem []BridgeValidator
	for _, val := range members {
		mem = append(mem, val.ToExternal())
	}
	vs := Valset{Nonce: uint64(nonce), Members: mem, Height: height, RewardAmount: rewardAmount, RewardToken: rewardToken.GetAddress()}
	return &vs,
		nil
}

// GetCheckpoint returns the checkpoint
func (v Valset) GetCheckpoint(gravityIDstring string) []byte {

	// error case here should not occur outside of testing since the above is a constant
	contractAbi, abiErr := abi.JSON(strings.NewReader(ValsetCheckpointABIJSON))
	if abiErr != nil {
		panic("Bad ABI constant!")
	}

	// the contract argument is not a arbitrary length array but a fixed length 32 byte
	// array, therefore we have to utf8 encode the string (the default in this case) and
	// then copy the variable length encoded data into a fixed length array. This function
	// will panic if gravityId is too long to fit in 32 bytes
	gravityID, err := strToFixByteArray(gravityIDstring)
	if err != nil {
		panic(err)
	}

	// this should never happen, unless an invalid paramater value has been set by the chain
	err = ValidateEthAddress(v.RewardToken)
	if err != nil {
		panic(err)
	}
	rewardToken := gethcommon.HexToAddress(v.RewardToken)

	if v.RewardAmount.BigInt() == nil {
		// this must be programmer error
		panic("Invalid reward amount passed in valset GetCheckpoint!")
	}
	rewardAmount := v.RewardAmount.BigInt()

	checkpointBytes := []uint8("checkpoint")
	var checkpoint [32]uint8
	copy(checkpoint[:], checkpointBytes[:])

	memberAddresses := make([]gethcommon.Address, len(v.Members))
	convertedPowers := make([]*big.Int, len(v.Members))
	for i, m := range v.Members {
		memberAddresses[i] = gethcommon.HexToAddress(m.EthereumAddress)
		convertedPowers[i] = big.NewInt(int64(m.Power))
	}
	// the word 'checkpoint' needs to be the same as the 'name' above in the checkpointAbiJson
	// but other than that it's a constant that has no impact on the output. This is because
	// it gets encoded as a function name which we must then discard.
	bytes, packErr := contractAbi.Pack("checkpoint", gravityID, checkpoint, big.NewInt(int64(v.Nonce)), memberAddresses, convertedPowers, rewardAmount, rewardToken)

	// this should never happen outside of test since any case that could crash on encoding
	// should be filtered above.
	if packErr != nil {
		panic(fmt.Sprintf("Error packing checkpoint! %s/n", packErr))
	}

	// we hash the resulting encoded bytes discarding the first 4 bytes these 4 bytes are the constant
	// method name 'checkpoint'. If you where to replace the checkpoint constant in this code you would
	// then need to adjust how many bytes you truncate off the front to get the output of abi.encode()
	hash := crypto.Keccak256Hash(bytes[4:])
	return hash.Bytes()
}

// WithoutEmptyMembers returns a new Valset without member that have 0 power or an empty Ethereum address.
func (v *Valset) WithoutEmptyMembers() *Valset {
	if v == nil {
		return nil
	}
	r := Valset{
		Nonce:        v.Nonce,
		Members:      make([]BridgeValidator, 0, len(v.Members)),
		Height:       0,
		RewardAmount: sdk.Int{},
		RewardToken:  "",
	}
	for i := range v.Members {
		if _, err := v.Members[i].ToInternal(); err == nil {
			r.Members = append(r.Members, v.Members[i])
		}
	}
	return &r
}

// Valsets is a collection of valset
type Valsets []Valset

func (v Valsets) Len() int {
	return len(v)
}

func (v Valsets) Less(i, j int) bool {
	return v[i].Nonce > v[j].Nonce
}

func (v Valsets) Swap(i, j int) {
	v[i], v[j] = v[j], v[i]
}

// GetFees returns the total fees contained within a given batch
func (b OutgoingTxBatch) GetFees() sdk.Int {
	sum := sdk.ZeroInt()
	for _, t := range b.Transactions {
		sum.Add(t.Erc20Fee.Amount)
	}
	return sum
}

// This interface is implemented by all the types that are used
// to create transactions on Ethereum that are signed by validators.
// The naming here could be improved.
type EthereumSigned interface {
	GetCheckpoint(gravityIDstring string) []byte
}

var (
	_ EthereumSigned = &Valset{}
	_ EthereumSigned = &OutgoingTxBatch{}
	_ EthereumSigned = &OutgoingLogicCall{}
)
