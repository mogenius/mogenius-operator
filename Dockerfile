FROM golang:1.23-alpine AS builder

LABEL org.opencontainers.image.description mogenius-k8s-manager: TODO add commit-log here.

RUN apk add --no-cache curl bash

ENV VERIFY_CHECKSUM=false

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

RUN go build -ldflags="-extldflags= \
  -X 'mogenius-k8s-manager/version.GitCommitHash=${COMMIT_HASH}' \
  -X 'mogenius-k8s-manager/version.Branch=${GIT_BRANCH}' \
  -X 'mogenius-k8s-manager/version.BuildTimestamp=${BUILD_TIMESTAMP}' \
  -X 'mogenius-k8s-manager/version.Ver=$VERSION'" -o bin/mogenius-k8s-manager .

RUN curl -fsSL -o get_helm.sh https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3
RUN chmod 700 get_helm.sh
RUN ./get_helm.sh
RUN rm get_helm.sh

FROM docker:dind

ARG GOOS
ARG GOARCH
ARG GOARM

ENV GOOS=${GOOS}
ENV GOARCH=${GOARCH}
ENV GOARM=${GOARM}

RUN apk add --no-cache curl nfs-utils ca-certificates jq bash

# RUN apk add --no-cache \
#     curl \
#     openssl \
#     nfs-utils \
#     ca-certificates

# RUN gem install -N rails
# RUN gem install -N bundler
# RUN npm install -g @vue/cli
# RUN npm install -g @angular/cli
# RUN npm install -g @nestjs/cli
# RUN npm install -g gatsby-cli
# RUN npm install -g create-next-app next react react-dom

# Install HELM
# RUN curl -fsSL -o get_helm.sh https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3
# RUN chmod 700 get_helm.sh
# RUN ./get_helm.sh
# RUN rm get_helm.sh

# Install Popeye
# RUN if [ "${GOARCH}" = "amd64" ]; then \
#       curl -fsSL -o popeye.tar.gz https://github.com/derailed/popeye/releases/download/v0.11.1/popeye_Linux_x86_64.tar.gz; \
#     elif [ "${GOARCH}" = "arm64" ]; then \
#       curl -fsSL -o popeye.tar.gz https://github.com/derailed/popeye/releases/download/v0.11.1/popeye_Linux_arm64.tar.gz; \
#     elif [ "${GOARCH}" = "arm" ]; then \
#       curl -fsSL -o popeye.tar.gz https://github.com/derailed/popeye/releases/download/v0.11.1/popeye_Linux_arm.tar.gz; \
#     else \
#       echo "Unsupported architecture"; \
#       exit 1; \
#     fi
# RUN tar -xvf popeye.tar.gz popeye
# RUN chmod +x popeye
# RUN mv popeye /usr/local/bin/popeye
# RUN rm popeye.tar.gz

# Install kubectl
# RUN VERSION=$(curl -L -s https://dl.k8s.io/release/stable.txt) curl -LO "https://dl.k8s.io/release/${VERSION}/bin/linux/${GOARCH}/kubectl"
# RUN chmod +x kubectl
# RUN mv kubectl /usr/local/bin/kubectl

# Install grype
# RUN curl -sSfL https://raw.githubusercontent.com/anchore/grype/main/install.sh | sh -s -- -b /usr/local/bin

RUN adduser -s /bin/sh -D mogee

WORKDIR /app

COPY --from=builder ["/app/bin/mogenius-k8s-manager", "."]
COPY --from=builder ["/usr/local/bin/helm", "/usr/local/bin/helm"]

ENV GIN_MODE=release

ENV HELM_CACHE_HOME="/db/helm-data/helm/cache"
ENV HELM_CONFIG_HOME="/db/helm-data/helm"
ENV HELM_DATA_HOME="/db/helm-data/helm"
ENV HELM_PLUGINS="/db/helm-data/helm/plugins"
ENV HELM_REGISTRY_CONFIG="/db/helm-data/helm/config.json"
ENV HELM_REPOSITORY_CACHE="/db/helm-data/helm/cache/repository"
ENV HELM_REPOSITORY_CONFIG="/db/helm-data/helm/repositories.yaml"

ENTRYPOINT /usr/local/bin/dockerd --iptables=false --dns 1.1.1.1 > docker-daemon.log 2>&1 & /app/mogenius-k8s-manager cluster
