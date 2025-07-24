# this script is for generating protobuf files for the new google.golang.org/protobuf API
set -eo pipefail

echo "Cleaning API directory"
(cd api; find ./ -type f \( -iname \*.pulsar.go -o -iname \*.pb.go -o -iname \*.cosmos_orm.go -o -iname \*.pb.gw.go \) -delete; find . -empty -type d -delete; cd ..)

echo "Generating API module"
(cd proto; buf generate --template buf.gen.pulsar.yaml; cd ../ ;)
echo "Cleaning up bad import statements in gravity files"
(find ./api -type f -name '*.go' -exec sed -i 's|\("cosmossdk\.io/api/gravity\)\([^"]*\)"|"github.com/Gravity-Bridge/Gravity-Bridge/module/api/gravity\2"|g' {} +)

