package apptesting

import (
	"github.com/cosmos/cosmos-sdk/baseapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
	ibctesting "github.com/cosmos/ibc-go/v4/testing"
	"github.com/cosmos/ibc-go/v4/testing/simapp"
	"github.com/stretchr/testify/suite"

	"github.com/tendermint/tendermint/crypto/ed25519"
	tmtypes "github.com/tendermint/tendermint/proto/tendermint/types"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/app"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
)

var (
	ChainID = "GRAVITY"
)

type AppTestHelper struct {
	suite.Suite

	App     *app.Gravity
	HostApp *simapp.SimApp

	IbcEnabled   bool
	Coordinator  *ibctesting.Coordinator
	GravityChain *ibctesting.TestChain
	HostChain    *ibctesting.TestChain
	TransferPath *ibctesting.Path

	QueryHelper  *baseapp.QueryServiceTestHelper
	TestAccs     []sdk.AccAddress
	IcaAddresses map[string]string
	Ctx          sdk.Context
}

// AppTestHelper Constructor
func (s *AppTestHelper) Setup() {
	s.App = app.InitGravityTestApp(true)
	// nolint: exhaustruct
	s.Ctx = s.App.BaseApp.NewContext(false, tmtypes.Header{Height: 1, ChainID: ChainID})
	s.QueryHelper = &baseapp.QueryServiceTestHelper{
		GRPCQueryRouter: s.App.GRPCQueryRouter(),
		Ctx:             s.Ctx,
	}
	s.TestAccs = CreateRandomAccounts(3)
	s.IbcEnabled = false
	s.IcaAddresses = make(map[string]string)
}

func (s *AppTestHelper) FundAccount(ctx sdk.Context, addr sdk.AccAddress, amounts sdk.Coins) {
	bankkeeper := s.App.GetBankKeeper()
	bankkeeper.MintCoins(ctx, minttypes.ModuleName, amounts) //nolint:all

	bankkeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, addr, amounts) //nolint:all
}

func (s *AppTestHelper) FundModule(ctx sdk.Context, moduleName string, amounts sdk.Coins) {
	bankkeeper := s.App.GetBankKeeper()
	bankkeeper.MintCoins(ctx, minttypes.ModuleName, amounts) //nolint:all

	bankkeeper.SendCoinsFromModuleToModule(ctx, minttypes.ModuleName, moduleName, amounts) //nolint:all
}

// Generate random account addresss
func CreateRandomAccounts(numAccts int) []sdk.AccAddress {
	testAddrs := make([]sdk.AccAddress, numAccts)
	for i := 0; i < numAccts; i++ {
		pk := ed25519.GenPrivKey().PubKey()
		testAddrs[i] = sdk.AccAddress(pk.Address())
	}

	return testAddrs
}
