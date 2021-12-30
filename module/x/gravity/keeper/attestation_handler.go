package keeper

import (
	"fmt"
	distrkeeper "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
	"math/big"
	"strconv"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
	distypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
)

// Check that distKeeper implements the expected type
var _ types.DistributionKeeper = (*distrkeeper.Keeper)(nil)

// AttestationHandler processes `observed` Attestations
type AttestationHandler struct {
	// NOTE: If you add anything to this struct, add a nil check to ValidateMembers below!
	keeper     *Keeper
	bankKeeper *bankkeeper.BaseKeeper
	distKeeper *distrkeeper.Keeper
}

// Check for nil members
func (a AttestationHandler) ValidateMembers() {
	if a.keeper     == nil { panic("Nil keeper!") }
	if a.bankKeeper == nil { panic("Nil bankKeeper!") }
	if a.distKeeper == nil { panic("Nil distKeeper!") }
}

// SendToCommunityPool handles sending incorrect deposits to the community pool, since the deposits
// have already been made on Ethereum there's nothing we can do to reverse them, and we should at least
// make use of the tokens which would otherwise be lost
func (a AttestationHandler) SendToCommunityPool(ctx sdk.Context, coins sdk.Coins) error {
	if err := a.bankKeeper.SendCoinsFromModuleToModule(ctx, types.ModuleName, distypes.ModuleName, coins); err != nil {
		return sdkerrors.Wrap(err, "transfer to community pool failed")
	}
	feePool := (*a.distKeeper).GetFeePool(ctx)
	feePool.CommunityPool = feePool.CommunityPool.Add(sdk.NewDecCoinsFromCoins(coins...)...)
	(*a.distKeeper).SetFeePool(ctx, feePool)
	return nil
}

// Handle is the entry point for Attestation processing.
func (a AttestationHandler) Handle(ctx sdk.Context, att types.Attestation, claim types.EthereumClaim) error {
	switch claim := claim.(type) {
	// deposit in this context means a deposit into the Ethereum side of the bridge
	case *types.MsgSendToCosmosClaim:
		invalidAddress := false
		receiverAddress, addressErr := types.IBCAddressFromBech32(claim.CosmosReceiver)
		if addressErr != nil {
			invalidAddress = true
		}
		tokenAddress, errTokenAddress := types.NewEthAddress(claim.TokenContract)
		ethereumSender, errEthereumSender := types.NewEthAddress(claim.EthereumSender)
		// these are not possible unless the validators get together and submit
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

		// While not strictly necessary, explicitly making the receiver a native address
		// insulates us from the implicit address conversion done in x/bank's account store iterator
		nativeReceiver, err := types.GetNativePrefixedAccAddress(receiverAddress)

		if err != nil {
			invalidAddress = true
		}

		// Checks the address if it's inside the blacklisted address list and marks
		// if it's inside the list.
		if a.keeper.IsOnBlacklist(ctx, *ethereumSender) {
			invalidAddress = true
		}

		// Check if coin is Cosmos-originated asset and get denom
		isCosmosOriginated, denom := a.keeper.ERC20ToDenomLookup(ctx, *tokenAddress)
		coins := sdk.Coins{sdk.NewCoin(denom, claim.Amount)}

		if !isCosmosOriginated {
			// We need to mint eth-originated coins (aka vouchers)
			// Make sure that users are not bridging an impossible amount
			prevSupply := a.bankKeeper.GetSupply(ctx, denom)
			newSupply := new(big.Int).Add(prevSupply.Amount.BigInt(), claim.Amount.BigInt())
			if newSupply.BitLen() > 256 { // new supply overflows uint256
				a.keeper.logger(ctx).Error("Deposit Overflow",
					"claim type", claim.GetType(),
					"nonce", fmt.Sprint(claim.GetEventNonce()),
				)
				return sdkerrors.Wrap(types.ErrIntOverflowAttestation, "invalid supply after SendToCosmos attestation")
			}

			if err := a.bankKeeper.MintCoins(ctx, types.ModuleName, coins); err != nil {
				// in this case we have lost tokens! They are in the bridge, but not
				// in the community pool our out in some users balance, every instance of this
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
		}

		if !invalidAddress { // valid address so far, try to lock up the coins in the requested cosmos address
			if err := a.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, nativeReceiver, coins); err != nil {
				// someone attempted to send tokens to a blacklisted user from Ethereum, log and send to Community pool
				hash, _ := claim.ClaimHash()
				a.keeper.logger(ctx).Error("Blacklisted deposit",
					"cause", err.Error(),
					"claim type", claim.GetType(),
					"id", types.GetAttestationKey(claim.GetEventNonce(), hash),
					"nonce", fmt.Sprint(claim.GetEventNonce()),
				)
				invalidAddress = true
			}
		}

		// for whatever reason above, blacklisted, invalid string, etc this deposit is not valid
		// we can't send the tokens back on the Ethereum side, and if we don't put them somewhere on
		// the cosmos side they will be lost an inaccessible even though they are locked in the bridge.
		// so we deposit the tokens into the community pool for later use
		if invalidAddress {
			if err = a.SendToCommunityPool(ctx, coins); err != nil {
				hash, _ := claim.ClaimHash()
				a.keeper.logger(ctx).Error("Failed community pool send",
					"cause", err.Error(),
					"claim type", claim.GetType(),
					"id", types.GetAttestationKey(claim.GetEventNonce(), hash),
					"nonce", fmt.Sprint(claim.GetEventNonce()),
				)
				return sdkerrors.Wrap(err, "failed to send to Community pool")
			}
			ctx.EventManager().EmitEvent(
				sdk.NewEvent(
					types.EventTypeInvalidSendToCosmosReceiver,
					sdk.NewAttribute("MsgSendToCosmosAmount", claim.Amount.String()),
					sdk.NewAttribute("MsgSendToCosmosNonce", strconv.Itoa(int(claim.GetEventNonce()))),
					sdk.NewAttribute("MsgSendToCosmosToken", tokenAddress.GetAddress()),
					sdk.NewAttribute("MsgSendToCosmosSender", claim.EthereumSender),
				),
			)
		} else {
			ctx.EventManager().EmitEvent(
				sdk.NewEvent(
					sdk.EventTypeMessage,
					sdk.NewAttribute("MsgSendToCosmosAmount", claim.Amount.String()),
					sdk.NewAttribute("MsgSendToCosmosNonce", strconv.Itoa(int(claim.GetEventNonce()))),
					sdk.NewAttribute("MsgSendToCosmosToken", tokenAddress.GetAddress()),
				),
			)
		}
	// withdraw in this context means a withdraw from the Ethereum side of the bridge
	case *types.MsgBatchSendToEthClaim:
		contract, err := types.NewEthAddress(claim.TokenContract)
		if err != nil {
			return sdkerrors.Wrap(err, "invalid token contract on batch")
		}
		a.keeper.OutgoingTxBatchExecuted(ctx, *contract, claim.BatchNonce)
		ctx.EventManager().EmitEvent(
			sdk.NewEvent(
				sdk.EventTypeMessage,
				sdk.NewAttribute("MsgBatchSendToEthClaim", strconv.Itoa(int(claim.BatchNonce))),
			),
		)
		return nil
	case *types.MsgERC20DeployedClaim:
		tokenAddress, err := types.NewEthAddress(claim.TokenContract)
		if err != nil {
			return sdkerrors.Wrap(err, "invalid token contract on claim")
		}
		// Check if it already exists
		existingERC20, exists := a.keeper.GetCosmosOriginatedERC20(ctx, claim.CosmosDenom)
		if exists {
			return sdkerrors.Wrap(
				types.ErrInvalid,
				fmt.Sprintf("ERC20 %s already exists for denom %s", existingERC20, claim.CosmosDenom))
		}

		// Check if denom exists
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

		ctx.EventManager().EmitEvent(
			sdk.NewEvent(
				sdk.EventTypeMessage,
				sdk.NewAttribute("MsgERC20DeployedClaimToken", tokenAddress.GetAddress()),
				sdk.NewAttribute("MsgERC20DeployedClaim", strconv.Itoa(int(claim.GetEventNonce()))),
			),
		)
	case *types.MsgValsetUpdatedClaim:
		rewardAddress, err := types.NewEthAddress(claim.RewardToken)
		if err != nil {
			return sdkerrors.Wrap(err, "invalid reward token on claim")
		}
		// TODO here we should check the contents of the validator set against
		// the store, if they differ we should take some action to indicate to the
		// user that bridge highjacking has occurred
		a.keeper.SetLastObservedValset(ctx, types.Valset{
			Nonce:        claim.ValsetNonce,
			Members:      claim.Members,
			Height:       0,
			RewardAmount: claim.RewardAmount,
			RewardToken:  claim.RewardToken,
		})
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
				if err := a.bankKeeper.MintCoins(ctx, types.ModuleName, coins); err != nil {
					ctx.EventManager().EmitEvent(
						sdk.NewEvent(
							sdk.EventTypeMessage,
							sdk.NewAttribute("MsgValsetUpdatedClaim", strconv.Itoa(int(claim.GetEventNonce()))),
						),
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
		ctx.EventManager().EmitEvent(
			sdk.NewEvent(
				sdk.EventTypeMessage,
				sdk.NewAttribute("MsgValsetUpdatedClaim", strconv.Itoa(int(claim.GetEventNonce()))),
			),
		)

	default:
		panic(fmt.Sprintf("Invalid event type for attestations %s", claim.GetType()))
	}
	return nil
}
