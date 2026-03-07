package cmd

import (
	"fmt"
	"log/slog"
	"mogenius-operator/src/ai"
	"mogenius-operator/src/argocd"
	"mogenius-operator/src/assert"
	"mogenius-operator/src/config"
	"mogenius-operator/src/containerenumerator"
	"mogenius-operator/src/core"
	"mogenius-operator/src/cpumonitor"
	"mogenius-operator/src/helm"
	mokubernetes "mogenius-operator/src/kubernetes"
	"mogenius-operator/src/logging"
	"mogenius-operator/src/networkmonitor"
	"mogenius-operator/src/rammonitor"
	"mogenius-operator/src/services"
	"mogenius-operator/src/shutdown"
	"mogenius-operator/src/store"
	"mogenius-operator/src/structs"
	"mogenius-operator/src/valkeyclient"
	"mogenius-operator/src/watcher"
	"mogenius-operator/src/websocket"
	"mogenius-operator/src/xterm"
	"strconv"
)

// clusterSystems holds all services needed for the full operator cluster mode.
// It embeds baseSystems so that fields like versionModule and logger are promoted.
type clusterSystems struct {
	baseSystems
	valkeyClient          valkeyclient.ValkeyClient
	watcherModule         watcher.WatcherModule
	jobClients            []websocket.WebsocketClient
	eventConnectionClient websocket.WebsocketClient
	networkmonitor        networkmonitor.NetworkMonitor
	mocore                core.Core
	moKubernetes          core.MoKubernetes
	workspaceManager      core.WorkspaceManager
	apiModule             core.Api
	socketApi             core.SocketApi
	httpApi               core.HttpService
	xtermService          core.XtermService
	aiWebsocketConnection ai.AiWebsocketConnection
	valkeyLoggerService   core.ValkeyLogger
	podStatsCollector     core.PodStatsCollector
	nodeMetricsCollector  core.NodeMetricsCollector
	dbstatsService        core.ValkeyStatsDb
	leaderElector         core.LeaderElector
	reconciler            core.Reconciler
	sealedSecret          core.SealedSecretManager
	argocd                argocd.Argocd
	aiManager             ai.AiManager
}

// initializeClusterSystems layers all cluster-mode services on top of the shared base.
func initializeClusterSystems(
	base baseSystems,
	logManagerModule logging.SlogManager,
	configModule *config.Config,
	valkeyLogChannel chan logging.LogLine,
) clusterSystems {
	assert.Assert(logManagerModule != nil)
	assert.Assert(configModule != nil)
	assert.Assert(base.valkeyClient != nil)
	assert.Assert(base.clientProvider != nil)
	assert.Assert(base.logger != nil)

	watcherModule := watcher.NewWatcher(logManagerModule.CreateLogger("watcher"), base.clientProvider)
	shutdown.Add(watcherModule.UnwatchAll)

	numApiClients, err := strconv.Atoi(configModule.Get("MO_API_SERVER_CLIENTS"))
	assert.Assert(err == nil, "MO_API_SERVER_CLIENTS must be a valid integer", err)
	if numApiClients < 1 {
		numApiClients = 1
	}
	jobClients := make([]websocket.WebsocketClient, numApiClients)
	for i := range numApiClients {
		jobClients[i] = websocket.NewWebsocketClient(logManagerModule.CreateLogger(fmt.Sprintf("websocket-job-client-%d", i)))
		shutdown.Add(jobClients[i].Terminate)
	}
	eventConnectionClient := websocket.NewWebsocketClient(logManagerModule.CreateLogger("websocket-events-client"))
	shutdown.Add(eventConnectionClient.Terminate)

	containerEnumerator := containerenumerator.NewContainerEnumerator(logManagerModule.CreateLogger("container-enumerator"), configModule, base.clientProvider)
	cpuMonitor := cpumonitor.NewCpuMonitor(logManagerModule.CreateLogger("cpu-monitor"), configModule, base.clientProvider, containerEnumerator)
	ramMonitor := rammonitor.NewRamMonitor(logManagerModule.CreateLogger("ram-monitor"), configModule, base.clientProvider, containerEnumerator)
	networkMonitor := networkmonitor.NewNetworkMonitor(logManagerModule.CreateLogger("network-monitor"), configModule, containerEnumerator, configModule.Get("MO_HOST_PROC_PATH"))

	ownerCacheService := store.NewOwnerCacheService(logManagerModule.CreateLogger("owner-cache"), configModule)
	aiManager := ai.NewAiManager(logManagerModule.CreateLogger("ai-manager"), base.valkeyClient, configModule, ownerCacheService, eventConnectionClient, mokubernetes.GetSecret)

	// Initialize AI tools with kubernetes functions.
	ai.K8sUpdateUnstructuredResource = mokubernetes.UpdateUnstructuredResource
	ai.K8sDeleteUnstructuredResource = mokubernetes.DeleteUnstructuredResource
	ai.K8sCreateUnstructuredResource = mokubernetes.CreateUnstructuredResource
	ai.K8sGetUnstructuredResourceFromStore = mokubernetes.GetUnstructuredResourceFromStore
	ai.K8sGetPodLogs = mokubernetes.GetPodLogs

	// Package-level setups for subsystems that are cluster-mode-only.
	helm.Setup(logManagerModule, configModule, base.valkeyClient)
	services.Setup(logManagerModule, configModule, base.clientProvider)
	structs.Setup(logManagerModule)
	xterm.Setup(logManagerModule, base.valkeyClient)
	err = store.Setup(logManagerModule, base.valkeyClient, configModule.Get("MO_AUDIT_LOG_LIMIT"))
	assert.Assert(err == nil, err)

	argocdModule := argocd.NewArgoCd(logManagerModule, configModule, base.clientProvider, base.valkeyClient)
	workspaceManager := core.NewWorkspaceManager(configModule, base.clientProvider)
	apiModule := core.NewApi(logManagerModule.CreateLogger("api"), base.valkeyClient, configModule)
	aiApi := core.NewAiApi(logManagerModule.CreateLogger("apApi"), aiManager)
	httpApi := core.NewHttpApi(logManagerModule, configModule)
	socketApi := core.NewSocketApi(logManagerModule.CreateLogger("socketapi"), configModule, jobClients, eventConnectionClient, base.valkeyClient, argocdModule)
	xtermService := core.NewXtermService(logManagerModule.CreateLogger("xterm-service"))
	aiWebsocketConnection := ai.NewAiWebsocketConnection(logManagerModule.CreateLogger("ai-websocket-connection"), aiManager)
	valkeyLoggerService := core.NewValkeyLogger(base.valkeyClient, valkeyLogChannel)
	dbstatsService := core.NewValkeyStatsModule(logManagerModule.CreateLogger("db-stats"), configModule, base.valkeyClient, ownerCacheService)
	podStatsCollector := core.NewPodStatsCollector(logManagerModule.CreateLogger("pod-stats-collector"), configModule, base.clientProvider)
	nodeMetricsCollector := core.NewNodeMetricsCollector(
		logManagerModule.CreateLogger("traffic-collector"),
		configModule,
		base.clientProvider,
		cpuMonitor,
		ramMonitor,
		networkMonitor,
	)
	moKubernetes := core.NewMoKubernetes(logManagerModule.CreateLogger("mokubernetes"), configModule, base.clientProvider)
	mocore := core.NewCore(logManagerModule.CreateLogger("core"), configModule, base.clientProvider, base.valkeyClient, eventConnectionClient, jobClients)
	leaderElector := core.NewLeaderElector(logManagerModule.CreateLogger("leader-elector"), configModule, base.clientProvider)
	reconciler := core.NewReconciler(logManagerModule.CreateLogger("reconciler"), configModule, base.clientProvider, aiApi)
	sealedSecret := core.NewSealedSecretManager(logManagerModule.CreateLogger("sealed-secret"), configModule, base.clientProvider)

	// Link phase: wire service dependencies.
	mocore.Link(moKubernetes)
	podStatsCollector.Link(dbstatsService)
	nodeMetricsCollector.Link(dbstatsService, leaderElector)
	socketApi.Link(httpApi, xtermService, dbstatsService, apiModule, moKubernetes, sealedSecret, aiApi, aiWebsocketConnection)
	moKubernetes.Link(dbstatsService)
	httpApi.Link(socketApi, dbstatsService, apiModule, reconciler)
	apiModule.Link(workspaceManager)
	reconciler.Link(leaderElector)

	return clusterSystems{
		baseSystems:           base,
		valkeyClient:          base.valkeyClient,
		watcherModule:         watcherModule,
		jobClients:            jobClients,
		eventConnectionClient: eventConnectionClient,
		networkmonitor:        networkMonitor,
		mocore:                mocore,
		moKubernetes:          moKubernetes,
		workspaceManager:      workspaceManager,
		apiModule:             apiModule,
		socketApi:             socketApi,
		httpApi:               httpApi,
		xtermService:          xtermService,
		aiWebsocketConnection: aiWebsocketConnection,
		valkeyLoggerService:   valkeyLoggerService,
		podStatsCollector:     podStatsCollector,
		nodeMetricsCollector:  nodeMetricsCollector,
		dbstatsService:        dbstatsService,
		leaderElector:         leaderElector,
		reconciler:            reconciler,
		sealedSecret:          sealedSecret,
		argocd:                argocdModule,
		aiManager:             aiManager,
	}
}

func RunCluster(logManagerModule logging.SlogManager, configModule *config.Config, cmdLogger *slog.Logger, valkeyLogChannel chan logging.LogLine) {
	go func() {
		defer shutdown.SendShutdownSignal(true)
		configModule.Validate()

		base := initializeBaseSystems(logManagerModule, configModule, cmdLogger)
		systems := initializeClusterSystems(base, logManagerModule, configModule, valkeyLogChannel)

		systems.versionModule.PrintVersionInfo()

		err := systems.mocore.Initialize()
		if err != nil {
			cmdLogger.Error("failed to initialize kubernetes resources", "error", err)
			return
		}

		cmdLogger.Info("🖥️  🖥️  🖥️  CURRENT CONTEXT", "foundContext", mokubernetes.CurrentContextName())

		systems.httpApi.Run()
		systems.socketApi.Run()
		systems.podStatsCollector.Run()
		systems.nodeMetricsCollector.Orchestrate()
		systems.valkeyLoggerService.Run()
		systems.dbstatsService.Run()
		systems.reconciler.Run()
		systems.leaderElector.Run()
		systems.aiManager.Run()

		// services have to be started before this otherwise watcher events will get missing
		err = mokubernetes.WatchStoreResources(systems.watcherModule, systems.aiManager, systems.eventConnectionClient)
		if err != nil {
			cmdLogger.Error("failed to start watcher", "error", err)
			return
		}

		cmdLogger.Info("SYSTEM STARTUP COMPLETE")

		// connect socket after everything is ready
		systems.mocore.InitializeWebsocketEventServer()
		systems.mocore.InitializeWebsocketApiServers()

		select {}
	}()

	shutdown.Listen()
}
