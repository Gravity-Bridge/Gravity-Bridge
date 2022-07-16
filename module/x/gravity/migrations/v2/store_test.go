package v2_test

import (
	"bytes"
	"encoding/hex"
	"strings"
	"testing"

	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	gethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "github.com/umee-network/Gravity-Bridge/module/config"
	"github.com/umee-network/Gravity-Bridge/module/x/gravity/keeper"
	v1 "github.com/umee-network/Gravity-Bridge/module/x/gravity/migrations/v1"
	v2 "github.com/umee-network/Gravity-Bridge/module/x/gravity/migrations/v2"
	"github.com/umee-network/Gravity-Bridge/module/x/gravity/types"
)

const denom string = "graviton"
const tokenContract string = "0x2a24af0501a534fca004ee1bd667b783f205a546"
const ethAddr string = "0x2a24af0501a534fca004ee1bd667b783f205a546"

// denomToERC20Key hasn't changed
func oldGetDenomToERC20Key(denom string) string {
	return v1.DenomToERC20Key + denom
}

// ERC20ToDenomKey HAS changed. Currently: ERC20ToDenomKey + string(erc20.GetAddress().Bytes())
func oldGetERC20ToDenomKey(erc20 string) string {
	return v1.ERC20ToDenomKey + erc20
}

func oldEthAddressByValidatorKey(validator sdk.ValAddress) string {
	if err := sdk.VerifyAddressFormat(validator); err != nil {
		panic(sdkerrors.Wrap(err, "invalid validator address"))
	}
	return v1.EthAddressByValidatorKey + string(validator.Bytes())
}

func oldGetValidatorByEthAddressKey(ethAddress string) string {
	return v1.ValidatorByEthAddressKey + string([]byte(ethAddress))
}

func oldGetOutgoingTxPoolKey(fee types.InternalERC20Token, id uint64) string {
	// sdkInts have a size limit of 255 bits or 32 bytes
	// therefore this will never panic and is always safe
	amount := make([]byte, 32)
	amount = fee.Amount.BigInt().FillBytes(amount)

	amount = append(amount, v2.UInt64Bytes(id)...)
	amount = append([]byte(fee.Contract.GetAddress().Hex()), amount...)
	amount = append([]byte(v1.OutgoingTXPoolKey), amount...)
	return v1.ConvertByteArrToString(amount)
}

func oldGetOutgoingTxBatchKey(tokenContract string, nonce uint64) string {
	return v1.OutgoingTXBatchKey + tokenContract + v1.ConvertByteArrToString(v2.UInt64Bytes(nonce))
}

func TestMigrateCosmosOriginatedDenomToERC20(t *testing.T) {
	input := keeper.CreateTestEnv(t)
	defer func() { input.Context.Logger().Info("Asserting invariants at test end"); input.AssertInvariants() }()

	input.Context.KVStore(input.GravityStoreKey).Set([]byte(oldGetDenomToERC20Key(denom)), []byte(tokenContract))

	err := v2.MigrateStore(input.Context, input.GravityStoreKey, input.Marshaler)
	assert.NoError(t, err)

	addr, found := input.GravityKeeper.GetCosmosOriginatedERC20(input.Context, denom)
	assert.True(t, found)
	// Triple check that the migration worked
	assert.Equal(t, tokenContract, strings.ToLower(addr.GetAddress().Hex()))
	assert.Equal(t, gethcommon.HexToAddress(tokenContract), addr.GetAddress())
	assert.Equal(t, gethcommon.HexToAddress(tokenContract).Bytes(), addr.GetAddress().Bytes())
}

func TestMigrateCosmosOriginatedERC20ToDenom(t *testing.T) {
	input := keeper.CreateTestEnv(t)
	defer func() { input.Context.Logger().Info("Asserting invariants at test end"); input.AssertInvariants() }()

	input.Context.KVStore(input.GravityStoreKey).Set([]byte(oldGetERC20ToDenomKey(tokenContract)), []byte(denom))

	err := v2.MigrateStore(input.Context, input.GravityStoreKey, input.Marshaler)
	assert.NoError(t, err)

	tokenAddr, err := types.NewEthAddress(tokenContract)
	assert.NoError(t, err)

	storedDenom, found := input.GravityKeeper.GetCosmosOriginatedDenom(input.Context, *tokenAddr)
	assert.True(t, found)
	assert.Equal(t, denom, storedDenom)

	var erc20ToDenoms = []types.ERC20ToDenom{}
	input.GravityKeeper.IterateERC20ToDenom(input.Context, func(key []byte, erc20ToDenom *types.ERC20ToDenom) bool {
		erc20ToDenoms = append(erc20ToDenoms, *erc20ToDenom)
		return false
	})
	fromStoreTokenAddr, err := types.NewEthAddress(erc20ToDenoms[0].Erc20)
	assert.NoError(t, err)
	assert.Equal(t, denom, erc20ToDenoms[0].Denom)
	assert.Equal(t, fromStoreTokenAddr, tokenAddr)
}

func TestMigrateEthAddressByValidator(t *testing.T) {
	input := keeper.CreateTestEnv(t)
	defer func() { input.Context.Logger().Info("Asserting invariants at test end"); input.AssertInvariants() }()

	validator, err := sdk.ValAddressFromBech32("gravityvaloper1jpz0ahls2chajf78nkqczdwwuqcu97w6j77vg6")
	assert.NoError(t, err)

	input.Context.KVStore(input.GravityStoreKey).Set([]byte(oldEthAddressByValidatorKey(validator)), []byte(ethAddr))
	input.Context.KVStore(input.GravityStoreKey).Set([]byte(oldEthAddressByValidatorKey(validator)), []byte(ethAddr))

	err = v2.MigrateStore(input.Context, input.GravityStoreKey, input.Marshaler)
	assert.NoError(t, err)

	valEthAddr, found := input.GravityKeeper.GetEthAddressByValidator(input.Context, validator)
	assert.True(t, found)
	assert.Equal(t, ethAddr, strings.ToLower(valEthAddr.GetAddress().Hex()))
	assert.Equal(t, gethcommon.HexToAddress(ethAddr), valEthAddr.GetAddress())
	assert.Equal(t, gethcommon.HexToAddress(ethAddr).Bytes(), valEthAddr.GetAddress().Bytes())

}

func TestMigrateValidatorByEthAddressKey(t *testing.T) {
	input := keeper.CreateTestEnv(t)
	defer func() { input.Context.Logger().Info("Asserting invariants at test end"); input.AssertInvariants() }()

	validator, err := sdk.ValAddressFromBech32("gravityvaloper1jpz0ahls2chajf78nkqczdwwuqcu97w6j77vg6")
	assert.NoError(t, err)
	invalidEthAddr := "0x2a24af0501a534fca004ee1bd667b783f205a546"
	ethAddr := "0x2a24af0501A534fcA004eE1bD667b783F205A546"
	input.Context.KVStore(input.GravityStoreKey).
		Set([]byte(oldGetValidatorByEthAddressKey(invalidEthAddr)), []byte(validator))
	input.Context.KVStore(input.GravityStoreKey).
		Set([]byte(oldGetValidatorByEthAddressKey(ethAddr)), []byte(validator))

	err = v2.MigrateStore(input.Context, input.GravityStoreKey, input.Marshaler)
	assert.NoError(t, err)

	addr, err := types.NewEthAddress(ethAddr)
	assert.NoError(t, err)

	key := types.GetValidatorByEthAddressKey(*addr)
	res := input.Context.KVStore(input.GravityStoreKey).Get([]byte(key))
	assert.Equal(t, validator.Bytes(), res)

	assert.Nil(t, input.Context.KVStore(input.GravityStoreKey).
		Get([]byte(oldGetValidatorByEthAddressKey(invalidEthAddr))))
	assert.Nil(t, input.Context.KVStore(input.GravityStoreKey).
		Get([]byte(oldGetValidatorByEthAddressKey(ethAddr))))
}

func TestMigrateBatchConfirms(t *testing.T) {
	input := keeper.CreateTestEnv(t)
	defer func() { input.Context.Logger().Info("Asserting invariants at test end"); input.AssertInvariants() }()

	orch, err := sdk.AccAddressFromBech32("gravity1jpz0ahls2chajf78nkqczdwwuqcu97w6r48jzw")
	assert.NoError(t, err)
	ethAddr := "0x2a24af0501a534fca004ee1bd667b783f205a546"

	key := v1.BatchConfirmKey + ethAddr + v1.ConvertByteArrToString(v2.UInt64Bytes(123)) + string(orch.Bytes())

	confirm := &types.MsgConfirmBatch{
		Nonce:         123,
		TokenContract: ethAddr,
		EthSigner:     "",
		Orchestrator:  orch.String(),
		Signature:     "",
	}
	confirmBytes := input.Marshaler.MustMarshal(confirm)
	input.Context.KVStore(input.GravityStoreKey).
		Set([]byte(key), confirmBytes)

	err = v2.MigrateStore(input.Context, input.GravityStoreKey, input.Marshaler)
	assert.NoError(t, err)

	addr, err := types.NewEthAddress(ethAddr)
	assert.NoError(t, err)

	newKey := types.GetBatchConfirmKey(*addr, 123, orch)
	entity := input.Context.KVStore(input.GravityStoreKey).Get(newKey)
	assert.Equal(t, entity, confirmBytes)
}

func TestMigrateOutgoingTxs(t *testing.T) {
	input := keeper.CreateTestEnv(t)
	defer func() { input.Context.Logger().Info("Asserting invariants at test end"); input.AssertInvariants() }()

	addr, err := types.NewEthAddress("0x2a24af0501a534fca004ee1bd667b783f205a546")
	assert.NoError(t, err)

	outtx := &types.OutgoingTransferTx{
		Id:          2,
		Erc20Fee:    types.NewERC20Token(3, addr.GetAddress().Hex()),
		Sender:      "gravity1jpz0ahls2chajf78nkqczdwwuqcu97w6r48jzw",
		DestAddress: addr.GetAddress().Hex(),
		Erc20Token:  types.NewERC20Token(101, addr.GetAddress().Hex()),
	}

	internalTx, err := outtx.GetErc20Fee().ToInternal()
	assert.NoError(t, err)

	oldKey := oldGetOutgoingTxPoolKey(*internalTx, outtx.Id)

	inputBytes := input.Marshaler.MustMarshal(outtx)
	input.Context.KVStore(input.GravityStoreKey).Set([]byte(oldKey), inputBytes)

	err = v2.MigrateStore(input.Context, input.GravityStoreKey, input.Marshaler)
	assert.NoError(t, err)

	key := types.GetOutgoingTxPoolKey(*internalTx, outtx.Id)
	res := input.Context.KVStore(input.GravityStoreKey).Get([]byte(key))
	assert.Equal(t, inputBytes, res)
}

func TestMigrateOutgoingTxBatches(t *testing.T) {
	input := keeper.CreateTestEnv(t)
	defer func() { input.Context.Logger().Info("Asserting invariants at test end"); input.AssertInvariants() }()

	addr, err := types.NewEthAddress("0x2a24af0501a534fca004ee1bd667b783f205a546")
	assert.NoError(t, err)

	batch := types.OutgoingTxBatch{
		BatchNonce:    1,
		BatchTimeout:  0,
		Transactions:  []types.OutgoingTransferTx{},
		TokenContract: addr.GetAddress().Hex(),
		Block:         123,
	}
	assert.NoError(t, err)

	oldKey := oldGetOutgoingTxBatchKey(addr.GetAddress().Hex(), batch.BatchNonce)

	inputBytes := input.Marshaler.MustMarshal(&batch)
	input.Context.KVStore(input.GravityStoreKey).Set([]byte(oldKey), inputBytes)

	err = v2.MigrateStore(input.Context, input.GravityStoreKey, input.Marshaler)
	assert.NoError(t, err)

	key := types.GetOutgoingTxBatchKey(*addr, batch.BatchNonce)
	res := input.Context.KVStore(input.GravityStoreKey).Get([]byte(key))
	assert.Equal(t, inputBytes, res)
}

func TestMigrateStoreForUnusedKeys(t *testing.T) {

	// create old format prefixes KV store
	gravityKey := sdk.NewKVStoreKey("gravity")
	ctx := testutil.DefaultContext(gravityKey, sdk.NewTransientStoreKey("transient-test"))
	store := ctx.KVStore(gravityKey)

	marshaler := keeper.MakeTestMarshaler()
	dummyValue := []byte("dummy")

	// test cases for unused keys consisting of dummy values, since we should only confirm they will be deleted.
	unusedKeysForDeletion := []struct {
		name         string
		oldPrefixKey string
		value        []byte
	}{
		{
			"OracleClaimKey",
			v1.OracleClaimKey,
			dummyValue,
		},
		{
			"DenomiatorPrefix",
			v1.DenomiatorPrefix,
			dummyValue,
		},
		{
			"SecondIndexNonceByClaimKey",
			v1.SecondIndexNonceByClaimKey,
			dummyValue,
		},
	}

	// Create store with old format prefix keys and prepare for migration
	for _, tc := range unusedKeysForDeletion {
		store.Set([]byte(tc.oldPrefixKey), tc.value)
	}

	// Run migrations
	err := v2.MigrateStore(ctx, gravityKey, marshaler)
	require.NoError(t, err)

	// checks results of migration: nothing in store with old key prefix format - new prefix key format is not defined!
	for _, tc := range unusedKeysForDeletion {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			oldStore := store.Get([]byte(tc.oldPrefixKey))
			require.Equal(t, len(oldStore), 0)
		})
	}
}

// Migrations Store tests consist of the following steps::
// 1. creating store with old format key prefixes
// 2. migration of store
// 3. check migration results: make sure the new keys are set in store and old values can be obtained
func TestMigrateStoreKeys(t *testing.T) {

	// create old format prefixes KV store
	gravityKey := sdk.NewKVStoreKey("gravity")
	ctx := testutil.DefaultContext(gravityKey, sdk.NewTransientStoreKey("transient-test"))
	store := ctx.KVStore(gravityKey)

	marshaler := keeper.MakeTestMarshaler()
	dummyValue := []byte("dummy")

	migrateKeyTestCases := []struct {
		name         string
		oldPrefixKey string
		newPrefixKey []byte
		value        []byte
	}{
		{
			"LastObservedEventNonceKey",
			v1.LastObservedEventNonceKey,
			v2.LastObservedEventNonceKey,
			dummyValue,
		},
		{
			"KeyLastTXPoolID",
			v1.KeyLastTXPoolID,
			v2.KeyLastTXPoolID,
			dummyValue,
		},
		{
			"KeyLastOutgoingBatchID",
			v1.KeyLastOutgoingBatchID,
			v2.KeyLastOutgoingBatchID,
			dummyValue,
		},
		{
			"LastObservedEthereumBlockHeightKey",
			v1.LastObservedEthereumBlockHeightKey,
			v2.LastObservedEthereumBlockHeightKey,
			dummyValue,
		},
		{
			"LastSlashedValsetNonce",
			v1.LastSlashedValsetNonce,
			v2.LastSlashedValsetNonce,
			dummyValue,
		},
		{
			"LatestValsetNonce",
			v1.LatestValsetNonce,
			v2.LatestValsetNonce,
			dummyValue,
		},
		{
			"LastSlashedBatchBlock",
			v1.LastSlashedBatchBlock,
			v2.LastSlashedBatchBlock,
			dummyValue,
		},
		{
			"LastSlashedLogicCallBlock",
			v1.LastSlashedLogicCallBlock,
			v2.LastSlashedLogicCallBlock,
			dummyValue,
		},
		{
			"LastUnBondingBlockHeight",
			v1.LastUnBondingBlockHeight,
			v2.LastUnBondingBlockHeight,
			dummyValue,
		},
		{
			"LastObservedValsetKey",
			v1.LastObservedValsetKey,
			v2.LastObservedValsetKey,
			dummyValue,
		},
	}

	// Create store with old format prefix keys and prepare for migration
	for _, tc := range migrateKeyTestCases {
		store.Set([]byte(tc.oldPrefixKey), tc.value)
	}

	// Run migrations
	err := v2.MigrateStore(ctx, gravityKey, marshaler)
	require.NoError(t, err)

	// checks results of migration
	for _, tc := range migrateKeyTestCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.value, store.Get(tc.newPrefixKey))
		})
	}
}

func TestMigrateStoreKeysFromKeys(t *testing.T) {
	// create old prefixes KV store
	gravityKey := sdk.NewKVStoreKey("gravity")
	ctx := testutil.DefaultContext(gravityKey, sdk.NewTransientStoreKey("transient-test"))
	store := ctx.KVStore(gravityKey)

	marshaler := keeper.MakeTestMarshaler()
	dummyValue := []byte("dummy")

	nonce := uint64(1234)

	accAddr, _ := sdk.AccAddressFromBech32("gravity1ahx7f8wyertuus9r20284ej0asrs085ceqtfnm")
	valAddr, _ := sdk.ValAddressFromBech32("gravityvaloper1jpz0ahls2chajf78nkqczdwwuqcu97w6j77vg6")
	ethAddrStr := "0xdD80fdC958aD08C02105AF7a6A6642BC489E22F7"
	ethAddr, _ := types.NewEthAddress(ethAddrStr)

	dummyCheckpoint := "0x1de95c9ace999f8ec70c6dc8d045942da2612950567c4861aca959c0650194da"

	// test cases for keys consisting of prefix key and value
	// OLD key: string prefix key + some value (address, denom name)
	migrateKeysFromKeysTestCases := []struct {
		name         string
		oldPrefixKey string
		newPrefixKey []byte
		value        []byte
	}{
		{
			"ValidatorByEthAddressKey",
			v1.GetValidatorByEthAddressKey(*ethAddr),
			v2.GetValidatorByEthAddressKey(*ethAddr),
			valAddr.Bytes(),
		},
		{
			"LastEventNonceByValidatorKey",
			v1.GetLastEventNonceByValidatorKey(valAddr),
			v2.GetLastEventNonceByValidatorKey(valAddr),
			v2.UInt64Bytes(nonce),
		},
		{
			"KeyOrchestratorAddress",
			v1.GetOrchestratorAddressKey(accAddr),
			v2.GetOrchestratorAddressKey(accAddr),
			valAddr.Bytes(),
		},
		{
			"ERC20ToDenomKey",
			v1.GetERC20ToDenomKey(*ethAddr),
			v2.GetERC20ToDenomKey(*ethAddr),
			[]byte(denom),
		},
		{
			"PastEthSignatureCheckpointKey",
			v1.GetPastEthSignatureCheckpointKey([]byte(dummyCheckpoint)),
			v2.GetPastEthSignatureCheckpointKey([]byte(dummyCheckpoint)),
			dummyValue,
		},
	}

	// test cases where value in store contains ethereum address
	migrateKeysFromKeysEthValueTestCases := []struct {
		name         string
		oldPrefixKey string
		newPrefixKey []byte
		value        []byte
	}{
		{
			"EthAddressByValidatorKey",
			v1.GetEthAddressByValidatorKey(valAddr),
			v2.GetEthAddressByValidatorKey(valAddr),
			[]byte(ethAddrStr),
		},
		{
			"DenomToERC20Key",
			v1.GetDenomToERC20Key(denom),
			v2.GetDenomToERC20Key(denom),
			[]byte(ethAddrStr),
		},
	}

	// Create store with old prefix keys and prepare for migration
	for _, tc := range migrateKeysFromKeysTestCases {
		store.Set([]byte(tc.oldPrefixKey), tc.value)
	}
	for _, tc := range migrateKeysFromKeysEthValueTestCases {
		store.Set([]byte(tc.oldPrefixKey), tc.value)
	}

	// Run migrations
	err := v2.MigrateStore(ctx, gravityKey, marshaler)
	require.NoError(t, err)

	// checks results of migration
	for _, tc := range migrateKeysFromKeysTestCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.value, store.Get(tc.newPrefixKey))
		})
	}

	for _, tc := range migrateKeysFromKeysEthValueTestCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, gethcommon.HexToAddress(ethAddrStr).Bytes(), store.Get(tc.newPrefixKey))
		})
	}
}

func TestMigrateStoreKeysFromValues(t *testing.T) {

	// create old prefixes KV store
	gravityKey := sdk.NewKVStoreKey("gravity")
	ctx := testutil.DefaultContext(gravityKey, sdk.NewTransientStoreKey("transient-test"))
	store := ctx.KVStore(gravityKey)

	marshaler := keeper.MakeTestMarshaler()

	ethAddr, _ := types.NewEthAddress("0x2a24af0501a534fca004ee1bd667b783f205a546")
	tokenContract, _ := types.NewEthAddress("0x429881672B9AE42b8EbA0E26cD9C73711b891Ca5")
	accAddr, _ := sdk.AccAddressFromBech32("gravity1ahx7f8wyertuus9r20284ej0asrs085ceqtfnm")

	dummyOutgoingTxBatch := types.OutgoingTxBatch{
		BatchNonce:    1,
		BatchTimeout:  0,
		Transactions:  []types.OutgoingTransferTx{},
		TokenContract: ethAddr.GetAddress().String(),
		Block:         123,
	}
	dummyCornerCaseBatch := types.OutgoingTxBatch{
		BatchNonce:    128,
		BatchTimeout:  0,
		Transactions:  []types.OutgoingTransferTx{},
		TokenContract: ethAddr.GetAddress().String(),
		Block:         123,
	}

	dummyValset := types.Valset{
		Nonce:        1,
		Members:      []types.BridgeValidator{},
		Height:       128,
		RewardAmount: sdk.NewInt(1),
		RewardToken:  "footoken",
	}

	dummyValsetConfirm := types.MsgValsetConfirm{
		Nonce:        1,
		Orchestrator: accAddr.String(),
		EthAddress:   ethAddr.GetAddress().String(),
		Signature:    "dummySignature",
	}

	dummyBatchConfirm := types.MsgConfirmBatch{
		Nonce:         1,
		TokenContract: tokenContract.GetAddress().String(),
		EthSigner:     ethAddr.GetAddress().String(),
		Orchestrator:  accAddr.String(),
		Signature:     "dummySignature",
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
		BlockHeight:    1,
		TokenContract:  "0x00000000000000000001",
		Amount:         sdk.NewInt(10000000000 + int64(1)),
		EthereumSender: "0x00000000000000000002",
		CosmosReceiver: "0x00000000000000000003",
		Orchestrator:   "0x00000000000000000004",
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
	}
	decInvalidationId, err := hex.DecodeString(confirm.InvalidationId)
	require.NoError(t, err)

	// creating test cases
	// OLD key: string prefix key + some value (address, nonce....)
	// NEW KEYS are generated during migration from values.
	migrateKeysFromValuesTestCases := []struct {
		name         string
		oldPrefixKey string
		newPrefixKey []byte
		value        []byte
	}{
		{
			"OutgoingTXBatchKey",
			v1.GetOutgoingTxBatchKey(*ethAddr, dummyOutgoingTxBatch.BatchNonce),
			v2.GetOutgoingTxBatchKey(*ethAddr, dummyOutgoingTxBatch.BatchNonce),
			marshaler.MustMarshal(&dummyOutgoingTxBatch),
		},
		{
			"OutgoingTXBatchKey - Bytes Corner case",
			v1.GetOutgoingTxBatchKey(*ethAddr, dummyCornerCaseBatch.BatchNonce),
			v2.GetOutgoingTxBatchKey(*ethAddr, dummyCornerCaseBatch.BatchNonce),
			marshaler.MustMarshal(&dummyCornerCaseBatch),
		},
		{
			"ValsetRequestKey",
			v1.GetValsetKey(dummyValset.Nonce),
			v2.GetValsetKey(dummyValset.Nonce),
			marshaler.MustMarshal(&dummyValset),
		},
		{
			"ValsetConfirmKey",
			v1.GetValsetConfirmKey(dummyValsetConfirm.Nonce, accAddr),
			v2.GetValsetConfirmKey(dummyValsetConfirm.Nonce, accAddr),
			marshaler.MustMarshal(&dummyValsetConfirm),
		},
		{
			"OracleAttestationKey",
			v1.GetAttestationKey(nonce, hash),
			v2.GetAttestationKey(nonce, hash),
			marshaler.MustMarshal(dummyAttestation),
		},
		{
			"OutgoingTXPoolKey",
			v1.GetOutgoingTxPoolKey(*dummyInternalOutgoingTransferTx.Erc20Fee, dummyInternalOutgoingTransferTx.Id),
			v2.GetOutgoingTxPoolKey(*dummyInternalOutgoingTransferTx.Erc20Fee, dummyInternalOutgoingTransferTx.Id),
			bz,
		},
		{
			"BatchConfirmKey",
			v1.GetBatchConfirmKey(*tokenContract, dummyBatchConfirm.Nonce, accAddr),
			v2.GetBatchConfirmKey(*tokenContract, dummyBatchConfirm.Nonce, accAddr),
			marshaler.MustMarshal(&dummyBatchConfirm),
		},
		{
			"KeyOutgoingLogicCall",
			v1.GetOutgoingLogicCallKey(call.InvalidationId, call.InvalidationNonce),
			v2.GetOutgoingLogicCallKey(call.InvalidationId, call.InvalidationNonce),
			marshaler.MustMarshal(&call),
		},
		{
			"KeyOutgoingLogicConfirm",
			v1.GetLogicConfirmKey(decInvalidationId, confirm.InvalidationNonce, valAccAdd),
			v2.GetLogicConfirmKey(decInvalidationId, confirm.InvalidationNonce, valAccAdd),
			marshaler.MustMarshal(&confirm),
		},
	}

	// Create store with old prefix keys and prepare for migration
	for _, tc := range migrateKeysFromValuesTestCases {
		store.Set([]byte(tc.oldPrefixKey), tc.value)
	}

	// Run migrations
	err = v2.MigrateStore(ctx, gravityKey, marshaler)
	require.NoError(t, err)

	// Check migration results:
	for _, tc := range migrateKeysFromValuesTestCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.value, store.Get(tc.newPrefixKey))
		})
	}
}

func TestMigrateInvalidStore(t *testing.T) {

	// create old prefixes KV store
	gravityKey := sdk.NewKVStoreKey("gravity")

	marshaler := keeper.MakeTestMarshaler()

	ethAddr, _ := types.NewEthAddress("0x2a24af0501a534fca004ee1bd667b783f205a546")
	// 43 chars length
	invalidEthAddress := "0x2a24af0501a534fca004ee1bd667b783f205a5465"
	// invalid instead of gravity prefix
	invalidAccAddress := "invalid1ahx7f8wyertuus9r20284ej0asrs085ceqtfnm"

	dummyOutgoingTxBatch := types.OutgoingTxBatch{
		BatchNonce:    1,
		BatchTimeout:  0,
		Transactions:  []types.OutgoingTransferTx{},
		TokenContract: invalidEthAddress,
		Block:         123,
	}
	dummyValsetConfirm := types.MsgValsetConfirm{
		Nonce:        1,
		Orchestrator: invalidAccAddress,
		EthAddress:   ethAddr.GetAddress().String(),
		Signature:    "dummySignature",
	}

	// creating test cases
	migrateInvalidStoreTestCases := []struct {
		name         string
		oldPrefixKey string
		value        []byte
	}{
		{
			"OutgoingTXBatchKey - Invalid Ethereum address",
			v1.OutgoingTXBatchKey + invalidEthAddress + v1.ConvertByteArrToString(v2.UInt64Bytes(dummyOutgoingTxBatch.BatchNonce)),
			marshaler.MustMarshal(&dummyOutgoingTxBatch),
		},
		{
			"ValsetConfirmKey - Invalid Orchestrator address",
			v1.ValsetConfirmKey + v1.ConvertByteArrToString(v2.UInt64Bytes(dummyValsetConfirm.Nonce)) + invalidAccAddress,
			marshaler.MustMarshal(&dummyValsetConfirm),
		},
	}

	// Create store with old prefix key for each test case and try to migrate it
	// migration will fails
	for _, tc := range migrateInvalidStoreTestCases {
		ctx := testutil.DefaultContext(gravityKey, sdk.NewTransientStoreKey("transient-test"))
		store := ctx.KVStore(gravityKey)
		store.Set([]byte(tc.oldPrefixKey), tc.value)

		err := v2.MigrateStore(ctx, gravityKey, marshaler)

		t.Run(tc.name, func(t *testing.T) {
			require.Error(t, err)
		})
	}
}
