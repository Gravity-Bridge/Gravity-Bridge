package types

import (
	"fmt"
	"strings"
	time "time"

	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types/v1beta1"
)

const (
	ProposalTypeUpdateHrpIbcChannel = "UpdateHrpIbcChannel"
	ProposalTypeDeleteHrpIbcChannel = "DeleteHrpIbcChannel"
)

func init() {
	govtypes.RegisterProposalType(ProposalTypeUpdateHrpIbcChannel)
	govtypes.RegisterProposalType(ProposalTypeDeleteHrpIbcChannel)
}

// nolint: exhaustruct
var _ govtypes.Content = &UpdateHrpIbcChannelProposal{}

func NewUpdateHrpIBCRecordProposal(title, description, hrp, sourceChannel string, toHeightOffset uint64, toTimeOffset time.Duration) govtypes.Content {
	return &UpdateHrpIbcChannelProposal{
		Title:             title,
		Description:       description,
		Hrp:               hrp,
		SourceChannel:     sourceChannel,
		IcsToHeightOffset: toHeightOffset,
		IcsToTimeOffset:   toTimeOffset,
	}
}

func (p *UpdateHrpIbcChannelProposal) GetTitle() string { return p.Title }

func (p *UpdateHrpIbcChannelProposal) GetDescription() string { return p.Description }

func (p *UpdateHrpIbcChannelProposal) ProposalRoute() string { return RouterKey }

func (p *UpdateHrpIbcChannelProposal) ProposalType() string {
	return ProposalTypeUpdateHrpIbcChannel
}

func (p *UpdateHrpIbcChannelProposal) ValidateBasic() error {
	err := govtypes.ValidateAbstract(p)
	if err != nil {
		return err
	}
	if strings.TrimSpace(p.SourceChannel) == "" {
		return ErrInvalidIBCData
	}
	if p.IcsToHeightOffset == 0 && p.IcsToTimeOffset == 0 {
		return ErrInvalidOffsetHeightTimeout
	}
	return ValidateHrp(p.Hrp)
}

func (p UpdateHrpIbcChannelProposal) String() string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf(`Update HRP IBC Channel Proposal:
  Title:          %s
  Description:    %s
  HRP:            %s
  Source Channel: %s
`, p.Title, p.Description, p.Hrp, p.SourceChannel))
	return b.String()
}

func NewDeleteHrpIbcChannelProposal(title, description, hrp string) govtypes.Content {
	return &DeleteHrpIbcChannelProposal{
		Title:       title,
		Description: description,
		Hrp:         hrp,
	}
}

func (p *DeleteHrpIbcChannelProposal) GetTitle() string { return p.Title }

func (p *DeleteHrpIbcChannelProposal) GetDescription() string { return p.Description }

func (p *DeleteHrpIbcChannelProposal) ProposalRoute() string { return RouterKey }

func (p *DeleteHrpIbcChannelProposal) ProposalType() string {
	return ProposalTypeDeleteHrpIbcChannel
}

func (p *DeleteHrpIbcChannelProposal) ValidateBasic() error {
	err := govtypes.ValidateAbstract(p)
	if err != nil {
		return err
	}
	return ValidateHrp(p.Hrp)
}

func (p DeleteHrpIbcChannelProposal) String() string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf(`Delete HRP IBC Channel Proposal:
  Title:          %s
  Description:    %s
  HRP:            %s
`, p.Title, p.Description, p.Hrp))
	return b.String()
}
