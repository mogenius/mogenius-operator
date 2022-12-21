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

> :warning: **This is the first public mogenius-k8s-manager release. If you experience issues we are very happy about your feedback or contribution.**

# Table of contents
- [What is mogenius-k8s-manager?](#what-is-mogenius-k8s-manager)
- [Installation](#installation)
- [How does mogenius-k8s-manager work?](#how-does-mogenius-k8s-manager-work)
- [Configuration](#configuration)
- [Known Issues](#known-issues)
- [Credits](#credits)

# What is mogenius-k8s-manager?
mogenius-k8s-manager is a leight-weight Kubernetes traffic monitoring tool that can be deployed as daemonset in every cluster. Local and external traffic is monitored in real time and can be viewed through a simple web interface. This allows identification of high traffic applications, understanding container relations and optimizing Kubernetes setups. Even works with slim containers 🙃

# Installation
Just download it and run it. Don't forget to set the right cluster using kubectx or whatever tool you prefer.

## Mac
```

mogenius-k8s-manager_link=$(curl -s https://api.github.com/repos/mogenius/mogenius-k8s-manager/releases/latest | grep browser_download_url | cut -d '"' -f 4 | grep darwin  )

curl -s -L -o mogenius-k8s-manager ${mogenius-k8s-manager_link}

chmod 755 mogenius-k8s-manager

./mogenius-k8s-manager start
```

## Linux
```

mogenius-k8s-manager_link=$(curl -s https://api.github.com/repos/mogenius/mogenius-k8s-manager/releases/latest | grep browser_download_url | cut -d '"' -f 4 | grep linux )

curl -s -L -o mogenius-k8s-manager ${mogenius-k8s-manager_link}

chmod 755 mogenius-k8s-manager

./mogenius-k8s-manager start
```

## Windows
```

curl.exe -LO "https://github.com/mogenius/mogenius-k8s-manager/releases/download/v1.0.4/mogenius-k8s-manager-1.0.4-windows-amd64"
mogenius-k8s-manager-1.0.4-windows-amd64 start

```

⚠️ ⚠️ ⚠️ IMPORTANT: be sure to select the right context before running mogenius-k8s-manager ⚠️⚠️⚠️

```
./mogenius-k8s-manager start
```

# How does mogenius-k8s-manager work?
mogenius-k8s-manager will run a series of tasks in order to run within your cluster. Here's what happens in detail once you launch mogenius-k8s-manager:
1. A mogenius-k8s-manager namespace is created to isolate it from other workloads.
2. Set up RBAC for proper access control.
3. Start a memory-only redis. All DaemonSets will drop their data here.
4. Create a DaemonSet to scrape data from all nodes.
5. Launch a redis service to make the redis accessible via port forwarding.
6. Set up port forwarding for the redis service.
7. Start a web service locally to expose the mogenius-k8s-manager web application (which gathers the data from redis).
8. Launch the web application in a browser.

In other words: The DaemonSet will inspect all packages of the node (using special deployment capabilities). The data will be captured, summarized and sent to the redis (using certain thresholds). The local web app will gather the data from the redis periodically and display the data inside the web application.

As soon as you close the cli app (CTRL + C) the application will be removed from your cluster and the UI will stop receiving updates. When you restart it, it will resume gathering data without storing a state (meaning you start from 0).

To completely remove mogenius-k8s-manager from your cluster run:
```
./mogenius-k8s-manager clean
```

# TESTED WITH
We already checked multiple CNI configurations.

| Provider      | CNI         | Prefix    | K8S    | Tested|
| ------------- |:----------- |:---------:|:---------:| -----:|
| Azure         | Azure CNI   |       azv | 1.24.X, 1.23.x, 1.22.x |    👍 |
| Azure         | -           |      veth | 1.24.X, 1.23.x, 1.22.x |    👍 |
| Azure         | Calico      |      cali | 1.24.X, 1.23.x, 1.22.x |    👍 |
| DigitalOcean  | Cillium     |       lxc | 1.24.X, 1.23.X         |    👍 |
| AWS           | CNI         |       eni | 1.24.X, 1.23.x, 1.22.x |    👍 |
| AWS           | -           |       - |         - |      ❓ |
| AWS           | Calico      |       - |         - |      ❓ |
| Google Cloud  | CNI         |       - |         - |      ❓ |

If you have tested additional configurations: Let us know what works :-)
💥: 1.25.X is not yet supported (at least we saw a problem with Digital Ocean) because the CONFIG_CGROUP_PIDS flag is disabled by default.

# API
You can use following API endpoints to access the raw data:
```
http://127.0.0.1:1337/traffic/overview
http://127.0.0.1:1337/traffic/total
http://127.0.0.1:1337/traffic/flow
```

# Known Issues
- Sometimes port forwarding doesn't get established and mogenius-k8s-manager doesn't recognize it. Please just hit CTRL + C to recover from this state.

# Credits
We took great inspiration (and some lines of code) from [Mizu](https://github.com/up9inc/mizu).</br>
Awesome work from the folks at [UP9](https://up9.com/).</br>
Notice: The project has been renamed to Kubeshark and moved to https://github.com/kubeshark/kubeshark.</br>

---------------------
mogenius-k8s-manager was created by [mogenius](https://mogenius.com) - The Virtual DevOps platform
