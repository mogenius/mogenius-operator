# Syntax für BuildKit features (Secrets/SSH/Cache)
# syntax=docker/dockerfile:1.7

# Build-Stage sollte native Platform verwenden für schnelleres Kompilieren
FROM --platform=$BUILDPLATFORM golang:1.25.3 AS golang

FROM --platform=$BUILDPLATFORM ubuntu:noble AS build-env

ENV SNOOPY_VERSION=v0.3.5

COPY --from=golang /usr/local/go /usr/local/go

RUN mkdir -p /go
ENV GOPATH=/go
ENV PATH=${GOPATH}/bin:/usr/local/go/bin:${PATH}

# KEIN Token als ARG/ENV!
ARG TARGETPLATFORM
ARG BUILDPLATFORM

# Private GitHub-Repos für Go-Module freischalten (anpassen!)
ENV GOPRIVATE=github.com/deineorg/* \
    GONOSUMDB=github.com/deineorg/* \
    GOPROXY=direct

# Setup system
RUN set -x && \
    apt-get update && \
    apt-get install -y "curl" "jq" "clang" "llvm" "libelf-dev" "libbpf-dev" "git" "linux-headers-generic" "gcc" "libc6-dev" "make" "cmake" "libpcap-dev" "binutils" "build-essential" "binutils-gold" "iproute2" "lsb-release" "sudo" "ca-certificates" "wget" "just" "libssl-dev"

# Fetch Snoopy Release (Token als Secret mounten – NICHT im Image speichern)
RUN --mount=type=secret,id=GITHUB_TOKEN bash -euxo pipefail -c '\
  case "$TARGETPLATFORM" in \
      "linux/amd64")  ARCH="x86_64" ;; \
      "linux/arm64")  ARCH="aarch64" ;; \
      "linux/arm/v7") ARCH="armv7" ;; \
      "linux/ppc64le") ARCH="powerpc64le" ;; \
      "linux/riscv64") ARCH="riscv64" ;; \
      *) echo "Unsupported platform: $TARGETPLATFORM" >&2; exit 1 ;; \
  esac; \
  echo "Target platform: $TARGETPLATFORM, Architecture: $ARCH"; \
  TOKEN="$(cat /run/secrets/GITHUB_TOKEN)"; \
  API_URL="https://api.github.com/repos/mogenius/snoopy/releases/tags/$SNOOPY_VERSION"; \
  DOWNLOAD_URL="$(curl -s -H "Authorization: Bearer ${TOKEN}" -H "Accept: application/vnd.github+json" -H "X-GitHub-Api-Version: 2022-11-28" "$API_URL" \
    | jq -r ".assets[] | select(.name | contains(\"snoopy_${ARCH}\")) | .url")"; \
  test -n "$DOWNLOAD_URL"; \
  echo "Download URL: $DOWNLOAD_URL"; \
  curl -L -H "Authorization: Bearer ${TOKEN}" -H "Accept: application/octet-stream" "$DOWNLOAD_URL" -o /usr/local/bin/snoopy; \
  chmod +x /usr/local/bin/snoopy \
'

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

# -------------------- Builder-Stage --------------------
FROM build-env AS builder

LABEL org.opencontainers.image.description="mogenius-k8s-manager"

ENV VERIFY_CHECKSUM=false
ENV CGO_ENABLED=0

ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT

ARG COMMIT_HASH=NOT_SET
ARG GIT_BRANCH=NOT_SET
ARG BUILD_TIMESTAMP=NOT_SET
ARG VERSION=NOT_SET

# Module-Dateien zuerst kopieren und mit Secret auflösen
COPY go.mod go.sum ./
RUN --mount=type=secret,id=GITHUB_TOKEN bash -euxo pipefail -c '\
  install -m 0600 /dev/null $HOME/.netrc; \
  printf "machine github.com login x-access-token password %s\n" "$(cat /run/secrets/GITHUB_TOKEN)" > $HOME/.netrc; \
  git config --global url."https://github.com/".insteadOf "ssh://git@github.com/"; \
  git config --global url."https://github.com/".insteadOf "git@github.com:"; \
  go env -w GOPRIVATE="$GOPRIVATE" GONOSUMDB="$GONOSUMDB" GOPROXY="$GOPROXY"; \
  go mod download; \
  rm -f $HOME/.netrc; \
  git config --global --unset url."https://github.com/".insteadOf || true \
'

# Restlichen Source kopieren (maximiert Layer-Caching)
COPY . .

RUN go generate ./...

# Cross-compile für die Target-Platform
RUN GOOS=${TARGETOS} GOARCH=${TARGETARCH} GOARM=${TARGETVARIANT#v} \
    go build -trimpath -gcflags="all=-l" -ldflags="-s -w \
    -X '\''mogenius-k8s-manager/src/version.GitCommitHash='\'${COMMIT_HASH}\'' \
    -X '\''mogenius-k8s-manager/src/version.Branch='\'${GIT_BRANCH}\'' \
    -X '\''mogenius-k8s-manager/src/version.BuildTimestamp='\'${BUILD_TIMESTAMP}\'' \
    -X '\''mogenius-k8s-manager/src/version.Ver='\'${VERSION}\''" \
    -o "bin/mogenius-k8s-manager" \
    ./src/main.go

# -------------------- Release-Image --------------------
FROM ubuntu:noble AS release-image

ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT

ENV GOOS=${TARGETOS}
ENV GOARCH=${TARGETARCH}
ENV GOARM=${TARGETVARIANT}

COPY --from=builder "/app/bin/mogenius-k8s-manager" "/usr/local/bin/mogenius-k8s-manager"
COPY --from=builder "/usr/local/bin/snoopy" "/usr/local/bin/mogenius-snoopy"

RUN set -x && \
    apt-get update && \
    apt-get install -y --no-install-recommends "dumb-init" "nfs-common" "ca-certificates" "iproute2" && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

WORKDIR /app

# mogenius-k8s-manager release default settings
ENV MO_LOG_LEVEL="warn"

ENTRYPOINT ["dumb-init", "--"]
CMD ["mogenius-k8s-manager", "cluster"]