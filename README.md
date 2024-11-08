<p align="center">
  <img src="https://imagedelivery.net/T7YEW5IAgZJ0dY4-LDTpyQ/3ae4fcf0-289c-48d2-3323-d2c5bc932300/detail" alt="drawing" width="200"/>
</p>

# run locally
setup your .env file with the following content:
```.env
MO_API_KEY=
MO_CLUSTER_NAME=
MO_CLUSTER_MFA_ID=
MO_STAGE=
```
Then run:

```sh
set -o allexport; source .env; set +o allexport
go run main.go cluster
```

# run local instance with go

Adjust the config `~/.mogenius-k8s-manager/config.yaml` (might need to be copied there from [here](utils/config/config-local.yaml))
Assuming you already have a [prod operator running](https://docs.mogenius.com/cluster-management/installing-mogenius#mogenius-cli), you can adjust the deployment of the operator with e.g.:
`kubectl edit deployments -n mogenius mogenius-k8s-manager`

Get the api-key, mfa-id and cluster-name from the operator secret `mogenius/mogenius` and adjust the config.yaml accordingly.

change the replicas to 0, then you can run the local instance with:

```sh
go run -trimpath main.go cluster
```

# local docker image in docker-desktop kubernetes

RUN:

```sh
docker build -t localk8smanager --build-arg GOOS=linux --build-arg GOARCH=arm64 --build-arg BUILD_TIMESTAMP="$(date)" --build-arg COMMIT_HASH="XXX" --build-arg GIT_BRANCH=local-development --build-arg VERSION="6.6.6" -f Dockerfile .
```

Assuming you already have a [prod operator running](https://docs.mogenius.com/cluster-management/installing-mogenius#mogenius-cli), you can adjust the deployment of the operator with e.g.:
`kubectl edit deployments -n mogenius mogenius-k8s-manager`

```
FROM:
image: ghcr.io/mogenius/mogenius-k8s-manager:latest
imagePullPolicy: Always

TO:
image: localk8smanager:latest
imagePullPolicy: Never
```
After that simply restart the deployment and you are good to go.

# bolt-db debugging

```sh
apk add go
go install github.com/br0xen/boltbrowser@latest
cp /db/mogenius-stats-1.db mogenius-stats1.db
cp /db/mogenius-1.db mogenius1.db
/root/go/bin/boltbrowser mogenius-stats1.db
```

# Upgrade Modules

```sh
go get -u ./...
go mod tidy
```

# Testing

```sh
go test -v ./...

# clean cache
go clean -testcache
```

# Helm Install

```sh
helm repo add mo-public helm.mogenius.com/public
helm repo update
helm search repo mogenius-platform
helm install mogenius-platform mo-public/mogenius-platform \
  --set global.cluster_name="mo7-mogenius-io" \
  --set global.api_key="mo_7bf5c2b5-d7bc-4f0e-b8fc-b29d09108928_0hkga6vjum3p1mvezith" \
  --set global.namespace="mogenius"
```

# Helm Upgrade

```sh
helm repo update
helm upgrade mogenius-platform mo-public/mogenius-platform
```

# Helm Uninstall

```sh
helm uninstall mogenius-platform
```

# Clean Helm Cache

```sh
rm -rf ~/.helm/cache/archive/*
rm -rf ~/.helm/repository/cache/*
helm repo update
```

# ENV VARS

| NAME                       | DEFAULT                                     | DESCRIPTION |
| :---                       | :----                                       | ---: |
| api_key                    | [your_key]                                  | Api Key to access the server     |
| cluster_name               | [your_name]                                 | The Name of the Kubernetes Cluster.     | 
| own_namespace              | mogenius                                    | The Namespace of mogenius platform.     | 
| cluster_mfa_id             | [auto_generated]                            | UUID of the Kubernetes Cluster for MFA purpose.       | 
| run_in_cluster             | true                                        | If set to true, the application will run in the cluster (using the service account token). Otherwise it will try to load your local default context.     |
| bbolt_db_path              | bbolt_db_path                               | Path to the bbolt database. This db stores build-related information. |
| api_ws_server              | 127.0.0.1:8080                              | This depends on your stage. local/dev/prod. Prod: "k8s-ws.mogenius.com"     | 
| api_ws_path                | /ws                                         | The path of the api server.    | 
| event_server               | 127.0.0.1:8080                              | This depends on your stage. local/dev/prod. Prod: "k8s-dispatcher.mogenius.com"     | 
| event_path                 | /ws                                         | The path of the api server.     | 
| stage                      | prod                                        | Stage environment    | 
| log_kubernetes_events      | false                                       | If set to true, all kubernetes events will be logged to std-out.    | 
| default_mount_path         | /mo-data                                    | The mogenius mounts will be attached to this folder inside the k8s-manager.   | 
| ignore_namespaces          | ["kube-system"]                             | These namespaces will be ignored.   | 
| auto_mount_nfs             | true                                        | If set to true, nfs pvc will automatically be mounted.      | 
| ignore_resources_backup    | ["events.k8s.io/v1", "events.k8s.io/v1beta1", "metrics.k8s.io/v1beta1", "discovery.k8s.io/v1"]    |   List of all ignored resources while backup.     | 
| check_for_updates          | 3600                                        | Time interval between update checks in seconds.      | 
| helm_index                 | https://helm.mogenius.com/public/index.yaml | URL of the helm index file.      | 
| nfs_pod_prefix             | nfs-server-pod                              | A prefix for the nfs-server pod. This will always be applied in order to detect the pod. | 
| max_build_time             | 3600                                        | Timeout after when builds will be canceled in seconds.  (1h default) | 
| max_scan_time              | 200                                         | Timeout after when vulnerability scans will be canceled in seconds. | 
| git_user_email             | git@mogenius.com                            | Email address which is used when interacting with git. | 
| git_user_name              | mogenius git-user                           | User name which is used when interacting with git. | 
| git_default_branch         | main                                        | Default branch name which is used when creating a repository. | 
| git_add_ignored_file       | false                                       | Gits behaviour when adding ignored files. | 

# LINKS

- [Just](https://github.com/casey/just) - A Task Runner. Checkout the `Justfile` for details or use `just -l` for an quick overview.
- [AIR](https://github.com/cosmtrek/air) - Live reload for Go apps

# Quick Setup Ubuntu

```sh
#!/bin/bash

# INSTALL K3S
curl -sfL https://get.k3s.io | sh
echo 'export KUBECONFIG=/etc/rancher/k3s/k3s.yaml' >> ~/.bashrc
chmod a+r /etc/rancher/k3s/k3s.yaml
source ~/.bashrc

# INSTALL HELM
curl -fsSL -o get_helm.sh https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3
chmod 700 get_helm.sh
./get_helm.sh

# INSTALL K9S
curl -L -O https://github.com/derailed/k9s/releases/download/v0.30.8/k9s_Linux_amd64.tar.gz
tar -xzf k9s_Linux_amd64.tar.gz
mv k9s /usr/local/bin/.

# CLEANUP
rm LICENSE README.md k9s_Linux_amd64.tar.gz get_helm.sh
```

# Lint

```sh
golangci-lint run --fast=false --sort-results --max-same-issues=0 --timeout=1h
```

# Slim setup (IMPORTANT: DOES NOT MAKE THE IMAGE SMALLER IN OUR PARTICULAR CASE)

```sh
slim build --http-probe=false --exec "curl mogenius.com; git; docker info; helm" \
    --include-path-file /usr/local/bin/dockerd \
    --include-path-file /usr/local/bin/docker \
    --include-path-file /usr/local/bin/helm \
    --include-path-file /usr/bin/curl \
ghcr.io/mogenius/mogenius-k8s-manager-dev:v1.18.19-develop.92
```

---------------------

mogenius-k8s-manager was created by [mogenius](https://mogenius.com) - The Virtual DevOps platform
