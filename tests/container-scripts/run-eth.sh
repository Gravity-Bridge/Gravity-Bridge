#!/bin/bash
# Starts the Ethereum testnet chain in the background

# We run geth in "developer" mode (--dev)
#
# --dev.period 1 produces one block per second. A nonzero period is required
# (instead of the default "seal only when transactions are pending") because some
# tests, e.g. BATCH_TIMEOUT, rely on the Ethereum block height advancing on its
# own over time.
#
# Developer mode uses chain id 1337. The orchestrator already treats 1337 as a
# single-signer, no-reorg chain (see get_latest_safe_block in the orchestrator),
# so no Ethereum confirmation delay is applied.

# geth --dev discards a datadir that only exists as an ephemeral temp dir, so we
# pass an explicit --datadir and import the signer key into that same datadir.
# When the keystore already contains a key, --dev adopts it as the pre-funded,
# auto-unlocked developer/sealer account instead of generating a random one. This
# keeps the well-known miner address funded, which matters because the test-runner
# sends every Ethereum transaction from it.
# private key for 0xBf660843528035a5A4921534E156a27e64B231fE
GETH_DATADIR="${GETH_DATADIR:-$HOME/.ethereum}"
SIGNER_KEY_FILE=$(mktemp)
SIGNER_PASSWORD_FILE=$(mktemp)
echo "b1bab011e03a9862664706fc3bbaa1b16651528e5f0e7fbfcbfdd8be302a13e7" > "$SIGNER_KEY_FILE"
# empty password; this is a throwaway test key
: > "$SIGNER_PASSWORD_FILE"
# `|| true` so re-runs (key already imported) don't abort the script
geth --datadir "$GETH_DATADIR" account import --password "$SIGNER_PASSWORD_FILE" "$SIGNER_KEY_FILE" || true
rm -f "$SIGNER_KEY_FILE"

geth --identity "GravityTestnet" \
--datadir "$GETH_DATADIR" \
--dev \
--dev.period 1 \
--password "$SIGNER_PASSWORD_FILE" \
--http \
--http.addr="0.0.0.0" \
--http.api="eth,net,web3,txpool" \
--http.vhosts="*" \
--http.corsdomain="*" \
--nousb \
--verbosity=3 &> /geth.log
