package types

import (
	errorsmod "cosmossdk.io/errors"
)

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
		return errorsmod.Wrap(err, "invalid params")
	}
	if s.ActivePeriod != nil {
		if err := s.ActivePeriod.ValidateBasic(); err != nil {
			return errorsmod.Wrap(err, "invalid auction period")
		}
	}
	for i, auction := range s.ActiveAuctions {
		if err := auction.ValidateBasic(); err != nil {
			return errorsmod.Wrapf(err, "auction %d is invalid", i)
		}
	}

	return nil
}
