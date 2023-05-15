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
    bash \
    git \
    curl \
    build-base \
    libpcap-dev bash \
    nodejs \
    npm \
    coreutils \
    ruby-dev \
    openssl

RUN gem install -N rails
RUN gem install -N bundler
RUN npm install -g @vue/cli
RUN npm install -g @angular/cli
RUN npm install -g @nestjs/cli
RUN npm install -g gatsby-cli
RUN npm install -g create-next-app next react react-dom
RUN go install github.com/derailed/popeye@latest

# Install HELM
RUN curl -fsSL -o get_helm.sh https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3
RUN chmod 700 get_helm.sh
RUN ./get_helm.sh
RUN rm get_helm.sh

WORKDIR /app

COPY --from=builder ["/app/bin/mogenius-k8s-manager", "."]

ENV GIN_MODE=release

ENTRYPOINT [ "/app/mogenius-k8s-manager", "cluster" ]