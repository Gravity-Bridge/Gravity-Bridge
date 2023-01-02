package keeper

import (
	"fmt"
	"sort"

	gethcommon "github.com/ethereum/go-ethereum/common"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	distrkeeper "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	paramtypes "github.com/cosmos/cosmos-sdk/x/params/types"
	slashingkeeper "github.com/cosmos/cosmos-sdk/x/slashing/keeper"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	ibctransferkeeper "github.com/cosmos/ibc-go/v3/modules/apps/transfer/keeper"
	"github.com/tendermint/tendermint/libs/log"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
	"github.com/cosmos/cosmos-sdk/store/prefix"
	ibctransfertypes "github.com/cosmos/ibc-go/v3/modules/core/05-port/types"
	bech32ibckeeper "github.com/osmosis-labs/bech32-ibc/x/bech32ibc/keeper"
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
	cdc               codec.BinaryCodec // The wire codec for binary encoding/decoding.
	bankKeeper        *bankkeeper.BaseKeeper
	StakingKeeper     *stakingkeeper.Keeper
	SlashingKeeper    *slashingkeeper.Keeper
	DistKeeper        *distrkeeper.Keeper
	accountKeeper     *authkeeper.AccountKeeper
	ibcTransferKeeper *ibctransferkeeper.Keeper
	bech32IbcKeeper   *bech32ibckeeper.Keeper
	ics4Wrapper       ibctransfertypes.ICS4Wrapper

	AttestationHandler interface {
		Handle(sdk.Context, types.Attestation, types.EthereumClaim) error
	}
}

// Check for nil members
func (k Keeper) ValidateMembers() {
	if k.bankKeeper == nil {
		panic("Nil bankKeeper!")
	}
	if k.StakingKeeper == nil {
		panic("Nil StakingKeeper!")
	}
	if k.SlashingKeeper == nil {
		panic("Nil SlashingKeeper!")
	}
	if k.DistKeeper == nil {
		panic("Nil DistKeeper!")
	}
	if k.accountKeeper == nil {
		panic("Nil accountKeeper!")
	}
	if k.ibcTransferKeeper == nil {
		panic("Nil ibcTransferKeeper!")
	}
	if k.bech32IbcKeeper == nil {
		panic("Nil bech32IbcKeeper!")
	}
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
	ibcTransferKeeper *ibctransferkeeper.Keeper,
	bech32IbcKeeper *bech32ibckeeper.Keeper,
	ics4Wrapper ibctransfertypes.ICS4Wrapper,
) Keeper {
	// set KeyTable if it has not already been set
	if !paramSpace.HasKeyTable() {
		paramSpace = paramSpace.WithKeyTable(types.ParamKeyTable())
	}

	k := Keeper{
		storeKey:   storeKey,
		paramSpace: paramSpace,

		cdc:                cdc,
		bankKeeper:         bankKeeper,
		StakingKeeper:      stakingKeeper,
		SlashingKeeper:     slashingKeeper,
		DistKeeper:         distKeeper,
		accountKeeper:      accKeeper,
		ibcTransferKeeper:  ibcTransferKeeper,
		bech32IbcKeeper:    bech32IbcKeeper,
		ics4Wrapper:        ics4Wrapper,
		AttestationHandler: nil,
	}
	attestationHandler := AttestationHandler{keeper: &k}
	attestationHandler.ValidateMembers()
	k.AttestationHandler = attestationHandler

	k.ValidateMembers()

	return k
}

////////////////////////
/////// HELPERS ////////
////////////////////////

// SendToCommunityPool handles incorrect SendToCosmos calls to the community pool, since the calls
// have already been made on evm chain there's nothing we can do to reverse them, and we should at least
// make use of the tokens which would otherwise be lost
func (k Keeper) SendToCommunityPool(ctx sdk.Context, coins sdk.Coins) error {
	if err := k.bankKeeper.SendCoinsFromModuleToModule(ctx, types.ModuleName, distrtypes.ModuleName, coins); err != nil {
		return sdkerrors.Wrap(err, "transfer to community pool failed")
	}
	feePool := k.DistKeeper.GetFeePool(ctx)
	feePool.CommunityPool = feePool.CommunityPool.Add(sdk.NewDecCoinsFromCoins(coins...)...)
	k.DistKeeper.SetFeePool(ctx, feePool)
	return nil
}

/////////////////////////////
//////// PARAMETERS /////////
/////////////////////////////

// GetParamsIfSet returns the parameters from the store if they exist, or an error
// This is useful for certain contexts where the store is not yet set up, like
// in an AnteHandler during InitGenesis
func (k Keeper) GetParamsIfSet(ctx sdk.Context) (params types.Params, err error) {
	for _, pair := range params.ParamSetPairs() {
		if !k.paramSpace.Has(ctx, pair.Key) {
			return types.Params{}, sdkerrors.Wrapf(sdkerrors.ErrNotFound, "the param key %s has not been set", string(pair.Key))
		}
		k.paramSpace.Get(ctx, pair.Key, pair.Value)
	}

	return
}

// GetParams returns the parameters from the store
func (k Keeper) GetParams(ctx sdk.Context) (params types.Params) {
	k.paramSpace.GetParamSet(ctx, &params)
	return
}

// SetParams sets the parameters in the store
func (k Keeper) SetParams(ctx sdk.Context, ps types.Params) {
	k.paramSpace.SetParamSet(ctx, &ps)
}

// GetBridgeContractAddress returns the bridge contract address on evm chain
func (k Keeper) GetBridgeContractAddress(ctx sdk.Context) *types.EthAddress {
	var a string
	k.paramSpace.Get(ctx, types.ParamsStoreKeyBridgeEthereumAddress, &a)
	addr, err := types.NewEthAddress(a)
	if err != nil {
		panic(sdkerrors.Wrapf(err, "found invalid bridge contract address in store: %v", a))
	}
	return addr
}

// GetBridgeChainID returns the chain id of the evm chain we are running against
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
// a vector of MsgSetOrchestratorAddress entires containing all the delegate keys for state
// export / import. This may seem at first glance to be excessively complicated, why not combine
// the EthAddress and Orchestrator address indexes and simply iterate one thing? The answer is that
// even though we set the evm and Orchestrator address in the same place we use them differently we
// always go from Orchestrator address to Validator address and from validator address to evm chain address
// we want to keep looking up the validator address for various reasons, so a direct Orchestrator to evm chain
// address mapping will mean having to keep two of the same data around just to provide lookups.
//
// For the time being this will serve
func (k Keeper) GetDelegateKeys(ctx sdk.Context) []types.MsgSetOrchestratorAddress {
	store := ctx.KVStore(k.storeKey)
	prefix := types.EthAddressByValidatorKey
	iter := store.Iterator(prefixRange(prefix))
	defer iter.Close()

	evmAddresses := make(map[string]gethcommon.Address)

	for ; iter.Valid(); iter.Next() {
		// the 'key' contains both the prefix and the value, so we need
		// to cut off the starting bytes, if you don't do this a valid
		// cosmos key will be made out of EthAddressByValidatorKey + the starting bytes
		// of the actual key
		key := iter.Key()[len(types.EthAddressByValidatorKey):]
		value := iter.Value()
		evmAddress, err := types.NewEthAddressFromBytes(value)
		if err != nil {
			panic(sdkerrors.Wrapf(err, "found invalid ethAddress %v under key %v", string(value), key))
		}
		valAddress := sdk.ValAddress(key)
		if err := sdk.VerifyAddressFormat(valAddress); err != nil {
			panic(sdkerrors.Wrapf(err, "invalid valAddress in key %v", valAddress))
		}
		evmAddresses[valAddress.String()] = evmAddress.GetAddress()
	}

	store = ctx.KVStore(k.storeKey)
	prefix = types.KeyOrchestratorAddress
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

	for valAddr, evmAddr := range evmAddresses {
		orch, ok := orchAddresses[valAddr]
		if !ok {
			// this should never happen unless the store
			// is somehow inconsistent
			panic("Can't find address")
		}
		result = append(result, types.MsgSetOrchestratorAddress{
			Orchestrator: orch,
			Validator:    valAddr,
			EthAddress:   evmAddr.Hex(),
		})

	}

	// we iterated over a map, so now we have to sort to ensure the
	// output here is deterministic, evm address chosen for no particular
	// reason
	sort.Slice(result, func(i, j int) bool {
		return result[i].EthAddress < result[j].EthAddress
	})

	return result
}

// IterateEthAddressesByValidator executes the given callback cb with every value stored under EthAddressByValidatorKey
// cb should return true if iteration must stop, false if it should continue
func (k Keeper) IterateEthAddressesByValidator(ctx sdk.Context, cb func(key []byte, value types.EthAddress) (stop bool)) {
	prefixStore := prefix.NewStore(ctx.KVStore(k.storeKey), types.EthAddressByValidatorKey)
	iter := prefixStore.Iterator(nil, nil)
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		key := iter.Key()
		val := iter.Value()
		value, err := types.NewEthAddressFromBytes(val)
		if err != nil {
			panic(fmt.Sprintf("Unable to unmarshal EthAddress for validator with key %v and bytes %v", key, val))
		}

		if cb(key, *value) {
			break
		}
	}
}

// IterateValidatorsByEthAddress executes the given callback cb with every value stored under ValidatorByEthAddressKey
// cb should return true if iteration must stop, false if it should continue
func (k Keeper) IterateValidatorsByEthAddress(ctx sdk.Context, cb func(key []byte, value sdk.ValAddress) (stop bool)) {
	prefixStore := prefix.NewStore(ctx.KVStore(k.storeKey), types.ValidatorByEthAddressKey)
	iter := prefixStore.Iterator(nil, nil)
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		key := iter.Key()
		val := iter.Value()
		value := sdk.ValAddress(val)

		if cb(key, value) {
			break
		}
	}
}

// IterateValidatorsByOrchestratorAddress executes the given callback cb with every value stored under KeyOrchestratorAddress
// cb should return true if iteration must stop, false if it should continue
func (k Keeper) IterateValidatorsByOrchestratorAddress(ctx sdk.Context, cb func(key []byte, value sdk.ValAddress) (stop bool)) {
	prefixStore := prefix.NewStore(ctx.KVStore(k.storeKey), types.KeyOrchestratorAddress)
	iter := prefixStore.Iterator(nil, nil)
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		key := iter.Key()
		val := iter.Value()
		value := sdk.ValAddress(val)

		if cb(key, value) {
			break
		}
	}
}

/////////////////////////////
//// Logic Call Slashing ////
/////////////////////////////

// SetLastSlashedLogicCallBlock returns true if the last slashed logic call block
// has been set in the store
func (k Keeper) HasLastSlashedLogicCallBlock(ctx sdk.Context, evmChainPrefix string) bool {
	store := ctx.KVStore(k.storeKey)
	return store.Has(types.AppendChainPrefix(types.LastSlashedLogicCallBlock, evmChainPrefix))
}

// SetLastSlashedLogicCallBlock sets the latest slashed logic call block height
func (k Keeper) SetLastSlashedLogicCallBlock(ctx sdk.Context, evmChainPrefix string, blockHeight uint64) {

	if k.HasLastSlashedLogicCallBlock(ctx, evmChainPrefix) && k.GetLastSlashedLogicCallBlock(ctx, evmChainPrefix) > blockHeight {
		panic("Attempted to decrement LastSlashedBatchBlock")
	}

	store := ctx.KVStore(k.storeKey)
	store.Set(types.AppendChainPrefix(types.LastSlashedLogicCallBlock, evmChainPrefix), types.UInt64Bytes(blockHeight))
}

// GetLastSlashedLogicCallBlock returns the latest slashed logic call block
func (k Keeper) GetLastSlashedLogicCallBlock(ctx sdk.Context, evmChainPrefix string) uint64 {
	store := ctx.KVStore(k.storeKey)
	bytes := store.Get(types.AppendChainPrefix(types.LastSlashedLogicCallBlock, evmChainPrefix))

	if len(bytes) == 0 {
		panic("Last slashed logic call block not initialized in genesis")
	}
	return types.UInt64FromBytesUnsafe(bytes)
}

// GetUnSlashedLogicCalls returns all the unslashed logic calls in state
func (k Keeper) GetUnSlashedLogicCalls(ctx sdk.Context, evmChainPrefix string, maxHeight uint64) (out []types.OutgoingLogicCall) {
	lastSlashedLogicCallBlock := k.GetLastSlashedLogicCallBlock(ctx, evmChainPrefix)
	calls := k.GetOutgoingLogicCalls(ctx, evmChainPrefix)
	for _, call := range calls {
		if call.CosmosBlockCreated > lastSlashedLogicCallBlock {
			out = append(out, call)
		}
	}
	return
}

/////////////////////////////
//////// Parameters /////////
/////////////////////////////

// prefixRange turns a prefix into a (start, end) range. The start is the given prefix value and
// the end is calculated by adding 1 bit to the start value. Nil is not allowed as prefix.
// Example: []byte{1, 3, 4} becomes []byte{1, 3, 5}
// []byte{15, 42, 255, 255} becomes []byte{15, 43, 0, 0}
//
// In case of an overflow the end is set to nil.
// Example: []byte{255, 255, 255, 255} becomes nil
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

// Checks if the provided evm address is on the Governance blacklist
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

// Returns true if the provided address is invalid to send to evm chain this could be
// for one of several reasons. (1) it is invalid in general like the Zero address, (2)
// it is invalid for a subset of ERC20 addresses or (3) it is on the governance deposit/withdraw
// blacklist. (2) is not yet implemented
// Blocking some addresses is technically motivated, if any ERC20 transfers in a batch fail the entire batch
// becomes impossible to execute.
func (k Keeper) InvalidSendToEthAddress(ctx sdk.Context, addr types.EthAddress, _erc20Addr types.EthAddress) bool {
	return k.IsOnBlacklist(ctx, addr) || addr == types.ZeroAddress()
}
