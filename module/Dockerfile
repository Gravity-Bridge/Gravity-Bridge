FROM golang:1.18-alpine as builder

# this comes from standard alpine nightly file
#  https://github.com/rust-lang/docker-rust-nightly/blob/master/alpine3.12/Dockerfile
# with some changes to support our toolchain, etc
RUN set -eux; apk add --no-cache ca-certificates build-base;
RUN apk add upx bash jq
# NOTE: add these to run with LEDGER_ENABLED=true
# RUN apk add libusb-dev linux-headers

WORKDIR /workspace
COPY . /workspace/

# # force it to use static lib (from above) not standard libgo_cosmwasm.so file
RUN make install LEDGER_ENABLED=false BUILD_TAGS=muslc