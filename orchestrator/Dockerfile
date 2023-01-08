FROM rust:alpine3.15

RUN apk add alpine-sdk perl upx

WORKDIR /workspace
COPY . /workspace/

RUN cargo build --all