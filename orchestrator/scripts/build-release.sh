#!/bin/bash
set -eux
cross build --target=x86_64-unknown-linux-musl --release  --bin gbt
cross build --target=aarch64-unknown-linux-musl --release  --bin gbt

mkdir -p bins

cp target/x86_64-unknown-linux-musl/release/gbt bins/

cp target/aarch64-unknown-linux-musl/release/gbt bins/gbt-arm
