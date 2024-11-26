export CGO_ENABLED := "0"

[private]
default:
    just --list --unsorted

# Run the application with flags similar to the production build
run: build-native
    dist/native/mogenius-k8s-manager cluster

# Build a native binary with flags similar to the production build
build-native:
    go build -trimpath -gcflags="all=-l" -ldflags="-s -w \
        -X 'mogenius-k8s-manager/src/version.GitCommitHash=XXXXXX' \
        -X 'mogenius-k8s-manager/src/version.Branch=local-development' \
        -X 'mogenius-k8s-manager/src/version.BuildTimestamp=$(date)' \
        -X 'mogenius-k8s-manager/src/version.Ver=6.6.6'" -o dist/native/mogenius-k8s-manager ./src/main.go

# Build binaries for all targets
build-all: build-linux-amd64 build-linux-arm64 build-linux-armv7

# Build binary for target linux-amd64
build-linux-amd64:
    GOOS=linux GOARCH=amd64 go build -trimpath -gcflags="all=-l" -ldflags="-s -w \
        -X 'mogenius-k8s-manager/src/version.GitCommitHash=XXXXXX' \
        -X 'mogenius-k8s-manager/src/version.Branch=local-development' \
        -X 'mogenius-k8s-manager/src/version.BuildTimestamp=$(date)' \
        -X 'mogenius-k8s-manager/src/version.Ver=6.6.6'" -o dist/amd64/mogenius-k8s-manager ./src/main.go

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
        -X 'mogenius-k8s-manager/src/version.GitCommitHash=XXXXXX' \
        -X 'mogenius-k8s-manager/src/version.Branch=local-development' \
        -X 'mogenius-k8s-manager/src/version.BuildTimestamp=$(date)' \
        -X 'mogenius-k8s-manager/src/version.Ver=6.6.6'" -o dist/arm64/mogenius-k8s-manager ./src/main.go

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
        -X 'mogenius-k8s-manager/src/version.GitCommitHash=XXXXXX' \
        -X 'mogenius-k8s-manager/src/version.Branch=local-development' \
        -X 'mogenius-k8s-manager/src/version.BuildTimestamp=$(date)' \
        -X 'mogenius-k8s-manager/src/version.Ver=6.6.6'" -o dist/armv7/mogenius-k8s-manager ./src/main.go

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

# Run tests and linters for quick iteration locally.
check: golangci-lint test-unit

# Execute unit tests
test-unit:
    go run gotest.tools/gotestsum@latest --format="testname" --hide-summary="skipped" --format-hide-empty-pkg --rerun-fails="0" -- -count=1 ./src/...

# Execute integration tests
test-integration:
    go run gotest.tools/gotestsum@latest --format="testname" --hide-summary="skipped" --format-hide-empty-pkg --rerun-fails="0" -- -count=1 ./test/...

# Execute golangci-lint
golangci-lint:
    go run github.com/golangci/golangci-lint/cmd/golangci-lint@latest run '--fast=false' --sort-results '--max-same-issues=0' '--timeout=1h'
