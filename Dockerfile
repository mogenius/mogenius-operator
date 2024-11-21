#
# BUILDER IMAGE
#
FROM golang:1.23-alpine AS builder

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

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .

RUN go build -trimpath -gcflags="all=-l" -ldflags="-s -w \
  -X 'mogenius-k8s-manager/src/version.GitCommitHash=${COMMIT_HASH}' \
  -X 'mogenius-k8s-manager/src/version.Branch=${GIT_BRANCH}' \
  -X 'mogenius-k8s-manager/src/version.BuildTimestamp=${BUILD_TIMESTAMP}' \
  -X 'mogenius-k8s-manager/src/version.Ver=$VERSION'" \
  -o "bin/mogenius-k8s-manager" \
  ./src/main.go

RUN apk add --no-cache upx
RUN upx -9 --lzma /app/bin/mogenius-k8s-manager

#
# FINAL IMAGE
#
FROM docker:dind

ARG GOOS
ARG GOARCH
ARG GOARM

ENV GOOS=${GOOS}
ENV GOARCH=${GOARCH}
ENV GOARM=${GOARM}

COPY --from=builder ["/app/bin/mogenius-k8s-manager", "."]

RUN apk add --no-cache dumb-init nfs-utils ca-certificates

WORKDIR /app

# e.g. "--dns 1.1.1.1"
ENV DOCKERD_ARGS=""

## mogenius-k8s-manager release default settings
ENV MO_LOG_LEVEL="warn"

ENTRYPOINT ["dumb-init", "--", "sh", "-c", "/usr/local/bin/dockerd --iptables=false ${DOCKERD_ARGS} > docker-daemon.log 2>&1 & /app/mogenius-k8s-manager cluster"]
