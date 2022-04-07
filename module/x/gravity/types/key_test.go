package types

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
)

func TestPrefixKeysSameLength(t *testing.T) {
	allKeys := getAllKeys()
	prefixKeys := allKeys[0:27]
	length := len(HashString("All keys should be same length when hashed"))

	for _, key := range prefixKeys {
		require.Equal(t, length, len(key))
	}
}

func TestNoDuplicateKeys(t *testing.T) {
	keys := getAllKeys()

	for i, key := range keys {
		keys[i] = nil
		require.NotContains(t, keys, key)
	}
}

func getAllKeys() [][]byte {
	i := 0
	inc := func(i *int) *int { *i += 1; return i }

	keys := make([][]byte, 47)

	keys[i] = EthAddressByValidatorKey
	keys[*inc(&i)] = ValidatorByEthAddressKey
	keys[*inc(&i)] = ValsetRequestKey
	keys[*inc(&i)] = ValsetConfirmKey
	keys[*inc(&i)] = OracleAttestationKey
	keys[*inc(&i)] = OutgoingTXPoolKey
	keys[*inc(&i)] = OutgoingTXBatchKey
	keys[*inc(&i)] = BatchConfirmKey
	keys[*inc(&i)] = LastEventNonceByValidatorKey
	keys[*inc(&i)] = LastObservedEventNonceKey
	keys[*inc(&i)] = SequenceKeyPrefix
	keys[*inc(&i)] = KeyLastTXPoolID
	keys[*inc(&i)] = KeyLastOutgoingBatchID
	keys[*inc(&i)] = KeyOrchestratorAddress
	keys[*inc(&i)] = KeyOutgoingLogicCall
	keys[*inc(&i)] = KeyOutgoingLogicConfirm
	keys[*inc(&i)] = LastObservedEvmBlockHeightKey
	keys[*inc(&i)] = DenomToERC20Key
	keys[*inc(&i)] = ERC20ToDenomKey
	keys[*inc(&i)] = LastSlashedValsetNonce
	keys[*inc(&i)] = LatestValsetNonce
	keys[*inc(&i)] = LastSlashedBatchBlock
	keys[*inc(&i)] = LastSlashedLogicCallBlock
	keys[*inc(&i)] = LastUnBondingBlockHeight
	keys[*inc(&i)] = LastObservedValsetKey
	keys[*inc(&i)] = PastEvmSignatureCheckpointKey

	// ChainPrefix
	dummyPrefix := "xyzchain"
	// sdk.AccAddress, sdk.ValAddress
	dummyAddr := []byte("gravity1ahx7f8wyertuus9r20284ej0asrs085ceqtfnm")
	// EthAddress
	ethAddr, _ := NewEthAddress("0xAb5801a7D398351b8bE11C439e05C5B3259aeC9B")
	dummyEthAddr := *ethAddr
	// Nonce
	dummyNonce := uint64(1)
	// Claim, InvalidationId
	dummyBytes := []byte("0xc783df8a850f42e7F7e57013759C285caa701eB6")
	// InternationalERC20Token
	dummyErc := InternalERC20Token{Amount: sdk.OneInt(), Contract: dummyEthAddr}
	// Denom
	dummyDenom := "footoken"

	keys[*inc(&i)] = GetOrchestratorAddressKey(dummyAddr)
	keys[*inc(&i)] = GetEthAddressByValidatorKey(dummyAddr)
	keys[*inc(&i)] = GetValidatorByEthAddressKey(dummyEthAddr)
	keys[*inc(&i)] = GetValsetKey(dummyPrefix, dummyNonce)
	keys[*inc(&i)] = GetValsetConfirmNoncePrefix(dummyPrefix, dummyNonce)
	keys[*inc(&i)] = GetValsetConfirmKey(dummyPrefix, dummyNonce, dummyAddr)
	keys[*inc(&i)] = GetAttestationKey(dummyPrefix, dummyNonce, dummyBytes)
	keys[*inc(&i)] = GetOutgoingTxPoolContractPrefix(dummyPrefix, dummyEthAddr)
	keys[*inc(&i)] = GetOutgoingTxPoolKey(dummyPrefix, dummyErc, dummyNonce)
	keys[*inc(&i)] = GetOutgoingTxBatchContractPrefix(dummyPrefix, dummyEthAddr)
	keys[*inc(&i)] = GetOutgoingTxBatchKey(dummyPrefix, dummyEthAddr, dummyNonce)
	keys[*inc(&i)] = GetBatchConfirmNonceContractPrefix(dummyPrefix, dummyEthAddr, dummyNonce)
	keys[*inc(&i)] = GetBatchConfirmKey(dummyPrefix, dummyEthAddr, dummyNonce, dummyAddr)
	keys[*inc(&i)] = GetLastEventNonceByValidatorKey(dummyPrefix, dummyAddr)
	keys[*inc(&i)] = GetDenomToERC20Key(dummyDenom)
	keys[*inc(&i)] = GetERC20ToDenomKey(dummyEthAddr)
	keys[*inc(&i)] = GetOutgoingLogicCallKey(dummyPrefix, dummyBytes, dummyNonce)
	keys[*inc(&i)] = GetLogicConfirmNonceInvalidationIdPrefix(dummyPrefix, dummyBytes, dummyNonce)
	keys[*inc(&i)] = GetLogicConfirmKey(dummyPrefix, dummyBytes, dummyNonce, dummyAddr)
	keys[*inc(&i)] = GetPastEthSignatureCheckpointKey(dummyBytes)

	return keys
}
