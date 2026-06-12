package types

import (
	"testing"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
)

// nolint: exhaustruct
func TestGenesisStateValidate(t *testing.T) {
	specs := map[string]struct {
		src    *GenesisState
		expErr bool
	}{
		"default params": {src: DefaultGenesisState(), expErr: false},
		"empty params": {src: &GenesisState{
			Params: &Params{
				GravityId:                    "",
				ContractSourceHash:           "",
				BridgeEthereumAddress:        "",
				BridgeChainId:                0,
				SignedValsetsWindow:          0,
				SignedBatchesWindow:          0,
				SignedLogicCallsWindow:       0,
				TargetBatchTimeout:           0,
				AverageBlockTime:             0,
				AverageEthereumBlockTime:     0,
				SlashFractionValset:          sdkmath.LegacyDec{},
				SlashFractionBatch:           sdkmath.LegacyDec{},
				SlashFractionLogicCall:       sdkmath.LegacyDec{},
				UnbondSlashingValsetsWindow:  0,
				SlashFractionBadEthSignature: sdkmath.LegacyDec{},
				ValsetReward: sdk.Coin{
					Denom:  "",
					Amount: sdkmath.Int{},
				},
			},
			GravityNonces:      GravityNonces{},
			Valsets:            []Valset{},
			ValsetConfirms:     []MsgValsetConfirm{},
			Batches:            []OutgoingTxBatch{},
			BatchConfirms:      []MsgConfirmBatch{},
			LogicCalls:         []OutgoingLogicCall{},
			LogicCallConfirms:  []MsgConfirmLogicCall{},
			Attestations:       []Attestation{},
			DelegateKeys:       []MsgSetOrchestratorAddress{},
			Erc20ToDenoms:      []ERC20ToDenom{},
			UnbatchedTransfers: []OutgoingTransferTx{},
		}, expErr: true},
		"invalid params": {src: &GenesisState{
			Params: &Params{
				GravityId:                    "foo",
				ContractSourceHash:           "laksdjflasdkfja",
				BridgeEthereumAddress:        "invalid-eth-address",
				BridgeChainId:                3279089,
				SignedValsetsWindow:          0,
				SignedBatchesWindow:          0,
				SignedLogicCallsWindow:       0,
				TargetBatchTimeout:           0,
				AverageBlockTime:             0,
				AverageEthereumBlockTime:     0,
				SlashFractionValset:          sdkmath.LegacyDec{},
				SlashFractionBatch:           sdkmath.LegacyDec{},
				SlashFractionLogicCall:       sdkmath.LegacyDec{},
				UnbondSlashingValsetsWindow:  0,
				SlashFractionBadEthSignature: sdkmath.LegacyDec{},
				ValsetReward: sdk.Coin{
					Denom:  "",
					Amount: sdkmath.Int{},
				},
			},
			GravityNonces:      GravityNonces{},
			Valsets:            []Valset{},
			ValsetConfirms:     []MsgValsetConfirm{},
			Batches:            []OutgoingTxBatch{},
			BatchConfirms:      []MsgConfirmBatch{},
			LogicCalls:         []OutgoingLogicCall{},
			LogicCallConfirms:  []MsgConfirmLogicCall{},
			Attestations:       []Attestation{},
			DelegateKeys:       []MsgSetOrchestratorAddress{},
			Erc20ToDenoms:      []ERC20ToDenom{},
			UnbatchedTransfers: []OutgoingTransferTx{},
		}, expErr: true},
	}
	for msg, spec := range specs {
		t.Run(msg, func(t *testing.T) {
			err := spec.src.ValidateBasic()
			if spec.expErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestStringToByteArray(t *testing.T) {
	specs := map[string]struct {
		testString string
		expErr     bool
	}{
		"16 bytes": {"lakjsdflaksdjfds", false},
		"32 bytes": {"lakjsdflaksdjfdslakjsdflaksdjfds", false},
		"33 bytes": {"€€€€€€€€€€€", true},
	}

	for msg, spec := range specs {
		t.Run(msg, func(t *testing.T) {
			_, err := strToFixByteArray(spec.testString)
			if spec.expErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestValidateCosmosBridgeableTokens(t *testing.T) {
	specs := map[string]struct {
		denoms []string
		expErr bool
	}{
		"empty list is valid":              {denoms: []string{}, expErr: false},
		"nil list is valid":               {denoms: nil, expErr: false},
		"single valid denom":              {denoms: []string{"uatom"}, expErr: false},
		"multiple valid denoms":           {denoms: []string{"uatom", "uosmo", "stake"}, expErr: false},
		"ibc denom is valid":              {denoms: []string{"ibc/27394FB092D2ECCD56123C74F36E4C1F926001CEADA9CA97EA622B25F41E5EB2"}, expErr: false},
		"duplicate denom rejected":        {denoms: []string{"uatom", "uatom"}, expErr: true},
		"gravity-prefixed denom rejected": {denoms: []string{"gravity0x429881672B9AE42b8EbA0E26cD9C73711b891Ca5"}, expErr: true},
		"invalid denom rejected":          {denoms: []string{"INVALID DENOM WITH SPACES"}, expErr: true},
	}

	for msg, spec := range specs {
		spec := spec
		t.Run(msg, func(t *testing.T) {
			err := validateCosmosBridgeableTokens(spec.denoms)
			if spec.expErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestParamsValidateBasicCosmosBridgeableTokens(t *testing.T) {
	base := DefaultParams()

	// valid: non-empty allowlist
	p := *base
	p.CosmosBridgeableTokens = []string{"uatom", "uosmo"}
	require.NoError(t, p.ValidateBasic())

	// invalid: duplicate entry
	p2 := *base
	p2.CosmosBridgeableTokens = []string{"uatom", "uatom"}
	require.Error(t, p2.ValidateBasic())

	// invalid: gravity-prefixed entry
	p3 := *base
	p3.CosmosBridgeableTokens = []string{"gravity0x429881672B9AE42b8EbA0E26cD9C73711b891Ca5"}
	require.Error(t, p3.ValidateBasic())
}

