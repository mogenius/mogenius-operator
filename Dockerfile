FROM golang:1.21-alpine AS builder

LABEL org.opencontainers.image.description mogenius-k8s-manager: TODO add commit-log here.

ARG GOOS
ARG GOARCH
ARG GOARM

# RUN apk add --no-cache \
    # libpcap-dev \
    # g++ \
    # perl-utils \
    # curl \
    # build-base \
    # binutils-gold \
    # bash 
    # clang \
    # llvm \
    # libbpf-dev \
    # linux-headers
RUN apk add --no-cache nfs-utils

ARG COMMIT_HASH=NOT_SET
ARG GIT_BRANCH=NOT_SET
ARG BUILD_TIMESTAMP=NOT_SET
ARG VERSION=NOT_SET

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -ldflags="-extldflags= \
  -X 'mogenius-k8s-manager/version.GitCommitHash=${COMMIT_HASH}' \
  -X 'mogenius-k8s-manager/version.Branch=${GIT_BRANCH}' \
  -X 'mogenius-k8s-manager/version.BuildTimestamp=${BUILD_TIMESTAMP}' \
  -X 'mogenius-k8s-manager/version.Ver=$VERSION'" -o bin/mogenius-k8s-manager .

FROM docker:dind 

ARG GOOS
ARG GOARCH
ARG GOARM

RUN apk add --no-cache \
    bash \
    git \
    curl \
    openssl \
    nfs-utils \
    ca-certificates

# RUN gem install -N rails
# RUN gem install -N bundler
# RUN npm install -g @vue/cli
# RUN npm install -g @angular/cli
# RUN npm install -g @nestjs/cli
# RUN npm install -g gatsby-cli
# RUN npm install -g create-next-app next react react-dom

# Install HELM
RUN curl -fsSL -o get_helm.sh https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3
RUN chmod 700 get_helm.sh
RUN ./get_helm.sh
RUN rm get_helm.sh

# Install Popeye
RUN if [ "${GOARCH}" = "amd64" ]; then \
      curl -fsSL -o popeye.tar.gz https://github.com/derailed/popeye/releases/download/v0.11.1/popeye_Linux_x86_64.tar.gz; \
    elif [ "${GOARCH}" = "arm64" ]; then \
      curl -fsSL -o popeye.tar.gz https://github.com/derailed/popeye/releases/download/v0.11.1/popeye_Linux_arm64.tar.gz; \
    elif [ "${GOARCH}" = "arm" ]; then \
      curl -fsSL -o popeye.tar.gz https://github.com/derailed/popeye/releases/download/v0.11.1/popeye_Linux_arm.tar.gz; \
    else \
      echo "Unsupported architecture"; \
      exit 1; \
    fi
RUN tar -xvf popeye.tar.gz popeye
RUN chmod +x popeye
RUN mv popeye /usr/local/bin/popeye
RUN rm popeye.tar.gz

# Install kubectl
RUN curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/${GOARCH}/kubectl"
RUN chmod +x kubectl
RUN mv kubectl /usr/local/bin/kubectl

# Install grype
RUN curl -sSfL https://raw.githubusercontent.com/anchore/grype/main/install.sh | sh -s -- -b /usr/local/bin

RUN adduser -s /bin/bash -D mogee

WORKDIR /app

COPY --from=builder ["/app/bin/mogenius-k8s-manager", "."]
COPY --from=builder ["/app/grype-json-template", "."]

ENV GIN_MODE=release

ENTRYPOINT /usr/local/bin/dockerd > docker-daemon.log 2>&1 & /app/mogenius-k8s-manager cluster