package keeper

import (
	"fmt"
	distrkeeper "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
	slashingkeeper "github.com/cosmos/cosmos-sdk/x/slashing/keeper"
	"sort"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	paramtypes "github.com/cosmos/cosmos-sdk/x/params/types"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/tendermint/tendermint/libs/log"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
)

// Check that our expected keeper types are implemented
var _ types.StakingKeeper = (*stakingkeeper.Keeper)(nil)
var _ types.SlashingKeeper = (*slashingkeeper.Keeper)(nil)
var _ types.DistributionKeeper = (*distrkeeper.Keeper)(nil)

// Keeper maintains the link to storage and exposes getter/setter methods for the various parts of the state machine
type Keeper struct {
	// NOTE: If you add anything to this struct, add a nil check to ValidateMembers below!
	storeKey   sdk.StoreKey // Unexposed key to access store from sdk.Context
	paramSpace paramtypes.Subspace

	// NOTE: If you add anything to this struct, add a nil check to ValidateMembers below!
	cdc            codec.BinaryCodec // The wire codec for binary encoding/decoding.
	bankKeeper     *bankkeeper.BaseKeeper
	StakingKeeper  *stakingkeeper.Keeper
	SlashingKeeper *slashingkeeper.Keeper
	DistKeeper     *distrkeeper.Keeper
	accountKeeper  *authkeeper.AccountKeeper

	AttestationHandler interface {
		Handle(sdk.Context, types.Attestation, types.EthereumClaim) error
	}
}

// Check for nil members
func (k Keeper) ValidateMembers() {
	if k.bankKeeper     == nil { panic("Nil bankKeeper!") }
	if k.StakingKeeper  == nil { panic("Nil StakingKeeper!") }
	if k.SlashingKeeper == nil { panic("Nil SlashingKeeper!") }
	if k.DistKeeper     == nil { panic("Nil DistKeeper!") }
	if k.accountKeeper  == nil { panic("Nil accountKeeper!") }
}

// NewKeeper returns a new instance of the gravity keeper
func NewKeeper(
	storeKey sdk.StoreKey,
	paramSpace paramtypes.Subspace,
	cdc codec.BinaryCodec,
	bankKeeper *bankkeeper.BaseKeeper,
	stakingKeeper *stakingkeeper.Keeper,
	slashingKeeper *slashingkeeper.Keeper,
	distKeeper *distrkeeper.Keeper,
	accKeeper *authkeeper.AccountKeeper,
) Keeper {
	// set KeyTable if it has not already been set
	if !paramSpace.HasKeyTable() {
		paramSpace = paramSpace.WithKeyTable(types.ParamKeyTable())
	}

	k := Keeper{
		storeKey:           storeKey,
		paramSpace:         paramSpace,

		cdc:                cdc,
		bankKeeper:         bankKeeper,
		StakingKeeper:      stakingKeeper,
		SlashingKeeper:     slashingKeeper,
		DistKeeper:         distKeeper,
		accountKeeper:      accKeeper,
		AttestationHandler: nil,
	}
	attestationHandler := AttestationHandler{
		keeper:     &k,
		bankKeeper: bankKeeper,
		distKeeper: distKeeper,
	}
	attestationHandler.ValidateMembers()
	k.AttestationHandler = attestationHandler

	k.ValidateMembers()

	return k
}

/////////////////////////////
//       PARAMETERS        //
/////////////////////////////

// GetParams returns the parameters from the store
func (k Keeper) GetParams(ctx sdk.Context) (params types.Params) {
	k.paramSpace.GetParamSet(ctx, &params)
	return
}

// SetParams sets the parameters in the store
func (k Keeper) SetParams(ctx sdk.Context, ps types.Params) {
	k.paramSpace.SetParamSet(ctx, &ps)
}

// GetBridgeContractAddress returns the bridge contract address on ETH
func (k Keeper) GetBridgeContractAddress(ctx sdk.Context) *types.EthAddress {
	var a string
	k.paramSpace.Get(ctx, types.ParamsStoreKeyBridgeContractAddress, &a)
	addr, err := types.NewEthAddress(a)
	if err != nil {
		panic(sdkerrors.Wrapf(err, "found invalid bridge contract address in store: %v", a))
	}
	return addr
}

// GetBridgeChainID returns the chain id of the ETH chain we are running against
func (k Keeper) GetBridgeChainID(ctx sdk.Context) uint64 {
	var a uint64
	k.paramSpace.Get(ctx, types.ParamsStoreKeyBridgeContractChainID, &a)
	return a
}

// GetGravityID returns the GravityID the GravityID is essentially a salt value
// for bridge signatures, provided each chain running Gravity has a unique ID
// it won't be possible to play back signatures from one bridge onto another
// even if they share a validator set.
//
// The lifecycle of the GravityID is that it is set in the Genesis file
// read from the live chain for the contract deployment, once a Gravity contract
// is deployed the GravityID CAN NOT BE CHANGED. Meaning that it can't just be the
// same as the chain id since the chain id may be changed many times with each
// successive chain in charge of the same bridge
func (k Keeper) GetGravityID(ctx sdk.Context) string {
	var a string
	k.paramSpace.Get(ctx, types.ParamsStoreKeyGravityID, &a)
	return a
}

// Set GravityID sets the GravityID the GravityID is essentially a salt value
// for bridge signatures, provided each chain running Gravity has a unique ID
// it won't be possible to play back signatures from one bridge onto another
// even if they share a validator set.
//
// The lifecycle of the GravityID is that it is set in the Genesis file
// read from the live chain for the contract deployment, once a Gravity contract
// is deployed the GravityID CAN NOT BE CHANGED. Meaning that it can't just be the
// same as the chain id since the chain id may be changed many times with each
// successive chain in charge of the same bridge
func (k Keeper) SetGravityID(ctx sdk.Context, v string) {
	k.paramSpace.Set(ctx, types.ParamsStoreKeyGravityID, v)
}

// logger returns a module-specific logger.
func (k Keeper) logger(ctx sdk.Context) log.Logger {
	return ctx.Logger().With("module", fmt.Sprintf("x/%s", types.ModuleName))
}

func (k Keeper) UnpackAttestationClaim(att *types.Attestation) (types.EthereumClaim, error) {
	var msg types.EthereumClaim
	err := k.cdc.UnpackAny(att.Claim, &msg)
	if err != nil {
		return nil, err
	} else {
		return msg, nil
	}
}

// GetDelegateKeys iterates both the EthAddress and Orchestrator address indexes to produce
// a vector of MsgSetOrchestratorAddress entires containing all the delgate keys for state
// export / import. This may seem at first glance to be excessively complicated, why not combine
// the EthAddress and Orchestrator address indexes and simply iterate one thing? The answer is that
// even though we set the Eth and Orchestrator address in the same place we use them differently we
// always go from Orchestrator address to Validator address and from validator address to Ethereum address
// we want to keep looking up the validator address for various reasons, so a direct Orchestrator to Ethereum
// address mapping will mean having to keep two of the same data around just to provide lookups.
//
// For the time being this will serve
func (k Keeper) GetDelegateKeys(ctx sdk.Context) []types.MsgSetOrchestratorAddress {
	store := ctx.KVStore(k.storeKey)
	prefix := []byte(types.EthAddressByValidatorKey)
	iter := store.Iterator(prefixRange(prefix))
	defer iter.Close()

	ethAddresses := make(map[string]string)

	for ; iter.Valid(); iter.Next() {
		// the 'key' contains both the prefix and the value, so we need
		// to cut off the starting bytes, if you don't do this a valid
		// cosmos key will be made out of EthAddressByValidatorKey + the startin bytes
		// of the actual key
		key := iter.Key()[len(types.EthAddressByValidatorKey):]
		value := iter.Value()
		ethAddress, err := types.NewEthAddress(string(value))
		if err != nil {
			panic(sdkerrors.Wrapf(err, "found invalid ethAddress %v under key %v", string(value), key))
		}
		valAddress := sdk.ValAddress(key)
		if err := sdk.VerifyAddressFormat(valAddress); err != nil {
			panic(sdkerrors.Wrapf(err, "invalid valAddress in key %v", valAddress))
		}
		ethAddresses[valAddress.String()] = ethAddress.GetAddress()
	}

	store = ctx.KVStore(k.storeKey)
	prefix = []byte(types.KeyOrchestratorAddress)
	iter = store.Iterator(prefixRange(prefix))
	defer iter.Close()

	orchAddresses := make(map[string]string)

	for ; iter.Valid(); iter.Next() {
		key := iter.Key()[len(types.KeyOrchestratorAddress):]
		value := iter.Value()
		orchAddress := sdk.AccAddress(key)
		if err := sdk.VerifyAddressFormat(orchAddress); err != nil {
			panic(sdkerrors.Wrapf(err, "invalid orchAddress in key %v", orchAddresses))
		}
		valAddress := sdk.ValAddress(value)
		if err := sdk.VerifyAddressFormat(valAddress); err != nil {
			panic(sdkerrors.Wrapf(err, "invalid val address stored for orchestrator %s", valAddress.String()))
		}

		orchAddresses[valAddress.String()] = orchAddress.String()
	}

	var result []types.MsgSetOrchestratorAddress

	for valAddr, ethAddr := range ethAddresses {
		orch, ok := orchAddresses[valAddr]
		if !ok {
			// this should never happen unless the store
			// is somehow inconsistent
			panic("Can't find address")
		}
		result = append(result, types.MsgSetOrchestratorAddress{
			Orchestrator: orch,
			Validator:    valAddr,
			EthAddress:   ethAddr,
		})

	}

	// we iterated over a map, so now we have to sort to ensure the
	// output here is deterministic, eth address chosen for no particular
	// reason
	sort.Slice(result[:], func(i, j int) bool {
		return result[i].EthAddress < result[j].EthAddress
	})

	return result
}

/////////////////////////////
//   Logic Call Slashing   //
/////////////////////////////

// SetLastSlashedLogicCallBlock returns true if the last slashed logic call block
// has been set in the store
func (k Keeper) HasLastSlashedLogicCallBlock(ctx sdk.Context) bool {
	store := ctx.KVStore(k.storeKey)
	return store.Has([]byte(types.LastSlashedLogicCallBlock))
}

// SetLastSlashedLogicCallBlock sets the latest slashed logic call block height
func (k Keeper) SetLastSlashedLogicCallBlock(ctx sdk.Context, blockHeight uint64) {

	if k.HasLastSlashedLogicCallBlock(ctx) && k.GetLastSlashedLogicCallBlock(ctx) > blockHeight {
		panic("Attempted to decrement LastSlashedBatchBlock")
	}

	store := ctx.KVStore(k.storeKey)
	store.Set([]byte(types.LastSlashedLogicCallBlock), types.UInt64Bytes(blockHeight))
}

// GetLastSlashedLogicCallBlock returns the latest slashed logic call block
func (k Keeper) GetLastSlashedLogicCallBlock(ctx sdk.Context) uint64 {
	store := ctx.KVStore(k.storeKey)
	bytes := store.Get([]byte(types.LastSlashedLogicCallBlock))

	if len(bytes) == 0 {
		panic("Last slashed logic call block not initialized in genesis")
	}
	return types.UInt64FromBytes(bytes)
}

// GetUnSlashedLogicCalls returns all the unslashed logic calls in state
func (k Keeper) GetUnSlashedLogicCalls(ctx sdk.Context, maxHeight uint64) (out []types.OutgoingLogicCall) {
	lastSlashedLogicCallBlock := k.GetLastSlashedLogicCallBlock(ctx)
	calls := k.GetOutgoingLogicCalls(ctx)
	for _, call := range calls {
		if call.Block > lastSlashedLogicCallBlock {
			out = append(out, call)
		}
	}
	return
}

/////////////////////////////
//       Parameters        //
/////////////////////////////

// prefixRange turns a prefix into a (start, end) range. The start is the given prefix value and
// the end is calculated by adding 1 bit to the start value. Nil is not allowed as prefix.
// 		Example: []byte{1, 3, 4} becomes []byte{1, 3, 5}
// 				 []byte{15, 42, 255, 255} becomes []byte{15, 43, 0, 0}
//
// In case of an overflow the end is set to nil.
//		Example: []byte{255, 255, 255, 255} becomes nil
// MARK finish-batches: this is where some crazy shit happens
func prefixRange(prefix []byte) ([]byte, []byte) {
	if prefix == nil {
		panic("nil key not allowed")
	}
	// special case: no prefix is whole range
	if len(prefix) == 0 {
		return nil, nil
	}

	// copy the prefix and update last byte
	end := make([]byte, len(prefix))
	copy(end, prefix)
	l := len(end) - 1
	end[l]++

	// wait, what if that overflowed?....
	for end[l] == 0 && l > 0 {
		l--
		end[l]++
	}

	// okay, funny guy, you gave us FFF, no end to this range...
	if l == 0 && end[0] == 0 {
		end = nil
	}
	return prefix, end
}

// DeserializeValidatorIterator returns validators from the validator iterator.
// Adding here in gravity keeper as cdc is not available inside endblocker.
func (k Keeper) DeserializeValidatorIterator(vals []byte) stakingtypes.ValAddresses {
	validators := stakingtypes.ValAddresses{
		Addresses: []string{},
	}
	k.cdc.MustUnmarshal(vals, &validators)
	return validators
}

// Checks if the provided Ethereum address is on the Governance blacklist
func (k Keeper) IsOnBlacklist(ctx sdk.Context, addr types.EthAddress) bool {
	params := k.GetParams(ctx)
	// Checks the address if it's inside the blacklisted address list and marks
	// if it's inside the list.
	for index := 0; index < len(params.EthereumBlacklist); index++ {
		baddr, err := types.NewEthAddress(params.EthereumBlacklist[index])
		if err != nil {
			// this should not be possible we validate on genesis load
			panic("unvalidated black list address!")
		}
		if *baddr == addr {
			return true
		}
	}
	return false
}

// Returns true if the provided address is invalid to send to Ethereum this could be
// for one of several reasons. (1) it is invalid in general like the Zero address, (2)
// it is invalid for a subset of ERC20 addresses or (3) it is on the governance deposit/withdraw
// blacklist. (2) is not yet implemented
// Blocking some addresses is technically motivated, if any ERC20 transfers in a batch fail the entire batch
// becomes impossible to execute.
func (k Keeper) InvalidSendToEthAddress(ctx sdk.Context, addr types.EthAddress, _erc20Addr types.EthAddress) bool {
	return k.IsOnBlacklist(ctx, addr) || addr == types.ZeroAddress()
}
