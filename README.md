<p align="center">
  <img src="ui/src/assets/logos/logo-horizontal.svg" alt="drawing" width="500"/>
</p>

<p align="center">
    <a href="https://github.com/mogenius/mogenius-k8s-manager/blob/main/LICENSE">
        <img alt="GitHub License" src="https://img.shields.io/github/license/mogenius/mogenius-k8s-manager?logo=GitHub&style=flat-square">
    </a>
    <a href="https://github.com/mogenius/mogenius-k8s-manager/releases/latest">
        <img alt="GitHub Latest Release" src="https://img.shields.io/github/v/release/mogenius/mogenius-k8s-manager?logo=GitHub&style=flat-square">
    </a>
    <a href="https://github.com/mogenius/mogenius-k8s-manager/releases">
      <img alt="GitHub all releases" src="https://img.shields.io/github/downloads/mogenius/mogenius-k8s-manager/total">
    </a>
    <a href="https://github.com/mogenius/mogenius-k8s-manager">
      <img alt="GitHub repo size" src="https://img.shields.io/github/repo-size/mogenius/mogenius-k8s-manager">
    </a>
    <a href="https://discord.gg/WSxnFHr4qm">
      <img alt="Discord" src="https://img.shields.io/discord/932962925788930088?logo=mogenius">
    </a>
</p>

<p align="center">
  <img src="assets/screenshot1.png" alt="drawing" width="800"/>
</p>
<br />
<br />

# local docker image in docker-desktop kubernetes
RUN:
```
docker build -t localk8smanager --build-arg GOOS=linux --build-arg GOARCH=arm64 --build-arg BUILD_TIMESTAMP="$(date)" --build-arg COMMIT_HASH="XXX" --build-arg GIT_BRANCH=local-development --build-arg VERSION="6.6.6" -f Dockerfile-dev .
```
mogenius-k8s-manager deployment in your cluster change:
```
image: ghcr.io/mogenius/mogenius-k8s-manager:latest
imagePullPolicy: Always

TO:
image: localk8smanager:latest
imagePullPolicy: Never
```
After that simply restart the deployment and you are good to go.

# bolt-db debugging
```
apk add go
go install github.com/br0xen/boltbrowser@latest
cp /db/mogenius-stats-1.db mogenius-stats1.db
cp /db/mogenius-1.db mogenius1.db
/root/go/bin/boltbrowser mogenius-stats1.db
```


# Helm Install
```
helm repo add mo-public helm.mogenius.com/public
helm repo update
helm search repo mogenius-platform
helm install mogenius-platform mo-public/mogenius-platform \
  --set global.cluster_name="mo7-mogenius-io" \
  --set global.api_key="mo_7bf5c2b5-d7bc-4f0e-b8fc-b29d09108928_0hkga6vjum3p1mvezith" \
  --set global.namespace="mogenius"
```

# Helm Upgrade
```
helm repo update
helm upgrade mogenius-platform mo-public/mogenius-platform
```

# Helm Uninstall
```
helm uninstall mogenius-platform
```

# Clean Helm Cache 
```
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
| default_container_registry | docker.io                                   | Default Container Image Registry.    | 
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
- [AIR](https://github.com/cosmtrek/air) - Live reload for Go apps

# Quick Setup Ubuntu
```
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

# Slim setup (IMPORTANT: DOES NOT MAKE THE IMAGE SMALLER IN OUR PARTICULAR CASE)
```
slim build --http-probe=false --exec "curl mogenius.com; git; docker info; helm" \
--include-path-file /usr/local/bin/dockerd \
--include-path-file /usr/local/bin/docker \
--include-path-file /usr/bin/git \
--include-path-file /usr/local/bin/helm \
--include-path-file /usr/bin/curl \
ghcr.io/mogenius/mogenius-k8s-manager-dev:v1.18.19-develop.92
```



---------------------
mogenius-k8s-manager was created by [mogenius](https://mogenius.com) - The Virtual DevOps platform


