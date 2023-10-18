package cli_test

import (
	"fmt"
	"strings"
	"time"

	"github.com/cosmos/cosmos-sdk/client/flags"
	clitestutil "github.com/cosmos/cosmos-sdk/testutil/cli"
	"github.com/cosmos/cosmos-sdk/testutil/network"
	"github.com/stretchr/testify/suite"
	tmcli "github.com/tendermint/tendermint/libs/cli"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/app"
	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/auction/client/cli"
)

// TODO: adding unit test for cli
type IntegrationTestSuite struct {
	suite.Suite

	cfg     network.Config
	network *network.Network
}

func (s *IntegrationTestSuite) SetupSuite() {
	s.T().Log("setting up integration test suite")

	s.cfg = app.DefaultConfig()

	genesisState := app.ModuleBasics.DefaultGenesis(s.cfg.Codec)

	s.cfg.GenesisState = genesisState

	s.network = network.New(s.T(), s.cfg)

	_, err := s.network.WaitForHeight(1)
	s.Require().NoError(err)

}

func (s *IntegrationTestSuite) TearDownSuite() {
	s.T().Log("tearing down integration test suite")
	s.network.Cleanup()
}

func (s *IntegrationTestSuite) TestGetCmdQueryParams() {
	s.SetupSuite()
	val := s.network.Validators[0]

	testCases := []struct {
		name           string
		args           []string
		expectedOutput string
	}{
		{
			"json output",
			[]string{fmt.Sprintf("--%s=json", tmcli.OutputFlag)},
			`{"auction_epoch":"100","auction_period":"1209600s","min_bid_amount":"10000","bid_gap":"50","auction_rate":"0.020000000000000000","allow_tokens":""}`,
		},
		{
			"text output",
			[]string{fmt.Sprintf("--%s=1", flags.FlagHeight), fmt.Sprintf("--%s=text", tmcli.OutputFlag)},
			``,
		},
	}

	for _, tc := range testCases {
		tc := tc

		s.Run(tc.name, func() {
			cmd := cli.GetCmdQueryParams()
			clientCtx := val.ClientCtx

			out, err := clitestutil.ExecTestCLICmd(clientCtx, cmd, tc.args)
			s.Require().NoError(err)
			s.Require().Equal(tc.expectedOutput, strings.TrimSpace(out.String()))
		})
	}
}

func (s *IntegrationTestSuite) TestGetCmdAuctionPeriods() {
	s.SetupSuite()
	val := s.network.Validators[0]
	id := "1"

	// because when 30 epochs in beginblock will automatically startMewAuctionPeriod,
	// default params.AuctionEpoch=10 so we will have more than 2 AuctionPeriodId
	_, err := s.network.WaitForHeightWithTimeout(30, 20*time.Second)
	s.Require().NoError(err)

	testCases := []struct {
		name           string
		args           []string
		expectedOutput string
	}{
		{
			"test",
			[]string{id, fmt.Sprintf("--%s=json", tmcli.OutputFlag)},
			``,
		},
	}

	for _, tc := range testCases {
		tc := tc

		s.Run(tc.name, func() {
			cmd := cli.GetCmdAuctionPeriods()
			clientCtx := val.ClientCtx

			out, err := clitestutil.ExecTestCLICmd(clientCtx, cmd, tc.args)
			s.Require().NoError(err)
			s.Require().Equal(tc.expectedOutput, strings.TrimSpace(out.String()))
		})
	}
}

func (s *IntegrationTestSuite) TestGetCmdAuction() {
	s.SetupSuite()
	val := s.network.Validators[0]
	auction_id := "1"
	period_id := "1"

	// because when 30 epochs in beginblock will automatically startMewAuctionPeriod,
	// default params.AuctionEpoch=10 so we will have more than 2 AuctionPeriodId
	_, err := s.network.WaitForHeightWithTimeout(30, 20*time.Second)
	s.Require().NoError(err)

	testCases := []struct {
		name           string
		args           []string
		expectedOutput string
	}{
		{
			"test",
			[]string{auction_id, period_id, fmt.Sprintf("--%s=json", tmcli.OutputFlag)},
			``,
		},
	}

	for _, tc := range testCases {
		tc := tc

		s.Run(tc.name, func() {
			cmd := cli.GetCmdAuction()
			clientCtx := val.ClientCtx

			out, err := clitestutil.ExecTestCLICmd(clientCtx, cmd, tc.args)
			s.Require().NoError(err)
			s.Require().Equal(tc.expectedOutput, strings.TrimSpace(out.String()))
		})
	}
}
func (s *IntegrationTestSuite) TestGetCmdAllAuction() {
	s.SetupSuite()
	val := s.network.Validators[0]
	address := s.network.Validators[0].Address
	period_id := "1"

	// because when 30 epochs in beginblock will automatically startMewAuctionPeriod,
	// default params.AuctionEpoch=10 so we will have more than 2 AuctionPeriodId
	_, err := s.network.WaitForHeightWithTimeout(30, 20*time.Second)
	s.Require().NoError(err)

	testCases := []struct {
		name           string
		args           []string
		expectedOutput string
	}{
		{
			"test",
			[]string{address.String(), period_id, fmt.Sprintf("--%s=json", tmcli.OutputFlag)},
			``,
		},
	}

	for _, tc := range testCases {
		tc := tc

		s.Run(tc.name, func() {
			cmd := cli.GetCmdAllAuction()
			clientCtx := val.ClientCtx

			out, err := clitestutil.ExecTestCLICmd(clientCtx, cmd, tc.args)
			s.Require().NoError(err)
			s.Require().Equal(tc.expectedOutput, strings.TrimSpace(out.String()))
		})
	}
}
