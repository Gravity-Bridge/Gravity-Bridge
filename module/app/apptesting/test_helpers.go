package apptesting

import (
	"github.com/stretchr/testify/suite"

	"github.com/cosmos/cosmos-sdk/baseapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/ibc-go/v6/testing/simapp"

	"github.com/tendermint/tendermint/crypto/ed25519"
	tmtypes "github.com/tendermint/tendermint/proto/tendermint/types"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/app"
)

var (
	ChainID = "gravity-test-1"
)

type AppTestHelper struct {
	suite.Suite

	App     *app.Gravity
	HostApp *simapp.SimApp

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

// Returns 1 * 10^6 as an int64
func OneAtom() int64 {
	return 1_000000
}
