# Syntax für BuildKit features
# syntax=docker/dockerfile:1

# Build-Stage sollte native Platform verwenden für schnelleres Kompilieren
FROM --platform=$BUILDPLATFORM golang:1.25.4 AS golang

FROM --platform=$BUILDPLATFORM ubuntu:noble AS build-env

ENV SNOOPY_VERSION=v0.3.6

COPY --from=golang /usr/local/go /usr/local/go

RUN mkdir /go
ENV GOPATH=/go
ENV PATH=${GOPATH}/bin:/usr/local/go/bin:${PATH}

# Build-time argument for GitHub Token
ARG GITHUB_TOKEN
ARG TARGETPLATFORM
ARG BUILDPLATFORM
ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT

# Setup system - OHNE linux-headers-generic und just (installieren wir später)
RUN set -x && \
    apt-get update && \
    apt-get install -y \
        curl jq clang llvm libelf-dev libbpf-dev git \
        gcc libc6-dev make cmake libpcap-dev binutils \
        build-essential binutils-gold iproute2 lsb-release \
        sudo ca-certificates wget libssl-dev && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

# Linux headers nur für amd64 (für eBPF development)
RUN if [ "$TARGETARCH" = "amd64" ]; then \
        apt-get update && \
        apt-get install -y linux-headers-generic && \
        apt-get clean && \
        rm -rf /var/lib/apt/lists/*; \
    fi

# Just manuell für die richtige Architektur installieren
RUN case ${TARGETARCH} in \
        "amd64")  JUST_ARCH="x86_64-unknown-linux-musl"  ;; \
        "arm64")  JUST_ARCH="aarch64-unknown-linux-musl" ;; \
        "arm")    JUST_ARCH="arm-unknown-linux-musleabihf" ;; \
        *) echo "Unsupported arch for just: $TARGETARCH"; exit 1;; \
    esac && \
    wget -qO- "https://github.com/casey/just/releases/latest/download/just-${JUST_ARCH}.tar.gz" | \
    tar xz -C /usr/local/bin

# Fetch the latest release download URL for the specific architecture
RUN case "$TARGETPLATFORM" in \
        "linux/amd64") ARCH="x86_64";; \
        "linux/arm64") ARCH="aarch64";; \
        "linux/arm/v7") ARCH="armv7";; \
        "linux/ppc64le") ARCH="powerpc64le";; \
        "linux/riscv64") ARCH="riscv64";; \
        *) echo "Unsupported platform: $TARGETPLATFORM"; exit 1;; \
    esac && \
    echo "Target platform: $TARGETPLATFORM, Architecture: $ARCH" && \
    DOWNLOAD_URL=$(curl -s -H "Authorization: Bearer $GITHUB_TOKEN" \
    -H "Accept: application/vnd.github+json" \
    -H "X-GitHub-Api-Version: 2022-11-28" \
    "https://api.github.com/repos/mogenius/snoopy/releases/tags/$SNOOPY_VERSION" | \
    jq -r ".assets[] | select(.name | contains(\"snoopy_$ARCH\")) | .url") && \
    echo "Download URL: $DOWNLOAD_URL" && \
    curl -L -H "Authorization: Bearer $GITHUB_TOKEN" -H "Accept: application/octet-stream" $DOWNLOAD_URL -o snoopy && \
    chmod +x snoopy && \
    mv snoopy /usr/local/bin/snoopy

# Install bpftool
RUN set -x && \
    ln -sf /usr/include/asm-generic/ /usr/include/asm && \
    git clone --recurse-submodules https://github.com/libbpf/bpftool.git /opt/bpftool && \
    cd /opt/bpftool/src && \
    make install

RUN echo "Build platform: $BUILDPLATFORM, Target platform: $TARGETPLATFORM"

# dlv installation - nur für Build-Platform, nicht für Target
RUN case "$BUILDPLATFORM" in \
        linux/amd64|linux/arm64) go install -v github.com/go-delve/delve/cmd/dlv@latest; ;; \
        *) echo "dlv not supported for build platform $BUILDPLATFORM, skipping installation." ;; \
    esac

RUN go install sigs.k8s.io/controller-tools/cmd/controller-gen@latest
RUN git config --global --add safe.directory "/app"

WORKDIR /app

RUN go version
RUN bpftool version

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

COPY go.mod go.sum ./
RUN go mod download
COPY . .

RUN go generate ./...

# Cross-compile für die Target-Platform
# WICHTIG: GOARM nur für arm/v7 setzen, nicht für arm64!
RUN set -x && \
    echo "Building for TARGETOS=${TARGETOS} TARGETARCH=${TARGETARCH} TARGETVARIANT=${TARGETVARIANT}" && \
    export GOOS=${TARGETOS} && \
    export GOARCH=${TARGETARCH} && \
    if [ "${TARGETARCH}" = "arm" ] && [ -n "${TARGETVARIANT}" ]; then \
        export GOARM=${TARGETVARIANT#v}; \
        echo "Setting GOARM=${GOARM}"; \
    fi && \
    go build -v -trimpath \
        -gcflags="all=-l" \
        -ldflags="-s -w \
            -X 'mogenius-operator/src/version.GitCommitHash=${COMMIT_HASH}' \
            -X 'mogenius-operator/src/version.Branch=${GIT_BRANCH}' \
            -X 'mogenius-operator/src/version.BuildTimestamp=${BUILD_TIMESTAMP}' \
            -X 'mogenius-operator/src/version.Ver=${VERSION}'" \
        -o bin/mogenius-operator \
        ./src/main.go

# Final Image sollte die Target-Platform verwenden
FROM --platform=$TARGETPLATFORM ubuntu:noble AS release-image

ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT

ENV GOOS=${TARGETOS}
ENV GOARCH=${TARGETARCH}
ENV GOARM=${TARGETVARIANT}

COPY --from=builder "/app/bin/mogenius-operator" "/usr/local/bin/mogenius-operator"
COPY --from=builder "/usr/local/bin/snoopy" "/usr/local/bin/mogenius-snoopy"

RUN set -x && \
    apt-get update && \
    apt-get install -y --no-install-recommends "dumb-init" "nfs-common" "ca-certificates" "iproute2" && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

WORKDIR /app

# mogenius-operator release default settings
ENV MO_LOG_LEVEL="warn"

ENTRYPOINT ["dumb-init", "--"]
CMD ["mogenius-operator", "cluster"]