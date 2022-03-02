package keeper

import (
	"fmt"
	ibctransfertypes "github.com/cosmos/ibc-go/v2/modules/apps/transfer/types"
	ibcclienttypes "github.com/cosmos/ibc-go/v2/modules/core/02-client/types"
	"math/big"
	"strconv"
	"strings"

	distrkeeper "github.com/cosmos/cosmos-sdk/x/distribution/keeper"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	distypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
)

// Check that distKeeper implements the expected type
var _ types.DistributionKeeper = (*distrkeeper.Keeper)(nil)

// AttestationHandler processes `observed` Attestations
type AttestationHandler struct {
	// NOTE: If you add anything to this struct, add a nil check to ValidateMembers below!
	keeper *Keeper
}

// Check for nil members
func (a AttestationHandler) ValidateMembers() {
	if a.keeper == nil {
		panic("Nil keeper!")
	}
}

// SendToCommunityPool handles sending incorrect deposits to the community pool, since the deposits
// have already been made on Ethereum there's nothing we can do to reverse them, and we should at least
// make use of the tokens which would otherwise be lost
func (a AttestationHandler) SendToCommunityPool(ctx sdk.Context, coins sdk.Coins) error {
	if err := a.keeper.bankKeeper.SendCoinsFromModuleToModule(ctx, types.ModuleName, distypes.ModuleName, coins); err != nil {
		return sdkerrors.Wrap(err, "transfer to community pool failed")
	}
	feePool := (*a.keeper.DistKeeper).GetFeePool(ctx)
	feePool.CommunityPool = feePool.CommunityPool.Add(sdk.NewDecCoinsFromCoins(coins...)...)
	(*a.keeper.DistKeeper).SetFeePool(ctx, feePool)
	return nil
}

// Handle is the entry point for Attestation processing, only attestations with sufficient validator submissions
// should be processed through this function, solidifying their effect in chain state
func (a AttestationHandler) Handle(ctx sdk.Context, att types.Attestation, claim types.EthereumClaim) error {
	switch claim := claim.(type) {

	case *types.MsgSendToCosmosClaim:
		return a.handleSendToCosmos(ctx, *claim)

	case *types.MsgBatchSendToEthClaim:
		return a.handleBatchSendToEth(ctx, *claim)

	case *types.MsgERC20DeployedClaim:

		return a.handleErc20Deployed(ctx, *claim)

	case *types.MsgValsetUpdatedClaim:
		return a.handleValsetUpdated(ctx, *claim)

	default:
		panic(fmt.Sprintf("Invalid event type for attestations %s", claim.GetType()))
	}
}

// Upon acceptance of sufficient validator SendToCosmos claims: transfer tokens to the appropriate cosmos account
// The cosmos receiver can be a native account (e.g. gravity1abc...) or a foreign account (e.g. cosmos1abc...)
// In the event of a native receiver, bank module handles the transfer, otherwise an IBC transfer is initiated
// Note: Previously SendToCosmos was referred to as a bridge "Deposit", as tokens are deposited into the gravity contract
func (a AttestationHandler) handleSendToCosmos(ctx sdk.Context, claim types.MsgSendToCosmosClaim) error {
	invalidAddress := false
	receiverAddress, addressErr := types.IBCAddressFromBech32(claim.CosmosReceiver)
	if addressErr != nil {
		invalidAddress = true
	}
	tokenAddress, errTokenAddress := types.NewEthAddress(claim.TokenContract)
	ethereumSender, errEthereumSender := types.NewEthAddress(claim.EthereumSender)
	// nil address is not possible unless the validators get together and submit
	// a bogus event, this would create lost tokens stuck in the bridge
	// and not accessible to anyone
	if errTokenAddress != nil {
		hash, _ := claim.ClaimHash()
		a.keeper.logger(ctx).Error("Invalid token contract",
			"cause", errTokenAddress.Error(),
			"claim type", claim.GetType(),
			"id", types.GetAttestationKey(claim.GetEventNonce(), hash),
			"nonce", fmt.Sprint(claim.GetEventNonce()),
		)
		return sdkerrors.Wrap(errTokenAddress, "invalid token contract on claim")
	}
	// likewise nil sender would have to be caused by a bogus event
	if errEthereumSender != nil {
		hash, _ := claim.ClaimHash()
		a.keeper.logger(ctx).Error("Invalid ethereum sender",
			"cause", errEthereumSender.Error(),
			"claim type", claim.GetType(),
			"id", types.GetAttestationKey(claim.GetEventNonce(), hash),
			"nonce", fmt.Sprint(claim.GetEventNonce()),
		)
		return sdkerrors.Wrap(errTokenAddress, "invalid ethereum sender on claim")
	}

	// Block blacklisted asset transfers
	// (these funds are unrecoverable for the blacklisted sender, they will instead be sent to community pool)
	if a.keeper.IsOnBlacklist(ctx, *ethereumSender) {
		invalidAddress = true
	}

	// Check if coin is Cosmos-originated asset and get denom
	isCosmosOriginated, denom := a.keeper.ERC20ToDenomLookup(ctx, *tokenAddress)
	coin := sdk.NewCoin(denom, claim.Amount)
	coins := sdk.Coins{coin}

	moduleAddr := a.keeper.accountKeeper.GetModuleAddress(types.ModuleName)
	if !isCosmosOriginated { // We need to mint eth-originated coins (aka vouchers)
		preMintBalance := a.keeper.bankKeeper.GetBalance(ctx, moduleAddr, denom)
		// Ensure that users are not bridging an impossible amount, only 2^256 - 1 tokens can exist on ethereum
		prevSupply := a.keeper.bankKeeper.GetSupply(ctx, denom)
		newSupply := new(big.Int).Add(prevSupply.Amount.BigInt(), claim.Amount.BigInt())
		if newSupply.BitLen() > 256 { // new supply overflows uint256
			a.keeper.logger(ctx).Error("Deposit Overflow",
				"claim type", claim.GetType(),
				"nonce", fmt.Sprint(claim.GetEventNonce()),
			)
			return sdkerrors.Wrap(types.ErrIntOverflowAttestation, "invalid supply after SendToCosmos attestation")
		}

		if err := a.keeper.bankKeeper.MintCoins(ctx, types.ModuleName, coins); err != nil {
			// in this case we have lost tokens! They are in the bridge, but not
			// in the community pool or out in some users balance, every instance of this
			// error needs to be detected and resolved
			hash, _ := claim.ClaimHash()
			a.keeper.logger(ctx).Error("Failed minting",
				"cause", err.Error(),
				"claim type", claim.GetType(),
				"id", types.GetAttestationKey(claim.GetEventNonce(), hash),
				"nonce", fmt.Sprint(claim.GetEventNonce()),
			)
			return sdkerrors.Wrapf(err, "mint vouchers coins: %s", coins)
		}

		postMintBalance := a.keeper.bankKeeper.GetBalance(ctx, moduleAddr, denom)
		if !postMintBalance.Sub(preMintBalance).Amount.Equal(claim.Amount) {
			panic(fmt.Sprintf(
				"Somehow minted incorrect amount! Previous balance %v Post-mint balance %v claim amount %v",
				preMintBalance.String(), postMintBalance.String(), claim.Amount.String()),
			)
		}
	}

	if !invalidAddress { // address appears valid, attempt to send minted/locked coins to receiver
		preSendBalance := a.keeper.bankKeeper.GetBalance(ctx, moduleAddr, denom)
		err := a.sendCoinToCosmosAccount(ctx, claim, receiverAddress, coin)
		if err != nil {
			invalidAddress = true
		} else {
			postSendBalance := a.keeper.bankKeeper.GetBalance(ctx, moduleAddr, denom)
			if !preSendBalance.Sub(postSendBalance).Amount.Equal(claim.Amount) {
				panic(fmt.Sprintf(
					"Somehow sent incorrect amount! Previous balance %v Post-send balance %v claim amount %v",
					preSendBalance.String(), postSendBalance.String(), claim.Amount.String()),
				)
			}
		}
	}

	// for whatever reason above, blacklisted, invalid string, etc this deposit is not valid
	// we can't send the tokens back on the Ethereum side, and if we don't put them somewhere on
	// the cosmos side they will be lost an inaccessible even though they are locked in the bridge.
	// so we deposit the tokens into the community pool for later use via governance vote
	if invalidAddress {
		if err := a.SendToCommunityPool(ctx, coins); err != nil {
			hash, _ := claim.ClaimHash()
			a.keeper.logger(ctx).Error("Failed community pool send",
				"cause", err.Error(),
				"claim type", claim.GetType(),
				"id", types.GetAttestationKey(claim.GetEventNonce(), hash),
				"nonce", fmt.Sprint(claim.GetEventNonce()),
			)
			return sdkerrors.Wrap(err, "failed to send to Community pool")
		}

		ctx.EventManager().EmitTypedEvent(
			&types.EventInvalidSendToCosmosReceiver{
				Amount: claim.Amount.String(),
				Nonce:  strconv.Itoa(int(claim.GetEventNonce())),
				Token:  tokenAddress.GetAddress().Hex(),
				Sender: claim.EthereumSender,
			},
		)

	} else {
		ctx.EventManager().EmitTypedEvent(
			&types.EventSendToCosmos{
				Amount: claim.Amount.String(),
				Nonce:  strconv.Itoa(int(claim.GetEventNonce())),
				Token:  tokenAddress.GetAddress().Hex(),
			},
		)
	}

	return nil
}

// Upon acceptance of sufficient validator BatchSendToEth claims: burn ethereum originated vouchers, invalidate pending
// batches with lower claim.BatchNonce, and clean up state
// Note: Previously SendToEth was referred to as a bridge "Withdrawal", as tokens are withdrawn from the gravity contract
func (a AttestationHandler) handleBatchSendToEth(ctx sdk.Context, claim types.MsgBatchSendToEthClaim) error {
	contract, err := types.NewEthAddress(claim.TokenContract)
	if err != nil {
		return sdkerrors.Wrap(err, "invalid token contract on batch")
	}
	a.keeper.OutgoingTxBatchExecuted(ctx, *contract, claim.BatchNonce)

	ctx.EventManager().EmitTypedEvent(
		&types.EventBatchSendToEthClaim{
			Nonce: strconv.Itoa(int(claim.BatchNonce)),
		},
	)

	return nil
}

// Upon acceptance of sufficient ERC20 Deployed claims, register claim.TokenContract as the canonical ethereum
// representation of the metadata governance previously voted for
func (a AttestationHandler) handleErc20Deployed(ctx sdk.Context, claim types.MsgERC20DeployedClaim) error {
	tokenAddress, err := types.NewEthAddress(claim.TokenContract)
	if err != nil {
		return sdkerrors.Wrap(err, "invalid token contract on claim")
	}
	// Disallow re-registration when a token already has a canonical representation
	existingERC20, exists := a.keeper.GetCosmosOriginatedERC20(ctx, claim.CosmosDenom)
	if exists {
		return sdkerrors.Wrap(
			types.ErrInvalid,
			fmt.Sprintf("ERC20 %s already exists for denom %s", existingERC20.GetAddress().Hex(), claim.CosmosDenom))
	}

	// Check if denom metadata has been accepted by governance
	metadata, ok := a.keeper.bankKeeper.GetDenomMetaData(ctx, claim.CosmosDenom)
	if !ok || metadata.Base == "" {
		return sdkerrors.Wrap(types.ErrUnknown, fmt.Sprintf("denom not found %s", claim.CosmosDenom))
	}

	// Check if attributes of ERC20 match Cosmos denom
	if claim.Name != metadata.Name {
		return sdkerrors.Wrap(
			types.ErrInvalid,
			fmt.Sprintf("ERC20 name %s does not match denom name %s", claim.Name, metadata.Description))
	}

	if claim.Symbol != metadata.Symbol {
		return sdkerrors.Wrap(
			types.ErrInvalid,
			fmt.Sprintf("ERC20 symbol %s does not match denom symbol %s", claim.Symbol, metadata.Display))
	}

	// ERC20 tokens use a very simple mechanism to tell you where to display the decimal point.
	// The "decimals" field simply tells you how many decimal places there will be.
	// Cosmos denoms have a system that is much more full featured, with enterprise-ready token denominations.
	// There is a DenomUnits array that tells you what the name of each denomination of the
	// token is.
	// To correlate this with an ERC20 "decimals" field, we have to search through the DenomUnits array
	// to find the DenomUnit which matches up to the main token "display" value. Then we take the
	// "exponent" from this DenomUnit.
	// If the correct DenomUnit is not found, it will default to 0. This will result in there being no decimal places
	// in the token's ERC20 on Ethereum. So, for example, if this happened with Atom, 1 Atom would appear on Ethereum
	// as 1 million Atoms, having 6 extra places before the decimal point.
	// This will only happen with a Denom Metadata which is for all intents and purposes invalid, but I am not sure
	// this is checked for at any other point.
	decimals := uint32(0)
	for _, denomUnit := range metadata.DenomUnits {
		if denomUnit.Denom == metadata.Display {
			decimals = denomUnit.Exponent
			break
		}
	}

	if decimals != uint32(claim.Decimals) {
		return sdkerrors.Wrap(
			types.ErrInvalid,
			fmt.Sprintf("ERC20 decimals %d does not match denom decimals %d", claim.Decimals, decimals))
	}

	// Add to denom-erc20 mapping
	a.keeper.setCosmosOriginatedDenomToERC20(ctx, claim.CosmosDenom, *tokenAddress)

	ctx.EventManager().EmitTypedEvent(
		&types.EventERC20DeployedClaim{
			Token: tokenAddress.GetAddress().Hex(),
			Nonce: strconv.Itoa(int(claim.GetEventNonce())),
		},
	)
	return nil
}

// Upon acceptance of sufficient ValsetUpdated claims: update LastObservedValset, mint cosmos-originated relayer rewards
// so that the reward holder can send them to cosmos
func (a AttestationHandler) handleValsetUpdated(ctx sdk.Context, claim types.MsgValsetUpdatedClaim) error {
	rewardAddress, err := types.NewEthAddress(claim.RewardToken)
	if err != nil {
		return sdkerrors.Wrap(err, "invalid reward token on claim")
	}

	claimSet := types.Valset{
		Nonce:        claim.ValsetNonce,
		Members:      claim.Members,
		Height:       0, // Fill out later when used
		RewardAmount: claim.RewardAmount,
		RewardToken:  claim.RewardToken,
	}
	// check the contents of the validator set against the store, if they differ we know that the bridge has been
	// highjacked
	if claim.ValsetNonce != 0 { // Handle regular valsets
		trustedValset := a.keeper.GetValset(ctx, claim.ValsetNonce)
		if trustedValset == nil {
			ctx.Logger().Error("Received attestation for a valset which does not exist in store", "nonce", claim.ValsetNonce, "claim", claim)
			return sdkerrors.Wrapf(types.ErrInvalidValset, "attested valset (%v) does not exist in store", claim.ValsetNonce)
		}
		observedValset := claimSet
		observedValset.Height = trustedValset.Height // overwrite the height, since it's not part of the claim

		if _, err := trustedValset.Equal(observedValset); err != nil {
			panic(fmt.Sprintf("Potential bridge highjacking: observed valset (%+v) does not match stored valset (%+v)! %s", observedValset, trustedValset, err.Error()))
		}

		a.keeper.SetLastObservedValset(ctx, observedValset)
	} else { // The 0th valset is not stored on chain init, but we need to set it as the last one
		// Do not update Height, it's the first valset
		a.keeper.SetLastObservedValset(ctx, claimSet)
	}

	// if the reward is greater than zero and the reward token
	// is valid then some reward was issued by this validator set
	// and we need to either add to the total tokens for a Cosmos native
	// token, or burn non cosmos native tokens
	if claim.RewardAmount.GT(sdk.ZeroInt()) && claim.RewardToken != types.ZeroAddressString {
		// Check if coin is Cosmos-originated asset and get denom
		isCosmosOriginated, denom := a.keeper.ERC20ToDenomLookup(ctx, *rewardAddress)
		if isCosmosOriginated {
			// If it is cosmos originated, mint some coins to account
			// for coins that now exist on Ethereum and may eventually come
			// back to Cosmos.
			//
			// Note the flow is
			// user relays valset and gets reward -> event relayed to cosmos mints tokens to module
			// -> user sends tokens to cosmos and gets the minted tokens from the module
			//
			// it is not possible for this to be a race condition thanks to the event nonces
			// no matter how long it takes to relay the valset updated event the deposit event
			// for the user will always come after.
			//
			// Note we are minting based on the claim! This is important as the reward value
			// could change between when this event occurred and the present
			coins := sdk.Coins{sdk.NewCoin(denom, claim.RewardAmount)}
			if err := a.keeper.bankKeeper.MintCoins(ctx, types.ModuleName, coins); err != nil {
				ctx.EventManager().EmitTypedEvent(
					&types.EventValsetUpdatedClaim{
						Nonce: strconv.Itoa(int(claim.GetEventNonce())),
					},
				)
				return sdkerrors.Wrapf(err, "unable to mint cosmos originated coins %v", coins)
			}
		} else {
			// // If it is not cosmos originated, burn the coins (aka Vouchers)
			// // so that we don't think we have more in the bridge than we actually do
			// coins := sdk.Coins{sdk.NewCoin(denom, claim.RewardAmount)}
			// a.bankKeeper.BurnCoins(ctx, types.ModuleName, coins)

			// if you want to issue Ethereum originated tokens remove this panic and uncomment
			// the above code but note that you will have to constantly replenish the tokens in the
			// module or your chain will eventually halt.
			panic("Can not use Ethereum originated token as reward!")
		}
	}
	ctx.EventManager().EmitTypedEvent(
		&types.EventValsetUpdatedClaim{
			Nonce: strconv.Itoa(int(claim.GetEventNonce())),
		},
	)

	return nil
}

// Transfer tokens to gravity native accounts via bank module or foreign accounts via ibc-transfer
// If the bech32 prefix is not registered with bech32ibc module or if ibc-transfer fails immediately, send tokens to
// gravity1... re-prefixed account
// (e.g. claim.CosmosReceiver = "cosmos1<account><cosmos-suffix>", tokens will be received by gravity1<account><gravity-suffix>)
func (a AttestationHandler) sendCoinToCosmosAccount(
	ctx sdk.Context, claim types.MsgSendToCosmosClaim, receiver sdk.AccAddress, coin sdk.Coin,
) error {
	accountPrefix, err := types.GetPrefixFromBech32(receiver.String())
	if err != nil {
		hash, _ := claim.ClaimHash()
		a.keeper.logger(ctx).Error("Invalid bech32 CosmosReceiver",
			"cause", err.Error(), "address", receiver.String(),
			"claim type", claim.GetType(),
			"id", types.GetAttestationKey(claim.GetEventNonce(), hash),
			"nonce", fmt.Sprint(claim.GetEventNonce()),
		)
		return err
	}
	nativePrefix, err := a.keeper.bech32IbcKeeper.GetNativeHrp(ctx)
	if err != nil {
		// In a real environment bech32ibc panics on InitGenesis and on Send with their bech32ics20 module, which
		// prevents all MsgSend + MsgMultiSend transfers, in a testing environment it is possible to hit this condition,
		// so we should panic as well. This will cause a chain halt, and prevent attestation handling until prefix is set
		panic("SendToCosmos failure: bech32ibc NativeHrp has not been set!")
	}

	if accountPrefix == nativePrefix { // Send to a native gravity account
		return a.sendCoinToLocalAddress(ctx, claim, receiver, accountPrefix, nativePrefix, coin)
	} else { // Try to send tokens to IBC chain, fall back to native send on errors
		// Discover the IBC chain to send tokens to
		hrpIbcRecord, err := a.keeper.bech32IbcKeeper.GetHrpIbcRecord(ctx, accountPrefix)
		if err != nil {
			hash, _ := claim.ClaimHash()
			a.keeper.logger(ctx).Error("Unregistered foreign prefix",
				"cause", err.Error(), "address", receiver.String(),
				"claim type", claim.GetType(),
				"id", types.GetAttestationKey(claim.GetEventNonce(), hash),
				"nonce", fmt.Sprint(claim.GetEventNonce()),
			)

			// Fall back to sending tokens to native account
			return sdkerrors.Wrap(
				a.sendCoinToLocalAddress(ctx, claim, receiver, accountPrefix, nativePrefix, coin),
				"Unregistered foreign prefix, send via x/bank",
			)
		}

		// Send via IBC
		err = a.sendCoinToIbcAddress(ctx, receiver, coin, hrpIbcRecord.SourceChannel, claim)

		if err != nil {
			a.keeper.logger(ctx).Error(
				"SendToCosmos IBC auto forwarding failed, sending to local gravity account instead",
				"cosmos-receiver", claim.CosmosReceiver, "cosmos-denom", coin.Denom, "amount", coin.Amount.String(),
				"ethereum-contract", claim.TokenContract, "sender", claim.EthereumSender, "event-nonce", claim.EventNonce,
			)
			// Fall back to sending tokens to native account
			return sdkerrors.Wrap(
				a.sendCoinToLocalAddress(ctx, claim, receiver, accountPrefix, nativePrefix, coin),
				"IBC Transfer failure, send via x/bank",
			)
		}
	}
	return nil
}

// Send tokens via bank keeper to a native gravity address, re-prefixing receiver to a gravity native address if necessary
// Note: This should only be used as part of SendToCosmos attestation handling and is not a good solution for general use
func (a AttestationHandler) sendCoinToLocalAddress(
	ctx sdk.Context, claim types.MsgSendToCosmosClaim, receiver sdk.AccAddress, accountPrefix string,
	nativePrefix string, coin sdk.Coin) (err error) {
	// Panic on invalid input to avoid attestation from being mishandled until bug fix
	if strings.TrimSpace(accountPrefix) == "" || strings.TrimSpace(nativePrefix) == "" {
		panic("invalid call to sendCoinToLocalAddress: provided accountPrefix and/or nativePrefix is empty!")
	}
	if receiver.String()[:len(accountPrefix)] != accountPrefix {
		panic("invalid call to sendCoinToLocalAddress: provided accountPrefix is not the receiver's actual prefix!")
	}

	// Re-prefix the account if necessary
	if accountPrefix != nativePrefix {
		receiver, err = types.GetNativePrefixedAccAddress(ctx, *a.keeper.bech32IbcKeeper, receiver)
		if err != nil || receiver.String()[:len(nativePrefix)] != nativePrefix {
			// Unable to send
			return sdkerrors.Wrapf(types.ErrInvalid,
				"invalid result from GetNativePrefixedAccAddress: err %v result %s expectedPrefix %s",
				err, receiver.String(), nativePrefix,
			)
		}

		a.keeper.logger(ctx).Info("SendToCosmos Ibc transfer failed, sending to re-prefixed gravity address",
			"original-receiver", claim.CosmosReceiver, "re-prefixed-receiver", receiver.String())
	}

	err = a.keeper.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, receiver, sdk.NewCoins(coin))
	if err != nil {
		// someone attempted to send tokens to a blacklisted user from Ethereum, log and send to Community pool
		hash, _ := claim.ClaimHash()
		a.keeper.logger(ctx).Error("Blacklisted deposit",
			"cause", err.Error(),
			"claim type", claim.GetType(),
			"id", types.GetAttestationKey(claim.GetEventNonce(), hash),
			"nonce", fmt.Sprint(claim.GetEventNonce()),
		)
	} else { // no error
		a.keeper.logger(ctx).Info("SendToCosmos to local gravity receiver", "eth-sender", claim.EthereumSender,
			"receiver", receiver.String(), "denom", coin.Denom, "amount", coin.Amount.String(),
			claim.EventNonce, "eth-contract", claim.TokenContract, "eth-block-height", claim.BlockHeight,
			"cosmos-block-height", ctx.BlockHeight(),
		)
		ctx.EventManager().EmitTypedEvent(&types.EventSendToCosmosLocal{
			Nonce:    fmt.Sprint(claim.EventNonce),
			Receiver: receiver.String(),
			Token:    coin.Denom,
			Amount:   coin.Amount.String(),
		})
	}

	return err // returns nil if no error
}

// Send tokens via ibc-transfer module to foreign cosmos account
// The ibc MsgTransfer is sent with all zero timeouts, as retrying a failed send is not an easy option
// Note: This should only be used as part of SendToCosmos attestation handling and is not a good solution for general use
func (a AttestationHandler) sendCoinToIbcAddress(ctx sdk.Context, receiver sdk.AccAddress, coin sdk.Coin, channel string, claim types.MsgSendToCosmosClaim) error {
	portId := a.keeper.ibcTransferKeeper.GetPort(ctx)
	// Gravity module minted/locked the coins, so it must be the sender
	from := a.keeper.accountKeeper.GetModuleAccount(ctx, types.ModuleName).GetAddress()

	ibcTransferMsg := ibctransfertypes.NewMsgTransfer(
		portId,
		channel,
		coin,
		from.String(),
		receiver.String(),
		// zero-valued timeouts means the packet never times out
		ibcclienttypes.Height{RevisionHeight: 0, RevisionNumber: 0},
		0,
	)

	// Attempt to transfer the newly minted/unlocked coins via
	_, err := a.keeper.ibcTransferKeeper.Transfer(sdk.WrapSDKContext(ctx), ibcTransferMsg)

	// Log + emit event
	if err == nil {
		a.keeper.logger(ctx).Info("SendToCosmos IBC auto-forward", "eth-sender", claim.EthereumSender,
			"ibc-receiver", receiver.String(), "denom", coin.Denom, "amount", coin.Amount.String(), "ibc-port", portId,
			"ibc-channel", channel, "timeout-height", ibcTransferMsg.TimeoutHeight.String(),
			"timeout-timestamp", ibcTransferMsg.TimeoutTimestamp, "claim-nonce", claim.EventNonce, "eth-contract", claim.TokenContract,
			"eth-block-height", claim.BlockHeight, "cosmos-block-height", ctx.BlockHeight(),
		)

		ctx.EventManager().EmitTypedEvent(&types.EventSendToCosmosIbc{
			Nonce:    fmt.Sprint(claim.EventNonce),
			Receiver: receiver.String(),
			Token:    coin.Denom,
			Amount:   coin.Amount.String(),
			Channel:  channel,
		})
	}

	return err // Returns nil
}
