package v3_test

import (
	"bytes"
	"encoding/hex"
	"testing"

	_ "github.com/Gravity-Bridge/Gravity-Bridge/module/config"
	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/keeper"
	v2 "github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/migrations/v2"
	v3 "github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/migrations/v3"
	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/std"
	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/cosmos/cosmos-sdk/x/auth"
	"github.com/cosmos/cosmos-sdk/x/auth/vesting"
	"github.com/cosmos/cosmos-sdk/x/bank"
	"github.com/cosmos/cosmos-sdk/x/capability"
	"github.com/cosmos/cosmos-sdk/x/crisis"
	"github.com/cosmos/cosmos-sdk/x/distribution"
	distrclient "github.com/cosmos/cosmos-sdk/x/distribution/client"
	"github.com/cosmos/cosmos-sdk/x/evidence"
	"github.com/cosmos/cosmos-sdk/x/genutil"
	"github.com/cosmos/cosmos-sdk/x/gov"
	"github.com/cosmos/cosmos-sdk/x/mint"
	"github.com/cosmos/cosmos-sdk/x/params"
	paramsclient "github.com/cosmos/cosmos-sdk/x/params/client"
	"github.com/cosmos/cosmos-sdk/x/slashing"
	"github.com/cosmos/cosmos-sdk/x/staking"
	"github.com/cosmos/cosmos-sdk/x/upgrade"
	upgradeclient "github.com/cosmos/cosmos-sdk/x/upgrade/client"
	"github.com/stretchr/testify/require"
)

const denom string = "graviton"

func TestMigrateStoreKeys(t *testing.T) {

	keeper.SetupTestConfig()

	// create old prefixes KV store
	gravityKey := sdk.NewKVStoreKey("gravity")
	ctx := testutil.DefaultContext(gravityKey, sdk.NewTransientStoreKey("transient-test"))
	store := ctx.KVStore(gravityKey)

	marshaler := keeper.MakeTestMarshaler()

	ethAddr, _ := types.NewEthAddress("0x2a24af0501a534fca004ee1bd667b783f205a546")
	tokenContract, _ := types.NewEthAddress("0x429881672B9AE42b8EbA0E26cD9C73711b891Ca5")
	accAddr, _ := sdk.AccAddressFromBech32("gravity1ahx7f8wyertuus9r20284ej0asrs085ceqtfnm")
	validatorAddr, _ := sdk.ValAddressFromBech32("gravityvaloper1jpz0ahls2chajf78nkqczdwwuqcu97w6j77vg6")

	dummyValue := []byte("dummy")
	dummyCheckpoint := "0x1de95c9ace999f8ec70c6dc8d045942da2612950567c4861aca959c0650194da"

	dummyOutgoingTxBatch := types.OutgoingTxBatch{
		BatchNonce:         1,
		BatchTimeout:       0,
		Transactions:       []types.OutgoingTransferTx{},
		TokenContract:      ethAddr.GetAddress().String(),
		CosmosBlockCreated: 123,
	}

	dummyValset := types.Valset{
		Nonce:        1,
		Members:      []types.BridgeValidator{},
		Height:       128,
		RewardAmount: sdk.NewInt(1),
		RewardToken:  "footoken",
	}

	dummyValsetConfirm := types.MsgValsetConfirm{
		Nonce:          1,
		Orchestrator:   accAddr.String(),
		EthAddress:     ethAddr.GetAddress().String(),
		Signature:      "dummySignature",
		EvmChainPrefix: keeper.EthChainPrefix,
	}

	dummyBatchConfirm := types.MsgConfirmBatch{
		Nonce:          1,
		TokenContract:  tokenContract.GetAddress().String(),
		EthSigner:      ethAddr.GetAddress().String(),
		Orchestrator:   accAddr.String(),
		Signature:      "dummySignature",
		EvmChainPrefix: keeper.EthChainPrefix,
	}

	// additional data for creating InternalOutgoingTransferTx
	tokenId := uint64(1)
	myReceiver, _ := types.NewEthAddress("0xd041c41EA1bf0F006ADBb6d2c9ef9D425dE5eaD7")
	tokenFee, err := types.NewInternalERC20Token(sdk.NewIntFromUint64(3), tokenContract.GetAddress().String())
	require.NoError(t, err)
	tokenAmount, err := types.NewInternalERC20Token(sdk.NewIntFromUint64(101), tokenContract.GetAddress().String())
	require.NoError(t, err)

	dummyInternalOutgoingTransferTx, _ := types.NewInternalOutgoingTransferTx(tokenId, accAddr.String(), myReceiver.GetAddress().String(), tokenAmount.ToExternal(), tokenFee.ToExternal())
	require.NoError(t, err)

	val := dummyInternalOutgoingTransferTx.ToExternal()
	bz, err := marshaler.Marshal(&val)
	require.NoError(t, err)

	// additinal data for creating Attestation
	nonce := uint64(1)

	msg := types.MsgSendToCosmosClaim{
		EventNonce:     nonce,
		EthBlockHeight: 1,
		TokenContract:  "0x00000000000000000001",
		Amount:         sdk.NewInt(10000000000 + int64(1)),
		EthereumSender: "0x00000000000000000002",
		CosmosReceiver: "0x00000000000000000003",
		Orchestrator:   "0x00000000000000000004",
		EvmChainPrefix: keeper.EthChainPrefix,
	}
	any, _ := codectypes.NewAnyWithValue(&msg)

	hash, err := msg.ClaimHash()
	require.NoError(t, err)

	dummyAttestation := &types.Attestation{
		Observed: false,
		Height:   uint64(1),
		Claim:    any,
	}

	// additional data for creating OutgoingLogicCall and MsgConfirmLogicCall
	logicContract := "0x510ab76899430424d209a6c9a5b9951fb8a6f47d"
	payload := []byte("fake bytes")
	invalidationId := []byte("GravityTesting")
	invalidationNonce := 1

	token := []types.ERC20Token{{
		Contract: tokenContract.GetAddress().String(),
		Amount:   sdk.NewIntFromUint64(5000),
	}}

	call := types.OutgoingLogicCall{
		Transfers:            token,
		Fees:                 token,
		LogicContractAddress: logicContract,
		Payload:              payload,
		Timeout:              10000,
		InvalidationId:       invalidationId,
		InvalidationNonce:    uint64(invalidationNonce),
	}

	var valAddr sdk.AccAddress = bytes.Repeat([]byte{byte(1)}, 20)
	valAccAdd, err := sdk.AccAddressFromBech32(valAddr.String())
	require.NoError(t, err)

	confirm := types.MsgConfirmLogicCall{
		InvalidationId:    hex.EncodeToString(invalidationId),
		InvalidationNonce: 1,
		EthSigner:         "dummySignature",
		Orchestrator:      valAddr.String(),
		Signature:         "dummySignature",
		EvmChainPrefix:    keeper.EthChainPrefix,
	}
	decInvalidationId, err := hex.DecodeString(confirm.InvalidationId)
	require.NoError(t, err)

	dummyPendingIbcAutoForward := types.PendingIbcAutoForward{
		ForeignReceiver: "0x00000000000000000001",
		Token:           &sdk.Coin{Denom: "", Amount: sdk.ZeroInt()},
		IbcChannel:      "channel-0",
		EventNonce:      0,
	}

	// creating test cases
	// OLD key: []byte prefix key + some value (address, nonce....)
	// NEW key: []byte prefix key + chain prefix + some value (address, nonce....)
	migrateTestCases := []struct {
		name   string
		oldKey []byte
		newKey []byte
		value  []byte
	}{
		{
			"LastObservedEventNonceKey",
			v2.LastObservedEventNonceKey,
			types.AppendChainPrefix(types.LastObservedEventNonceKey, v3.EthereumChainPrefix),
			dummyValue,
		},
		{
			"KeyLastTXPoolID",
			v2.KeyLastTXPoolID,
			types.AppendChainPrefix(types.KeyLastTXPoolID, v3.EthereumChainPrefix),
			dummyValue,
		},
		{
			"KeyLastOutgoingBatchID",
			v2.KeyLastOutgoingBatchID,
			types.AppendChainPrefix(types.KeyLastOutgoingBatchID, v3.EthereumChainPrefix),
			dummyValue,
		},
		{
			"LastObservedEthereumBlockHeightKey",
			v2.LastObservedEthereumBlockHeightKey,
			types.AppendChainPrefix(types.LastObservedEvmBlockHeightKey, v3.EthereumChainPrefix),
			dummyValue,
		},
		{
			"LastSlashedValsetNonce",
			v2.LastSlashedValsetNonce,
			types.AppendChainPrefix(types.LastSlashedValsetNonce, v3.EthereumChainPrefix),
			dummyValue,
		},
		{
			"LatestValsetNonce",
			v2.LatestValsetNonce,
			types.AppendChainPrefix(types.LatestValsetNonce, v3.EthereumChainPrefix),
			dummyValue,
		},
		{
			"LastSlashedBatchBlock",
			v2.LastSlashedBatchBlock,
			types.AppendChainPrefix(types.LastSlashedBatchBlock, v3.EthereumChainPrefix),
			dummyValue,
		},
		{
			"LastSlashedLogicCallBlock",
			v2.LastSlashedLogicCallBlock,
			types.AppendChainPrefix(types.LastSlashedLogicCallBlock, v3.EthereumChainPrefix),
			dummyValue,
		},
		{
			"LastUnBondingBlockHeight",
			v2.LastUnBondingBlockHeight,
			types.LastUnBondingBlockHeight,
			types.UInt64Bytes(1),
		},
		{
			"LastObservedValsetKey",
			v2.LastObservedValsetKey,
			types.AppendChainPrefix(types.LastObservedValsetKey, v3.EthereumChainPrefix),
			dummyValue,
		},
		{
			"ValidatorByEthAddressKey",
			v2.GetValidatorByEthAddressKey(*ethAddr),
			types.GetValidatorByEthAddressKey(*ethAddr),
			validatorAddr.Bytes(),
		},
		{
			"LastEventNonceByValidatorKey",
			v2.GetLastEventNonceByValidatorKey(validatorAddr),
			types.GetLastEventNonceByValidatorKey(v3.EthereumChainPrefix, validatorAddr),
			v2.UInt64Bytes(nonce),
		},
		{
			"KeyOrchestratorAddress",
			v2.GetOrchestratorAddressKey(accAddr),
			types.GetOrchestratorAddressKey(accAddr),
			validatorAddr.Bytes(),
		},
		{
			"ERC20ToDenomKey",
			v2.GetERC20ToDenomKey(*ethAddr),
			types.GetERC20ToDenomKey(v3.EthereumChainPrefix, *ethAddr),
			[]byte(denom),
		},
		{
			"PastEthSignatureCheckpointKey",
			v2.GetPastEthSignatureCheckpointKey([]byte(dummyCheckpoint)),
			types.GetPastEvmSignatureCheckpointKey(v3.EthereumChainPrefix, []byte(dummyCheckpoint)),
			dummyValue,
		},
		{
			"EthAddressByValidatorKey",
			v2.GetEthAddressByValidatorKey(validatorAddr),
			types.GetEthAddressByValidatorKey(validatorAddr),
			ethAddr.GetAddress().Bytes(),
		},
		{
			"DenomToERC20Key",
			v2.GetDenomToERC20Key(denom),
			types.GetDenomToERC20Key(v3.EthereumChainPrefix, denom),
			ethAddr.GetAddress().Bytes(),
		},
		{
			"OutgoingTxBatchKey",
			v2.GetOutgoingTxBatchKey(*ethAddr, dummyOutgoingTxBatch.BatchNonce),
			types.GetOutgoingTxBatchKey(v3.EthereumChainPrefix, *ethAddr, dummyOutgoingTxBatch.BatchNonce),
			marshaler.MustMarshal(&dummyOutgoingTxBatch),
		},
		{
			"ValsetRequestKey",
			v2.GetValsetKey(dummyValset.Nonce),
			types.GetValsetKey(v3.EthereumChainPrefix, dummyValset.Nonce),
			marshaler.MustMarshal(&dummyValset),
		},
		{
			"ValsetConfirmKey",
			v2.GetValsetConfirmKey(dummyValsetConfirm.Nonce, accAddr),
			types.GetValsetConfirmKey(v3.EthereumChainPrefix, dummyValsetConfirm.Nonce, accAddr),
			marshaler.MustMarshal(&dummyValsetConfirm),
		},
		{
			"OracleAttestationKey",
			v2.GetAttestationKey(nonce, hash),
			types.GetAttestationKey(v3.EthereumChainPrefix, nonce, hash),
			marshaler.MustMarshal(dummyAttestation),
		},
		{
			"OutgoingTXPoolKey",
			v2.GetOutgoingTxPoolKey(*dummyInternalOutgoingTransferTx.Erc20Fee, dummyInternalOutgoingTransferTx.Id),
			types.GetOutgoingTxPoolKey(v3.EthereumChainPrefix, *dummyInternalOutgoingTransferTx.Erc20Fee, dummyInternalOutgoingTransferTx.Id),
			bz,
		},
		{
			"BatchConfirmKey",
			v2.GetBatchConfirmKey(*tokenContract, dummyBatchConfirm.Nonce, accAddr),
			types.GetBatchConfirmKey(v3.EthereumChainPrefix, *tokenContract, dummyBatchConfirm.Nonce, accAddr),
			marshaler.MustMarshal(&dummyBatchConfirm),
		},
		{
			"KeyOutgoingLogicCall",
			v2.GetOutgoingLogicCallKey(call.InvalidationId, call.InvalidationNonce),
			types.GetOutgoingLogicCallKey(v3.EthereumChainPrefix, call.InvalidationId, call.InvalidationNonce),
			marshaler.MustMarshal(&call),
		},
		{
			"KeyOutgoingLogicConfirm",
			v2.GetLogicConfirmKey(decInvalidationId, confirm.InvalidationNonce, valAccAdd),
			types.GetLogicConfirmKey(v3.EthereumChainPrefix, decInvalidationId, confirm.InvalidationNonce, valAccAdd),
			marshaler.MustMarshal(&confirm),
		},
		{
			"PendingIbcAutoForwards",
			types.GetPendingIbcAutoForwardKey("", dummyPendingIbcAutoForward.EventNonce),
			types.GetPendingIbcAutoForwardKey(v3.EthereumChainPrefix, dummyPendingIbcAutoForward.EventNonce),
			marshaler.MustMarshal(&dummyPendingIbcAutoForward),
		},
	}

	// Create store with old keys and prepare for migration
	for _, tc := range migrateTestCases {
		store.Set(tc.oldKey, tc.value)
	}

	// Run migrations
	err = v3.MigrateStore(ctx, gravityKey, marshaler)
	require.NoError(t, err)

	// Check migration results:
	for _, tc := range migrateTestCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.value, store.Get(tc.newKey))
		})
	}
}

func TestMigrateAttestation(t *testing.T) {
	// create old prefixes KV store
	gravityKey := sdk.NewKVStoreKey("gravity")
	ctx := testutil.DefaultContext(gravityKey, sdk.NewTransientStoreKey("transient-test"))
	store := ctx.KVStore(gravityKey)
	marshaler := MakeTestMarshaler()

	nonce := uint64(1)

	msg := types.MsgBatchSendToEthClaim{
		EventNonce:     nonce,
		EthBlockHeight: 1,
		BatchNonce:     nonce,
		TokenContract:  "0x00000000000000000001",
		Orchestrator:   "0x00000000000000000004",
	}
	msgAny, _ := codectypes.NewAnyWithValue(&msg)

	_, err := msg.ClaimHash()
	require.NoError(t, err)

	dummyAttestation := &types.Attestation{
		Observed: false,
		Height:   uint64(1),
		Claim:    msgAny,
	}
	oldClaimHash, err := v2.MsgBatchSendToEthClaimHash(msg)
	require.NoError(t, err)
	newClaimHash, err := msg.ClaimHash()
	require.NoError(t, err)
	attestationOldKey := v2.GetAttestationKey(nonce, oldClaimHash)

	store.Set(attestationOldKey, marshaler.MustMarshal(dummyAttestation))

	// Run migrations
	err = v3.MigrateStore(ctx, gravityKey, marshaler)
	require.NoError(t, err)

	oldKeyEntry := store.Get(attestationOldKey)
	newKeyEntry := store.Get(types.GetAttestationKey(v3.EthereumChainPrefix, nonce, newClaimHash))
	// Check migration results:
	require.Empty(t, oldKeyEntry)
	require.NotEqual(t, oldKeyEntry, newKeyEntry)
	require.NotEqual(t, newKeyEntry, []byte(""))
	require.NotEmpty(t, newKeyEntry)
}

// Need to duplicate these because of cyclical imports
// ModuleBasics is a mock module basic manager for testing
var (
	ModuleBasics = module.NewBasicManager(
		auth.AppModuleBasic{},
		genutil.AppModuleBasic{},
		bank.AppModuleBasic{},
		capability.AppModuleBasic{},
		staking.AppModuleBasic{},
		mint.AppModuleBasic{},
		distribution.AppModuleBasic{},
		gov.NewAppModuleBasic(
			paramsclient.ProposalHandler, distrclient.ProposalHandler, upgradeclient.ProposalHandler, upgradeclient.CancelProposalHandler,
		),
		params.AppModuleBasic{},
		crisis.AppModuleBasic{},
		slashing.AppModuleBasic{},
		upgrade.AppModuleBasic{},
		evidence.AppModuleBasic{},
		vesting.AppModuleBasic{},
	)
)

// MakeTestMarshaler creates a proto codec for use in testing
func MakeTestMarshaler() codec.Codec {
	interfaceRegistry := codectypes.NewInterfaceRegistry()
	std.RegisterInterfaces(interfaceRegistry)
	ModuleBasics.RegisterInterfaces(interfaceRegistry)
	types.RegisterInterfaces(interfaceRegistry)
	return codec.NewProtoCodec(interfaceRegistry)
}
