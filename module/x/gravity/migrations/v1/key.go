package v1

import (
	"strings"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

var (
	StoreKey = "gravity"
	// EthAddressByValidatorKey indexes cosmos validator account addresses
	// i.e. gravity1ahx7f8wyertuus9r20284ej0asrs085ceqtfnm
	EthAddressByValidatorKey = "EthAddressValidatorKey"

	// ValidatorByEthAddressKey indexes ethereum addresses
	// i.e. 0xAb5801a7D398351b8bE11C439e05C5B3259aeC9B
	ValidatorByEthAddressKey = "ValidatorByEthAddressKey"

	// ValsetRequestKey indexes valset requests by nonce
	ValsetRequestKey = "ValsetRequestKey"

	// ValsetConfirmKey indexes valset confirmations by nonce and the validator account address
	// i.e gravity1ahx7f8wyertuus9r20284ej0asrs085ceqtfnm
	ValsetConfirmKey = "ValsetConfirmKey"

	// OracleClaimKey Claim details by nonce and validator address
	// i.e. gravityvaloper1ahx7f8wyertuus9r20284ej0asrs085ceqtfnm
	// A claim is named more intuitively than an Attestation, it is literally
	// a validator making a claim to have seen something happen. Claims are
	// attached to attestations which can be thought of as 'the event' that
	// will eventually be executed.
	OracleClaimKey = "OracleClaimKey"

	// OracleAttestationKey attestation details by nonce and validator address
	// i.e. gravityvaloper1ahx7f8wyertuus9r20284ej0asrs085ceqtfnm
	// An attestation can be thought of as the 'event to be executed' while
	// the Claims are an individual validator saying that they saw an event
	// occur the Attestation is 'the event' that multiple claims vote on and
	// eventually executes
	OracleAttestationKey = "OracleAttestationKey"

	// OutgoingTXPoolKey indexes the last nonce for the outgoing tx pool
	OutgoingTXPoolKey = "OutgoingTXPoolKey"

	// DenomiatorPrefix indexes token contract addresses from ETH on gravity
	DenomiatorPrefix = "DenomiatorPrefix"

	// OutgoingTXBatchKey indexes outgoing tx batches under a nonce and token address
	OutgoingTXBatchKey = "OutgoingTXBatchKey"

	// BatchConfirmKey indexes validator confirmations by token contract address
	BatchConfirmKey = "BatchConfirmKey"

	// SecondIndexNonceByClaimKey indexes latest nonce for a given claim type
	SecondIndexNonceByClaimKey = "SecondIndexNonceByClaimKey"

	// LastEventNonceByValidatorKey indexes lateset event nonce by validator
	LastEventNonceByValidatorKey = "LastEventNonceByValidatorKey"

	// LastObservedEventNonceKey indexes the latest event nonce
	LastObservedEventNonceKey = "LastObservedEventNonceKey"

	// SequenceKeyPrefix indexes different txids
	SequenceKeyPrefix = "SequenceKeyPrefix"

	// KeyLastTXPoolID indexes the lastTxPoolID
	KeyLastTXPoolID = SequenceKeyPrefix + "lastTxPoolId"

	// KeyLastOutgoingBatchID indexes the lastBatchID
	KeyLastOutgoingBatchID = SequenceKeyPrefix + "lastBatchId"

	// KeyOrchestratorAddress indexes the validator keys for an orchestrator
	KeyOrchestratorAddress = "KeyOrchestratorAddress"

	// KeyOutgoingLogicCall indexes the outgoing logic calls
	KeyOutgoingLogicCall = "KeyOutgoingLogicCall"

	// KeyOutgoingLogicConfirm indexes the outgoing logic confirms
	KeyOutgoingLogicConfirm = "KeyOutgoingLogicConfirm"

	// LastObservedEthereumBlockHeightKey indexes the latest Ethereum block height
	LastObservedEthereumBlockHeightKey = "LastObservedEthereumBlockHeightKey"

	// DenomToERC20Key prefixes the index of Cosmos originated asset denoms to ERC20s
	DenomToERC20Key = "DenomToERC20Key"

	// ERC20ToDenomKey prefixes the index of Cosmos originated assets ERC20s to denoms
	ERC20ToDenomKey = "ERC20ToDenomKey"

	// LastSlashedValsetNonce indexes the latest slashed valset nonce
	LastSlashedValsetNonce = "LastSlashedValsetNonce"

	// LatestValsetNonce indexes the latest valset nonce
	LatestValsetNonce = "LatestValsetNonce"

	// LastSlashedBatchBlock indexes the latest slashed batch block height
	LastSlashedBatchBlock = "LastSlashedBatchBlock"

	// LastSlashedLogicCallBlock indexes the latest slashed logic call block height
	LastSlashedLogicCallBlock = "LastSlashedLogicCallBlock"

	// LastUnBondingBlockHeight indexes the last validator unbonding block height
	LastUnBondingBlockHeight = "LastUnBondingBlockHeight"

	// LastObservedValsetNonceKey indexes the latest observed valset nonce
	// HERE THERE BE DRAGONS, do not use this value as an up to date validator set
	// on Ethereum it will always lag significantly and may be totally wrong at some
	// times.
	LastObservedValsetKey = "LastObservedValsetKey"

	// PastEthSignatureCheckpointKey indexes eth signature checkpoints that have existed
	PastEthSignatureCheckpointKey = "PastEthSignatureCheckpointKey"
)

// GetEthAddressByValidatorPrefix returns
// prefix
// [0x0]
func GetEthAddressByValidatorPrefix() string {
	return EthAddressByValidatorKey
}

// GetEthAddressByValidatorKey returns the following key format
// prefix              cosmos-validator
// [0x0][gravityvaloper1ahx7f8wyertuus9r20284ej0asrs085ceqtfnm]
func GetEthAddressByValidatorKey(validator sdk.ValAddress) string {
	if err := sdk.VerifyAddressFormat(validator); err != nil {
		panic(sdkerrors.Wrap(err, "invalid validator address"))
	}
	return GetEthAddressByValidatorPrefix() + string(validator.Bytes())
}

// GetValidatorByEthAddressPrefix returns
// prefix
// [0xf9]
func GetValidatorByEthAddressPrefix() string {
	return ValidatorByEthAddressKey
}

// GetValidatorByEthAddressKey returns the following key format
// prefix              cosmos-validator
// [0xf9][0xAb5801a7D398351b8bE11C439e05C5B3259aeC9B]
func GetValidatorByEthAddressKey(ethAddress types.EthAddress) string {
	return GetValidatorByEthAddressPrefix() + string([]byte(ethAddress.GetAddress().String()))
}

// GetLastEventNonceByValidatorPrefix returns
// prefix
// [0x0]
func GetLastEventNonceByValidatorPrefix() string {
	return LastEventNonceByValidatorKey
}

// GetOrchestratorAddressPrefix returns
// prefix
// [0xe8]
func GetOrchestratorAddressPrefix() string {
	return KeyOrchestratorAddress
}

// GetOrchestratorAddressKey returns the following key format
// prefix address
// [0xe8][gravity1ahx7f8wyertuus9r20284ej0asrs085ceqtfnm]
func GetOrchestratorAddressKey(orc sdk.AccAddress) string {
	if err := sdk.VerifyAddressFormat(orc); err != nil {
		panic(sdkerrors.Wrap(err, "invalid orchestrator address"))
	}
	return GetOrchestratorAddressPrefix() + string(orc.Bytes())
}

func GetDenomToERC20Key(denom string) string {
	return DenomToERC20Key + denom
}

func GetERC20ToDenomKey(erc20 types.EthAddress) string {
	return ERC20ToDenomKey + erc20.GetAddress().String()
}

// GetOutgoingTxBatchContractPrefix returns the following format
// prefix     eth-contract-address
// [0xa][0xc783df8a850f42e7F7e57013759C285caa701eB6]
func GetOutgoingTxBatchContractPrefix(tokenContract types.EthAddress) string {
	return OutgoingTXBatchKey + tokenContract.GetAddress().String()
}

// GetOutgoingTxBatchKey returns the following key format
// prefix     eth-contract-address                     nonce
// [0xa][0xc783df8a850f42e7F7e57013759C285caa701eB6][0 0 0 0 0 0 0 1]
func GetOutgoingTxBatchKey(tokenContract types.EthAddress, nonce uint64) string {
	return GetOutgoingTxBatchContractPrefix(tokenContract) + ConvertByteArrToString(types.UInt64Bytes(nonce))
}

// GetOutgoingTxPoolContractPrefix returns
// prefix	feeContract
// [0x6][0xc783df8a850f42e7F7e57013759C285caa701eB6]
// This prefix is used for iterating over unbatched transactions for a given contract
func GetOutgoingTxPoolContractPrefix(contractAddress types.EthAddress) string {
	return OutgoingTXPoolKey + contractAddress.GetAddress().String()
}

// GetOutgoingTxPoolKey returns the following key format
// prefix	feeContract		feeAmount     id
// [0x6][0xc783df8a850f42e7F7e57013759C285caa701eB6][1000000000][0 0 0 0 0 0 0 1]
func GetOutgoingTxPoolKey(fee types.InternalERC20Token, id uint64) string {
	// sdkInts have a size limit of 255 bits or 32 bytes
	// therefore this will never panic and is always safe
	amount := make([]byte, 32)
	amount = fee.Amount.BigInt().FillBytes(amount)

	amount = append(amount, types.UInt64Bytes(id)...)
	amount = append([]byte(fee.Contract.GetAddress().String()), amount...)
	amount = append([]byte(OutgoingTXPoolKey), amount...)
	return ConvertByteArrToString(amount)
}

func GetOutgoingLogicCallKey(invalidationId []byte, invalidationNonce uint64) string {
	a := KeyOutgoingLogicCall + string(invalidationId)
	return a + string(types.UInt64Bytes(invalidationNonce))
}

func GetLogicConfirmNonceInvalidationIdPrefix(invalidationId []byte, invalidationNonce uint64) string {
	return KeyOutgoingLogicConfirm + string(invalidationId) + ConvertByteArrToString(types.UInt64Bytes(invalidationNonce))
}

func GetLogicConfirmKey(invalidationId []byte, invalidationNonce uint64, validator sdk.AccAddress) string {
	if err := sdk.VerifyAddressFormat(validator); err != nil {
		panic(sdkerrors.Wrap(err, "invalid validator address"))
	}
	return GetLogicConfirmNonceInvalidationIdPrefix(invalidationId, invalidationNonce) + string(validator.Bytes())
}

// GetAttestationKey returns the following key format
// prefix     nonce                             claim-details-hash
// [0x5][0 0 0 0 0 0 0 1][fd1af8cec6c67fcf156f1b61fdf91ebc04d05484d007436e75342fc05bbff35a]
// An attestation is an event multiple people are voting on, this function needs the claim
// details because each Attestation is aggregating all claims of a specific event, lets say
// validator X and validator y were making different claims about the same event nonce
// Note that the claim hash does NOT include the claimer address and only identifies an event
func GetAttestationKey(eventNonce uint64, claimHash []byte) string {
	key := make([]byte, len(OracleAttestationKey)+len(types.UInt64Bytes(0))+len(claimHash))
	copy(key[0:], OracleAttestationKey)
	copy(key[len(OracleAttestationKey):], types.UInt64Bytes(eventNonce))
	copy(key[len(OracleAttestationKey)+len(types.UInt64Bytes(0)):], claimHash)
	return ConvertByteArrToString(key)
}

// GetValsetKey returns
// prefix
// [0x0]
func GetValsetPrefix() string {
	return ValsetRequestKey
}

// GetValsetKey returns the following key format
// prefix    nonce
// [0x0][0 0 0 0 0 0 0 1]
func GetValsetKey(nonce uint64) string {
	return GetValsetPrefix() + string(types.UInt64Bytes(nonce))
}

// GetValsetConfirmNoncePrefix returns the following format
// prefix   nonce
// [0x0][0 0 0 0 0 0 0 1]
func GetValsetConfirmNoncePrefix(nonce uint64) string {
	return ValsetConfirmKey + ConvertByteArrToString(types.UInt64Bytes(nonce))
}

// GetValsetConfirmKey returns the following key format
// prefix   nonce                    validator-address
// [0x0][0 0 0 0 0 0 0 1][gravity1ahx7f8wyertuus9r20284ej0asrs085ceqtfnm]
// MARK finish-batches: this is where the key is created in the old (presumed working) code
func GetValsetConfirmKey(nonce uint64, validator sdk.AccAddress) string {
	if err := sdk.VerifyAddressFormat(validator); err != nil {
		panic(sdkerrors.Wrap(err, "invalid validator address"))
	}
	return GetValsetConfirmNoncePrefix(nonce) + string(validator.Bytes())
}

// GetBatchConfirmNonceContractPrefix returns
// prefix           eth-contract-address                BatchNonce
// [0xe1][0xc783df8a850f42e7F7e57013759C285caa701eB6][0 0 0 0 0 0 0 1]
func GetBatchConfirmNonceContractPrefix(tokenContract types.EthAddress, batchNonce uint64) string {
	return BatchConfirmKey + tokenContract.GetAddress().String() + ConvertByteArrToString(types.UInt64Bytes(batchNonce))
}

// GetBatchConfirmKey returns the following key format
// prefix           eth-contract-address                BatchNonce                       Validator-address
// [0xe1][0xc783df8a850f42e7F7e57013759C285caa701eB6][0 0 0 0 0 0 0 1][gravityvaloper1ahx7f8wyertuus9r20284ej0asrs085ceqtfnm]
// TODO this should be a sdk.ValAddress
func GetBatchConfirmKey(tokenContract types.EthAddress, batchNonce uint64, validator sdk.AccAddress) string {
	if err := sdk.VerifyAddressFormat(validator); err != nil {
		panic(sdkerrors.Wrap(err, "invalid validator address"))
	}
	return GetBatchConfirmNonceContractPrefix(tokenContract, batchNonce) + string(validator.Bytes())
}

// GetLastEventNonceByValidatorKey indexes lateset event nonce by validator
// GetLastEventNonceByValidatorKey returns the following key format
// prefix              cosmos-validator
// [0x0][gravity1ahx7f8wyertuus9r20284ej0asrs085ceqtfnm]
func GetLastEventNonceByValidatorKey(validator sdk.ValAddress) string {
	if err := sdk.VerifyAddressFormat(validator); err != nil {
		panic(sdkerrors.Wrap(err, "invalid validator address"))
	}
	return GetLastEventNonceByValidatorPrefix() + string(validator.Bytes())
}

// GetPastEthSignatureCheckpointKey returns the following key format
// prefix    checkpoint
// [0x0][ checkpoint bytes ]
func GetPastEthSignatureCheckpointKey(checkpoint []byte) string {
	return PastEthSignatureCheckpointKey + ConvertByteArrToString(checkpoint)
}

func ConvertByteArrToString(value []byte) string {
	var ret strings.Builder
	for i := 0; i < len(value); i++ {
		ret.WriteString(string(value[i]))
	}
	return ret.String()
}
