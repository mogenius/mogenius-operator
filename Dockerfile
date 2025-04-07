FROM golang:1.24.2 AS golang
FROM quay.io/clastix/kubectl:v1.32.0 AS kubectl

FROM ubuntu:noble AS build-env

COPY --from=golang /usr/local/go /usr/local/go
COPY --from=kubectl /usr/local/bin/kubectl /usr/local/bin/kubectl

RUN mkdir /go
ENV GOPATH=/go
ENV PATH=${GOPATH}/bin:/usr/local/go/bin:${PATH}

# Setup system
RUN set -x && \
    apt-get update && \
    apt-get install -y "clang" "llvm" "libelf-dev" "libbpf-dev" "git" "linux-headers-generic" "gcc" "libc6-dev" "make" "cmake" "libpcap-dev" "binutils" "build-essential" "binutils-gold" "iproute2" "lsb-release" "sudo" "ca-certificates" "wget" "just"

# Install bpftool
RUN set -x && \
    ln -sf /usr/include/asm-generic/ /usr/include/asm && \
    git clone --recurse-submodules https://github.com/libbpf/bpftool.git /opt/bpftool && \
    cd /opt/bpftool/src && \
    make install

RUN case `uname -m` in \
        x86_64) go install -v github.com/go-delve/delve/cmd/dlv@latest; ;; \
        aarch64) go install -v github.com/go-delve/delve/cmd/dlv@latest; ;; \
        armv7l|ppc64le|s390x) echo "dlv not supported for this architecture, skipping installation." ;; \
        *) echo "Unsupported architecture, exiting..."; exit 1 ;; \
    esac
RUN go install sigs.k8s.io/controller-tools/cmd/controller-gen@latest
RUN git config --global --add safe.directory "/app"

WORKDIR /app

RUN go version
RUN kubectl --help
RUN bpftool version

FROM build-env AS builder

LABEL org.opencontainers.image.description="mogenius-k8s-manager"

ENV VERIFY_CHECKSUM=false
ENV CGO_ENABLED=0

ARG GOOS
ARG GOARCH
ARG GOARM

ARG COMMIT_HASH=NOT_SET
ARG GIT_BRANCH=NOT_SET
ARG BUILD_TIMESTAMP=NOT_SET
ARG VERSION=NOT_SET

COPY go.mod go.sum ./
RUN go mod download
COPY . .

RUN go generate ./...
RUN go build -trimpath -gcflags="all=-l" -ldflags="-s -w \
    -X 'mogenius-k8s-manager/src/version.GitCommitHash=${COMMIT_HASH}' \
    -X 'mogenius-k8s-manager/src/version.Branch=${GIT_BRANCH}' \
    -X 'mogenius-k8s-manager/src/version.BuildTimestamp=${BUILD_TIMESTAMP}' \
    -X 'mogenius-k8s-manager/src/version.Ver=$VERSION'" \
    -o "bin/mogenius-k8s-manager" \
    ./src/main.go

FROM ubuntu:noble AS release-image

ARG GOOS
ARG GOARCH
ARG GOARM

ENV GOOS=${GOOS}
ENV GOARCH=${GOARCH}
ENV GOARM=${GOARM}

COPY --from=builder "/app/bin/mogenius-k8s-manager" "/usr/local/bin/mogenius-k8s-manager"

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
