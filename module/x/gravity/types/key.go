package types

import (
	"strings"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

const (
	// ModuleName is the name of the module
	ModuleName = "gravity"

	// StoreKey to be used when creating the KVStore
	StoreKey = ModuleName

	// RouterKey is the module name router key
	RouterKey = ModuleName

	// QuerierRoute to be used for querierer msgs
	QuerierRoute = ModuleName
)

var (
	// EthAddressByValidatorKey indexes cosmos validator account addresses
	// i.e. cosmos1ahx7f8wyertuus9r20284ej0asrs085case3kn
	EthAddressByValidatorKey = "EthAddressValidatorKey"

	// ValidatorByEthAddressKey indexes ethereum addresses
	// i.e. 0xAb5801a7D398351b8bE11C439e05C5B3259aeC9B
	ValidatorByEthAddressKey = "ValidatorByEthAddressKey"

	// ValsetRequestKey indexes valset requests by nonce
	ValsetRequestKey = "ValsetRequestKey"

	// ValsetConfirmKey indexes valset confirmations by nonce and the validator account address
	// i.e cosmos1ahx7f8wyertuus9r20284ej0asrs085case3kn
	ValsetConfirmKey = "ValsetConfirmKey"

	// OracleClaimKey Claim details by nonce and validator address
	// i.e. cosmosvaloper1ahx7f8wyertuus9r20284ej0asrs085case3kn
	// A claim is named more intuitively than an Attestation, it is literally
	// a validator making a claim to have seen something happen. Claims are
	// attached to attestations which can be thought of as 'the event' that
	// will eventually be executed.
	OracleClaimKey = "OracleClaimKey"

	// OracleAttestationKey attestation details by nonce and validator address
	// i.e. cosmosvaloper1ahx7f8wyertuus9r20284ej0asrs085case3kn
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

	// OutgoingTXBatchBlockKey indexes outgoing tx batches under a block height and token address
	OutgoingTXBatchBlockKey = "OutgoingTXBatchBlockKey"

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

// GetOrchestratorAddressKey returns the following key format
// prefix
// [0xe8][cosmos1ahx7f8wyertuus9r20284ej0asrs085case3kn]
func GetOrchestratorAddressKey(orc sdk.AccAddress) string {
	return KeyOrchestratorAddress + string(orc.Bytes())
}

// GetEthAddressByValidatorKey returns the following key format
// prefix              cosmos-validator
// [0x0][cosmosvaloper1ahx7f8wyertuus9r20284ej0asrs085case3kn]
func GetEthAddressByValidatorKey(validator sdk.ValAddress) string {
	return EthAddressByValidatorKey + string(validator.Bytes())
}

// GetValidatorByEthAddressKey returns the following key format
// prefix              cosmos-validator
// [0xf9][0xAb5801a7D398351b8bE11C439e05C5B3259aeC9B]
func GetValidatorByEthAddressKey(ethAddress EthAddress) string {
	return ValidatorByEthAddressKey + string([]byte(ethAddress.GetAddress()))
}

// GetValsetKey returns the following key format
// prefix    nonce
// [0x0][0 0 0 0 0 0 0 1]
func GetValsetKey(nonce uint64) string {
	return ValsetRequestKey + string(UInt64Bytes(nonce))
}

// GetValsetConfirmKey returns the following key format
// prefix   nonce                    validator-address
// [0x0][0 0 0 0 0 0 0 1][cosmos1ahx7f8wyertuus9r20284ej0asrs085case3kn]
// MARK finish-batches: this is where the key is created in the old (presumed working) code
func GetValsetConfirmKey(nonce uint64, validator sdk.AccAddress) string {
	return ValsetConfirmKey + convertByteArrToString(UInt64Bytes(nonce)) + convertByteArrToString(validator.Bytes())
}

// GetClaimKey returns the following key format
// prefix type               cosmos-validator-address                       nonce                             attestation-details-hash
// [0x0][0 0 0 1][cosmosvaloper1ahx7f8wyertuus9r20284ej0asrs085case3kn][0 0 0 0 0 0 0 1][fd1af8cec6c67fcf156f1b61fdf91ebc04d05484d007436e75342fc05bbff35a]
// The Claim hash identifies a unique event, for example it would have a event nonce, a sender and a receiver. Or an event nonce and a batch nonce. But
// the Claim is stored indexed with the claimer key to make sure that it is unique.
func GetClaimKey(details EthereumClaim) string {
	var detailsHash []byte
	if details != nil {
		var err error
		detailsHash, err = details.ClaimHash()
		if err != nil {
			panic(sdkerrors.Wrap(err, "unable to compute claim hash"))
		}
	} else {
		panic("No claim without details!")
	}
	claimTypeLen := len([]byte{byte(details.GetType())})
	nonceBz := UInt64Bytes(details.GetEventNonce())
	key := make([]byte, len(OracleClaimKey)+claimTypeLen+sdk.AddrLen+len(nonceBz)+len(detailsHash))
	copy(key[0:], OracleClaimKey)
	copy(key[len(OracleClaimKey):], []byte{byte(details.GetType())})
	// TODO this is the delegate address, should be stored by the valaddress
	copy(key[len(OracleClaimKey)+claimTypeLen:], details.GetClaimer())
	copy(key[len(OracleClaimKey)+claimTypeLen+sdk.AddrLen:], nonceBz)
	copy(key[len(OracleClaimKey)+claimTypeLen+sdk.AddrLen+len(nonceBz):], detailsHash)
	return convertByteArrToString(key)
}

// GetAttestationKey returns the following key format
// prefix     nonce                             claim-details-hash
// [0x5][0 0 0 0 0 0 0 1][fd1af8cec6c67fcf156f1b61fdf91ebc04d05484d007436e75342fc05bbff35a]
// An attestation is an event multiple people are voting on, this function needs the claim
// details because each Attestation is aggregating all claims of a specific event, lets say
// validator X and validator y where making different claims about the same event nonce
// Note that the claim hash does NOT include the claimer address and only identifies an event
func GetAttestationKey(eventNonce uint64, claimHash []byte) string {
	key := make([]byte, len(OracleAttestationKey)+len(UInt64Bytes(0))+len(claimHash))
	copy(key[0:], OracleAttestationKey)
	copy(key[len(OracleAttestationKey):], UInt64Bytes(eventNonce))
	copy(key[len(OracleAttestationKey)+len(UInt64Bytes(0)):], claimHash)
	return convertByteArrToString(key)
}

// GetOutgoingTxPoolContractPrefix returns the following key format
// prefix	feeContract
// [0x6][0xc783df8a850f42e7F7e57013759C285caa701eB6]
// This prefix is used for iterating over unbatched transactions for a given contract
func GetOutgoingTxPoolContractPrefix(contractAddress EthAddress) string {
	return OutgoingTXPoolKey + contractAddress.GetAddress()
}

// GetOutgoingTxPoolKey returns the following key format
// prefix	feeContract		feeAmount     id
// [0x6][0xc783df8a850f42e7F7e57013759C285caa701eB6][1000000000][0 0 0 0 0 0 0 1]
func GetOutgoingTxPoolKey(fee InternalERC20Token, id uint64) string {
	// sdkInts have a size limit of 255 bits or 32 bytes
	// therefore this will never panic and is always safe
	amount := make([]byte, 32)
	amount = fee.Amount.BigInt().FillBytes(amount)

	a := append(amount, UInt64Bytes(id)...)
	b := append([]byte(fee.Contract.GetAddress()), a...)
	r := append([]byte(OutgoingTXPoolKey), b...)
	return convertByteArrToString(r)
}

// GetOutgoingTxBatchKey returns the following key format
// prefix     nonce                     eth-contract-address
// [0xa][0 0 0 0 0 0 0 1][0xc783df8a850f42e7F7e57013759C285caa701eB6]
func GetOutgoingTxBatchKey(tokenContract EthAddress, nonce uint64) string {
	return OutgoingTXBatchKey + tokenContract.GetAddress() + string(UInt64Bytes(nonce))
}

// GetOutgoingTxBatchBlockKey returns the following key format
// prefix     blockheight
// [0xb][0 0 0 0 2 1 4 3]
func GetOutgoingTxBatchBlockKey(block uint64) string {
	return OutgoingTXBatchBlockKey + string(UInt64Bytes(block))
}

// GetBatchConfirmKey returns the following key format
// prefix           eth-contract-address                BatchNonce                       Validator-address
// [0xe1][0xc783df8a850f42e7F7e57013759C285caa701eB6][0 0 0 0 0 0 0 1][cosmosvaloper1ahx7f8wyertuus9r20284ej0asrs085case3kn]
// TODO this should be a sdk.ValAddress
func GetBatchConfirmKey(tokenContract EthAddress, batchNonce uint64, validator sdk.AccAddress) string {
	a := append(UInt64Bytes(batchNonce), validator.Bytes()...)
	b := append([]byte(tokenContract.GetAddress()), a...)
	c := BatchConfirmKey + string(b)
	return c
}

// GetLastEventNonceByValidatorKey indexes lateset event nonce by validator
// GetLastEventNonceByValidatorKey returns the following key format
// prefix              cosmos-validator
// [0x0][cosmos1ahx7f8wyertuus9r20284ej0asrs085case3kn]
func GetLastEventNonceByValidatorKey(validator sdk.ValAddress) string {
	return LastEventNonceByValidatorKey + string(validator.Bytes())
}

func GetDenomToERC20Key(denom string) string {
	return DenomToERC20Key + denom
}

func GetERC20ToDenomKey(erc20 EthAddress) string {
	return ERC20ToDenomKey + erc20.GetAddress()
}

func GetOutgoingLogicCallKey(invalidationId []byte, invalidationNonce uint64) string {
	a := KeyOutgoingLogicCall + string(invalidationId)
	return a + string(UInt64Bytes(invalidationNonce))
}

func GetLogicConfirmKey(invalidationId []byte, invalidationNonce uint64, validator sdk.AccAddress) string {
	interm := KeyOutgoingLogicConfirm + string(invalidationId)
	interm = interm + string(UInt64Bytes(invalidationNonce))
	return interm + convertByteArrToString(validator.Bytes())
}

// GetPastEthSignatureCheckpointKey returns the following key format
// prefix    checkpoint
// [0x0][ checkpoint bytes ]
func GetPastEthSignatureCheckpointKey(checkpoint []byte) string {
	return PastEthSignatureCheckpointKey + convertByteArrToString(checkpoint)
}

func convertByteArrToString(value []byte) string {
	var ret strings.Builder
	for i := 0; i < len(value); i++ {
		ret.WriteString(string(value[i]))
	}
	return ret.String()
}
