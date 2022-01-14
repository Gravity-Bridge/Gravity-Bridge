package app

import (
	"fmt"
	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/keeper"
	sdk "github.com/cosmos/cosmos-sdk/types"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	distrkeeper "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	"github.com/stretchr/testify/require"
	tmtypes "github.com/tendermint/tendermint/types"
	"testing"
)

// Checks that given a modified genesis file, all validators and delegators affected by the delegation rewards
// withdrawal bug can successfully withdraw rewards/commission, relevant invariants all pass
// Instructions for creation of a fixed genesis file:
// 1. Obtain a halted chain's genesis and add it to module/app/tests
// 2. Add slashing events to account for all the missed gravity-slashing that occurred (check commit history for examples)
// 3. Run this test on the modified genesis, look at the "Updating historical rewards..." lines for each validator and
//		find the converging "new" value
// 4. Find each affected validator's most recent "validator_historical_rewards" entry and modify the "cumulative_rewards_ratio"
//		to have the converged "new" value from (3)
// 5. Rerun the test and ensure that every value output in the "Updating historical rewards..." lines are the converged value
//		If the value continues to change, perform steps (3) and (4) again as necessary
// 6. Ensure there are no output lines like "Error running _____ invariants!"
func TestWithdrawPanicModifiedGenesis(t *testing.T) {
	affectedValidators := []string{
		"gravityvaloper1gfeh4793ug8l6nu69n8lqssvkdqs8qghld0r9t",
		"gravityvaloper18u9pws4989n7fmx5pduev7starj8wqgg4jcxr3",
		"gravityvaloper1vu04rhxr4sq65vfd074m5gjzuggwukzhncmnzk",
		"gravityvaloper19lhajns3drveve4q8jc4kus6knjcmf2jq3jrsd",
		"gravityvaloper166mwr42yq2khtmwcv9akwjc0f3e62lwswd6ggu",
		"gravityvaloper1cfj5aksn6tgz5m9885qwp8dwaa844qe24uxwv2",
		"gravityvaloper104vmnec7p96qgzzdzddv5jan070v7vj6r4zw4x",
		"gravityvaloper134f6petuvgwtmrpwsfk9ng0fpkmqxqtf8s27q9",
		"gravityvaloper12qx44c75dd8ykckexxxxjn7ygehwxkqjh04uke",
		"gravityvaloper19uwnjc4fhftygyzr8wv5m8m0l4kjzkpsamzk2r",
		"gravityvaloper1xf869399xq4pnxsllzvnwdcra8ly3huj5cnwh5",
		"gravityvaloper1d63hvdgwy64sfex2kyujd0xyfhmu7r7cgxt5ru",
		"gravityvaloper1kp6kn7jazzn6leqvqzdm9ftmlpp72l40tz9dqh",
		"gravityvaloper1qtc3axqj2pf9h92arc2lmk7keyvrzqc847w8r9",
		"gravityvaloper1lu9p4tl02nl979l9huk33x8rgnwzwmysap0s60",
		"gravityvaloper1xmspr43xsfu6ycjgdqqllntc62rzvncv0d74fz",
	}

	input := keeper.CreateTestEnv(t, true)
	ctx := input.Context
	// Read the genesis file
	genDoc, err := tmtypes.GenesisDocFromFile("tests/withdraw-panic-genesis.json")
	require.NoError(t, err)

	keeper.InitFromGenesis(t, genDoc, input)

	// CreateTestEnv sets the ctx blockheight to something like 1234567, update it to be the genesis height
	ctx = ctx.WithBlockHeight(genDoc.InitialHeight)
	for _, val := range affectedValidators {
		valAddr, err := sdk.ValAddressFromBech32(val)
		require.NoError(t, err)
		validator, found := input.StakingKeeper.GetValidator(ctx, valAddr)
		if !found {
			panic(fmt.Sprintf("We didn't find validator %s", val))
		}
		for count := 0; count < 10; count++ {
			validatorCurrentPeriod := input.DistKeeper.GetValidatorCurrentRewards(ctx, valAddr).Period

			withdrawTotal := sdk.NewDecCoins(sdk.NewDecCoin("ugraviton", sdk.NewInt(0)))
			delegations := input.StakingKeeper.GetValidatorDelegations(ctx, valAddr)
			for _, del := range delegations {
				delegatorReward := input.DistKeeper.CalculateDelegationRewards(ctx, validator, del, validatorCurrentPeriod-1)
				withdrawTotal = withdrawTotal.Add(delegatorReward...)
			}
			commish := input.DistKeeper.GetValidatorAccumulatedCommission(ctx, valAddr).Commission
			withdrawTotal = withdrawTotal.Add(commish...)
			outstanding := input.DistKeeper.GetValidatorOutstandingRewards(ctx, valAddr)
			diff, neg := outstanding.Rewards.SafeSub(withdrawTotal)
			latestHist := input.DistKeeper.GetValidatorHistoricalRewards(ctx, valAddr, validatorCurrentPeriod-1)
			latestRewards := latestHist.CumulativeRewardRatio.AmountOf("ugraviton")
			if neg { // too few tokens outstanding, need to reduce the historical reward frac
				absDiff := diff.AmountOf("ugraviton").Abs()
				overstep := absDiff.QuoTruncate(validator.Tokens.ToDec())
				latestHist.CumulativeRewardRatio = sdk.NewDecCoins(sdk.NewDecCoinFromDec("ugraviton", latestRewards.Sub(overstep)))
			} else { // too many tokens outstanding, need to increase the historical reward frac
				overstep := diff.AmountOf("ugraviton").QuoTruncate(validator.Tokens.ToDec())
				latestHist.CumulativeRewardRatio = sdk.NewDecCoins(sdk.NewDecCoinFromDec("ugraviton", latestRewards.Add(overstep)))
			}
			fmt.Println("Updating historical rewards", "validator", val, "old", latestRewards.String(), "new", latestHist.CumulativeRewardRatio.AmountOf("ugraviton").String())

			// Update the historical rewards fraction
			input.DistKeeper.SetValidatorHistoricalRewards(ctx, valAddr, validatorCurrentPeriod-1, latestHist)
		}
		// Actually perform the withdrawals
		actualWithdrawTotal := sdk.NewDecCoins(sdk.NewDecCoin("ugraviton", sdk.NewInt(0)))
		delegations := input.StakingKeeper.GetValidatorDelegations(ctx, valAddr)
		for _, del := range delegations {
			delegatorReward, err := input.DistKeeper.WithdrawDelegationRewards(ctx, del.GetDelegatorAddr(), valAddr)
			require.NoError(t, err)
			actualWithdrawTotal = actualWithdrawTotal.Add(sdk.NewDecCoinsFromCoins(delegatorReward...)...)
		}
		commission, err := input.DistKeeper.WithdrawValidatorCommission(ctx, valAddr)
		require.NoError(t, err)
		actualWithdrawTotal = actualWithdrawTotal.Add(sdk.NewDecCoinsFromCoins(commission...)...)
		newOutstanding := input.DistKeeper.GetValidatorOutstandingRewards(ctx, valAddr)
		fmt.Println("Fixed withdrawal", "validator", val, "outstanding", newOutstanding, "withdrawTotal", actualWithdrawTotal)
	}

	errMsg, fail := distrkeeper.AllInvariants(input.DistKeeper)(ctx)
	if fail {
		fmt.Println("Error running distribution invariants", "errorMessage", errMsg)
		panic("Error running distribution invariants!")
	} else {
		fmt.Println("Successful distribution invariants")
	}
	errMsg, fail = bankkeeper.AllInvariants(input.BankKeeper)(ctx)
	if fail {
		fmt.Println("Error running bank invariants", "errorMessage", errMsg)
		panic("Error running bank invariants!")
	} else {
		fmt.Println("Successful bank invariants")
	}
	errMsg, fail = stakingkeeper.AllInvariants(input.StakingKeeper)(ctx)
	if fail {
		fmt.Println("Error running steak invariants", "errorMessage", errMsg)
		panic("Error running steak invariants!")
	} else {
		fmt.Println("Successful steak invariants")
	}
}
