package types

import sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"

func DefaultGenesis() *GenesisState {

	return &GenesisState{
		// this line is used by starport scaffolding # genesis/types/default
		Params: DefaultParams(),
	}
}

// ValidateBasic validates genesis state by looping through the params and
// calling their validation functions
func (s GenesisState) ValidateBasic() error {
	if err := s.Params.Validate(); err != nil {
		return sdkerrors.Wrap(err, "params")
	}
	return nil
}
