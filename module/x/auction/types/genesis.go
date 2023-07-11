package types

func DefaultGenesis() *GenesisState {

	return &GenesisState{
		// this line is used by starport scaffolding # genesis/types/default
		Params: DefaultParams(),
	}
}
