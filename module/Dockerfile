FROM golang:1.18-alpine3.17 as builder
WORKDIR /workspace
COPY . /workspace
# this comes from standard alpine nightly file
#  https://github.com/rust-lang/docker-rust-nightly/blob/master/alpine3.12/Dockerfile
# with some changes to support our toolchain, etc
RUN set -eux; apk add --no-cache ca-certificates build-base;
RUN apk add upx
# NOTE: add these to run with LEDGER_ENABLED=true
# RUN apk add libusb-dev linux-headers
RUN make build LEDGER_ENABLED=false BUILD_TAGS=muslc
RUN upx --best --lzma /workspace/build/gravity

FROM alpine:3.17 
COPY --from=builder /workspace/build/gravity /usr/bin/gravity