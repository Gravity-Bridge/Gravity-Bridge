package types

import (
	"time"

	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	bank "github.com/cosmos/cosmos-sdk/x/bank/types"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	gravitytypes "github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
)

// StakingKeeper defines the expected staking keeper methods
type StakingKeeper interface {
	GetBondedValidatorsByPower(ctx sdk.Context) []stakingtypes.Validator
	GetLastValidatorPower(ctx sdk.Context, operator sdk.ValAddress) int64
	GetLastTotalPower(ctx sdk.Context) (power sdk.Int)
	IterateValidators(sdk.Context, func(index int64, validator stakingtypes.ValidatorI) (stop bool))
	ValidatorQueueIterator(ctx sdk.Context, endTime time.Time, endHeight int64) sdk.Iterator
	GetParams(ctx sdk.Context) stakingtypes.Params
	GetValidator(ctx sdk.Context, addr sdk.ValAddress) (validator stakingtypes.Validator, found bool)
	IterateBondedValidatorsByPower(sdk.Context, func(index int64, validator stakingtypes.ValidatorI) (stop bool))
	IterateLastValidators(sdk.Context, func(index int64, validator stakingtypes.ValidatorI) (stop bool))
	Validator(sdk.Context, sdk.ValAddress) stakingtypes.ValidatorI
	ValidatorByConsAddr(sdk.Context, sdk.ConsAddress) stakingtypes.ValidatorI
	Slash(sdk.Context, sdk.ConsAddress, int64, int64, sdk.Dec)
	Jail(sdk.Context, sdk.ConsAddress)
}

// BankKeeper defines the expected bank keeper methods
type BankKeeper interface {
	SendCoinsFromModuleToAccount(ctx sdk.Context, senderModule string, recipientAddr sdk.AccAddress, amt sdk.Coins) error
	SendCoinsFromAccountToModule(ctx sdk.Context, senderAddr sdk.AccAddress, recipientModule string, amt sdk.Coins) error
	SendCoinsFromModuleToModule(ctx sdk.Context, senderModule string, recipientModule string, amt sdk.Coins) error
	MintCoins(ctx sdk.Context, name string, amt sdk.Coins) error
	BurnCoins(ctx sdk.Context, name string, amt sdk.Coins) error
	GetAllBalances(ctx sdk.Context, addr sdk.AccAddress) sdk.Coins
	GetDenomMetaData(ctx sdk.Context, denom string) bank.Metadata
}

type SlashingKeeper interface {
	GetValidatorSigningInfo(ctx sdk.Context, address sdk.ConsAddress) (info slashingtypes.ValidatorSigningInfo, found bool)
}

type DistributionKeeper interface {
	FundCommunityPool(ctx sdk.Context, amount sdk.Coins, sender sdk.AccAddress) error
	GetFeePool(ctx sdk.Context) (feePool distrtypes.FeePool)
	SetFeePool(ctx sdk.Context, feePool distrtypes.FeePool)
}

type GravityKeeper interface {
	SendToCommunityPool(ctx sdk.Context, coins sdk.Coins) error
	GetParams(ctx sdk.Context) (params gravitytypes.Params)
	SetParams(ctx sdk.Context, ps gravitytypes.Params)
	GetBridgeContractAddress(ctx sdk.Context) *gravitytypes.EthAddress
	GetBridgeChainID(ctx sdk.Context) uint64
	GetGravityID(ctx sdk.Context) string
	SetGravityID(ctx sdk.Context, v string)
	UnpackAttestationClaim(att *gravitytypes.Attestation) (gravitytypes.EthereumClaim, error)
	GetDelegateKeys(ctx sdk.Context) []gravitytypes.MsgSetOrchestratorAddress
	HasLastSlashedLogicCallBlock(ctx sdk.Context) bool
	SetLastSlashedLogicCallBlock(ctx sdk.Context, blockHeight uint64)
	GetLastSlashedLogicCallBlock(ctx sdk.Context) uint64
	GetUnSlashedLogicCalls(ctx sdk.Context, maxHeight uint64) (out []gravitytypes.OutgoingLogicCall)
	DeserializeValidatorIterator(vals []byte) stakingtypes.ValAddresses
	IsOnBlacklist(ctx sdk.Context, addr gravitytypes.EthAddress) bool
	InvalidSendToEthAddress(ctx sdk.Context, addr gravitytypes.EthAddress, _erc20Addr gravitytypes.EthAddress) bool
	Attest(ctx sdk.Context, claim gravitytypes.EthereumClaim, anyClaim *codectypes.Any) (*gravitytypes.Attestation, error)
	TryAttestation(ctx sdk.Context, att *gravitytypes.Attestation)
	SetAttestation(ctx sdk.Context, eventNonce uint64, claimHash []byte, att *gravitytypes.Attestation)
	GetAttestation(ctx sdk.Context, eventNonce uint64, claimHash []byte) *gravitytypes.Attestation
	DeleteAttestation(ctx sdk.Context, att gravitytypes.Attestation)
	GetAttestationMapping(ctx sdk.Context) (attestationMapping map[uint64][]gravitytypes.Attestation, orderedKeys []uint64)
	IterateAttestations(ctx sdk.Context, reverse bool, cb func([]byte, gravitytypes.Attestation) bool)
	GetMostRecentAttestations(ctx sdk.Context, limit uint64) []gravitytypes.Attestation
	GetLastObservedEventNonce(ctx sdk.Context) uint64
	GetLastObservedEthereumBlockHeight(ctx sdk.Context) gravitytypes.LastObservedEthereumBlockHeight
	SetLastObservedEthereumBlockHeight(ctx sdk.Context, ethereumHeight uint64)
	GetLastObservedValset(ctx sdk.Context) *gravitytypes.Valset
	SetLastObservedValset(ctx sdk.Context, valset gravitytypes.Valset)
	SetLastEventNonceByValidator(ctx sdk.Context, validator sdk.ValAddress, nonce uint64)
	BuildOutgoingTXBatch(ctx sdk.Context, contract gravitytypes.EthAddress, maxElements uint) (*gravitytypes.InternalOutgoingTxBatch, error)
	OutgoingTxBatchExecuted(ctx sdk.Context, tokenContract gravitytypes.EthAddress, claim gravitytypes.MsgBatchSendToEthClaim)
	StoreBatch(ctx sdk.Context, batch gravitytypes.InternalOutgoingTxBatch)
	DeleteBatch(ctx sdk.Context, batch gravitytypes.InternalOutgoingTxBatch)
	GetOutgoingTXBatch(ctx sdk.Context, tokenContract gravitytypes.EthAddress, nonce uint64) *gravitytypes.InternalOutgoingTxBatch
	CancelOutgoingTXBatch(ctx sdk.Context, tokenContract gravitytypes.EthAddress, nonce uint64) error
	IterateOutgoingTXBatches(ctx sdk.Context, cb func(key []byte, batch gravitytypes.InternalOutgoingTxBatch) bool)
	GetOutgoingTxBatches(ctx sdk.Context) (out []gravitytypes.InternalOutgoingTxBatch)
	GetLastOutgoingBatchByTokenType(ctx sdk.Context, token gravitytypes.EthAddress) *gravitytypes.InternalOutgoingTxBatch
	HasLastSlashedBatchBlock(ctx sdk.Context) bool
	SetLastSlashedBatchBlock(ctx sdk.Context, blockHeight uint64)
	GetLastSlashedBatchBlock(ctx sdk.Context) uint64
	GetUnSlashedBatches(ctx sdk.Context, maxHeight uint64) (out []gravitytypes.InternalOutgoingTxBatch)
	GetCosmosOriginatedDenom(ctx sdk.Context, tokenContract gravitytypes.EthAddress) (string, bool)
	GetCosmosOriginatedERC20(ctx sdk.Context, denom string) (*gravitytypes.EthAddress, bool)
	DenomToERC20Lookup(ctx sdk.Context, denom string) (bool, *gravitytypes.EthAddress, error)
	RewardToERC20Lookup(ctx sdk.Context, coin sdk.Coin) (*gravitytypes.EthAddress, sdk.Int)
	ERC20ToDenomLookup(ctx sdk.Context, tokenContract gravitytypes.EthAddress) (bool, string)
	IterateERC20ToDenom(ctx sdk.Context, cb func([]byte, *gravitytypes.ERC20ToDenom) bool)
	CheckBadSignatureEvidence(ctx sdk.Context, msg *gravitytypes.MsgSubmitBadSignatureEvidence) error
	SetPastEthSignatureCheckpoint(ctx sdk.Context, checkpoint []byte)
	GetPastEthSignatureCheckpoint(ctx sdk.Context, checkpoint []byte) (found bool)
	HandleUnhaltBridgeProposal(ctx sdk.Context, p *gravitytypes.UnhaltBridgeProposal) error
	HandleAirdropProposal(ctx sdk.Context, p *gravitytypes.AirdropProposal) error
	HandleIBCMetadataProposal(ctx sdk.Context, p *gravitytypes.IBCMetadataProposal) error
	ValidatePendingIbcAutoForward(ctx sdk.Context, forward gravitytypes.PendingIbcAutoForward) error
	GetNextPendingIbcAutoForward(ctx sdk.Context) *gravitytypes.PendingIbcAutoForward
	PendingIbcAutoForwards(ctx sdk.Context, limit uint64) []*gravitytypes.PendingIbcAutoForward
	ProcessPendingIbcAutoForwards(ctx sdk.Context, forwardsToClear uint64) error
	ProcessNextPendingIbcAutoForward(ctx sdk.Context) (stop bool, err error)
	GetBatchConfirm(ctx sdk.Context, nonce uint64, tokenContract gravitytypes.EthAddress, validator sdk.AccAddress) *gravitytypes.MsgConfirmBatch
	SetBatchConfirm(ctx sdk.Context, batch *gravitytypes.MsgConfirmBatch) []byte
	DeleteBatchConfirms(ctx sdk.Context, batch gravitytypes.InternalOutgoingTxBatch)
	IterateBatchConfirmByNonceAndTokenContract(ctx sdk.Context, nonce uint64, tokenContract gravitytypes.EthAddress, cb func([]byte, gravitytypes.MsgConfirmBatch) bool)
	GetBatchConfirmByNonceAndTokenContract(ctx sdk.Context, nonce uint64, tokenContract gravitytypes.EthAddress) (out []gravitytypes.MsgConfirmBatch)
	SetOrchestratorValidator(ctx sdk.Context, val sdk.ValAddress, orch sdk.AccAddress)
	GetOrchestratorValidator(ctx sdk.Context, orch sdk.AccAddress) (validator stakingtypes.Validator, found bool)
	GetOrchestratorValidatorAddr(ctx sdk.Context, orch sdk.AccAddress) (validator sdk.ValAddress, found bool)
	SetEthAddressForValidator(ctx sdk.Context, validator sdk.ValAddress, ethAddr gravitytypes.EthAddress)
	GetEthAddressByValidator(ctx sdk.Context, validator sdk.ValAddress) (ethAddress *gravitytypes.EthAddress, found bool)
	GetValidatorByEthAddress(ctx sdk.Context, ethAddr gravitytypes.EthAddress) (validator stakingtypes.Validator, found bool)
	GetOutgoingLogicCall(ctx sdk.Context, invalidationID []byte, invalidationNonce uint64) *gravitytypes.OutgoingLogicCall
	SetOutgoingLogicCall(ctx sdk.Context, call gravitytypes.OutgoingLogicCall)
	DeleteOutgoingLogicCall(ctx sdk.Context, invalidationID []byte, invalidationNonce uint64)
	IterateOutgoingLogicCalls(ctx sdk.Context, cb func([]byte, gravitytypes.OutgoingLogicCall) bool)
	GetOutgoingLogicCalls(ctx sdk.Context) (out []gravitytypes.OutgoingLogicCall)
	CancelOutgoingLogicCall(ctx sdk.Context, invalidationId []byte, invalidationNonce uint64) error
	SetLogicCallConfirm(ctx sdk.Context, msg *gravitytypes.MsgConfirmLogicCall)
	GetLogicCallConfirm(ctx sdk.Context, invalidationId []byte, invalidationNonce uint64, val sdk.AccAddress) *gravitytypes.MsgConfirmLogicCall
	DeleteLogicCallConfirm(ctx sdk.Context, invalidationID []byte, invalidationNonce uint64, val sdk.AccAddress)
	IterateLogicConfirmByInvalidationIDAndNonce(ctx sdk.Context, invalidationID []byte, invalidationNonce uint64, cb func([]byte, *gravitytypes.MsgConfirmLogicCall) bool)
	GetLogicConfirmByInvalidationIDAndNonce(ctx sdk.Context, invalidationId []byte, invalidationNonce uint64) (out []gravitytypes.MsgConfirmLogicCall)
	SetValsetRequest(ctx sdk.Context) gravitytypes.Valset
	StoreValset(ctx sdk.Context, valset gravitytypes.Valset)
	HasValsetRequest(ctx sdk.Context, nonce uint64) bool
	DeleteValset(ctx sdk.Context, nonce uint64)
	CheckLatestValsetNonce(ctx sdk.Context) bool
	GetLatestValsetNonce(ctx sdk.Context) uint64
	SetLatestValsetNonce(ctx sdk.Context, nonce uint64)
	GetValset(ctx sdk.Context, nonce uint64) *gravitytypes.Valset
	IterateValsets(ctx sdk.Context, cb func(key []byte, val *gravitytypes.Valset) bool)
	GetValsets(ctx sdk.Context) (out []gravitytypes.Valset)
	GetLatestValset(ctx sdk.Context) (out *gravitytypes.Valset)
	SetLastSlashedValsetNonce(ctx sdk.Context, nonce uint64)
	GetLastSlashedValsetNonce(ctx sdk.Context) uint64
	SetLastUnBondingBlockHeight(ctx sdk.Context, unbondingBlockHeight uint64)
	GetLastUnBondingBlockHeight(ctx sdk.Context) uint64
	GetUnSlashedValsets(ctx sdk.Context, signedValsetsWindow uint64) (out []*gravitytypes.Valset)
	IterateValsetBySlashedValsetNonce(ctx sdk.Context, lastSlashedValsetNonce uint64, cb func([]byte, *gravitytypes.Valset) bool)
	GetCurrentValset(ctx sdk.Context) (gravitytypes.Valset, error)
	GetValsetConfirm(ctx sdk.Context, nonce uint64, validator sdk.AccAddress) *gravitytypes.MsgValsetConfirm
	SetValsetConfirm(ctx sdk.Context, valsetConf gravitytypes.MsgValsetConfirm) []byte
	GetValsetConfirms(ctx sdk.Context, nonce uint64) (confirms []gravitytypes.MsgValsetConfirm)
	DeleteValsetConfirms(ctx sdk.Context, nonce uint64)
	AddToOutgoingPool(ctx sdk.Context, sender sdk.AccAddress, counterpartReceiver gravitytypes.EthAddress, amount sdk.Coin, fee sdk.Coin) (uint64, error)
	RemoveFromOutgoingPoolAndRefund(ctx sdk.Context, txId uint64, sender sdk.AccAddress) error
	GetUnbatchedTxByFeeAndId(ctx sdk.Context, fee gravitytypes.InternalERC20Token, txID uint64) (*gravitytypes.InternalOutgoingTransferTx, error)
	GetUnbatchedTxById(ctx sdk.Context, txID uint64) (*gravitytypes.InternalOutgoingTransferTx, error)
	GetUnbatchedTransactionsByContract(ctx sdk.Context, contractAddress gravitytypes.EthAddress) []*gravitytypes.InternalOutgoingTransferTx
	GetUnbatchedTransactions(ctx sdk.Context) []*gravitytypes.InternalOutgoingTransferTx
	IterateUnbatchedTransactionsByContract(ctx sdk.Context, contractAddress gravitytypes.EthAddress, cb func(key []byte, tx *gravitytypes.InternalOutgoingTransferTx) bool)
	IterateUnbatchedTransactions(ctx sdk.Context, prefixKey []byte, cb func(key []byte, tx *gravitytypes.InternalOutgoingTransferTx) bool)
	GetBatchFeeByTokenType(ctx sdk.Context, tokenContractAddr gravitytypes.EthAddress, maxElements uint) *gravitytypes.BatchFees
	GetAllBatchFees(ctx sdk.Context, maxElements uint) (batchFees []gravitytypes.BatchFees)
}
