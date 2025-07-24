package app

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoregistry"
	"pgregory.net/rapid"

	"github.com/cosmos/cosmos-proto/rapidproto"

	authapi "cosmossdk.io/api/cosmos/auth/v1beta1"
	v1beta1 "cosmossdk.io/api/cosmos/base/v1beta1"
	msgv1 "cosmossdk.io/api/cosmos/msg/v1"
	txv1beta1 "cosmossdk.io/api/cosmos/tx/v1beta1"
	"cosmossdk.io/x/tx/signing/aminojson"
	signing_testutil "cosmossdk.io/x/tx/signing/testutil"
	"github.com/cosmos/cosmos-sdk/testutil/testdata"
	"github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/bech32"
	signingtypes "github.com/cosmos/cosmos-sdk/types/tx/signing"
	"github.com/cosmos/cosmos-sdk/x/auth/migrations/legacytx"
	"github.com/cosmos/cosmos-sdk/x/auth/signing"
	"github.com/cosmos/cosmos-sdk/x/auth/tx"

	auctionapi "github.com/Gravity-Bridge/Gravity-Bridge/module/api/auction/v1"
	gravityv1api "github.com/Gravity-Bridge/Gravity-Bridge/module/api/gravity/v1"
	auctiontypes "github.com/Gravity-Bridge/Gravity-Bridge/module/x/auction/types"
	gravityv1types "github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
)

// TestAminoJSON_Equivalence tests that x/tx/Encoder encoding is equivalent to the legacy Encoder encoding.
// A custom generator is used to generate random messages that are then encoded using both encoders.  The custom
// generator only supports proto.Message (which implement the protoreflect API) so in order to test legacy gogo types
// we end up with a workflow as follows:
//
// 1. Generate a random protobuf proto.Message using the custom generator
// 2. Marshal the proto.Message to protobuf binary bytes
// 3. Unmarshal the protobuf bytes to a gogoproto.Message
// 4. Marshal the gogoproto.Message to amino JSON bytes
// 5. Marshal the proto.Message to amino JSON bytes
// 6. Compare the amino JSON bytes from steps 4 and 5
//
// In order for step 3 to work certain restrictions on the data generated in step 1 must be enforced and are described
// by the mutation of genOpts passed to the generator.
// nolint: exhaustruct
func TestAminoJSON_Equivalence(t *testing.T) {
	encCfg := NewEncodingConfig()
	legacytx.RegressionTestingAminoCodec = encCfg.Amino
	aj := aminojson.NewEncoder(aminojson.EncoderOptions{DoNotSortFields: true})

	GenOpts := rapidproto.GeneratorOptions{
		Resolver:  protoregistry.GlobalTypes,
		FieldMaps: []rapidproto.FieldMapper{GeneratorFieldMapper},
	}

	testedMsgs := []GeneratedType{
		// Gravity
		GenType(&gravityv1types.MsgSetOrchestratorAddress{}, &gravityv1api.MsgSetOrchestratorAddress{}, GenOpts.WithDisallowNil()),
		GenType(&gravityv1types.MsgBatchSendToEthClaim{}, &gravityv1api.MsgBatchSendToEthClaim{}, GenOpts.WithDisallowNil()),
		GenType(&gravityv1types.MsgCancelSendToEth{}, &gravityv1api.MsgCancelSendToEth{}, GenOpts.WithDisallowNil()),
		GenType(&gravityv1types.MsgConfirmBatch{}, &gravityv1api.MsgConfirmBatch{}, GenOpts.WithDisallowNil()),
		GenType(&gravityv1types.MsgConfirmLogicCall{}, &gravityv1api.MsgConfirmLogicCall{}, GenOpts.WithDisallowNil()),
		GenType(&gravityv1types.MsgERC20DeployedClaim{}, &gravityv1api.MsgERC20DeployedClaim{}, GenOpts.WithDisallowNil()),
		GenType(&gravityv1types.MsgExecuteIbcAutoForwards{}, &gravityv1api.MsgExecuteIbcAutoForwards{}, GenOpts.WithDisallowNil()),
		GenType(&gravityv1types.MsgLogicCallExecutedClaim{}, &gravityv1api.MsgLogicCallExecutedClaim{}, GenOpts.WithDisallowNil()),
		GenType(&gravityv1types.MsgRequestBatch{}, &gravityv1api.MsgRequestBatch{}, GenOpts.WithDisallowNil()),
		GenType(&gravityv1types.MsgSendToCosmosClaim{}, &gravityv1api.MsgSendToCosmosClaim{}, GenOpts.WithDisallowNil()),
		GenType(&gravityv1types.MsgSendToEth{}, &gravityv1api.MsgSendToEth{}, GenOpts.WithDisallowNil()),
		GenType(&gravityv1types.MsgSubmitBadSignatureEvidence{}, &gravityv1api.MsgSubmitBadSignatureEvidence{}, GenOpts.WithDisallowNil()),
		GenType(&gravityv1types.MsgValsetConfirm{}, &gravityv1api.MsgValsetConfirm{}, GenOpts.WithDisallowNil()),
		GenType(&gravityv1types.MsgValsetUpdatedClaim{}, &gravityv1api.MsgValsetUpdatedClaim{}, GenOpts.WithDisallowNil()),
		// The gravity v2 messages do not have legacy amino configuration, so we do not chack them
		// GenType(&gravityv2types.MsgAirdropProposal{}, &gravityv2api.MsgAirdropProposal{}, GenOpts.WithDisallowNil()),
		// GenType(&gravityv2types.MsgIBCMetadataProposal{}, &gravityv2api.MsgIBCMetadataProposal{}, GenOpts.WithDisallowNil()),
		// GenType(&gravityv2types.MsgUnhaltBridgeProposal{}, &gravityv2api.MsgUnhaltBridgeProposal{}, GenOpts.WithDisallowNil()),
		// GenType(&gravityv2types.MsgUpdateParamsProposal{}, &gravityv2api.MsgUpdateParamsProposal{}, GenOpts.WithDisallowNil()),

		// auction
		GenType(&auctiontypes.MsgBid{}, &auctionapi.MsgBid{}, GenOpts.WithDisallowNil()),
		GenType(&auctiontypes.MsgUpdateParamsProposal{}, &auctionapi.MsgUpdateParamsProposal{}, GenOpts.WithDisallowNil()),

		// bech32ibc does not have any messages, so we don't check it
	}

	for _, tt := range testedMsgs {
		desc := tt.Pulsar.ProtoReflect().Descriptor()
		name := string(desc.FullName())
		t.Run(name, func(t *testing.T) {
			gen := rapidproto.MessageGenerator(tt.Pulsar, tt.Opts)
			fmt.Printf("testing %s\n", tt.Pulsar.ProtoReflect().Descriptor().FullName())
			rapid.Check(t, func(t *rapid.T) {
				// uncomment to debug; catch a panic and inspect application state
				// defer func() {
				//	if r := recover(); r != nil {
				//		//fmt.Printf("Panic: %+v\n", r)
				//		t.FailNow()
				//	}
				// }()

				msg := gen.Draw(t, "msg")
				postFixPulsarMessage(msg)

				gogo := tt.Gogo
				sanity := tt.Pulsar

				protoBz, err := proto.Marshal(msg)
				require.NoError(t, err)

				err = proto.Unmarshal(protoBz, sanity)
				require.NoError(t, err)

				err = encCfg.Codec.Unmarshal(protoBz, gogo)
				require.NoError(t, err)

				legacyAminoJSON, err := encCfg.Amino.MarshalJSON(gogo)
				require.NoError(t, err)
				aminoJSON, err := aj.Marshal(msg)
				require.NoError(t, err)
				require.Equal(t, string(legacyAminoJSON), string(aminoJSON))

				// test amino json signer handler equivalence
				if !proto.HasExtension(desc.Options(), msgv1.E_Signer) {
					// not signable
					return
				}

				handlerOptions := signing_testutil.HandlerArgumentOptions{
					ChainID:       "test-chain",
					Memo:          "sometestmemo",
					Msg:           tt.Pulsar,
					AccNum:        1,
					AccSeq:        2,
					SignerAddress: "signerAddress",
					Fee: &txv1beta1.Fee{
						Amount: []*v1beta1.Coin{{Denom: "uatom", Amount: "1000"}},
					},
				}

				signerData, txData, err := signing_testutil.MakeHandlerArguments(handlerOptions)
				require.NoError(t, err)

				handler := aminojson.NewSignModeHandler(aminojson.SignModeHandlerOptions{})
				signBz, err := handler.GetSignBytes(context.Background(), signerData, txData)
				require.NoError(t, err)

				legacyHandler := tx.NewSignModeLegacyAminoJSONHandler()
				txBuilder := encCfg.TxConfig.NewTxBuilder()
				require.NoError(t, txBuilder.SetMsgs([]types.Msg{tt.Gogo}...))
				txBuilder.SetMemo(handlerOptions.Memo)
				txBuilder.SetFeeAmount(types.Coins{types.NewInt64Coin("uatom", 1000)})
				theTx := txBuilder.GetTx()

				legacySigningData := signing.SignerData{
					ChainID:       handlerOptions.ChainID,
					Address:       handlerOptions.SignerAddress,
					AccountNumber: handlerOptions.AccNum,
					Sequence:      handlerOptions.AccSeq,
				}
				legacySignBz, err := legacyHandler.GetSignBytes(signingtypes.SignMode_SIGN_MODE_LEGACY_AMINO_JSON,
					legacySigningData, theTx)
				require.NoError(t, err)
				require.Equal(t, string(legacySignBz), string(signBz))
			})
		})
	}
}

// nolint: exhaustruct
func postFixPulsarMessage(msg proto.Message) {
	if m, ok := msg.(*authapi.ModuleAccount); ok {
		if m.BaseAccount == nil {
			m.BaseAccount = &authapi.BaseAccount{}
		}
		_, _, bz := testdata.KeyTestPubAddr()
		// always set address to a valid bech32 address
		text, err := bech32.ConvertAndEncode("cosmos", bz)
		if err != nil {
			panic(fmt.Sprintf("failed to convert and encode address: %v", err))
		}
		m.BaseAccount.Address = text

		// see negative test
		if len(m.Permissions) == 0 {
			m.Permissions = nil
		}
	}

	if m, ok := msg.(*gravityv1api.MsgSendToCosmosClaim); ok {
		// fmt.Println("postFixPulsarMessage: MsgSendToCosmosClaim ", m)
		if m.Amount == "" {
			m.Amount = "0"
		}
		_, err := strconv.ParseUint(m.Amount, 10, 64)
		if err != nil {
			m.Amount = "0"
		}
	}

	if m, ok := msg.(*gravityv1api.MsgValsetUpdatedClaim); ok {
		// fmt.Println("postFixPulsarMessage: MsgSendToCosmosClaim ", m)
		if m.RewardAmount == "" {
			m.RewardAmount = "0"
		}
		_, err := strconv.ParseUint(m.RewardAmount, 10, 64)
		if err != nil {
			m.RewardAmount = "0"
		}
	}
}
