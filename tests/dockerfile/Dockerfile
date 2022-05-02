FROM fedora:35
ENV GOPATH=/go
ENV PATH=$PATH:/go/bin
RUN dnf install -y git make gcc gcc-c++ which iproute iputils procps-ng vim-minimal tmux net-tools htop tar jq npm openssl-devel perl rust cargo golang wget
# only required for deployment script
RUN npm install -g ts-node && npm install -g typescript
ADD https://gethstore.blob.core.windows.net/builds/geth-linux-amd64-1.10.10-bb74230f.tar.gz /geth/
RUN cd /geth && tar -xvf * && mv /geth/**/geth /usr/bin/geth
# Download gaia as a IBC test chain
ADD https://github.com/cosmos/gaia/releases/download/v7.0.0/gaiad-v7.0.0-linux-amd64 /usr/bin/gaiad
# Setup Hermes for IBC connections between chains
ADD https://github.com/informalsystems/ibc-rs/releases/download/v0.13.0/hermes-v0.13.0-x86_64-unknown-linux-gnu.tar.gz /tmp/
RUN cd /tmp/ && tar -xvf hermes-v0.13.0-x86_64-unknown-linux-gnu.tar.gz && mv hermes /usr/bin/
RUN mkdir /ibc-relayer-logs && touch /ibc-relayer-logs/hermes-logs && touch /ibc-relayer-logs/channel-creation
# the actual source code for this repo, this **only** includes checked in files!
# this is a bit of a pain but it does speed things up a lot
ADD gravity.tar.gz /
# build the test runner specifically to cache a release artifact
RUN pushd /gravity/orchestrator/test_runner && PATH=$PATH:$HOME/.cargo/bin cargo build --bin test-runner --release
RUN pushd /gravity/module/ && PATH=$PATH:/usr/local/go/bin GOPROXY=https://proxy.golang.org make && PATH=$PATH:/usr/local/go/bin make install
RUN pushd /gravity/solidity/ && HUSKY_SKIP_INSTALL=1 npm install && npm run typechain
