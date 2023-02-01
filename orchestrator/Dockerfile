FROM rust:1.66.0-alpine3.17 as builder
WORKDIR /workspace
COPY . /workspace
# musl-dev to include h files, and make + perl to build openssl-sys
RUN apk add musl-dev make perl upx
RUN rustup default stable
RUN cargo build -p gbt --release
RUN upx --best --lzma /workspace/target/release/gbt

FROM alpine:3.17 
COPY --from=builder /workspace/target/release/gbt /usr/bin/gbt