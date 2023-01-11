#!/usr/bin/env bash

set -eo pipefail

# go get ./...
# apk add nodejs-current 
# npm install -g ts-proto

BASEDIR=$(dirname $0)
PROJECTDIR=$BASEDIR/../..
# default is tmp folder
SOURCEDIR=$(realpath ${1:-$PROJECTDIR/tmp})

COSMOS_SDK_DIR=${COSMOS_SDK_DIR:-$(go list -f "{{ .Dir }}" -m github.com/cosmos/cosmos-sdk)}
IBC_DIR=${IBC_DIR:-$(go list -f "{{ .Dir }}" -m github.com/cosmos/ibc-go/v3)}

# scan all folders that contain proto file
proto_dirs=$(find $PROJECTDIR/proto -path -prune -o -name '*.proto' -print0 | xargs -0 -n1 dirname | sort | uniq)
proto_files=()

for dir in $proto_dirs; do
  proto_files=("${proto_files[@]} $(find "${dir}" -maxdepth 1 -name '*.proto')")
done

# create dir & file if it does not exist
rm -rf $SOURCEDIR/ts
mkdir -p $SOURCEDIR/ts  
NPM_PATH=$(npm config get prefix)

buf alpha protoc \
    --plugin=$NPM_PATH/bin/protoc-gen-ts_proto \
    --proto_path="$PROJECTDIR/proto" \
    --proto_path="$IBC_DIR/proto" \
    --proto_path="$COSMOS_SDK_DIR/third_party/proto" \
    --proto_path="$COSMOS_SDK_DIR/proto" \
    --ts_proto_opt=forceLong=string \
    --ts_proto_opt=esModuleInterop=true \
    --ts_proto_opt=outputClientImpl=false \
    --ts_proto_out=$SOURCEDIR/ts \
    ${proto_files[@]}