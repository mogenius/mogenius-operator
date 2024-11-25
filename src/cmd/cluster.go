package cmd

import (
	"fmt"
	"log/slog"
	"mogenius-k8s-manager/src/assert"
	"mogenius-k8s-manager/src/config"
	"mogenius-k8s-manager/src/controllers"
	"mogenius-k8s-manager/src/crds"
	dbstats "mogenius-k8s-manager/src/db-stats"
	"mogenius-k8s-manager/src/dtos"
	"mogenius-k8s-manager/src/helm"
	"mogenius-k8s-manager/src/httpService"
	mokubernetes "mogenius-k8s-manager/src/kubernetes"
	"mogenius-k8s-manager/src/logging"
	"mogenius-k8s-manager/src/services"
	servicesExternal "mogenius-k8s-manager/src/services-external"
	"mogenius-k8s-manager/src/shutdown"
	socketclient "mogenius-k8s-manager/src/socket-client"
	"mogenius-k8s-manager/src/store"
	"mogenius-k8s-manager/src/structs"
	"mogenius-k8s-manager/src/utils"
	"mogenius-k8s-manager/src/version"
	"mogenius-k8s-manager/src/watcher"
	"mogenius-k8s-manager/src/xterm"
	"strconv"
)

func RunCluster(logManagerModule logging.LogManagerModule, configModule *config.Config, cmdLogger *slog.Logger) error {
	go func() {
		versionModule := version.NewVersion()
		watcherModule := watcher.NewWatcher()

		configModule.Validate()

		var err error
		helm.Setup(logManagerModule, configModule)
		err = mokubernetes.Setup(logManagerModule, configModule, watcherModule)
		assert.Assert(err == nil, err)
		controllers.Setup(logManagerModule)
		crds.Setup(logManagerModule)
		dbstats.Setup(logManagerModule, configModule)
		dtos.Setup(logManagerModule)
		services.Setup(logManagerModule, configModule)
		servicesExternal.Setup(logManagerModule, configModule)
		socketclient.Setup(logManagerModule, configModule)
		store.Setup(logManagerModule)
		structs.Setup(logManagerModule, configModule)
		utils.Setup(logManagerModule, configModule)
		xterm.Setup(logManagerModule, configModule)
		httpApi := httpService.NewHttpApi(logManagerModule, configModule)

		versionModule.PrintVersionInfo()
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

		store.Start()
		dbstats.Start()
		go httpApi.Run(":1337")
		err = mokubernetes.Start()
		if err != nil {
			cmdLogger.Error("Error starting kubernetes service", "error", err)
			shutdown.SendShutdownSignal(true)
			select {}
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
			services.DISABLEQUEUE = true
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
			services.DISABLEQUEUE = false
			services.ProcessQueue() // Process the queue maybe there are builds left to build
		}()

		mokubernetes.InitOrUpdateCrds()

		go structs.ConnectToEventQueue()
		go structs.ConnectToJobQueue()

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

		socketclient.StartK8sManager()
	}()

	shutdown.Listen()

	return nil
}
