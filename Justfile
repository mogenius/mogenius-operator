export CGO_ENABLED := "0"

set dotenv-load

[private]
default:
    just --list --unsorted

# Run the application with flags similar to the production build
run: build
    dist/native/mogenius-k8s-manager cluster

run-privileged: build
    sudo -E dist/native/mogenius-k8s-manager cluster

# Build a native binary with flags similar to the production build
build: generate
    go build -trimpath -gcflags="all=-l" -ldflags="-s -w \
        -X 'mogenius-k8s-manager/src/utils.DevBuild=yes' \
        -X 'mogenius-k8s-manager/src/version.GitCommitHash=$(git rev-parse --short HEAD)' \
        -X 'mogenius-k8s-manager/src/version.Branch=$(git branch | grep \* | cut -d ' ' -f2 | tr '[:upper:]' '[:lower:]')' \
        -X 'mogenius-k8s-manager/src/version.BuildTimestamp=$(date -Iseconds)' \
        -X 'mogenius-k8s-manager/src/version.Ver=$(git describe --tags $(git rev-list --tags --max-count=1))+dev'" -o dist/native/mogenius-k8s-manager ./src/main.go
    dist/native/mogenius-k8s-manager patterns --output=yaml > generated/spec.yaml
    dist/native/mogenius-k8s-manager patterns --output=typescript > generated/client.ts

# Build binaries for all targets
build-all: build-linux-amd64 build-linux-arm64 build-linux-armv7

# Build binary for target linux-amd64
build-linux-amd64:
    GOOS=linux GOARCH=amd64 go build -trimpath -gcflags="all=-l" -ldflags="-s -w \
        -X 'mogenius-k8s-manager/src/utils.DevBuild=yes' \
        -X 'mogenius-k8s-manager/src/version.GitCommitHash=$(git rev-parse --short HEAD)' \
        -X 'mogenius-k8s-manager/src/version.Branch=$(git branch | grep \* | cut -d ' ' -f2 | tr '[:upper:]' '[:lower:]')' \
        -X 'mogenius-k8s-manager/src/version.BuildTimestamp=$(date -Iseconds)' \
        -X 'mogenius-k8s-manager/src/version.Ver=$(git describe --tags $(git rev-list --tags --max-count=1))+dev'" -o dist/amd64/mogenius-k8s-manager ./src/main.go

# Build docker image for target linux-amd64
build-docker-linux-amd64:
    #!/usr/bin/env sh
    GIT_BRANCH=$(git branch | grep \* | cut -d ' ' -f2 | tr '[:upper:]' '[:lower:]')
    COMMIT_HASH=$(git rev-parse --short HEAD)
    GIT_BRANCH=$(git rev-parse --abbrev-ref HEAD)
    BUILD_TIMESTAMP=$(date -Iseconds)
    VERSION=$(git describe --tags $(git rev-list --tags --max-count=1))
    set -x
    docker buildx build --platform=linux/amd64 -f Dockerfile \
        --build-arg GOOS=linux \
        --build-arg GOARCH=amd64 \
        --build-arg VERSION="$VERSION" \
        --build-arg BUILD_TIMESTAMP="$BUILD_TIMESTAMP" \
        --build-arg GIT_BRANCH="$GIT_BRANCH" \
        --build-arg COMMIT_HASH="$COMMIT_HASH" \
        -t ghcr.io/mogenius/mogenius-k8s-manager-dev:$VERSION-amd64 \
        -t ghcr.io/mogenius/mogenius-k8s-manager-dev:latest-amd64 \
        .

# Build binary for target linux-arm64
build-linux-arm64:
    GOOS=linux GOARCH=amd64 go build -trimpath -gcflags="all=-l" -ldflags="-s -w \
        -X 'mogenius-k8s-manager/src/utils.DevBuild=yes' \
        -X 'mogenius-k8s-manager/src/version.GitCommitHash=$(git rev-parse --short HEAD)' \
        -X 'mogenius-k8s-manager/src/version.Branch=$(git branch | grep \* | cut -d ' ' -f2 | tr '[:upper:]' '[:lower:]')' \
        -X 'mogenius-k8s-manager/src/version.BuildTimestamp=$(date -Iseconds)' \
        -X 'mogenius-k8s-manager/src/version.Ver=$(git describe --tags $(git rev-list --tags --max-count=1))+dev'" -o dist/arm64/mogenius-k8s-manager ./src/main.go

# Build docker image for target linux-arm64
build-docker-linux-arm64:
    #!/usr/bin/env sh
    GIT_BRANCH=$(git branch | grep \* | cut -d ' ' -f2 | tr '[:upper:]' '[:lower:]')
    COMMIT_HASH=$(git rev-parse --short HEAD)
    GIT_BRANCH=$(git rev-parse --abbrev-ref HEAD)
    BUILD_TIMESTAMP=$(date -Iseconds)
    VERSION=$(git describe --tags $(git rev-list --tags --max-count=1))
    set -x
    docker buildx build --platform=linux/arm64 -f Dockerfile \
        --build-arg GOOS=linux \
        --build-arg GOARCH=arm64 \
        --build-arg VERSION="$VERSION" \
        --build-arg BUILD_TIMESTAMP="$BUILD_TIMESTAMP" \
        --build-arg GIT_BRANCH="$GIT_BRANCH" \
        --build-arg COMMIT_HASH="$COMMIT_HASH" \
        -t ghcr.io/mogenius/mogenius-k8s-manager-dev:$VERSION-amd64 \
        -t ghcr.io/mogenius/mogenius-k8s-manager-dev:latest-amd64 \
        .

# Build binary for target linux-armv7
build-linux-armv7:
    GOOS=linux GOARCH=arm go build -trimpath -gcflags="all=-l" -ldflags="-s -w \
        -X 'mogenius-k8s-manager/src/utils.DevBuild=yes' \
        -X 'mogenius-k8s-manager/src/version.GitCommitHash=$(git rev-parse --short HEAD)' \
        -X 'mogenius-k8s-manager/src/version.Branch=$(git branch | grep \* | cut -d ' ' -f2 | tr '[:upper:]' '[:lower:]')' \
        -X 'mogenius-k8s-manager/src/version.BuildTimestamp=$(date -Iseconds)' \
        -X 'mogenius-k8s-manager/src/version.Ver=$(git describe --tags $(git rev-list --tags --max-count=1))+dev'" -o dist/armv7/mogenius-k8s-manager ./src/main.go

# Build docker image for target linux-armv7
build-docker-linux-armv7:
    #!/usr/bin/env sh
    GIT_BRANCH=$(git branch | grep \* | cut -d ' ' -f2 | tr '[:upper:]' '[:lower:]')
    COMMIT_HASH=$(git rev-parse --short HEAD)
    GIT_BRANCH=$(git rev-parse --abbrev-ref HEAD)
    BUILD_TIMESTAMP=$(date -Iseconds)
    VERSION=$(git describe --tags $(git rev-list --tags --max-count=1))
    set -x
    docker buildx build --platform=linux/arm64 -f Dockerfile \
        --build-arg GOOS=linux \
        --build-arg GOARCH=arm \
        --build-arg VERSION="$VERSION" \
        --build-arg BUILD_TIMESTAMP="$BUILD_TIMESTAMP" \
        --build-arg GIT_BRANCH="$GIT_BRANCH" \
        --build-arg COMMIT_HASH="$COMMIT_HASH" \
        -t ghcr.io/mogenius/mogenius-k8s-manager-dev:$VERSION-amd64 \
        -t ghcr.io/mogenius/mogenius-k8s-manager-dev:latest-amd64 \
        .

# Install tools used by go generate
_install_controller_gen:
    go install sigs.k8s.io/controller-tools/cmd/controller-gen@latest

# Execute go generate
generate: _install_controller_gen
    go generate ./...

# Run tests and linters for quick iteration locally.
check: generate golangci-lint test-unit

# Execute unit tests
test-unit: generate
    go run gotest.tools/gotestsum@latest --format="testname" --hide-summary="skipped" --format-hide-empty-pkg --rerun-fails="0" -- -count=1 ./src/...

# Execute integration tests
test-integration: generate
    go run gotest.tools/gotestsum@latest --format="testname" --hide-summary="skipped" --format-hide-empty-pkg --rerun-fails="0" -- -count=1 ./test/...

# Execute golangci-lint
golangci-lint: generate
    go run github.com/golangci/golangci-lint/cmd/golangci-lint@latest run '--fast=false' --sort-results '--max-same-issues=0' '--timeout=1h' ./src/...
