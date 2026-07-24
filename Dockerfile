# syntax=docker/dockerfile:1
# Mogenius Operator Build
# Uses pre-built base images - no apt-get install or package downloads needed
#
# Cross-compilation support:
# For armv7, we use amd64 builder images with --platform=$BUILDPLATFORM
# Go cross-compiles natively, which is much faster than QEMU emulation

# =============================================================================
# Stage 1: Import pre-built tools from dedicated images
# =============================================================================

ARG GO_BUILDER_IMAGE=ghcr.io/mogenius/go-builder:latest
ARG BPFTOOL_IMAGE=ghcr.io/mogenius/bpftool:latest
# Pinned snoopy release (github.com/mogenius/snoopy). The release assets are
# built from the tagged source; the ghcr.io/mogenius/snoopy image only has
# non-human-readable date/sha tags, so we pin the release version instead.
ARG SNOOPY_VERSION=v0.4.10

# Get bpftool binary (target platform - armv7 binary for armv7 build)
FROM ${BPFTOOL_IMAGE} AS bpftool-source

# Download the snoopy binary for the target platform from the pinned GitHub
# release. Runs on the build platform (plain file download, no emulation).
FROM --platform=$BUILDPLATFORM alpine:3.24 AS snoopy-source
ARG SNOOPY_VERSION
ARG TARGETARCH
ARG TARGETVARIANT
RUN case "${TARGETARCH}${TARGETVARIANT}" in \
      amd64*)   SNOOPY_ARCH=x86_64 ;; \
      arm64*)   SNOOPY_ARCH=aarch64 ;; \
      armv7)    SNOOPY_ARCH=armv7 ;; \
      riscv64*) SNOOPY_ARCH=riscv64 ;; \
      ppc64le*) SNOOPY_ARCH=powerpc64le ;; \
      *) echo "unsupported TARGETARCH=${TARGETARCH} TARGETVARIANT=${TARGETVARIANT}" >&2; exit 1 ;; \
    esac && \
    wget -q -O /usr/local/bin/snoopy \
      "https://github.com/mogenius/snoopy/releases/download/${SNOOPY_VERSION}/snoopy_${SNOOPY_ARCH}" && \
    chmod +x /usr/local/bin/snoopy

# =============================================================================
# Stage 2: Build Environment (runs on build platform for cross-compilation)
# =============================================================================

FROM --platform=$BUILDPLATFORM ${GO_BUILDER_IMAGE} AS build-env

# Copy bpftool (from target platform image - cannot run on build platform if cross-compiling)
COPY --from=bpftool-source /usr/local/sbin/bpftool /usr/local/sbin/bpftool

# Verify tools that run on build platform
RUN go version 

WORKDIR /app

# =============================================================================
# Stage 3: Build the Operator
# =============================================================================

FROM build-env AS builder

LABEL org.opencontainers.image.description="mogenius-operator"

ENV VERIFY_CHECKSUM=false
ENV CGO_ENABLED=0

ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT

ARG COMMIT_HASH=NOT_SET
ARG GIT_BRANCH=NOT_SET
ARG BUILD_TIMESTAMP=NOT_SET
ARG VERSION=NOT_SET
ARG DEV_BUILD=no

# Download dependencies first (better layer caching)
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Generate code
RUN go generate ./...

# Build the operator binary
RUN set -e && \
    export GOOS=${TARGETOS:-linux} && \
    export GOARCH=${TARGETARCH} && \
    if [ "${TARGETARCH}" = "arm" ] && [ -n "${TARGETVARIANT}" ]; then \
        export GOARM=${TARGETVARIANT#v}; \
        echo "Cross-compiling for ARM with GOARM=${GOARM}"; \
    fi && \
    echo "=== Build Configuration ===" && \
    echo "GOOS: ${GOOS}" && \
    echo "GOARCH: ${GOARCH}" && \
    echo "GOARM: ${GOARM:-n/a}" && \
    echo "VERSION: ${VERSION}" && \
    echo "Host arch: $(uname -m)" && \
    echo "===========================" && \
    go mod tidy && \
    go build -v -trimpath \
        -gcflags='all=-l' \
        -ldflags="-s -w \
            -X mogenius-operator/src/utils.DevBuild=${DEV_BUILD} \
            -X mogenius-operator/src/version.GitCommitHash=${COMMIT_HASH} \
            -X mogenius-operator/src/version.Branch=${GIT_BRANCH} \
            -X mogenius-operator/src/version.BuildTimestamp=${BUILD_TIMESTAMP} \
            -X mogenius-operator/src/version.Ver=${VERSION}" \
        -o bin/mogenius-operator \
        ./src/main.go

# Verify binary was created
RUN ls -lh bin/ 

# =============================================================================
# Stage 4: Release Image
# =============================================================================

FROM scratch AS release-image

# CA certificates for TLS/WSS connections to platform API
COPY --from=alpine:3.24 /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

# nsenter from Alpine (links against musl) + musl dynamic linker
# The musl linker filename is arch-specific (x86_64, aarch64, armhf, ...)
COPY --from=alpine:3.24 /usr/bin/nsenter /usr/local/bin/nsenter
COPY --from=alpine:3.24 /lib/ld-musl-*.so.1 /lib/

# Operator binary (CGO_ENABLED=0, statically linked)
COPY --from=builder /app/bin/mogenius-operator /usr/local/bin/mogenius-operator

# Snoopy binary (Rust + musl target, statically linked)
COPY --from=snoopy-source /usr/local/bin/snoopy /usr/local/bin/mogenius-snoopy

WORKDIR /app

ENV PATH=/usr/local/bin
ENV MO_LOG_LEVEL="warn"

ENTRYPOINT ["/usr/local/bin/mogenius-operator"]
CMD ["cluster"]
