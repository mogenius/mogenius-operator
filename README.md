<p align="center">
  <img src="https://imagedelivery.net/T7YEW5IAgZJ0dY4-LDTpyQ/3ae4fcf0-289c-48d2-3323-d2c5bc932300/detail" alt="drawing" width="200"/>
</p>

## run locally

First you need to have the [mogenius helm chart installed and running](https://docs.mogenius.com/cluster-management/installing-mogenius).

Then create a file called `.env` with the mandatory settings:

```sh
## mogenius cluster configuration
MO_API_KEY=
MO_CLUSTER_NAME=
MO_CLUSTER_MFA_ID=

## Select the mogenius environment to run against:
##
## - "prod" to run against the prod APIs
## - "pre-prod": to run against the pre-prod APIs
## - "dev": to run against the dev APIs
## - "local": to run agains APIs on localhost
## - "": to define APIs manually using `MO_API_SERVER` and `MO_EVENT_SERVER`
MO_STAGE=dev

## A full list of available configs can be generated using:
##
## ```sh
## go run -trimpath src/main.go config
## ```
```

Get the `api-key`, `mfa-id` and `cluster-name` from the operator secret `mogenius/mogenius` and adjust the `.env` accordingly.

Change the replicas to `0`:

```sh
kubectl scale -n mogenius deployment mogenius-k8s-manager --replicas=0
```

Now mogenius can be run locally:

```sh
just run
```

## local docker image in docker-desktop kubernetes

RUN:

```sh
docker build -t localk8smanager --build-arg GOOS=linux --build-arg GOARCH=arm64 --build-arg BUILD_TIMESTAMP="$(date)" --build-arg COMMIT_HASH="XXX" --build-arg GIT_BRANCH=local-development --build-arg VERSION="6.6.6" -f Dockerfile .
```

Assuming you already have a [prod operator running](https://docs.mogenius.com/cluster-management/installing-mogenius#mogenius-cli), you can adjust the deployment of the operator with e.g. `kubectl edit deployments -n mogenius mogenius-k8s-manager`

```yaml
## FROM:
image: ghcr.io/mogenius/mogenius-k8s-manager:latest
imagePullPolicy: Always

## TO:
image: localk8smanager:latest
imagePullPolicy: Never
```

After that simply restart the deployment and you are good to go.

## bolt-db debugging

```sh
apk add go
go install github.com/br0xen/boltbrowser@latest
cp /data/db/mogenius-stats-3.db mogenius-stats-3.db
/root/go/bin/boltbrowser mogenius-stats-3.db
```

## Upgrade Modules

```sh
go get -u ./...
go mod tidy
```

## Testing/Linting Locally

```sh
# Run linter and unit tests locally
just check

# Run linter
just golangci-lint

# Run quick unit tests
just test-unit

# Run slow integration tests
just test-integration
```

## Helm Install

```sh
helm repo add mo-public helm.mogenius.com/public
helm repo update
helm search repo mogenius-platform
helm install mogenius-platform mo-public/mogenius-platform \
  --set global.cluster_name="mo7-mogenius-io" \
  --set global.api_key="mo_7bf5c2b5-d7bc-4f0e-b8fc-b29d09108928_0hkga6vjum3p1mvezith" \
  --set global.namespace="mogenius"
```

## Helm Upgrade

```sh
helm repo update
helm upgrade mogenius-platform mo-public/mogenius-platform
```

## Helm Uninstall

```sh
helm uninstall mogenius-platform
```

## Clean Helm Cache

```sh
rm -rf ~/.helm/cache/archive/*
rm -rf ~/.helm/repository/cache/*
helm repo update
```

## LINKS

- [Just](https://github.com/casey/just) - A Task Runner. Checkout the `Justfile` for details or use `just --list --unsorted` for an quick overview.

---------------------

mogenius-k8s-manager was created by [mogenius](https://mogenius.com) - The Virtual DevOps platform
