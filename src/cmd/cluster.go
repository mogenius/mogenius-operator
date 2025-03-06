package cmd

import (
	"fmt"
	"log/slog"
	"mogenius-k8s-manager/src/assert"
	"mogenius-k8s-manager/src/config"
	"mogenius-k8s-manager/src/helm"
	mokubernetes "mogenius-k8s-manager/src/kubernetes"
	"mogenius-k8s-manager/src/logging"
	"mogenius-k8s-manager/src/services"
	"mogenius-k8s-manager/src/shutdown"
	"mogenius-k8s-manager/src/store"
	"mogenius-k8s-manager/src/structs"
	"mogenius-k8s-manager/src/utils"
	"net/url"
	"strconv"
)

func RunCluster(logManagerModule logging.LogManagerModule, configModule *config.Config, cmdLogger *slog.Logger) error {
	go func() {
		defer func() {
			shutdown.SendShutdownSignal(true)
		}()
		configModule.Validate()

		systems := InitializeSystems(logManagerModule, configModule, cmdLogger)

		// DB (valkey) setup
		valkeyPwd, err := mokubernetes.GetValkeyPwd()
		if err != nil {
			cmdLogger.Error("Error getting valkey password", "error", err)
		}
		if valkeyPwd != nil {
			configModule.Set("MO_VALKEY_PASSWORD", *valkeyPwd)
		}
		err = systems.redisModule.Connect()
		assert.Assert(err == nil, err)
		err = systems.dbstatsModule.Start()
		assert.Assert(err == nil, err)
		// clean valkey on every startup
		err = store.DropAllResourcesFromValkey()
		if err != nil {
			cmdLogger.Error("Error dropping all resources from valkey", "error", err)
		}
		err = store.DropAllPodEventsFromValkey()
		if err != nil {
			cmdLogger.Error("Error dropping all pod events from valkey", "error", err)
		}

		systems.versionModule.PrintVersionInfo()
		cmdLogger.Info("🖥️  🖥️  🖥️  CURRENT CONTEXT", "foundContext", mokubernetes.CurrentContextName())

		clusterSecret, err := mokubernetes.CreateOrUpdateClusterSecret(nil)
		if err != nil {
			cmdLogger.Error("Error retrieving cluster secret. Aborting.", "error", err)
			shutdown.SendShutdownSignal(true)
			select {}
		}
		_, err = mokubernetes.CreateAndUpdateClusterConfigmap()
		if err != nil {
			cmdLogger.Error("Error retrieving cluster configmap. Aborting.", "error", err.Error())
			shutdown.SendShutdownSignal(true)
			select {}
		}
		err = mokubernetes.CreateOrUpdateResourceTemplateConfigmap()
		if err != nil {
			cmdLogger.Error("Error creating resource template configmap", "error", err)
			shutdown.SendShutdownSignal(true)
			select {}
		}

		utils.SetupClusterSecret(clusterSecret)

		go systems.httpApi.Run(configModule.Get("MO_HTTP_ADDR"))

		url, err := url.Parse(configModule.Get("MO_EVENT_SERVER"))
		assert.Assert(err == nil, err)
		err = systems.eventConnectionClient.SetUrl(*url)
		assert.Assert(err == nil, err)
		err = systems.eventConnectionClient.SetHeader(utils.HttpHeader(""))
		assert.Assert(err == nil, err)
		err = systems.eventConnectionClient.Connect()
		if err != nil {
			cmdLogger.Error("Failed to connect to mogenius api server. Aborting.", "url", url.String(), "error", err.Error())
			shutdown.SendShutdownSignal(true)
			select {}
		}
		assert.Assert(err == nil, "cant connect to mogenius api server - aborting startup", url.String(), err)

		configModule.OnChanged([]string{"MO_API_SERVER"}, func(key string, value string, isSecret bool) {
			url, err := url.Parse(value)
			assert.Assert(err == nil, err)
			err = systems.eventConnectionClient.SetUrl(*url)
			if err != nil {
				cmdLogger.Error("failed to update eventConnectionClient URL", "url", url.String(), "error", err)
			}
		})
		configModule.OnChanged([]string{
			"MO_API_KEY",
			"MO_CLUSTER_MFA_ID",
			"MO_CLUSTER_NAME",
		}, func(key string, value string, isSecret bool) {
			header, err := systems.eventConnectionClient.GetHeader()
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
			err = systems.eventConnectionClient.SetHeader(header)
			if err != nil {
				cmdLogger.Error("failed to update eventConnectionClient header", "header", header, "error", err)
			}
		})

		err = mokubernetes.Start(systems.eventConnectionClient)
		if err != nil {
			cmdLogger.Error("Error starting kubernetes service", "error", err)
			return
		}

		// INIT MOUNTS
		autoMountNfs, err := strconv.ParseBool(configModule.Get("MO_AUTO_MOUNT_NFS"))
		assert.Assert(err == nil, err)
		if autoMountNfs {
			volumesToMount, err := mokubernetes.GetVolumeMountsForK8sManager()
			if err != nil && configModule.Get("MO_STAGE") != utils.STAGE_LOCAL {
				cmdLogger.Error("GetVolumeMountsForK8sManager", "error", err)
			}
			for _, vol := range volumesToMount {
				mokubernetes.Mount(vol.Namespace, vol.VolumeName, nil)
			}
		}

		go func() {
			basicApps, userApps := services.InstallDefaultApplications()
			if basicApps != "" || userApps != "" {
				err := utils.ExecuteShellCommandSilent("Installing default applications ...", fmt.Sprintf("%s\n%s", basicApps, userApps))
				cmdLogger.Info("Seeding Commands ( 🪴 🪴 🪴 )", "userApps", userApps)
				if err != nil {
					cmdLogger.Error("Error installing default applications", "error", err)
					shutdown.SendShutdownSignal(true)
					select {}
				}
			}
		}()

		mokubernetes.InitOrUpdateCrds()

		url, err = url.Parse(configModule.Get("MO_API_SERVER"))
		assert.Assert(err == nil, err)
		err = systems.jobConnectionClient.SetUrl(*url)
		assert.Assert(err == nil, err)
		err = systems.jobConnectionClient.SetHeader(utils.HttpHeader(""))
		assert.Assert(err == nil, err)
		err = systems.jobConnectionClient.Connect()
		if err != nil {
			cmdLogger.Error("Failed to connect to mogenius api server. Aborting.", "url", url.String(), "error", err.Error())
			shutdown.SendShutdownSignal(true)
			select {}
		}
		assert.Assert(err == nil, "cant connect to mogenius api server - aborting startup", url.String(), err)

		configModule.OnChanged([]string{"MO_API_SERVER"}, func(key string, value string, isSecret bool) {
			url, err := url.Parse(value)
			assert.Assert(err == nil, err)
			err = systems.jobConnectionClient.SetUrl(*url)
			if err != nil {
				cmdLogger.Error("failed to update jobConnectionClient URL", "url", url.String(), "error", err)
			}
		})
		configModule.OnChanged([]string{
			"MO_API_KEY",
			"MO_CLUSTER_MFA_ID",
			"MO_CLUSTER_NAME",
		}, func(key string, value string, isSecret bool) {
			header, err := systems.jobConnectionClient.GetHeader()
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
			err = systems.jobConnectionClient.SetHeader(header)
			if err != nil {
				cmdLogger.Error("failed to update jobConnectionClient header", "header", header, "error", err)
			}
		})

		go structs.ConnectToEventQueue(systems.eventConnectionClient)
		go structs.ConnectToJobQueue(systems.jobConnectionClient)

		mokubernetes.CreateMogeniusContainerRegistryIngress()

		// Init Helm Config
		go func() {
			if err := helm.InitHelmConfig(); err != nil {
				cmdLogger.Error("Error initializing Helm Config", "error", err)
			} else {
				cmdLogger.Info("Helm Config initialized")
			}
		}()

		// Init Network Policy Configmap
		go func() {
			if err := mokubernetes.InitNetworkPolicyConfigMap(); err != nil {
				cmdLogger.Error("Error initializing Network Policy Configmap", "error", err)
			} else {
				cmdLogger.Info("Network Policy Configmap initialized")
			}
		}()

		systems.socketApi.Run()
	}()

	shutdown.Listen()

	return nil
}
