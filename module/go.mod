module github.com/Gravity-Bridge/Gravity-Bridge/module

go 1.16

require (
	github.com/cosmos/cosmos-sdk v0.44.6
	github.com/cosmos/ibc-go/v2 v2.1.0
	github.com/ethereum/go-ethereum v1.10.11
	github.com/gogo/protobuf v1.3.3
	github.com/golang/protobuf v1.5.2
	github.com/gorilla/mux v1.8.0
	github.com/grpc-ecosystem/grpc-gateway v1.16.0
	github.com/osmosis-labs/bech32-ibc v0.2.0-rc2
	github.com/pkg/errors v0.9.1
	github.com/rakyll/statik v0.1.7
	github.com/regen-network/cosmos-proto v0.3.1
	github.com/spf13/cast v1.4.1
	github.com/spf13/cobra v1.2.1
	github.com/spf13/viper v1.9.0
	github.com/stretchr/testify v1.7.0
	github.com/tendermint/tendermint v0.34.14
	github.com/tendermint/tm-db v0.6.4
	github.com/tharsis/ethermint v0.9.0
	google.golang.org/genproto v0.0.0-20211116182654-e63d96a377c4
	google.golang.org/grpc v1.42.0
)

replace github.com/gogo/protobuf => github.com/regen-network/protobuf v1.3.3-alpha.regen.1

replace github.com/gogo/grpc => google.golang.org/grpc v1.33.2

replace google.golang.org/grpc => google.golang.org/grpc v1.33.2
