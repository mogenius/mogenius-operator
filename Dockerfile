#
# BUILDER IMAGE
#
FROM golang:1.23-alpine AS builder

LABEL org.opencontainers.image.description mogenius-k8s-manager: TODO add commit-log here.

RUN apk add --no-cache curl bash

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

ARG TARGETARCH

RUN go build -trimpath -gcflags="all=-l" -ldflags="-s -w \
  -X 'mogenius-k8s-manager/version.GitCommitHash=${COMMIT_HASH}' \
  -X 'mogenius-k8s-manager/version.Branch=${GIT_BRANCH}' \
  -X 'mogenius-k8s-manager/version.BuildTimestamp=${BUILD_TIMESTAMP}' \
  -X 'mogenius-k8s-manager/version.Ver=$VERSION'" -o bin/mogenius-k8s-manager .

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

RUN apk add --no-cache dumb-init curl nfs-utils ca-certificates jq bash

RUN adduser -s /bin/sh -D mogee

WORKDIR /app

COPY --from=builder ["/app/bin/mogenius-k8s-manager", "."]

ENV GIN_MODE=release

ENV HELM_CACHE_HOME="/db/helm-data/helm/cache"
ENV HELM_CONFIG_HOME="/db/helm-data/helm"
ENV HELM_DATA_HOME="/db/helm-data/helm"
ENV HELM_PLUGINS="/db/helm-data/helm/plugins"
ENV HELM_REGISTRY_CONFIG="/db/helm-data/helm/config.json"
ENV HELM_REPOSITORY_CACHE="/db/helm-data/helm/cache/repository"
ENV HELM_REPOSITORY_CONFIG="/db/helm-data/helm/repositories.yaml"
# e.g. "--dns 1.1.1.1"
ENV DOCKERD_ARGS="" 

ENTRYPOINT ["dumb-init", "--", "sh", "-c", "/usr/local/bin/dockerd --iptables=false ${DOCKERD_ARGS} > docker-daemon.log 2>&1 & /app/mogenius-k8s-manager cluster"]
