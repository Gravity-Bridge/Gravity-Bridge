package ante

import (
	"fmt"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkante "github.com/cosmos/cosmos-sdk/x/auth/ante"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	feegrantkeeper "github.com/cosmos/cosmos-sdk/x/feegrant/keeper"
	ibcante "github.com/cosmos/ibc-go/v6/modules/core/ante"
	ibckeeper "github.com/cosmos/ibc-go/v6/modules/core/keeper"

	ethermintante "github.com/evmos/ethermint/app/ante"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/keeper"
)

// NewAnteHandler Constructs a new sdk.AnteHandler for the Gravity app.
// AnteHandlers are functions which validate Txs before their contained Msgs are executed, AnteHandlers are constructed
// from a chain of AnteDecorators, the final AnteDecorator in the chain being sdkante.Terminator.
// This custom AnteHandler constructor chains together the base CosmosSDK decorators with some other custom decorators,
// in order to support EIP-712 signed messages (enabling MetaMask signed transactions in browsers) and maintain existing
// functionality. Before EIP712 support was added, Gravity was able to support users signing a hash of the transaction data using uncommon
// wallet signing methods, the new EIP-712 support enables users to see what they are signing by presenting typed data in the
// confirmation screen. Additionally, Ledger support should work with EIP-712 when it would not without.
//
// For more information see https://github.com/evmos/ethermint/blob/v0.19.3/app/ante/ante.go, which inspired this handler.
func NewAnteHandler(
	options sdkante.HandlerOptions,
	gravityKeeper *keeper.Keeper,
	accountKeeper *authkeeper.AccountKeeper,
	bankKeeper *bankkeeper.BaseKeeper,
	feegrantKeeper *feegrantkeeper.Keeper,
	ibcKeeper *ibckeeper.Keeper,
	cdc codec.Codec,
	evmChainID string,
) (*sdk.AnteHandler, error) {
	if evmChainID == "" {
		return nil, fmt.Errorf("evmChainID not specified, EIP-712 signing will fail")
	}

	fullHandler := sdk.ChainAnteDecorators(
		// Do not support EVM txs (e.g. solidity contract call txs), easy mistake to make when using MetaMask
		ethermintante.RejectMessagesDecorator{},
		sdkante.NewSetUpContextDecorator(),
		// Allows exactly 1 type of extension options, and only one of that type to be provided
		NewGravityRejectExtensionsDecorator(cdc),
		sdkante.NewValidateBasicDecorator(),
		sdkante.NewTxTimeoutHeightDecorator(),
		sdkante.NewValidateMemoDecorator(accountKeeper),
		sdkante.NewConsumeGasForTxSizeDecorator(accountKeeper),
		sdkante.NewDeductFeeDecorator(accountKeeper, bankKeeper, feegrantKeeper, nil),
		sdkante.NewSetPubKeyDecorator(accountKeeper),
		sdkante.NewValidateSigCountDecorator(accountKeeper),
		sdkante.NewSigGasConsumeDecorator(accountKeeper, options.SigGasConsumer),
		// Delegates to EIP-712 verification OR to regular SDK verification depending on the extension option
		NewGravitySigVerificationDecorator(cdc, accountKeeper, options.SignModeHandler, evmChainID),
		sdkante.NewIncrementSequenceDecorator(accountKeeper),
		ibcante.NewRedundantRelayDecorator(ibcKeeper),
		// Enforces the minimum commission for Gravity Prop #1
		NewMinCommissionDecorator(cdc),
	)

	return &fullHandler, nil
}
