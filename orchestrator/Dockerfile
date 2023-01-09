FROM rust:1.66.0

RUN apt update && apt install curl nano -y

WORKDIR /workspace
COPY . /workspace/

RUN rustup default stable
RUN rustup update && rustup component add clippy rustfmt

RUN cargo install sccache
# Use sccache. Users can override this variable to disable caching. https://doc.rust-lang.org/cargo/guide/build-cache.html#shared-cache
ENV RUSTC_WRAPPER=sccache

RUN cargo build --all
CMD [ "cargo run -p gbt -V" ]