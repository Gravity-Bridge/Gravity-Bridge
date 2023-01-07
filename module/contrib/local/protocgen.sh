#!/usr/bin/env bash

set -eo pipefail

GOPATH=${GOPATH:-$(go env GOPATH)}
if [ -z $GOPATH ]; then
	echo "GOPATH not set!"
	exit 1
fi

if [[ $PATH != *"$GOPATH/bin"* ]]; then
	echo "GOPATH/bin must be added to PATH"
	exit 1
fi

COSMOS_SDK_DIR=${COSMOS_SDK_DIR:-$(go list -f "{{ .Dir }}" -m github.com/cosmos/cosmos-sdk)}

proto_dirs=$(find ./proto -path -prune -o -name '*.proto' -print0 | xargs -0 -n1 dirname | sort | uniq)
for dir in $proto_dirs; do
  buf alpha protoc \
  -I "proto" \
  -I="$COSMOS_SDK_DIR/third_party/proto" \
  -I="$COSMOS_SDK_DIR/proto" \
  --gocosmos_out=plugins=interfacetype+grpc,\
Mgoogle/protobuf/any.proto=github.com/cosmos/cosmos-sdk/codec/types:. \
  $(find "${dir}" -maxdepth 1 -name '*.proto')

  # # command to generate gRPC gateway (*.pb.gw.go in respective modules) files
  buf alpha protoc \
  -I "proto" \
  -I="$COSMOS_SDK_DIR/third_party/proto" \
  -I="$COSMOS_SDK_DIR/proto" \
  --grpc-gateway_out=logtostderr=true,Mgoogle/protobuf/any.proto=github.com/cosmos/cosmos-sdk/codec/types:. \
  $(find "${dir}" -maxdepth 1 -name '*.proto')

done

# move proto files to the right places
cp -r github.com/Gravity-Bridge/Gravity-Bridge/module/* ./
rm -rf github.com
