package types

import sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"

func DefaultGenesis() *GenesisState {

	return &GenesisState{
		Params:         DefaultParams(),
		ActivePeriod:   nil, // Initialized in init genesis
		ActiveAuctions: []Auction{},
	}
}

// ValidateBasic validates genesis state
func (s GenesisState) ValidateBasic() error {
	if err := s.Params.ValidateBasic(); err != nil {
		return sdkerrors.Wrap(err, "invalid params")
	}
	if s.ActivePeriod != nil {
		if err := s.ActivePeriod.ValidateBasic(); err != nil {
			return sdkerrors.Wrap(err, "invalid auction period")
		}
	}
	for i, auction := range s.ActiveAuctions {
		if err := auction.ValidateBasic(); err != nil {
			return sdkerrors.Wrapf(err, "auction %d is invalid", i)
		}
	}

	return nil
}
