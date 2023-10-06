package v4

// This file is a copy of the necessary gravity v4 definitions for Params, so that we can add new params before
// the chain starts up. This file is used for the v4 -> v5 migration, NOT for v3 -> v4!
// THE CONTENTS OF THIS FILE ARE LIKELY TO BE INCORRECT! ONLY THE BARE MINIMUM HAS BEEN COPIED
// AND SEVERAL METHODS HAVE BEEN REPLACED WITH NO-OPS! YOU HAVE BEEN WARNED!

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
	github_com_cosmos_cosmos_sdk_types "github.com/cosmos/cosmos-sdk/types"
	paramtypes "github.com/cosmos/cosmos-sdk/x/params/types"
	"github.com/gogo/protobuf/proto"
)

type Params struct {
	GravityId                    string                                  `protobuf:"bytes,1,opt,name=gravity_id,json=gravityId,proto3" json:"gravity_id,omitempty"`
	ContractSourceHash           string                                  `protobuf:"bytes,2,opt,name=contract_source_hash,json=contractSourceHash,proto3" json:"contract_source_hash,omitempty"`
	BridgeEthereumAddress        string                                  `protobuf:"bytes,4,opt,name=bridge_ethereum_address,json=bridgeEthereumAddress,proto3" json:"bridge_ethereum_address,omitempty"`
	BridgeChainId                uint64                                  `protobuf:"varint,5,opt,name=bridge_chain_id,json=bridgeChainId,proto3" json:"bridge_chain_id,omitempty"`
	SignedValsetsWindow          uint64                                  `protobuf:"varint,6,opt,name=signed_valsets_window,json=signedValsetsWindow,proto3" json:"signed_valsets_window,omitempty"`
	SignedBatchesWindow          uint64                                  `protobuf:"varint,7,opt,name=signed_batches_window,json=signedBatchesWindow,proto3" json:"signed_batches_window,omitempty"`
	SignedLogicCallsWindow       uint64                                  `protobuf:"varint,8,opt,name=signed_logic_calls_window,json=signedLogicCallsWindow,proto3" json:"signed_logic_calls_window,omitempty"`
	TargetBatchTimeout           uint64                                  `protobuf:"varint,9,opt,name=target_batch_timeout,json=targetBatchTimeout,proto3" json:"target_batch_timeout,omitempty"`
	AverageBlockTime             uint64                                  `protobuf:"varint,10,opt,name=average_block_time,json=averageBlockTime,proto3" json:"average_block_time,omitempty"`
	AverageEthereumBlockTime     uint64                                  `protobuf:"varint,11,opt,name=average_ethereum_block_time,json=averageEthereumBlockTime,proto3" json:"average_ethereum_block_time,omitempty"`
	SlashFractionValset          github_com_cosmos_cosmos_sdk_types.Dec  `protobuf:"bytes,12,opt,name=slash_fraction_valset,json=slashFractionValset,proto3,customtype=github.com/cosmos/cosmos-sdk/types.Dec" json:"slash_fraction_valset"`
	SlashFractionBatch           github_com_cosmos_cosmos_sdk_types.Dec  `protobuf:"bytes,13,opt,name=slash_fraction_batch,json=slashFractionBatch,proto3,customtype=github.com/cosmos/cosmos-sdk/types.Dec" json:"slash_fraction_batch"`
	SlashFractionLogicCall       github_com_cosmos_cosmos_sdk_types.Dec  `protobuf:"bytes,14,opt,name=slash_fraction_logic_call,json=slashFractionLogicCall,proto3,customtype=github.com/cosmos/cosmos-sdk/types.Dec" json:"slash_fraction_logic_call"`
	UnbondSlashingValsetsWindow  uint64                                  `protobuf:"varint,15,opt,name=unbond_slashing_valsets_window,json=unbondSlashingValsetsWindow,proto3" json:"unbond_slashing_valsets_window,omitempty"`
	SlashFractionBadEthSignature github_com_cosmos_cosmos_sdk_types.Dec  `protobuf:"bytes,16,opt,name=slash_fraction_bad_eth_signature,json=slashFractionBadEthSignature,proto3,customtype=github.com/cosmos/cosmos-sdk/types.Dec" json:"slash_fraction_bad_eth_signature"`
	ValsetReward                 github_com_cosmos_cosmos_sdk_types.Coin `protobuf:"bytes,17,opt,name=valset_reward,json=valsetReward,proto3" json:"valset_reward"`
	BridgeActive                 bool                                    `protobuf:"varint,18,opt,name=bridge_active,json=bridgeActive,proto3" json:"bridge_active,omitempty"`
	// addresses on this blacklist are forbidden from depositing or withdrawing
	// from Ethereum to the bridge
	EthereumBlacklist      []string `protobuf:"bytes,19,rep,name=ethereum_blacklist,json=ethereumBlacklist,proto3" json:"ethereum_blacklist,omitempty"`
	MinChainFeeBasisPoints uint64   `protobuf:"varint,20,opt,name=min_chain_fee_basis_points,json=minChainFeeBasisPoints,proto3" json:"min_chain_fee_basis_points,omitempty"`
}

// ParamSetPairs implements the ParamSet interface and returns all the key/value pairs
// pairs of auth module's parameters.
func (p *Params) ParamSetPairs() paramtypes.ParamSetPairs {
	return paramtypes.ParamSetPairs{
		paramtypes.NewParamSetPair(types.ParamsStoreKeyGravityID, &p.GravityId, validateGravityID),
		paramtypes.NewParamSetPair(types.ParamsStoreKeyContractHash, &p.ContractSourceHash, validateContractHash),
		paramtypes.NewParamSetPair(types.ParamsStoreKeyBridgeEthereumAddress, &p.BridgeEthereumAddress, validateBridgeContractAddress),
		paramtypes.NewParamSetPair(types.ParamsStoreKeyBridgeContractChainID, &p.BridgeChainId, validateBridgeChainID),
		paramtypes.NewParamSetPair(types.ParamsStoreKeySignedValsetsWindow, &p.SignedValsetsWindow, validateSignedValsetsWindow),
		paramtypes.NewParamSetPair(types.ParamsStoreKeySignedBatchesWindow, &p.SignedBatchesWindow, validateSignedBatchesWindow),
		paramtypes.NewParamSetPair(types.ParamsStoreKeySignedLogicCallsWindow, &p.SignedLogicCallsWindow, validateSignedLogicCallsWindow),
		paramtypes.NewParamSetPair(types.ParamsStoreKeyTargetBatchTimeout, &p.TargetBatchTimeout, validateTargetBatchTimeout),
		paramtypes.NewParamSetPair(types.ParamsStoreKeyAverageBlockTime, &p.AverageBlockTime, validateAverageBlockTime),
		paramtypes.NewParamSetPair(types.ParamsStoreKeyAverageEthereumBlockTime, &p.AverageEthereumBlockTime, validateAverageEthereumBlockTime),
		paramtypes.NewParamSetPair(types.ParamsStoreSlashFractionValset, &p.SlashFractionValset, validateSlashFractionValset),
		paramtypes.NewParamSetPair(types.ParamsStoreSlashFractionBatch, &p.SlashFractionBatch, validateSlashFractionBatch),
		paramtypes.NewParamSetPair(types.ParamStoreUnbondSlashingValsetsWindow, &p.UnbondSlashingValsetsWindow, validateUnbondSlashingValsetsWindow),
		paramtypes.NewParamSetPair(types.ParamStoreSlashFractionBadEthSignature, &p.SlashFractionBadEthSignature, validateSlashFractionBadEthSignature),
		paramtypes.NewParamSetPair(types.ParamStoreValsetRewardAmount, &p.ValsetReward, validateValsetRewardAmount),
		paramtypes.NewParamSetPair(types.ParamStoreBridgeActive, &p.BridgeActive, validateBridgeActive),
		paramtypes.NewParamSetPair(types.ParamStoreEthereumBlacklist, &p.EthereumBlacklist, validateEthereumBlacklistAddresses),
		paramtypes.NewParamSetPair(types.ParamStoreMinChainFeeBasisPoints, &p.MinChainFeeBasisPoints, validateMinChainFeeBasisPoints),
	}
}

// nolint: exhaustruct
func (m *Params) Reset()         { *m = Params{} }
func (m *Params) String() string { return proto.CompactTextString(m) }
func (*Params) ProtoMessage()    {}
func (m *Params) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Params.Merge(m, src)
}
func (m *Params) XXX_DiscardUnknown() {
	xxx_messageInfo_Params.DiscardUnknown(m)
}

var xxx_messageInfo_Params proto.InternalMessageInfo

func (m *Params) GetGravityId() string {
	if m != nil {
		return m.GravityId
	}
	return ""
}

func (m *Params) GetContractSourceHash() string {
	if m != nil {
		return m.ContractSourceHash
	}
	return ""
}

func (m *Params) GetBridgeEthereumAddress() string {
	if m != nil {
		return m.BridgeEthereumAddress
	}
	return ""
}

func (m *Params) GetBridgeChainId() uint64 {
	if m != nil {
		return m.BridgeChainId
	}
	return 0
}

func (m *Params) GetSignedValsetsWindow() uint64 {
	if m != nil {
		return m.SignedValsetsWindow
	}
	return 0
}

func (m *Params) GetSignedBatchesWindow() uint64 {
	if m != nil {
		return m.SignedBatchesWindow
	}
	return 0
}

func (m *Params) GetSignedLogicCallsWindow() uint64 {
	if m != nil {
		return m.SignedLogicCallsWindow
	}
	return 0
}

func (m *Params) GetTargetBatchTimeout() uint64 {
	if m != nil {
		return m.TargetBatchTimeout
	}
	return 0
}

func (m *Params) GetAverageBlockTime() uint64 {
	if m != nil {
		return m.AverageBlockTime
	}
	return 0
}

func (m *Params) GetAverageEthereumBlockTime() uint64 {
	if m != nil {
		return m.AverageEthereumBlockTime
	}
	return 0
}

func (m *Params) GetUnbondSlashingValsetsWindow() uint64 {
	if m != nil {
		return m.UnbondSlashingValsetsWindow
	}
	return 0
}

func (m *Params) GetValsetReward() github_com_cosmos_cosmos_sdk_types.Coin {
	if m != nil {
		return m.ValsetReward
	}
	// nolint: exhaustruct
	return github_com_cosmos_cosmos_sdk_types.Coin{}
}

func (m *Params) GetBridgeActive() bool {
	if m != nil {
		return m.BridgeActive
	}
	return false
}

func (m *Params) GetEthereumBlacklist() []string {
	if m != nil {
		return m.EthereumBlacklist
	}
	return nil
}

func (m *Params) GetMinChainFeeBasisPoints() uint64 {
	if m != nil {
		return m.MinChainFeeBasisPoints
	}
	return 0
}

func validateGravityID(i interface{}) error {
	v, ok := i.(string)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}
	if _, err := strToFixByteArray(v); err != nil {
		return err
	}
	return nil
}

func validateContractHash(i interface{}) error {
	// TODO: should we validate that the input here is a properly formatted
	// SHA256 (or other) hash?
	if _, ok := i.(string); !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}
	return nil
}

func validateBridgeChainID(i interface{}) error {
	if _, ok := i.(uint64); !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}
	return nil
}

func validateTargetBatchTimeout(i interface{}) error {
	val, ok := i.(uint64)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	} else if val < 60000 {
		return fmt.Errorf("invalid target batch timeout, less than 60 seconds is too short")
	}
	return nil
}

func validateAverageBlockTime(i interface{}) error {
	val, ok := i.(uint64)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	} else if val < 100 {
		return fmt.Errorf("invalid average Cosmos block time, too short for latency limitations")
	}
	return nil
}

func validateAverageEthereumBlockTime(i interface{}) error {
	val, ok := i.(uint64)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	} else if val < 100 {
		return fmt.Errorf("invalid average Ethereum block time, too short for latency limitations")
	}
	return nil
}

func validateBridgeContractAddress(i interface{}) error {
	v, ok := i.(string)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}
	if err := types.ValidateEthAddress(v); err != nil {
		// TODO: ensure that empty addresses are valid in params
		if !strings.Contains(err.Error(), "empty") {
			return err
		}
	}
	return nil
}

func validateSignedValsetsWindow(i interface{}) error {
	// TODO: do we want to set some bounds on this value?
	if _, ok := i.(uint64); !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}
	return nil
}

func validateUnbondSlashingValsetsWindow(i interface{}) error {
	// TODO: do we want to set some bounds on this value?
	if _, ok := i.(uint64); !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}
	return nil
}

func validateSlashFractionValset(i interface{}) error {
	// TODO: do we want to set some bounds on this value?
	if _, ok := i.(github_com_cosmos_cosmos_sdk_types.Dec); !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}
	return nil
}

func validateSignedBatchesWindow(i interface{}) error {
	// TODO: do we want to set some bounds on this value?
	if _, ok := i.(uint64); !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}
	return nil
}

func validateSignedLogicCallsWindow(i interface{}) error {
	// TODO: do we want to set some bounds on this value?
	if _, ok := i.(uint64); !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}
	return nil
}

func validateSlashFractionBatch(i interface{}) error {
	// TODO: do we want to set some bounds on this value?
	if _, ok := i.(github_com_cosmos_cosmos_sdk_types.Dec); !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}
	return nil
}

func validateSlashFractionBadEthSignature(i interface{}) error {
	// TODO: do we want to set some bounds on this value?
	if _, ok := i.(github_com_cosmos_cosmos_sdk_types.Dec); !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}
	return nil
}

func validateValsetRewardAmount(i interface{}) error {
	if _, ok := i.(github_com_cosmos_cosmos_sdk_types.Coin); !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}
	return nil
}

func validateBridgeActive(i interface{}) error {
	if _, ok := i.(bool); !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}
	return nil
}

func validateEthereumBlacklistAddresses(i interface{}) error {
	strArr, ok := i.([]string)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}
	for index, value := range strArr {
		if err := types.ValidateEthAddress(value); err != nil {

			if !strings.Contains(err.Error(), "empty, index is"+strconv.Itoa(index)) {
				return err
			}
		}
	}
	return nil
}

func validateMinChainFeeBasisPoints(i interface{}) error {
	v, ok := i.(uint64)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}
	if v >= 10000 {
		return fmt.Errorf("MinChainFeeBasisPoints is set to 10000 or more, this is an unreasonable fee amount")
	}
	return nil
}

func strToFixByteArray(s string) ([32]byte, error) {
	var out [32]byte
	if len([]byte(s)) > 32 {
		return out, fmt.Errorf("string too long")
	}
	copy(out[:], s)
	return out, nil
}
