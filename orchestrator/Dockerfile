FROM rust:1.66.0-alpine3.17 as builder
WORKDIR /workspace
COPY . /workspace/
RUN apk add upx
RUN rustup default stable
RUN cargo build --all
RUN upx --best --lzma /workspace/target/release/gbt

FROM alpine:3.17 
COPY --from=builder /workspace/target/release/gbt /usr/bin/gbt
CMD [ "cargo run -p gbt -V" ]