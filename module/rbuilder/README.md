# Reproducible Build System

This image is meant to provide a minimal deterministic
buildsystem for Cosmos SDK applications.

# Requirements And Usage

The client application's repository must include an
`.build.sh` executable file in the root folder meant to drive the build
process. The following environment variables are passed through
and made available to the `.build.sh` script:

* `APP` - the application's name.
* `VERSION` - the application's version.
* `COMMIT` - the application's VCS commit's hash.
* `TARGET_PLATFORMS` - whitespace-separated list of operating system/architecture pairs. `linux/amd64`, `linux/arm64`, `darwin/amd64`, and `windows/amd64` are supported. Default: `linux/amd64`.
* `LEDGER_ENABLED` - whether Ledger is enabled (default: `true`).
* `DEBUG` - run build with debug output. Default: empty (disabled).
* `BUILD_SCRIPT` - path to the build script. Default: `.build.sh`

The build's outputs are produced in the top-level `artifacts` directory. An example of `.build.sh` follows:

```bash
#!/bin/bash

set -ue

# Expect the following envvars to be set:
# - APP
# - VERSION
# - COMMIT
# - TARGET_OS
# - LEDGER_ENABLED
# - DEBUG

# Source builder's functions library
. /usr/local/share/tendermint/buildlib.sh

# These variables are now available
# - BASEDIR
# - OUTDIR

# Build for each os-architecture pair
for platform in ${TARGET_PLATFORMS} ; do
    # This function sets GOOS, GOARCH, and OS_FILE_EXT environment variables
    # according to the build target platform. OS_FILE_EXT is empty in all
    # cases except when the target platform is 'windows'.
    setup_build_env_for_platform "${platform}"

    make clean
    echo Building for $(go env GOOS)/$(go env GOARCH) >&2
    GOROOT_FINAL="$(go env GOROOT)" \
    make build \
        LDFLAGS=-buildid=${VERSION} \
        VERSION=${VERSION} \
        COMMIT=${COMMIT} \
        LEDGER_ENABLED=${LEDGER_ENABLED}
    mv ./build/${APP}${OS_FILE_EXT} ${OUTDIR}/${APP}-${VERSION}-$(go env GOOS)-$(go env GOARCH)${OS_FILE_EXT}

    # This function restore the build environment variables to their
    # original state.
    restore_build_env
done

# Generate and display build report.
generate_build_report
cat ${OUTDIR}/build_report
```

# Makefile integration

An example of integration with the client application's `Makefile` follows:

```Makefile
VERSION := $(shell echo $(shell git describe --always) | sed 's/^v//')
COMMIT := $(shell git log -1 --format='%H')
LEDGER_ENABLED ?= true

build-simd-all: go.sum
 docker pull tendermintdev/rbuilder:latest
 docker rm latest-build || true
 docker run --volume=$(CURDIR):/sources:ro \
        --env TARGET_PLATFORMS='linux/amd64 darwin/amd64 linux/arm64 windows/amd64'
        --env APP=simd \
        --env VERSION=$(VERSION) \
        --env COMMIT=$(COMMIT) \
        --env LEDGER_ENABLED=$(LEDGER_ENABLED) \
        --name latest-build tendermintdev/rbuilder:latest
 docker cp -a latest-build:/home/builder/artifacts/ $(CURDIR)/
```
