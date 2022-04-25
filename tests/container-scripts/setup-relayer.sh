#!/bin/bash
set -eux
HERMES_VERSION="v0.13.0"
HERMES_ARCHIVE_NAME="hermes-$HERMES_VERSION-x86_64-unknown-linux-gnu.tar.gz"

HERMES_URL="https://github.com/informalsystems/ibc-rs/releases/download/$HERMES_VERSION/$HERMES_ARCHIVE_NAME"
echo "Downloading hermes from $HERMES_URL"
wget $HERMES_URL
tar -C /usr/bin/ -vxzf $HERMES_ARCHIVE_NAME

mkdir /ibc-relayer-logs
touch /ibc-relayer-logs/hermes-logs
touch /ibc-relayer-logs/channel-creation