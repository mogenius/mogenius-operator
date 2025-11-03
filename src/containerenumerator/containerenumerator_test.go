package containerenumerator_test

import (
	"log/slog"
	"mogenius-k8s-manager/src/assert"
	"mogenius-k8s-manager/src/config"
	"mogenius-k8s-manager/src/containerenumerator"
	"mogenius-k8s-manager/src/k8sclient"
	"mogenius-k8s-manager/src/utils"
	"os"
	"testing"
)

func TestEmptyCgroup(t *testing.T) {
	cgroup := ""
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	configModule := config.NewConfig()
	configModule.Declare(config.ConfigDeclaration{
		Key:          "KUBERNETES_DEBUG",
		DefaultValue: utils.Pointer("false"),
	})
	configModule.Declare(config.ConfigDeclaration{
		Key:          "MO_HOST_PROC_PATH",
		DefaultValue: utils.Pointer("/proc"),
	})
	clientProvider := k8sclient.NewK8sClientProvider(logger, configModule)
	cne := containerenumerator.NewContainerEnumerator(slog.New(slog.NewJSONHandler(os.Stdout, nil)), configModule, clientProvider)
	_, err := cne.GetContainerIdFromCgroupWithPid(cgroup)
	assert.AssertT(t, err == containerenumerator.ErrorNoMatchFound)
}

func TestBaseCgroup(t *testing.T) {
	cgroup := "0::/"
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	configModule := config.NewConfig()
	configModule.Declare(config.ConfigDeclaration{
		Key:          "KUBERNETES_DEBUG",
		DefaultValue: utils.Pointer("false"),
	})
	configModule.Declare(config.ConfigDeclaration{
		Key:          "MO_HOST_PROC_PATH",
		DefaultValue: utils.Pointer("/proc"),
	})
	clientProvider := k8sclient.NewK8sClientProvider(logger, configModule)
	cne := containerenumerator.NewContainerEnumerator(slog.New(slog.NewJSONHandler(os.Stdout, nil)), configModule, clientProvider)
	_, err := cne.GetContainerIdFromCgroupWithPid(cgroup)
	assert.AssertT(t, err == containerenumerator.ErrorNoMatchFound)
}

func TestBasicCgroup(t *testing.T) {
	cgroup := "0::/system.slice/docker-01db6847f45cdd13b3cba393e5f352c6027761aa8d16a2a86f7b2dd2dc03c232.scope"
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	configModule := config.NewConfig()
	configModule.Declare(config.ConfigDeclaration{
		Key:          "KUBERNETES_DEBUG",
		DefaultValue: utils.Pointer("false"),
	})
	configModule.Declare(config.ConfigDeclaration{
		Key:          "MO_HOST_PROC_PATH",
		DefaultValue: utils.Pointer("/proc"),
	})
	clientProvider := k8sclient.NewK8sClientProvider(logger, configModule)
	cne := containerenumerator.NewContainerEnumerator(slog.New(slog.NewJSONHandler(os.Stdout, nil)), configModule, clientProvider)
	cid, err := cne.GetContainerIdFromCgroupWithPid(cgroup)
	assert.AssertT(t, err == nil, err)
	assert.AssertT(t, cid == "01db6847f45cdd13b3cba393e5f352c6027761aa8d16a2a86f7b2dd2dc03c232", cid)
}

func TestBasicNestedCgroup(t *testing.T) {
	cgroup := "0::/system.slice/docker-8a1a0fe17b454b2cc04fdf4a170cbc2da174e9be52b743ea4309afdb64e655cc.scope/kubepods/besteffort/pod014985db-fbc4-4130-b0bd-ce878c609340/c14ff86ea52c0a9c515430750773a03f4b2d744a5aaaf9e5c8154a5325f4c126"
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	configModule := config.NewConfig()
	configModule.Declare(config.ConfigDeclaration{
		Key:          "KUBERNETES_DEBUG",
		DefaultValue: utils.Pointer("false"),
	})
	configModule.Declare(config.ConfigDeclaration{
		Key:          "MO_HOST_PROC_PATH",
		DefaultValue: utils.Pointer("/proc"),
	})
	clientProvider := k8sclient.NewK8sClientProvider(logger, configModule)
	cne := containerenumerator.NewContainerEnumerator(slog.New(slog.NewJSONHandler(os.Stdout, nil)), configModule, clientProvider)
	cid, err := cne.GetContainerIdFromCgroupWithPid(cgroup)
	assert.AssertT(t, err == nil, err)
	assert.AssertT(t, cid == "c14ff86ea52c0a9c515430750773a03f4b2d744a5aaaf9e5c8154a5325f4c126", cid)
}

func TestNestedCgroupSameEngine(t *testing.T) {
	cgroup := "0::/system.slice/docker-8a1a0fe17b454b2cc04fdf4a170cbc2da174e9be52b743ea4309afdb64e655cc.scope/docker-c14ff86ea52c0a9c515430750773a03f4b2d744a5aaaf9e5c8154a5325f4c126.scope"
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	configModule := config.NewConfig()
	configModule.Declare(config.ConfigDeclaration{
		Key:          "KUBERNETES_DEBUG",
		DefaultValue: utils.Pointer("false"),
	})
	configModule.Declare(config.ConfigDeclaration{
		Key:          "MO_HOST_PROC_PATH",
		DefaultValue: utils.Pointer("/proc"),
	})
	clientProvider := k8sclient.NewK8sClientProvider(logger, configModule)
	cne := containerenumerator.NewContainerEnumerator(slog.New(slog.NewJSONHandler(os.Stdout, nil)), configModule, clientProvider)
	cid, err := cne.GetContainerIdFromCgroupWithPid(cgroup)
	assert.AssertT(t, err == nil, err)
	assert.AssertT(t, cid == "c14ff86ea52c0a9c515430750773a03f4b2d744a5aaaf9e5c8154a5325f4c126", cid)
}

func TestNestedCgroupThreeLayers(t *testing.T) {
	cgroup := "0::/system.slice/docker-8a1a0fe17b454b2cc04fdf4a170cbc2da174e9be52b743ea4309afdb64e655cc.scope/kubepods/besteffort/pod014985db-fbc4-4130-b0bd-ce878c609340/c14ff86ea52c0a9c515430750773a03f4b2d744a5aaaf9e5c8154a5325f4c126/kubepods/besteffort/pod23d64312-0ae9-471b-9af3-6c606f8e4fc5/6bbf0ddf216c09e3e0a2e60331faed61f1c254d72341e9a6db20afd44d45338b"
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	configModule := config.NewConfig()
	configModule.Declare(config.ConfigDeclaration{
		Key:          "KUBERNETES_DEBUG",
		DefaultValue: utils.Pointer("false"),
	})
	configModule.Declare(config.ConfigDeclaration{
		Key:          "MO_HOST_PROC_PATH",
		DefaultValue: utils.Pointer("/proc"),
	})
	clientProvider := k8sclient.NewK8sClientProvider(logger, configModule)
	cne := containerenumerator.NewContainerEnumerator(slog.New(slog.NewJSONHandler(os.Stdout, nil)), configModule, clientProvider)
	cid, err := cne.GetContainerIdFromCgroupWithPid(cgroup)
	assert.AssertT(t, err == nil, err)
	assert.AssertT(t, cid == "6bbf0ddf216c09e3e0a2e60331faed61f1c254d72341e9a6db20afd44d45338b", cid)
}

func TestCgroupMatches(t *testing.T) {
	data := map[string]string{
		// Source: Bene's Macbook
		"0::/../eeb5987a709f847212df81424765ef935f947638609a927f2f5bfc814eb30167/kubelet.slice/kubelet-kubepods.slice/kubelet-kubepods-burstable.slice/kubelet-kubepods-burstable-podfdddaf06cf500dcb43e30469b6880934.slice/cri-containerd-70c533efd0df9ba48efd36c4067e96b2b51c0a10a91e185dd96f24f106543120.scope":       "70c533efd0df9ba48efd36c4067e96b2b51c0a10a91e185dd96f24f106543120",
		"0::/../eeb5987a709f847212df81424765ef935f947638609a927f2f5bfc814eb30167/kubelet.slice/kubelet-kubepods.slice/kubelet-kubepods-burstable.slice/kubelet-kubepods-burstable-pod4303e03ea23d5969fa721be37f037c6b.slice/cri-containerd-736ecb86a1dfd13ef2e64c1f4401b4a89e884d94fc50f017fa06689511d17960.scope":       "736ecb86a1dfd13ef2e64c1f4401b4a89e884d94fc50f017fa06689511d17960",
		"0::/../eeb5987a709f847212df81424765ef935f947638609a927f2f5bfc814eb30167/kubelet.slice/kubelet-kubepods.slice/kubelet-kubepods-burstable.slice/kubelet-kubepods-burstable-pod4303e03ea23d5969fa721be37f037c6b.slice/cri-containerd-5b9952d6035b1285f3223f7c7d8b6e2cb776231dac71076e1aba96720426aa4b.scope":       "5b9952d6035b1285f3223f7c7d8b6e2cb776231dac71076e1aba96720426aa4b",
		"0::/../43f65646702bb950e9fc17c6f1c90448fc4d4768fd5a3d6060ae3d3f7bd8c7d8/kubelet.slice/kubelet-kubepods.slice/kubelet-kubepods-besteffort.slice/kubelet-kubepods-besteffort-podcf22a4ea_b71a_47e6_9efb_2015ede2a081.slice/cri-containerd-871eb3c6bf966f59e8f0924477971b70981a40eec2baa54ddbae5cfa2d91a185.scope": "871eb3c6bf966f59e8f0924477971b70981a40eec2baa54ddbae5cfa2d91a185",
	}
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	configModule := config.NewConfig()
	configModule.Declare(config.ConfigDeclaration{
		Key:          "KUBERNETES_DEBUG",
		DefaultValue: utils.Pointer("false"),
	})
	configModule.Declare(config.ConfigDeclaration{
		Key:          "MO_HOST_PROC_PATH",
		DefaultValue: utils.Pointer("/proc"),
	})
	clientProvider := k8sclient.NewK8sClientProvider(logger, configModule)
	cne := containerenumerator.NewContainerEnumerator(slog.New(slog.NewJSONHandler(os.Stdout, nil)), configModule, clientProvider)
	errorCount := 0
	for cgroup, expectedContainerId := range data {
		cid, err := cne.GetContainerIdFromCgroupWithPid(cgroup)
		if err != nil {
			t.Errorf("received error from parsing cgroup: %s", err.Error())
			errorCount += 1
			continue
		}
		if cid != expectedContainerId {
			t.Errorf(`expected cid "%s" got "%s"`, expectedContainerId, cid)
			errorCount += 1
			continue
		}
	}

	assert.AssertT(t, errorCount == 0, "expected 0 errors", errorCount)
}
