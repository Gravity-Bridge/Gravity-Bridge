package types

import (
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
)

const (
	ProposalUpdateAllowList = "UpdateAllowList"
)

func (p *UpdateAllowListProposal) ProposalRoute() string { return RouterKey }

func (p *UpdateAllowListProposal) ProposalType() string {
	return ProposalUpdateAllowList
}

func (p *UpdateAllowListProposal) ValidateBasic() error {
	err := govtypes.ValidateAbstract(p)
	if err != nil {
		return err
	}
	return nil
}
