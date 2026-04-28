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
	moreconciler "mogenius-operator/src/reconciler"
	"mogenius-operator/src/services"
	"mogenius-operator/src/shell"
	"mogenius-operator/src/shutdown"
	"mogenius-operator/src/store"
	"mogenius-operator/src/structs"
	"mogenius-operator/src/utils"
	"mogenius-operator/src/valkeyclient"
	"mogenius-operator/src/watcher"
	"mogenius-operator/src/websocket"
	"mogenius-operator/src/xterm"
	"os"
	"strconv"
	"strings"
	"time"
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
	reconciler            moreconciler.Reconciler
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

	// Emit real-time audit log events to the frontend via WebSocket
	store.OnAuditLogCreated = func(entry store.AuditLogEntry) {
		datagram := structs.Datagram{
			Id:        utils.NanoId(),
			Pattern:   "AuditLogEvent",
			Payload:   entry,
			CreatedAt: entry.CreatedAt,
			User:      entry.User,
			Workspace: entry.Workspace,
		}
		structs.ReportEventToServer(eventConnectionClient, datagram)
	}

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
	xterm.SetupPortForward(base.clientProvider.ClientConfig(), base.clientProvider.K8sClientSet())

	argocdModule := argocd.NewArgoCd(logManagerModule, configModule, base.clientProvider, base.valkeyClient)
	workspaceManager := core.NewWorkspaceManager(configModule, base.clientProvider)
	apiModule := core.NewApi(logManagerModule.CreateLogger("api"), base.valkeyClient, configModule)
	aiApi := core.NewAiApi(logManagerModule.CreateLogger("apApi"), aiManager)
	httpApi := core.NewHttpApi(logManagerModule, configModule)
	alertmanager := core.NewAlertmanagerService(logManagerModule.CreateLogger("alertmanager"), configModule)
	socketApi := core.NewSocketApi(logManagerModule.CreateLogger("socketapi"), configModule, jobClients, eventConnectionClient, base.valkeyClient, argocdModule, alertmanager)
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
	reconciler := moreconciler.NewReconcilerFactory(logManagerModule.CreateLogger("reconciler"), base.clientProvider, configModule, base.valkeyClient).Build()
	sealedSecret := core.NewSealedSecretManager(logManagerModule.CreateLogger("sealed-secret"), configModule, base.clientProvider)

	// Link phase: wire service dependencies.
	mocore.Link(moKubernetes)
	podStatsCollector.Link(dbstatsService)
	nodeMetricsCollector.Link(dbstatsService, leaderElector)
	socketApi.Link(httpApi, xtermService, dbstatsService, apiModule, moKubernetes, sealedSecret, aiApi, aiWebsocketConnection)
	moKubernetes.Link(dbstatsService)
	httpApi.Link(socketApi, dbstatsService, apiModule, reconciler)
	apiModule.Link(workspaceManager)

	// Register AI filters ConfigMap watcher — fires on the object-level subscription
	watcherModule.OnObjectCreated("ConfigMap", configModule.Get("MO_OWN_NAMESPACE"), utils.AI_FILTERS_CONFIGMAP_NAME, aiApi.HandleConfigMapChange)
	watcherModule.OnObjectUpdated("ConfigMap", configModule.Get("MO_OWN_NAMESPACE"), utils.AI_FILTERS_CONFIGMAP_NAME, aiApi.HandleConfigMapChange)

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

// logStep prints a startup progress line directly to stderr, bypassing slog
// so it is always visible regardless of the configured log level.
func logStep(name string) {
	fmt.Fprintf(os.Stderr, "  %s %s\n", shell.Colorize("✓", shell.Green), name)
}

// printReady prints the final ready banner to stderr.
func printReady(version string, addr string, startTime time.Time) {
	elapsed := time.Since(startTime).Round(time.Millisecond)
	separator := shell.Colorize(strings.Repeat("─", 48), shell.Faint)

	fmt.Fprintf(os.Stderr, "\n%s\n", separator)
	fmt.Fprintf(os.Stderr, "  %s\n", shell.Colorize("mogenius-operator ready", shell.Green, shell.Bold))
	fmt.Fprintf(os.Stderr, "\n")
	fmt.Fprintf(os.Stderr, "  %-12s %s\n", shell.Colorize("Version", shell.Faint), version)
	fmt.Fprintf(os.Stderr, "  %-12s %s\n", shell.Colorize("HTTP", shell.Faint), addr)
	fmt.Fprintf(os.Stderr, "  %-12s %s\n", shell.Colorize("Started in", shell.Faint), elapsed)
	fmt.Fprintf(os.Stderr, "%s\n\n", separator)
}

func RunCluster(logManagerModule logging.SlogManager, configModule *config.Config, cmdLogger *slog.Logger, valkeyLogChannel chan logging.LogLine) {
	go func() {
		defer shutdown.SendShutdownSignal(true)
		startTime := time.Now()

		configModule.Validate()

		base := initializeBaseSystems(logManagerModule, configModule, cmdLogger)
		logStep("Base systems initialized (kubernetes client, valkey, store)")

		systems := initializeClusterSystems(base, logManagerModule, configModule, valkeyLogChannel)
		logStep("Cluster systems initialized (websocket, monitors, helm, ai)")

		systems.versionModule.PrintVersionInfo()

		err := systems.mocore.Initialize()
		if err != nil {
			cmdLogger.Error("failed to initialize kubernetes resources", "error", err)
			return
		}
		logStep("Core initialized (valkey, cluster secret, CRDs)")

		systems.httpApi.Run()
		logStep("HTTP API server started on " + configModule.Get("MO_HTTP_ADDR"))

		systems.socketApi.Run()
		logStep("Socket API started")

		systems.podStatsCollector.Run()
		logStep("Pod stats collector started")

		systems.nodeMetricsCollector.Orchestrate()
		logStep("Node metrics collector started")

		systems.valkeyLoggerService.Run()
		logStep("Valkey logger started")

		systems.dbstatsService.Run()
		logStep("DB stats service started")

		systems.leaderElector.OnLeading(func() {

			systems.reconciler.Start()
			logStep("Reconciler started")
		})

		systems.leaderElector.OnLeadingEnded(func() {

			systems.reconciler.Stop()
			logStep("Reconciler stopped")
		})

		systems.leaderElector.Run()
		logStep("Leader elector started")

		systems.aiManager.Run()
		logStep("AI manager started")

		// services have to be started before this otherwise watcher events will get missing
		err = mokubernetes.WatchStoreResources(systems.watcherModule, systems.aiManager, systems.eventConnectionClient)
		if err != nil {
			cmdLogger.Error("failed to start watcher", "error", err)
			return
		}
		logStep("Kubernetes resource watcher started")

		// connect socket after everything is ready
		systems.mocore.InitializeWebsocketEventServer()
		logStep("WebSocket event server connected")

		systems.mocore.InitializeWebsocketApiServers()
		logStep("WebSocket API server(s) connected")

		printReady(
			systems.versionModule.Version,
			configModule.Get("MO_HTTP_ADDR"),
			startTime,
		)

		select {}
	}()

	shutdown.Listen()
}
