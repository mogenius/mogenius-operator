package networkmonitor_test

import (
	"mogenius-k8s-manager/src/assert"
	"mogenius-k8s-manager/src/networkmonitor"
	"testing"
)

func TestEmptyCgroup(t *testing.T) {
	cgroup := ""
	cne := networkmonitor.NewContainerNetworkEnumerator()
	_, err := cne.GetContainerIdFromCgroupWithPid(cgroup)
	assert.AssertT(t, err == networkmonitor.NoMatchFound)
}

func TestBaseCgroup(t *testing.T) {
	cgroup := "0::/"
	cne := networkmonitor.NewContainerNetworkEnumerator()
	_, err := cne.GetContainerIdFromCgroupWithPid(cgroup)
	assert.AssertT(t, err == networkmonitor.NoMatchFound)
}

func TestBasicCgroup(t *testing.T) {
	cgroup := "0::/system.slice/docker-01db6847f45cdd13b3cba393e5f352c6027761aa8d16a2a86f7b2dd2dc03c232.scope"
	cne := networkmonitor.NewContainerNetworkEnumerator()
	cid, err := cne.GetContainerIdFromCgroupWithPid(cgroup)
	assert.AssertT(t, err == nil, err)
	assert.AssertT(t, cid == "01db6847f45cdd13b3cba393e5f352c6027761aa8d16a2a86f7b2dd2dc03c232", cid)
}

func TestBasicNestedCgroup(t *testing.T) {
	cgroup := "0::/system.slice/docker-8a1a0fe17b454b2cc04fdf4a170cbc2da174e9be52b743ea4309afdb64e655cc.scope/kubepods/besteffort/pod014985db-fbc4-4130-b0bd-ce878c609340/c14ff86ea52c0a9c515430750773a03f4b2d744a5aaaf9e5c8154a5325f4c126"
	cne := networkmonitor.NewContainerNetworkEnumerator()
	cid, err := cne.GetContainerIdFromCgroupWithPid(cgroup)
	assert.AssertT(t, err == nil, err)
	assert.AssertT(t, cid == "c14ff86ea52c0a9c515430750773a03f4b2d744a5aaaf9e5c8154a5325f4c126", cid)
}

func TestNestedCgroupSameEngine(t *testing.T) {
	cgroup := "0::/system.slice/docker-8a1a0fe17b454b2cc04fdf4a170cbc2da174e9be52b743ea4309afdb64e655cc.scope/docker-c14ff86ea52c0a9c515430750773a03f4b2d744a5aaaf9e5c8154a5325f4c126.scope"
	cne := networkmonitor.NewContainerNetworkEnumerator()
	cid, err := cne.GetContainerIdFromCgroupWithPid(cgroup)
	assert.AssertT(t, err == nil, err)
	assert.AssertT(t, cid == "c14ff86ea52c0a9c515430750773a03f4b2d744a5aaaf9e5c8154a5325f4c126", cid)
}

func TestNestedCgroupThreeLayers(t *testing.T) {
	cgroup := "0::/system.slice/docker-8a1a0fe17b454b2cc04fdf4a170cbc2da174e9be52b743ea4309afdb64e655cc.scope/kubepods/besteffort/pod014985db-fbc4-4130-b0bd-ce878c609340/c14ff86ea52c0a9c515430750773a03f4b2d744a5aaaf9e5c8154a5325f4c126/kubepods/besteffort/pod23d64312-0ae9-471b-9af3-6c606f8e4fc5/6bbf0ddf216c09e3e0a2e60331faed61f1c254d72341e9a6db20afd44d45338b"
	cne := networkmonitor.NewContainerNetworkEnumerator()
	cid, err := cne.GetContainerIdFromCgroupWithPid(cgroup)
	assert.AssertT(t, err == nil, err)
	assert.AssertT(t, cid == "6bbf0ddf216c09e3e0a2e60331faed61f1c254d72341e9a6db20afd44d45338b", cid)
}
