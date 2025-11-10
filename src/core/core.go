package core

import (
	"fmt"
	"log/slog"
	"mogenius-operator/src/assert"
	"mogenius-operator/src/config"
	"mogenius-operator/src/helm"
	"mogenius-operator/src/k8sclient"
	mokubernetes "mogenius-operator/src/kubernetes"
	"mogenius-operator/src/services"
	"mogenius-operator/src/shutdown"
	"mogenius-operator/src/store"
	"mogenius-operator/src/utils"
	"mogenius-operator/src/valkeyclient"
	"mogenius-operator/src/websocket"
	"net/url"
	"strconv"
	"sync"
)

type Core interface {
	Initialize() error
	InitializeClusterSecret()
	InitializeWebsocketEventServer()
	InitializeWebsocketApiServer()
	InitializeValkey()
	Link(
		moKubernetes MoKubernetes,
	)
}

type core struct {
	logger                *slog.Logger
	config                config.ConfigModule
	clientProvider        k8sclient.K8sClientProvider
	valkeyClient          valkeyclient.ValkeyClient
	eventConnectionClient websocket.WebsocketClient
	jobConnectionClient   websocket.WebsocketClient

	moKubernetes MoKubernetes
}

func NewCore(
	logger *slog.Logger,
	configModule config.ConfigModule,
	clientProviderModule k8sclient.K8sClientProvider,
	valkeyClient valkeyclient.ValkeyClient,
	eventConnectionClient websocket.WebsocketClient,
	jobConnectionClient websocket.WebsocketClient,
) Core {
	self := &core{}

	self.logger = logger
	self.config = configModule
	self.clientProvider = clientProviderModule
	self.valkeyClient = valkeyClient
	self.eventConnectionClient = eventConnectionClient
	self.jobConnectionClient = jobConnectionClient

	return self
}

func (self *core) Link(
	moKubernetes MoKubernetes,
) {
	self.moKubernetes = moKubernetes
}

func (self *core) InitializeClusterSecret() {
	clusterSecret, err := self.moKubernetes.CreateOrUpdateClusterSecret()
	if err != nil {
		self.logger.Error("failed to retrieve cluster secret", "error", err)
		shutdown.SendShutdownSignal(true)
		select {}
	}

	if clusterSecret.ClusterMfaId != "" {
		err := self.config.TrySet("MO_API_KEY", clusterSecret.ApiKey)
		if err != nil {
			self.logger.Debug("failed to set MO_API_KEY", "error", err)
		}
		err = self.config.TrySet("MO_CLUSTER_NAME", clusterSecret.ClusterName)
		if err != nil {
			self.logger.Debug("failed to set MO_CLUSTER_NAME", "error", err)
		}
		err = self.config.TrySet("MO_CLUSTER_MFA_ID", clusterSecret.ClusterMfaId)
		if err != nil {
			self.logger.Debug("failed to set MO_CLUSTER_MFA_ID", "error", err)
		}

		// migration needed
		if clusterSecret.RedisDataModelVersion == "0" || clusterSecret.RedisDataModelVersion == "" {
			self.logger.Info("Migrating Redis Data Model to version 1 ...")
			err := store.DropKey(self.valkeyClient, self.logger, "logs:*")
			if err != nil {
				self.logger.Error("failed to DropKey (logs)", "error", err)
			}
			err = store.DropKey(self.valkeyClient, self.logger, "pod-stats:*")
			if err != nil {
				self.logger.Error("failed to DropKey (pod-stats)", "error", err)
			}
			err = store.DropKey(self.valkeyClient, self.logger, "traffic-stats:*")
			if err != nil {
				self.logger.Error("failed to DropKey (traffic-stats)", "error", err)
			}
		}
	}
}

func (self *core) InitializeValkey() {
	if !self.config.IsSet("MO_VALKEY_PASSWORD") {
		valkeyPwd, err := mokubernetes.GetValkeyPwd()
		if err != nil {
			self.logger.Error("failed to get valkey password", "error", err)
		}
		if valkeyPwd != nil {
			self.config.Set("MO_VALKEY_PASSWORD", *valkeyPwd)
		}
	}
	err := self.valkeyClient.Connect()
	if err != nil {
		self.logger.Error("failed to connect to valkey", "error", err)
		shutdown.SendShutdownSignal(true)
		select {}
	}
}

func (self *core) InitializeWebsocketEventServer() {
	url, err := url.Parse(self.config.Get("MO_EVENT_SERVER"))
	assert.Assert(err == nil, err)
	err = self.eventConnectionClient.SetUrl(*url)
	assert.Assert(err == nil, err)
	err = self.eventConnectionClient.SetHeader(utils.HttpHeader(""))
	assert.Assert(err == nil, err)
	err = self.eventConnectionClient.Connect()
	if err != nil {
		self.logger.Error("Failed to connect to mogenius api server. Aborting.", "url", url.String(), "error", err.Error())
		shutdown.SendShutdownSignal(true)
		select {}
	}
	assert.Assert(err == nil, "cant connect to mogenius api server - aborting startup", url.String(), err)

	self.config.OnChanged([]string{"MO_API_SERVER"}, func(key string, value string, isSecret bool) {
		url, err := url.Parse(value)
		assert.Assert(err == nil, err)
		err = self.eventConnectionClient.SetUrl(*url)
		if err != nil {
			self.logger.Error("failed to update eventConnectionClient URL", "url", url.String(), "error", err)
		}
	})
	self.config.OnChanged([]string{
		"MO_API_KEY",
		"MO_CLUSTER_MFA_ID",
		"MO_CLUSTER_NAME",
	}, func(key string, value string, isSecret bool) {
		header, err := self.eventConnectionClient.GetHeader()
		assert.Assert(err == nil, err)
		if key == "MO_API_KEY" {
			header["x-authorization"] = []string{value}
		}

		if key == "MO_CLUSTER_MFA_ID" {
			header["x-cluster-mfa-id"] = []string{value}
		}

		if key == "MO_CLUSTER_NAME" {
			header["x-cluster-name"] = []string{value}
		}
		err = self.eventConnectionClient.SetHeader(header)
		if err != nil {
			self.logger.Error("failed to update eventConnectionClient header", "header", header, "error", err)
		}
	})
}

func (self *core) InitializeWebsocketApiServer() {
	url, err := url.Parse(self.config.Get("MO_API_SERVER"))
	assert.Assert(err == nil, err)
	err = self.jobConnectionClient.SetUrl(*url)
	assert.Assert(err == nil, err)
	err = self.jobConnectionClient.SetHeader(utils.HttpHeader(""))
	assert.Assert(err == nil, err)
	err = self.jobConnectionClient.Connect()
	if err != nil {
		self.logger.Error("Failed to connect to mogenius api server. Aborting.", "url", url.String(), "error", err.Error())
		shutdown.SendShutdownSignal(true)
		select {}
	}
	assert.Assert(err == nil, "cant connect to mogenius api server - aborting startup", url.String(), err)

	self.config.OnChanged([]string{"MO_API_SERVER"}, func(key string, value string, isSecret bool) {
		url, err := url.Parse(value)
		assert.Assert(err == nil, err)
		err = self.jobConnectionClient.SetUrl(*url)
		if err != nil {
			self.logger.Error("failed to update jobConnectionClient URL", "url", url.String(), "error", err)
		}
	})
	self.config.OnChanged([]string{
		"MO_API_KEY",
		"MO_CLUSTER_MFA_ID",
		"MO_CLUSTER_NAME",
	}, func(key string, value string, isSecret bool) {
		header, err := self.jobConnectionClient.GetHeader()
		assert.Assert(err == nil, err)
		if key == "MO_API_KEY" {
			header["x-authorization"] = []string{value}
		}

		if key == "MO_CLUSTER_MFA_ID" {
			header["x-cluster-mfa-id"] = []string{value}
		}

		if key == "MO_CLUSTER_NAME" {
			header["x-cluster-name"] = []string{value}
		}
		err = self.jobConnectionClient.SetHeader(header)
		if err != nil {
			self.logger.Error("failed to update jobConnectionClient header", "header", header, "error", err)
		}
	})
}

func (self *core) Initialize() error {
	self.InitializeValkey()
	self.InitializeClusterSecret()

	err := self.moKubernetes.CreateOrUpdateResourceTemplateConfigmap()
	if err != nil {
		return fmt.Errorf("failed to create resource template configmap: %s", err)
	}

	// INIT MOUNTS
	autoMountNfs, err := strconv.ParseBool(self.config.Get("MO_AUTO_MOUNT_NFS"))
	assert.Assert(err == nil, err)
	if autoMountNfs {
		volumesToMount, err := mokubernetes.GetVolumeMountsForK8sManager()
		if err != nil {
			self.logger.Error("GetVolumeMountsForK8sManager", "error", err)
		}
		for _, vol := range volumesToMount {
			mokubernetes.Mount(vol.Namespace, vol.VolumeName, nil)
		}
	}
	mokubernetes.InitOrUpdateCrds()

	var wg sync.WaitGroup

	wg.Go(func() {
		basicApps, userApps := services.InstallDefaultApplications()
		if basicApps != "" || userApps != "" {
			err := utils.ExecuteShellCommandSilent("Installing default applications ...", fmt.Sprintf("%s\n%s", basicApps, userApps))
			self.logger.Info("Seeding Commands ( 🪴 🪴 🪴 )", "userApps", userApps)
			if err != nil {
				self.logger.Error("Error installing default applications", "error", err)
				shutdown.SendShutdownSignal(true)
				select {}
			}
		}
	})

	// Init Helm Config
	wg.Go(func() {
		if err := helm.InitHelmConfig(); err != nil {
			self.logger.Error("failed to initialize Helm config", "error", err)
			return
		}
		self.logger.Info("Helm config initialized")
	})

	wg.Wait()

	return nil
}
