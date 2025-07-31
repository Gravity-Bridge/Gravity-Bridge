package gravity

import (
	autocliv1 "cosmossdk.io/api/cosmos/autocli/v1"
	api "github.com/Gravity-Bridge/Gravity-Bridge/module/api/gravity/v1"
)

func (am AppModule) AutoCLIOptions() *autocliv1.ModuleOptions {
	return &autocliv1.ModuleOptions{
		Query: &autocliv1.ServiceCommandDescriptor{
			Service: api.Query_ServiceDesc.ServiceName,
		},
		Tx: &autocliv1.ServiceCommandDescriptor{
			Service: api.Msg_ServiceDesc.ServiceName,
		},
	}
}
