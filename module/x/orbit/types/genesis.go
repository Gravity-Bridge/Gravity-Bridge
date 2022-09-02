package types

import (
	paramtypes "github.com/cosmos/cosmos-sdk/x/params/types"
)

// ParamKeyTable for auth module
func ParamKeyTable() paramtypes.KeyTable {
	return paramtypes.NewKeyTable().RegisterParamSet(&Params{})
}

func (p *Params) ParamSetPairs() paramtypes.ParamSetPairs {
	return paramtypes.ParamSetPairs{}
}

func (p Params) ValidateBasic() error { return nil }

func DefaultGenesisState() *GenesisState {
	return &GenesisState{Params: &Params{}}
}
