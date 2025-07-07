package gravity

import (
	autocliv1 "cosmossdk.io/api/cosmos/autocli/v1"
	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
)

func (am AppModule) AutoCLIOptions() *autocliv1.ModuleOptions {
	return &autocliv1.ModuleOptions{
		Query: &autocliv1.ServiceCommandDescriptor{
			Service: types.Query_serviceDesc.ServiceName,
		},
		Tx: &autocliv1.ServiceCommandDescriptor{
			Service: types.Msg_serviceDesc.ServiceName,
		},
	}
}
