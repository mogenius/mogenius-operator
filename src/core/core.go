package core

import (
	"fmt"
	"log/slog"
	"mogenius-k8s-manager/src/assert"
	"mogenius-k8s-manager/src/config"
	"mogenius-k8s-manager/src/helm"
	"mogenius-k8s-manager/src/k8sclient"
	mokubernetes "mogenius-k8s-manager/src/kubernetes"
	"mogenius-k8s-manager/src/services"
	"mogenius-k8s-manager/src/shutdown"
	"mogenius-k8s-manager/src/utils"
	"mogenius-k8s-manager/src/valkeyclient"
	"mogenius-k8s-manager/src/websocket"
	"net/url"
	"strconv"
)

type Core interface {
	Initialize() error
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

func (self *core) Initialize() error {
	clusterSecret, err := self.moKubernetes.CreateOrUpdateClusterSecret()
	if err != nil {
		return fmt.Errorf("failed retrieving cluster secret: %s", err)
	}

	err = self.moKubernetes.CreateOrUpdateResourceTemplateConfigmap()
	if err != nil {
		return fmt.Errorf("failed to create resource template configmap: %s", err)
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
	}

	// connect valkey
	if !self.config.IsSet("MO_VALKEY_PASSWORD") {
		valkeyPwd, err := mokubernetes.GetValkeyPwd()
		if err != nil {
			self.logger.Error("failed to get valkey password", "error", err)
		}
		if valkeyPwd != nil {
			self.config.Set("MO_VALKEY_PASSWORD", *valkeyPwd)
		}
	}
	err = self.valkeyClient.Connect()
	if err != nil {
		return fmt.Errorf("failed to connect to valkey: %s", err)
	}

	// connect to websocket to MO_EVENT_SERVER
	url, err := url.Parse(self.config.Get("MO_EVENT_SERVER"))
	assert.Assert(err == nil, err)
	err = self.eventConnectionClient.SetUrl(*url)
	assert.Assert(err == nil, err)
	err = self.eventConnectionClient.SetHeader(utils.HttpHeader(""))
	assert.Assert(err == nil, err)
	err = self.eventConnectionClient.Connect()
	if err != nil {
		self.logger.Error("Failed to connect to mogenius api server. Aborting.", "url", url.String(), "error", err.Error())
		return fmt.Errorf("failed to connect to mogenius api server: %s", err)
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

	// connect to websocket to MO_EVENT_SERVER
	url, err = url.Parse(self.config.Get("MO_API_SERVER"))
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

	// INIT MOUNTS
	autoMountNfs, err := strconv.ParseBool(self.config.Get("MO_AUTO_MOUNT_NFS"))
	assert.Assert(err == nil, err)
	if autoMountNfs {
		volumesToMount, err := mokubernetes.GetVolumeMountsForK8sManager()
		if err != nil && self.config.Get("MO_STAGE") != utils.STAGE_LOCAL {
			self.logger.Error("GetVolumeMountsForK8sManager", "error", err)
		}
		for _, vol := range volumesToMount {
			mokubernetes.Mount(vol.Namespace, vol.VolumeName, nil)
		}
	}

	go func() {
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
	}()

	mokubernetes.InitOrUpdateCrds()

	mokubernetes.CreateMogeniusContainerRegistryIngress()

	// Init Helm Config
	go func() {
		if err := helm.InitHelmConfig(); err != nil {
			self.logger.Error("failed to initialize Helm config", "error", err)
			return
		}
		self.logger.Info("Helm config initialized")
	}()

	// Init Network Policy Configmap
	go func() {
		if err := mokubernetes.InitNetworkPolicyConfigMap(); err != nil {
			self.logger.Error("Error initializing Network Policy Configmap", "error", err)
			return
		}
		self.logger.Info("Network Policy Configmap initialized")
	}()

	return nil
}
