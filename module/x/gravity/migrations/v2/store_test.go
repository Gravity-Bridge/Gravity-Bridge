package v2_test

import (
	"strings"
	"testing"

	_ "github.com/Gravity-Bridge/Gravity-Bridge/module/config"
	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/keeper"
	v2 "github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/migrations/v2"
	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	gethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
)

// denomToERC20Key hasn't changed
func oldGetDenomToERC20Key(denom string) string {
	return v2.DenomToERC20Key + denom
}

// ERC20ToDenomKey HAS changed. Currently: ERC20ToDenomKey + string(erc20.GetAddress().Bytes())
func oldGetERC20ToDenomKey(erc20 string) string {
	return v2.ERC20ToDenomKey + erc20
}

func oldEthAddressByValidatorKey(validator sdk.ValAddress) string {
	if err := sdk.VerifyAddressFormat(validator); err != nil {
		panic(sdkerrors.Wrap(err, "invalid validator address"))
	}
	return v2.EthAddressByValidatorKey + string(validator.Bytes())
}

func oldGetValidatorByEthAddressKey(ethAddress string) string {
	return v2.ValidatorByEthAddressKey + string([]byte(ethAddress))
}

func oldGetOutgoingTxPoolKey(fee types.InternalERC20Token, id uint64) string {
	// sdkInts have a size limit of 255 bits or 32 bytes
	// therefore this will never panic and is always safe
	amount := make([]byte, 32)
	amount = fee.Amount.BigInt().FillBytes(amount)

	a := append(amount, types.UInt64Bytes(id)...)
	b := append([]byte(fee.Contract.GetAddress().Hex()), a...)
	r := append([]byte(v2.OutgoingTXPoolKey), b...)
	return types.ConvertByteArrToString(r)
}

func oldGetOutgoingTxBatchKey(tokenContract string, nonce uint64) string {
	return v2.OutgoingTXBatchKey + tokenContract + ConvertByteArrToString(types.UInt64Bytes(nonce))
}

func TestMigrateCosmosOriginatedDenomToERC20(t *testing.T) {
	input := keeper.CreateTestEnv(t)
	defer func() { input.Context.Logger().Info("Asserting invariants at test end"); input.AssertInvariants() }()

	denom := "graviton"
	tokenContract := "0x2a24af0501a534fca004ee1bd667b783f205a546"
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

	denom := "graviton"
	tokenContract := "0x2a24af0501a534fca004ee1bd667b783f205a546"
	input.Context.KVStore(input.GravityStoreKey).Set([]byte(oldGetERC20ToDenomKey(tokenContract)), []byte(denom))

	err := v2.MigrateStore(input.Context, input.GravityStoreKey, input.Marshaler)
	assert.NoError(t, err)

	tokenAddr, err := types.NewEthAddress(tokenContract)
	assert.NoError(t, err)

	storedDenom, found := input.GravityKeeper.GetCosmosOriginatedDenom(input.Context, *tokenAddr)
	assert.True(t, found)
	assert.Equal(t, denom, storedDenom)

}

func TestMigrateEthAddressByValidator(t *testing.T) {
	input := keeper.CreateTestEnv(t)
	defer func() { input.Context.Logger().Info("Asserting invariants at test end"); input.AssertInvariants() }()

	validator, err := sdk.ValAddressFromBech32("gravityvaloper1jpz0ahls2chajf78nkqczdwwuqcu97w6j77vg6")
	assert.NoError(t, err)
	ethAddr := "0x2a24af0501a534fca004ee1bd667b783f205a546"
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

	key := v2.BatchConfirmKey + ethAddr + types.ConvertByteArrToString(types.UInt64Bytes(123)) + string(orch.Bytes())

	confirm := &types.MsgConfirmBatch{
		Nonce:         123,
		TokenContract: ethAddr,
		EthSigner:     "",
		Orchestrator:  "",
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
	entity := input.Context.KVStore(input.GravityStoreKey).Get([]byte(newKey))
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

func ConvertByteArrToString(value []byte) string {
	var ret strings.Builder
	for i := 0; i < len(value); i++ {
		ret.WriteString(string(value[i]))
	}
	return ret.String()
}
