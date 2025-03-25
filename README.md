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

## .env update
```
if [[ -f .env ]]; then export $(grep -v '^#' .env | xargs); fi
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

## eBPF Development
### how to run the example ebpf program?
  - run `go generate ./ebpf`
  - run `sudo go run ./cmd/main.go`
  OR
  - run `just ebpf`

### Generate load for eBPF to measure
  - run `ping -i 0.002 127.0.0.1` in another terminal

### docker-compose
  - run `docker-compose build`
  - run `docker-compose up -d`
  - run `docker-compose exec mogenius-operator sh`
  - run `just run`
  - run `nsenter --target=40368 --net=/proc/40368/ns/net -n ip -o --json link | jq`

### eBPF Docker examples
test if eBPF is still working:
```bash
docker build -t my-go-ebpf-app -f Dockerfile-Dev-Environment . && docker run --rm -it --privileged --pid=host --net=host my-go-ebpf-app sh -c "cd /app && just ebpf"
```

standalone container:
```bash
docker build -t my-go-ebpf-app -f Dockerfile-Dev-Environment . && docker run --rm -it --privileged --pid=host --net=host my-go-ebpf-app sh
```

standalone with your local KUBECONFIG & .env 🚀🚀🚀
```bash
docker build -t my-go-ebpf-app -f Dockerfile-Dev-Environment . && docker run --rm -it -v $KUBECONFIG:/root/.kube/config:ro -v "$(pwd)/.env:/app/.env:ro" --privileged --pid=host --net=host my-go-ebpf-app sh
```

to access valkey from within the container (and to background it):
```bash
kubectl port-forward svc/mogenius-k8s-manager-valkey 6379:6379 -n mogenius &
```

## LINKS

- [Just](https://github.com/casey/just) - A Task Runner. Checkout the `Justfile` for details or use `just --list --unsorted` for an quick overview.

---------------------

mogenius-k8s-manager was created by [mogenius](https://mogenius.com) - The Virtual DevOps platform
