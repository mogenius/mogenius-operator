export CGO_ENABLED := "0"

# Run the application with flags similar to the production build
run:
    go run -trimpath -gcflags="all=-l" -ldflags="-s -w \
        -X 'mogenius-k8s-manager/version.GitCommitHash=XXXXXX' \
        -X 'mogenius-k8s-manager/version.Branch=local-development' \
        -X 'mogenius-k8s-manager/version.BuildTimestamp=$(date)' \
        -X 'mogenius-k8s-manager/version.Ver=6.6.6'" main.go cluster

build-native:
    go build -trimpath -gcflags="all=-l" -ldflags="-s -w \
        -X 'mogenius-k8s-manager/version.GitCommitHash=XXXXXX' \
        -X 'mogenius-k8s-manager/version.Branch=local-development' \
        -X 'mogenius-k8s-manager/version.BuildTimestamp=$(date)' \
        -X 'mogenius-k8s-manager/version.Ver=6.6.6'" -o dist/ ./...

build-all: build-linux-amd64 build-linux-arm64 build-linux-armv7

build-linux-amd64:
    GOOS=linux GOARCH=amd64 go build -trimpath -gcflags="all=-l" -ldflags="-s -w \
        -X 'mogenius-k8s-manager/version.GitCommitHash=XXXXXX' \
        -X 'mogenius-k8s-manager/version.Branch=local-development' \
        -X 'mogenius-k8s-manager/version.BuildTimestamp=$(date)' \
        -X 'mogenius-k8s-manager/version.Ver=6.6.6'" -o dist/amd64/ ./...

build-linux-arm64:
    GOOS=linux GOARCH=amd64 go build -trimpath -gcflags="all=-l" -ldflags="-s -w \
        -X 'mogenius-k8s-manager/version.GitCommitHash=XXXXXX' \
        -X 'mogenius-k8s-manager/version.Branch=local-development' \
        -X 'mogenius-k8s-manager/version.BuildTimestamp=$(date)' \
        -X 'mogenius-k8s-manager/version.Ver=6.6.6'" -o dist/arm64/ ./...

build-linux-armv7:
    GOOS=linux GOARCH=arm go build -trimpath -gcflags="all=-l" -ldflags="-s -w \
        -X 'mogenius-k8s-manager/version.GitCommitHash=XXXXXX' \
        -X 'mogenius-k8s-manager/version.Branch=local-development' \
        -X 'mogenius-k8s-manager/version.BuildTimestamp=$(date)' \
        -X 'mogenius-k8s-manager/version.Ver=6.6.6'" -o dist/armv7/ ./...

test:
    go run gotest.tools/gotestsum@latest --format="testname" --hide-summary="skipped" -- -short ./...

test-short:
    go run gotest.tools/gotestsum@latest --format="testname" --hide-summary="skipped" -- -short ./...
