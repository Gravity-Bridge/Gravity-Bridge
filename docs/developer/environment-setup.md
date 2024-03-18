# Getting started

Welcome! This guide covers how to get your development machine setup to contribute to Gravity Bridge, as well as the basics of how the code is laid out.

If you find anything in this guide that is confusing or does not work, please open an issue or [chat with us](https://discord.gg/d3DshmHpXA). You can also find resources specific to the main Gravity Bridge instance (such as testnet RPC) in the [Gravity Docs](https://github.com/gravity-bridge/gravity-docs)

We're always happy to help new developers get started

## Language dependencies

Gravity bridge has three major components

[The Gravity bridge Solidity](https://github.com/Gravity-Bridge/Gravity-Bridge/tree/main/solidity) and associated tooling. This requires NodeJs
[The Gravity bridge Cosmos Module and test chain](https://github.com/Gravity-Bridge/Gravity-Bridge/tree/main/module). this requires Go.
[The Gravity bridge tools](https://github.com/Gravity-Bridge/Gravity-Bridge/tree/main/orchestrator) these require Rust.

### Installing Go

Follow the official guide [here](https://golang.org/doc/install)

Make sure that the go/bin directory is in your path by adding this to your shell profile (~/.bashrc or ~/.zprofile)

```
export PATH=$PATH:$(go env GOPATH)/bin
```

### Installing NodeJS

Follow the official guide [here](https://nodejs.org/en/)

### Installing Rust

Use the official toolchain installer [here](https://rustup.rs/)

### Alternate installation

If you are a linux user and prefer your package manager to manually installed dev dependencies you can try these.

**Fedora**
`sudo dnf install golang rust cargo npm -y`

**Ubuntu**
` audo apt-get update && sudo apt-get install golang rust cargo npm -y`

## Getting everything built

At this step download the repo

```
git clone https://github.com/Gravity-Bridge/Gravity-Bridge/
```

### Solidity

Change directory into the `Gravity-Bridge/solidity` folder and run

```
# Install JavaScript dependencies
HUSKY_SKIP_INSTALL=1 npm install

# Build the Gravity bridge Solidity contract, run this after making any changes
npm run typechain
```

You should also try running the tests

```
# run the Hardhat Ethereum testing chain
npm run evm
```

In another terminal

```
# actually run the tests, connecting to the running EVM in your other terminal
npm run test
```

### Go

Change directory into the `Gravity-Bridge/module` folder and run

```bash
# Installing the protobuf tooling
sudo make proto-tools

# Install protobufs plugins

go install github.com/regen-network/cosmos-proto/protoc-gen-gocosmos
go get github.com/regen-network/cosmos-proto/protoc-gen-gocosmos
go install github.com/grpc-ecosystem/grpc-gateway/protoc-gen-grpc-gateway@v1.16.0
go get github.com/grpc-ecosystem/grpc-gateway/protoc-gen-grpc-gateway@v1.16.0
```

For MacOS, we need to install buf as well

```bash
brew install bufbuild/buf/buf
```

```bash
# generate new protobuf files from the definitions, this makes sure the previous instructions worked
# you will need to run this any time you change a proto file
make proto-gen

# build all code, including your newly generated go protobuf file
make

# run all the unit tests
make test
```

#### Dependency Errors

'''
go: downloading github.com/regen-network/protobuf v1.3.3-alpha.regen.1
../../../go/pkg/mod/github.com/tendermint/tendermint@v0.34.13/abci/types/types.pb.go:9:2: reading github.com/regen-network/protobuf/go.mod at revision v1.3.3-alpha.regen.1: unknown revision v1.3.3-alpha.regen.1
../../../go/pkg/mod/github.com/cosmos/cosmos-sdk@v0.44.2/types/tx/service.pb.go:12:2: reading github.com/regen-network/protobuf/go.mod at revision v1.3.3-alpha.regen.1: unknown revision v1.3.3-alpha.regen.1

```

If you see dependency errors like this, clean your cache and build again

```

go clean -modcache
make

```

### Rust

Change directory into the `Gravity-Bridge/orchestrator` folder and run

```

# build all crates

cargo build --all

# re-generate Rust protobuf code

# you will need to do this every time you edit a proto file

cd proto-build && cargo run

```

### Tips for IDEs

- We strongly recommend installing [Rust Analyzer](https://rust-analyzer.github.io/) in your IDE.
- Launch VS Code in /solidity with the solidity extension enabled to get inline typechecking of the solidity contract
- Launch VS Code in /module/app with the go extension enabled to get inline typechecking of the dummy cosmos chain

## Running the integration tests

We provide a one button integration test that deploys a full arbitrary validator Cosmos chain and testnet Geth chain for both development + validation.
We believe having a in depth test environment reflecting the full deployment and production-like use of the code is essential to productive development.

Currently on every commit we send hundreds of transactions, dozens of validator set updates, and several transaction batches in our test environment.
This provides a high level of quality assurance for the Gravity bridge.

Because the tests build absolutely everything in this repository they do take a significant amount of time to run.
You may wish to simply push to a branch and have Github CI take care of the actual running of the tests.

### Running the integration test environment locally

The integration tests have two methods of operation, one that runs one of a pre-defined series of tests, another that produces a running local instance
of Gravity bridge for you as a developer to interact with. This is very useful for iterating quickly on changes.

```

# builds the original docker container, only have to run this once

./tests/build-container.sh

# This starts the Ethereum chain, Cosmos chain,

```
./tests/start-chains.sh
```

switch to a new terminal and run a test case. A list of all predefined tests can be found [here](https://github.com/Gravity-Bridge/Gravity-Bridge/blob/main/orchestrator/test_runner/src/main.rs#L169)

These test cases spawn Orchestrators as well as an IBC relayer, run through their test scenario and then exit. If you just want to have a fully functioning instance of GB running locally you can use this command.

```
./tests/run-tests.sh RUN_ORCH_ONLY
```

This will just kick off the orchestrators and ibc relayer and not run any particular test case.

# This runs a pre-defined test against the chains, keeping state between runs

./tests/run-tests.sh

# This provides shell access to the running testnet

# RPC endpoints are passed through the container to localhost:8545 (ethereum) and localhost:9090 (Cosmos GRPC)

docker exec -it gravity_test_instance /bin/bash

```

### Notes for Mac users

Due to a bug in Geth's mining feature it will typically eat up all CPU cores when running in a Mac VM, please set the environmental variable `HARDHAT` in order to use the lower CPU power Hardhat backend
hardhat can not execute tests that depend on transaction queues so keep in mind this isn't a perfect solution.

```

export HARDHAT=true
./tests/start-chains.sh

```

**Debugging**

To use a stepping debugger in VS Code, follow the "Working inside the container" instructions above, but set up a one node testnet using
`./tests/reload-code.sh 1`. Now kill the node with `pkill gravityd`. Start the debugger from within VS Code, and you will have a 1 node debuggable testnet.

### Running all up tests

All up tests are pre-defined test patterns that are run 'all up' which means including re-building all dependencies and deploying a fresh testnet for each test.
These tests _only_ work on checked in code. You must commit your latest changes to git.

A list of test patterns is defined [here](https://github.com/Gravity-Bridge/Gravity-Bridge/blob/main/orchestrator/test_runner/src/main.rs#L169)

To run an individual test run

```

bash tests/all-up-test.sh TEST_NAME

```

To run all the integration tests and check your code completely run

```

bash tests/run-all-up-tests.sh

```

This will run every available all up test. This will take quite some time, go get coffee and if your development machine is
particularly slow I recommend just pushing to Github. Average runtime per all up test on a modern linux machine is ~5 minutes each.

You can also use

```

bash tests/run-all-tests.sh

```

This is essentially a local emulation of the Github tests. Including linting and formatting plus the above all up test script.

## Next steps

Now that you are ready to edit, build, and test Gravity Bridge code you can view the [code structure intro](/docs/developer/code-structure.md)
```
