package gravity

import (
	autocliv1 "cosmossdk.io/api/cosmos/autocli/v1"
	api "github.com/Gravity-Bridge/Gravity-Bridge/module/api/gravity/v1"
)

func (am AppModule) AutoCLIOptions() *autocliv1.ModuleOptions {
	// nolint: exhaustruct
	return &autocliv1.ModuleOptions{
		// nolint: exhaustruct
		Query: &autocliv1.ServiceCommandDescriptor{
			Service: api.Query_ServiceDesc.ServiceName,
		},
		// nolint: exhaustruct
		Tx: &autocliv1.ServiceCommandDescriptor{
			Service: api.Msg_ServiceDesc.ServiceName,
		},
	}
}
