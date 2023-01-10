FROM rust:1.66.0

RUN apt update && apt install curl nano -y

WORKDIR /workspace
COPY . /workspace/

RUN rustup default stable

RUN cargo build --all
CMD [ "cargo run -p gbt -V" ]