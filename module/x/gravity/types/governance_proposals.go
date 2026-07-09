package types

import (
	fmt "fmt"
	"strings"

	sdk "github.com/cosmos/cosmos-sdk/types"
	govv1beta1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1beta1"
)

const (
	ProposalTypeUnhaltBridge           = "UnhaltBridge"
	ProposalTypeAirdrop                = "Airdrop"
	ProposalTypeCosmosBridgeableTokens = "CosmosBridgeableTokens"
)

func (p *UnhaltBridgeProposal) GetTitle() string { return p.Title }

func (p *UnhaltBridgeProposal) GetDescription() string { return p.Description }

func (p *UnhaltBridgeProposal) ProposalRoute() string { return RouterKey }

func (p *UnhaltBridgeProposal) ProposalType() string {
	return ProposalTypeUnhaltBridge
}

func (p *UnhaltBridgeProposal) ValidateBasic() error {
	err := govv1beta1.ValidateAbstract(p)
	if err != nil {
		return err
	}
	return nil
}

func (p UnhaltBridgeProposal) String() string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf(`Unhalt Bridge Proposal:
  Title:          %s
  Description:    %s
  target_nonce:   %d
`, p.Title, p.Description, p.TargetNonce))
	return b.String()
}

func (p *AirdropProposal) GetTitle() string { return p.Title }

func (p *AirdropProposal) GetDescription() string { return p.Description }

func (p *AirdropProposal) ProposalRoute() string { return RouterKey }

func (p *AirdropProposal) ProposalType() string {
	return ProposalTypeAirdrop
}

func (p *AirdropProposal) ValidateBasic() error {
	err := govv1beta1.ValidateAbstract(p)
	if err != nil {
		return err
	}
	return nil
}

func (p AirdropProposal) String() string {
	var b strings.Builder
	total := uint64(0)
	for _, v := range p.Amounts {
		total += v
	}
	parsedRecipients := make([]sdk.AccAddress, len(p.Recipients)/20)
	for i := 0; i < len(p.Recipients)/20; i++ {
		indexStart := i * 20
		indexEnd := indexStart + 20
		addr := p.Recipients[indexStart:indexEnd]
		parsedRecipients[i] = addr
	}
	recipients := ""
	for i, a := range parsedRecipients {
		recipients += fmt.Sprintf("Account: %s Amount: %d%s", a.String(), p.Amounts[i], p.Denom)
	}

	b.WriteString(fmt.Sprintf(`Airdrop Proposal:
  Title:          %s
  Description:    %s
  Total Amount:   %d%s
  Recipients:     %s
`, p.Title, p.Description, total, p.Denom, recipients))
	return b.String()
}

func (p *SetCosmosBridgeableTokensProposal) GetTitle() string { return p.Title }

func (p *SetCosmosBridgeableTokensProposal) GetDescription() string { return p.Description }

func (p *SetCosmosBridgeableTokensProposal) ProposalRoute() string { return RouterKey }

func (p *SetCosmosBridgeableTokensProposal) ProposalType() string {
	return ProposalTypeCosmosBridgeableTokens
}

func (p *SetCosmosBridgeableTokensProposal) ValidateBasic() error {
	if err := govv1beta1.ValidateAbstract(p); err != nil {
		return err
	}
	if len(p.Metadatas) == 0 {
		return fmt.Errorf("SetCosmosBridgeableTokensProposal must contain at least one metadata entry")
	}
	seen := make(map[string]struct{}, len(p.Metadatas))
	for _, m := range p.Metadatas {
		if err := m.Validate(); err != nil {
			return fmt.Errorf("invalid metadata for denom %s: %w", m.Base, err)
		}
		if _, dup := seen[m.Base]; dup {
			return fmt.Errorf("duplicate base denom in SetCosmosBridgeableTokensProposal: %s", m.Base)
		}
		seen[m.Base] = struct{}{}
	}
	return nil
}

func (p SetCosmosBridgeableTokensProposal) String() string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf(`Set Cosmos Bridgeable Tokens Proposal:
  Title:          %s
  Description:    %s
`, p.Title, p.Description))
	for _, m := range p.Metadatas {
		b.WriteString(fmt.Sprintf("  Denom: %s Name: %s Symbol: %s\n", m.Base, m.Name, m.Symbol))
	}
	return b.String()
}

func (p *DeleteCosmosBridgeableTokensProposal) GetTitle() string { return p.Title }

func (p *DeleteCosmosBridgeableTokensProposal) GetDescription() string { return p.Description }

func (p *DeleteCosmosBridgeableTokensProposal) ProposalRoute() string { return RouterKey }

func (p *DeleteCosmosBridgeableTokensProposal) ProposalType() string {
	return ProposalTypeCosmosBridgeableTokens
}

func (p *DeleteCosmosBridgeableTokensProposal) ValidateBasic() error {
	if err := govv1beta1.ValidateAbstract(p); err != nil {
		return err
	}
	if len(p.Metadatas) == 0 {
		return fmt.Errorf("DeleteCosmosBridgeableTokensProposal must contain at least one metadata entry")
	}
	seen := make(map[string]struct{}, len(p.Metadatas))
	for _, m := range p.Metadatas {
		if err := m.Validate(); err != nil {
			return fmt.Errorf("invalid metadata for denom %s: %w", m.Base, err)
		}
		if _, dup := seen[m.Base]; dup {
			return fmt.Errorf("duplicate base denom in DeleteCosmosBridgeableTokensProposal: %s", m.Base)
		}
		seen[m.Base] = struct{}{}
	}
	return nil
}

func (p DeleteCosmosBridgeableTokensProposal) String() string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf(`Delete Cosmos Bridgeable Tokens Proposal:
  Title:          %s
  Description:    %s
`, p.Title, p.Description))
	for _, m := range p.Metadatas {
		b.WriteString(fmt.Sprintf("  Denom: %s Name: %s Symbol: %s\n", m.Base, m.Name, m.Symbol))
	}
	return b.String()
}
