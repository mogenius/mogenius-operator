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

# LINKS
- [AIR](https://github.com/cosmtrek/air) - Live reload for Go apps


---------------------
mogenius-k8s-manager was created by [mogenius](https://mogenius.com) - The Virtual DevOps platform
