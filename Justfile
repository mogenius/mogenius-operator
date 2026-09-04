export CGO_ENABLED := "0"

set dotenv-load

[private]
default:
    just --list --unsorted

# Run the application with flags similar to the production build
run: build
    dist/native/mogenius-operator cluster

run-node-metrics: build
    dist/native/mogenius-operator nodemetrics

# disable mogenius-operator instances running in the cluster which interfere with local development
scale-down:
    kubectl -n mogenius scale deployment mogenius-operator --replicas=0
    kubectl -n mogenius patch daemonset mogenius-operator-node-metrics -p '{"spec": {"template": {"spec": {"nodeSelector": {"non-existing": "true"}}}}}'

# re-enable mogenius-operator instances running in the cluster which interfere with local development
scale-up:
    kubectl -n mogenius scale deployment mogenius-operator --replicas=1
    kubectl -n mogenius patch daemonset mogenius-operator-node-metrics --type json -p='[{"op": "remove", "path": "/spec/template/spec/nodeSelector/non-existing"}]'

# Build a native binary with flags similar to the production build
build: generate
    go build -trimpath -ldflags="-s -w \
        -X 'mogenius-operator/src/utils.DevBuild=yes' \
        -X 'mogenius-operator/src/version.GitCommitHash=$(git rev-parse --short HEAD)' \
        -X 'mogenius-operator/src/version.Branch=$(git branch | grep \* | cut -d ' ' -f2 | tr '[:upper:]' '[:lower:]')' \
        -X 'mogenius-operator/src/version.BuildTimestamp=$(date -Iseconds)' \
        -X 'mogenius-operator/src/version.Ver=$(git describe --tags $(git rev-list --tags --max-count=1))+dev'" -o dist/native/mogenius-operator ./src/main.go

# Build binaries for all targets
build-all: build-linux-amd64 build-linux-arm64 build-linux-armv7

# Build binary for target linux-amd64
build-linux-amd64:
    GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w \
        -X 'mogenius-operator/src/utils.DevBuild=yes' \
        -X 'mogenius-operator/src/version.GitCommitHash=$(git rev-parse --short HEAD)' \
        -X 'mogenius-operator/src/version.Branch=$(git branch | grep \* | cut -d ' ' -f2 | tr '[:upper:]' '[:lower:]')' \
        -X 'mogenius-operator/src/version.BuildTimestamp=$(date -Iseconds)' \
        -X 'mogenius-operator/src/version.Ver=$(git describe --tags $(git rev-list --tags --max-count=1))+dev'" -o dist/amd64/mogenius-operator ./src/main.go

# Build binary for target linux-arm64
build-linux-arm64:
    GOOS=linux GOARCH=arm64 go build -trimpath -ldflags="-s -w \
        -X 'mogenius-operator/src/utils.DevBuild=yes' \
        -X 'mogenius-operator/src/version.GitCommitHash=$(git rev-parse --short HEAD)' \
        -X 'mogenius-operator/src/version.Branch=$(git branch | grep \* | cut -d ' ' -f2 | tr '[:upper:]' '[:lower:]')' \
        -X 'mogenius-operator/src/version.BuildTimestamp=$(date -Iseconds)' \
        -X 'mogenius-operator/src/version.Ver=$(git describe --tags $(git rev-list --tags --max-count=1))+dev'" -o dist/arm64/mogenius-operator ./src/main.go

# Build binary for target linux-armv7
build-linux-armv7:
    GOOS=linux GOARCH=arm go build -trimpath -ldflags="-s -w \
        -X 'mogenius-operator/src/utils.DevBuild=yes' \
        -X 'mogenius-operator/src/version.GitCommitHash=$(git rev-parse --short HEAD)' \
        -X 'mogenius-operator/src/version.Branch=$(git branch | grep \* | cut -d ' ' -f2 | tr '[:upper:]' '[:lower:]')' \
        -X 'mogenius-operator/src/version.BuildTimestamp=$(date -Iseconds)' \
        -X 'mogenius-operator/src/version.Ver=$(git describe --tags $(git rev-list --tags --max-count=1))+dev'" -o dist/armv7/mogenius-operator ./src/main.go

# Install tools used by go generate
_install_controller_gen:
    go install sigs.k8s.io/controller-tools/cmd/controller-gen@latest

# Execute go generate
generate: _install_controller_gen
    go generate ./...

# Run tests and linters for quick iteration locally.
check: generate golangci-lint test-unit test-helm

# Execute unit tests
test-unit: generate
    go run gotest.tools/gotestsum@latest --format="testname" --hide-summary="skipped" --format-hide-empty-pkg --rerun-fails="0" -- -count=1 ./src/...

# Execute Helm chart unit tests (requires the helm-unittest plugin)
test-helm:
    helm unittest -f 'unittests/*_test.yaml' helm/charts/mogenius-operator

# Execute integration tests
test-integration: generate
    go run gotest.tools/gotestsum@latest --format="testname" --hide-summary="skipped" --format-hide-empty-pkg --rerun-fails="0" -- -count=1 ./test/...

# Execute end-to-end integration tests (downloads envtest binaries on first run)
test-e2e: generate
    KUBEBUILDER_ASSETS=$(go run sigs.k8s.io/controller-runtime/tools/setup-envtest@latest use -p path) go run gotest.tools/gotestsum@latest --format="testname" --hide-summary="skipped" --format-hide-empty-pkg --rerun-fails="0" -- -tags e2e -count=1 -timeout=600s ./test/integration/...

# Execute golangci-lint
golangci-lint: generate
    go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest run '--max-same-issues=0' ./src/...

# Build a multi-arch container image as a local manifest list.
# parallelism caps the compiler's concurrent processes; see GO_BUILD_PARALLELISM
# in the Dockerfile. Raise it on a machine with RAM to spare.
docker-build image="ghcr.io/mogenius/mogenius-operator-dev:latest" platforms="linux/amd64,linux/arm64" parallelism="2":
    #!/usr/bin/env sh
    set -e
    COMMIT_HASH=$(git rev-parse --short HEAD)
    GIT_BRANCH=$(git rev-parse --abbrev-ref HEAD | tr '[:upper:]' '[:lower:]')
    BUILD_TIMESTAMP=$(date -Iseconds)
    VERSION=$(git describe --tags $(git rev-list --tags --max-count=1) 2>/dev/null || echo "dev")
    # A stale manifest of the same name would accumulate the previous run's architectures.
    podman manifest rm {{image}} 2>/dev/null || true
    set -x
    # One build per platform, appended to the same manifest. Handing podman the
    # whole platform list builds them concurrently, which multiplies the
    # compiler's peak memory by the number of architectures.
    for platform in $(echo {{platforms}} | tr ',' ' '); do
        podman build --platform "${platform}" --manifest {{image}} \
            --build-arg VERSION=${VERSION} \
            --build-arg BUILD_TIMESTAMP=${BUILD_TIMESTAMP} \
            --build-arg GIT_BRANCH=${GIT_BRANCH} \
            --build-arg COMMIT_HASH=${COMMIT_HASH} \
            --build-arg DEV_BUILD=yes \
            --build-arg GO_BUILD_PARALLELISM={{parallelism}} \
            -f ./Dockerfile .
    done
    podman manifest inspect {{image}}

# Build and push a multi-arch container image
docker-push image="ghcr.io/mogenius/mogenius-operator-dev:latest" platforms="linux/amd64,linux/arm64" parallelism="2": (docker-build image platforms parallelism)
    podman manifest push --all {{image}}
