FROM golang:1.19-alpine AS builder

LABEL org.opencontainers.image.description mogenius-k8s-manager: TODO add commit-log here.

ENV GOOS=linux

RUN apk add --no-cache \
    libpcap-dev \
    g++ \
    perl-utils \
    curl \
    build-base \
    binutils-gold \
    bash \
    clang \
    llvm \
    libbpf-dev \
    linux-headers

ARG COMMIT_HASH=NOT_SET
ARG GIT_BRANCH=NOT_SET
ARG BUILD_TIMESTAMP=NOT_SET
ARG NEXT_VERSION=NOT_SET

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN GOOS=linux GOARCH=amd64 go build -ldflags="-extldflags= \
  -X 'mogenius-k8s-manager/version.GitCommitHash=${COMMIT_HASH}' \
  -X 'mogenius-k8s-manager/version.Branch=${GIT_BRANCH}' \
  -X 'mogenius-k8s-manager/version.BuildTimestamp=${BUILD_TIMESTAMP}' \
  -X 'mogenius-k8s-manager/version.Ver=${NEXT_VERSION}'" -o bin/mogenius-k8s-manager .


FROM alpine:latest
RUN apk add --no-cache \
    libpcap-dev bash

WORKDIR /app

COPY --from=builder ["/app/bin/mogenius-k8s-manager", "."]

ENV GIN_MODE=release

ENTRYPOINT [ "/app/mogenius-k8s-manager", "cluster" ]