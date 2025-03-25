#
# BUILDER IMAGE
#
FROM golang:1.24-alpine AS builder

# TODO add commit-log here.
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

RUN apk add --no-cache clang llvm libelf libbpf-dev git linux-headers gcc musl-dev make cmake 

RUN ln -sf /usr/include/asm-generic/ /usr/include/asm
RUN git clone --recurse-submodules https://github.com/libbpf/bpftool.git
RUN make -C bpftool/src/ install

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .

RUN go install sigs.k8s.io/controller-tools/cmd/controller-gen@latest
RUN go generate ./...
RUN go build -trimpath -gcflags="all=-l" -ldflags="-s -w \
  -X 'mogenius-k8s-manager/src/version.GitCommitHash=${COMMIT_HASH}' \
  -X 'mogenius-k8s-manager/src/version.Branch=${GIT_BRANCH}' \
  -X 'mogenius-k8s-manager/src/version.BuildTimestamp=${BUILD_TIMESTAMP}' \
  -X 'mogenius-k8s-manager/src/version.Ver=$VERSION'" \
  -o "bin/mogenius-k8s-manager" \
  ./src/main.go

#
# FINAL IMAGE
#
FROM alpine:3.21

ARG GOOS
ARG GOARCH
ARG GOARM

ENV GOOS=${GOOS}
ENV GOARCH=${GOARCH}
ENV GOARM=${GOARM}

COPY --from=builder ["/app/bin/mogenius-k8s-manager", "/app/mogenius-k8s-manager"]

RUN apk add --no-cache dumb-init nfs-utils ca-certificates

WORKDIR /app

## mogenius-k8s-manager release default settings
ENV MO_LOG_LEVEL="warn"

ENTRYPOINT ["dumb-init", "--", "sh", "-c", "/app/mogenius-k8s-manager cluster"]
