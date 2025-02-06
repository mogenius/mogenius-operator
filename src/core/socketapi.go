package core

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"mogenius-k8s-manager/src/assert"
	"mogenius-k8s-manager/src/config"
	"mogenius-k8s-manager/src/controllers"
	"mogenius-k8s-manager/src/crds/v1alpha1"
	"mogenius-k8s-manager/src/dtos"
	"mogenius-k8s-manager/src/helm"
	"mogenius-k8s-manager/src/kubernetes"
	"mogenius-k8s-manager/src/services"
	"mogenius-k8s-manager/src/shell"
	"mogenius-k8s-manager/src/shutdown"
	"mogenius-k8s-manager/src/structs"
	"mogenius-k8s-manager/src/utils"
	"mogenius-k8s-manager/src/version"
	"mogenius-k8s-manager/src/websocket"
	"mogenius-k8s-manager/src/xterm"
	"net/url"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Masterminds/semver"
	jsoniter "github.com/json-iterator/go"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"
)

type SocketApi interface {
	Link(httpService HttpService, xtermService XtermService, apiService Api)
	Run()
}

type socketApi struct {
	logger *slog.Logger

	client  websocket.WebsocketClient
	config  config.ConfigModule
	dbstats kubernetes.BoltDbStats

	httpService  HttpService
	xtermService XtermService
	apiService   Api
}

func NewSocketApi(
	logger *slog.Logger,
	configModule config.ConfigModule,
	client websocket.WebsocketClient,
	dbstatsModule kubernetes.BoltDbStats,
) SocketApi {
	self := &socketApi{}
	self.config = configModule
	self.client = client
	self.logger = logger
	self.dbstats = dbstatsModule

	return self
}

func (self *socketApi) Link(httpService HttpService, xtermService XtermService, apiService Api) {
	assert.Assert(apiService != nil)
	assert.Assert(httpService != nil)
	assert.Assert(xtermService != nil)

	self.apiService = apiService
	self.httpService = httpService
	self.xtermService = xtermService
}

func (self *socketApi) Run() {
	assert.Assert(self.apiService != nil)
	assert.Assert(self.httpService != nil)
	assert.Assert(self.xtermService != nil)

	self.startK8sManager()
}

func (self *socketApi) startK8sManager() {
	self.updateCheck()
	self.versionTicker()

	go func() {
		for status := range structs.EventConnectionStatus {
			if status {
				// CONNECTED
				for {
					_, _, err := structs.EventQueueConnection.ReadMessage()
					if err != nil {
						self.logger.Error("failed to read message for event queue", "eventConnectionUrl", structs.EventConnectionUrl, "error", err)
						break
					}
				}
				structs.EventQueueConnection.Close()
			}
		}
	}()

	self.startMessageHandler()
}

func (self *socketApi) startMessageHandler() {
	var preparedFileName *string
	var preparedFileRequest *services.FilesUploadRequest
	var openFile *os.File

	maxGoroutines := 100
	semaphoreChan := make(chan struct{}, maxGoroutines)
	var wg sync.WaitGroup

	for !self.client.IsTerminated() {
		_, message, err := self.client.ReadMessage()
		if err != nil {
			self.logger.Error("failed to read message from websocket connection", "error", err)
			time.Sleep(time.Second) // wait before next attempt to read
			continue
		}
		rawDataStr := string(message)
		if rawDataStr == "" {
			continue
		}
		if strings.HasPrefix(rawDataStr, "######START_UPLOAD######;") {
			preparedFileName = utils.Pointer(fmt.Sprintf("%s.zip", utils.NanoId()))
			openFile, err = os.OpenFile(*preparedFileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				self.logger.Error("Cannot open uploadfile", "filename", *preparedFileName, "error", err)
			}
			continue
		}
		if strings.HasPrefix(rawDataStr, "######END_UPLOAD######;") {
			openFile.Close()
			if preparedFileName != nil && preparedFileRequest != nil {
				services.Uploaded(*preparedFileName, *preparedFileRequest)
			}
			os.Remove(*preparedFileName)

			var ack = structs.CreateDatagramAck("ack:files/upload:end", preparedFileRequest.Id)
			self.JobServerSendData(self.client, ack)

			preparedFileName = nil
			preparedFileRequest = nil
			continue
		}

		if preparedFileName != nil {
			_, err := openFile.Write([]byte(rawDataStr))
			if err != nil {
				self.logger.Error("Error writing to file", "error", err)
			}
			continue
		}

		datagram := structs.CreateEmptyDatagram()

		var json = jsoniter.ConfigCompatibleWithStandardLibrary
		err = json.Unmarshal([]byte(rawDataStr), &datagram)
		if err != nil {
			self.logger.Error("failed to unmarshal", "error", err)
		}
		validationErr := utils.ValidateJSON(datagram)
		if validationErr != nil {
			self.logger.Error("Received malformed Datagram", "pattern", datagram.Pattern)
			continue
		}

		datagram.DisplayReceiveSummary()

		if isSuppressed := utils.Contains(structs.SUPPRESSED_OUTPUT_PATTERN, datagram.Pattern); !isSuppressed {
			moDebug, err := strconv.ParseBool(self.config.Get("MO_DEBUG"))
			assert.Assert(err == nil, err)
			if moDebug {
				self.logger.Info("received datagram", "datagram", datagram)
			}
		}

		if slices.Contains(structs.COMMAND_REQUESTS, datagram.Pattern) {
			// ####### COMMAND
			semaphoreChan <- struct{}{}

			wg.Add(1)
			go func() {
				defer wg.Done()
				responsePayload := self.ExecuteCommandRequest(datagram, self.httpService)
				result := structs.Datagram{
					Id:        datagram.Id,
					Pattern:   datagram.Pattern,
					Payload:   responsePayload,
					CreatedAt: datagram.CreatedAt,
				}
				self.JobServerSendData(self.client, result)
				<-semaphoreChan
			}()
		} else if slices.Contains(structs.BINARY_REQUEST_UPLOAD, datagram.Pattern) {
			preparedFileRequest = ExecuteBinaryRequestUpload(datagram)

			var ack = structs.CreateDatagramAck("ack:files/upload:datagram", datagram.Id)
			self.JobServerSendData(self.client, ack)
		} else {
			self.logger.Error("Pattern not found", "pattern", datagram.Pattern)
		}
	}
	self.logger.Debug("api messagehandler finished as the websocket client was terminated")
}

var jobDataQueue []structs.Datagram = []structs.Datagram{}
var jobSendMutex sync.Mutex
var jobConnectionGuard = make(chan struct{}, 1)

func (self *socketApi) JobServerSendData(jobClient websocket.WebsocketClient, datagram structs.Datagram) {
	jobDataQueue = append(jobDataQueue, datagram)
	self.processJobNow(jobClient)
}

func (self *socketApi) processJobNow(jobClient websocket.WebsocketClient) {
	jobSendMutex.Lock()
	defer jobSendMutex.Unlock()
	for i := 0; i < len(jobDataQueue); i++ {
		element := jobDataQueue[i]
		err := jobClient.WriteJSON(element)
		if err == nil {
			element.DisplaySentSummary(i+1, len(jobDataQueue))
			if isSuppressed := utils.Contains(structs.SUPPRESSED_OUTPUT_PATTERN, element.Pattern); !isSuppressed {
				self.logger.Debug("sent summary", "payload", element.Payload)
			}
			jobDataQueue = self.removeJobIndex(jobDataQueue, i)
		} else {
			self.logger.Error("Error writing json in job queue", "error", err)
			return
		}
	}
}

func (self *socketApi) removeJobIndex(s []structs.Datagram, index int) []structs.Datagram {
	if len(s) > index {
		return append(s[:index], s[index+1:]...)
	}
	return s
}

func (self *socketApi) connectToJobQueue(jobClient websocket.WebsocketClient) {
	jobQueueCtx, cancel := context.WithCancel(context.Background())
	shutdown.Add(cancel)
	for {
		jobConnectionGuard <- struct{}{} // would block if guard channel is already filled
		go func() {
			ticker := time.NewTicker(1 * time.Second)
			quit := make(chan struct{})
			go func() {
				for {
					select {
					case <-jobQueueCtx.Done():
						return
					case <-quit:
						// close go routine
						return
					case <-ticker.C:
						self.processJobNow(jobClient)
					}
				}
			}()
			select {
			case <-jobQueueCtx.Done():
				return
			case <-jobConnectionGuard:
			}
			ticker.Stop()
			close(quit)
		}()
		select {
		case <-jobQueueCtx.Done():
			self.logger.Debug("shutting down jobqueue")
			return
		case <-time.After(structs.RETRYTIMEOUT * time.Second):
		}
		<-time.After(structs.RETRYTIMEOUT * time.Second)
	}
}

func (self *socketApi) versionTicker() {
	interval, err := strconv.Atoi(self.config.Get("MO_UPDATE_INTERVAL"))
	assert.Assert(err == nil, err)
	updateTicker := time.NewTicker(time.Second * time.Duration(interval))
	done := make(chan bool)

	go func() {
		for {
			select {
			case <-done:
				return
			case <-updateTicker.C:
				self.updateCheck()
			}
		}
	}()
}

func (self *socketApi) updateCheck() {
	if !utils.IsProduction() {
		self.logger.Info("Skipping updates ... [not production]")
		return
	}

	self.logger.Info("Checking for updates ...")

	helmData, err := utils.GetVersionData(utils.HELM_INDEX)
	if err != nil {
		self.logger.Error("GetVersionData", "error", err.Error())
		return
	}
	// VALIDATE RESPONSE
	if len(helmData.Entries) < 1 {
		self.logger.Error("HelmIndex Entries length <= 0. Check the HelmIndex for errors.", "HelmIndex", utils.HELM_INDEX)
		return
	}
	mogeniusPlatform, doesExist := helmData.Entries["mogenius-platform"]
	if !doesExist {
		self.logger.Error("HelmIndex does not contain the field 'mogenius-platform'. Check the HelmIndex for errors.", "HelmIndex", utils.HELM_INDEX)
		return
	}
	if len(mogeniusPlatform) <= 0 {
		self.logger.Error("Field 'mogenius-platform' does not contain a proper version. Check the HelmIndex for errors.", "HelmIndex", utils.HELM_INDEX)
		return
	}
	var mok8smanager *utils.HelmDependency = nil
	for _, dep := range mogeniusPlatform[0].Dependencies {
		if dep.Name == "mogenius-k8s-manager" {
			mok8smanager = &dep
			break
		}
	}
	if mok8smanager == nil {
		self.logger.Error("The umbrella chart 'mogenius-platform' does not contain a dependency for 'mogenius-k8s-manager'. Check the HelmIndex for errors.", "HelmIndex", utils.HELM_INDEX)
		return
	}

	if version.Ver != mok8smanager.Version {
		fmt.Printf("\n####################################################################\n"+
			"####################################################################\n"+
			"######                  %s                ######\n"+
			"######               %s              ######\n"+
			"######                                                        ######\n"+
			"######                    Available: %s                    ######\n"+
			"######                    In-Use:    %s                    ######\n"+
			"######                                                        ######\n"+
			"######   %s   ######\n", shell.Colorize("Not updating might result in service interruption.", shell.Red)+
			"####################################################################\n"+
			"####################################################################\n",
			shell.Colorize("NEW VERSION AVAILABLE!", shell.Blue),
			shell.Colorize(" UPDATE AS FAST AS POSSIBLE", shell.Yellow),
			shell.Colorize(mok8smanager.Version, shell.Green),
			shell.Colorize(version.Ver, shell.Red),
		)
		self.notUpToDateAction(helmData)
	} else {
		self.logger.Debug(" Up-To-Date: 👍", "version", version.Ver)
	}
}

func (self *socketApi) notUpToDateAction(helmData *utils.HelmData) {
	localVer, err := semver.NewVersion(version.Ver)
	if err != nil {
		self.logger.Error("Error parsing local version", "error", err)
		return
	}

	remoteVer, err := semver.NewVersion(helmData.Entries["mogenius-k8s-manager"][0].Version)
	if err != nil {
		self.logger.Error("Error parsing remote version", "error", err)
		return
	}

	constraint, err := semver.NewConstraint(">= " + version.Ver)
	if err != nil {
		self.logger.Error("Error parsing constraint version", "error", err)
		return
	}

	_, errors := constraint.Validate(remoteVer)
	for _, m := range errors {
		self.logger.Error("failed to validate semver constraint", "remoteVer", remoteVer, "error", m)
	}
	// Local version > Remote version (likely development version)
	if remoteVer.LessThan(localVer) {
		self.logger.Warn("Your local version is greater than the remote version. AI thinks: You are likely a developer.",
			"localVer", localVer.String(),
			"remoteVer", remoteVer.String(),
		)
		return
	}

	// MAYOR CHANGES: MUST UPGRADE TO CONTINUE
	if remoteVer.GreaterThan(localVer) && remoteVer.Major() > localVer.Major() {
		self.logger.Error("Your version is too low to continue. Please upgrade to and try again.\n",
			"localVer", localVer.String(),
			"remoteVer", remoteVer.String(),
		)
		shutdown.SendShutdownSignal(true)
		select {}
	}

	// MINOR&PATCH CHANGES: SHOULD UPGRADE
	if remoteVer.GreaterThan(localVer) {
		self.logger.Warn("Your version is out-dated. Please upgrade to avoid service interruption.",
			"localVer", localVer.String(),
			"remoteVer", remoteVer.String(),
		)
		return
	}
}

func (self *socketApi) ExecuteCommandRequest(datagram structs.Datagram, httpApi HttpService) interface{} {
	switch datagram.Pattern {
	case structs.PAT_K8SNOTIFICATION:
		self.logger.Info("Received pattern", "pattern", datagram.Pattern)
		return nil
	case structs.PAT_CLUSTERSTATUS:
		return kubernetes.ClusterStatus()
	case structs.PAT_CLUSTERRESOURCEINFO:
		nodeStats := kubernetes.GetNodeStats()
		loadBalancerExternalIps := kubernetes.GetClusterExternalIps()
		country, _ := utils.GuessClusterCountry()
		cniConfig, _ := self.dbstats.GetCniData()
		result := ClusterResourceInfoDto{
			NodeStats:               nodeStats,
			LoadBalancerExternalIps: loadBalancerExternalIps,
			Country:                 country,
			Provider:                string(utils.ClusterProviderCached),
			CniConfig:               cniConfig,
		}
		return result
	case structs.PAT_UPGRADEK8SMANAGER:
		data := K8sManagerUpgradeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return UpgradeK8sManager(data)

	case structs.PAT_CLUSTER_FORCE_RECONNECT:
		time.Sleep(1 * time.Second)
		return kubernetes.ClusterForceReconnect()

	case structs.PAT_CLUSTER_FORCE_DISCONNECT:
		time.Sleep(1 * time.Second)
		return kubernetes.ClusterForceDisconnect()

	case structs.PAT_SYSTEM_CHECK:
		return services.SystemCheck()
	case structs.PAT_CLUSTER_RESTART:
		self.logger.Info("😵😵😵 Received RESTART COMMAND. Restarting now ...")
		time.Sleep(1 * time.Second)
		os.Exit(0)
		return nil
	case structs.PAT_SYSTEM_PRINT_CURRENT_CONFIG:
		return self.config.AsEnvs()

	// case structs.PAT_IAC_FORCE_SYNC:
	// 	return NewMessageResponse(nil, iacmanager.SyncChanges())
	// case structs.PAT_IAC_GET_STATUS:
	// 	return NewMessageResponse(iacmanager.GetDataModel(), nil)
	// case structs.PAT_IAC_RESET_LOCAL_REPO:
	// 	return NewMessageResponse(nil, iacmanager.ResetLocalRepo())
	// case structs.PAT_IAC_RESET_FILE:
	// 	data := dtos.ResetFileRequest{}
	// 	structs.MarshalUnmarshal(&datagram, &data)
	// 	if err := utils.ValidateJSON(data); err != nil {
	// 		return err
	// 	}
	// 	return NewMessageResponse(nil, iacmanager.ResetFile(data.FilePath, data.CommitHash))

	case structs.PAT_ENERGY_CONSUMPTION:
		return services.EnergyConsumption()

	case structs.PAT_CLUSTER_SYNC_INFO:
		result, err := kubernetes.GetSyncRepoData()
		if err != nil {
			return err
		}
		return result

	// case structs.PAT_CLUSTER_SYNC_UPDATE:
	// 	data := dtos.SyncRepoData{}
	// 	structs.MarshalUnmarshal(&datagram, &data)
	// 	if err := utils.ValidateJSON(data); err != nil {
	// 		return err
	// 	}
	// 	data.AddSecretsToRedaction()
	// 	err := iacmanager.UpdateSyncRepoData(&data)
	// 	if err != nil {
	// 		return err
	// 	}
	// 	err = iacmanager.CheckRepoAccess()
	// 	if err != nil {
	// 		return err
	// 	}
	// 	return err

	case structs.PAT_INSTALL_TRAFFIC_COLLECTOR:
		result, err := services.InstallTrafficCollector()
		return NewMessageResponse(result, err)
	case structs.PAT_INSTALL_POD_STATS_COLLECTOR:
		result, err := services.InstallPodStatsCollector()
		return NewMessageResponse(result, err)
	case structs.PAT_INSTALL_METRICS_SERVER:
		result, err := services.InstallMetricsServer()
		return NewMessageResponse(result, err)
	case structs.PAT_INSTALL_INGRESS_CONTROLLER_TREAFIK:
		result, err := services.InstallIngressControllerTreafik()
		return NewMessageResponse(result, err)
	case structs.PAT_INSTALL_CERT_MANAGER:
		result, err := services.InstallCertManager()
		return NewMessageResponse(result, err)
	case structs.PAT_INSTALL_CLUSTER_ISSUER:
		data := services.ClusterIssuerInstallRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		data.AddSecretsToRedaction()
		result, err := services.InstallClusterIssuer(data.Email, 0)
		return NewMessageResponse(result, err)
	case structs.PAT_INSTALL_CONTAINER_REGISTRY:
		result, err := services.InstallContainerRegistry()
		return NewMessageResponse(result, err)
	case structs.PAT_INSTALL_EXTERNAL_SECRETS:
		result, err := services.InstallExternalSecrets()
		return NewMessageResponse(result, err)
	case structs.PAT_INSTALL_METALLB:
		result, err := services.InstallMetalLb()
		return NewMessageResponse(result, err)
	case structs.PAT_INSTALL_KEPLER:
		result, err := services.InstallKepler()
		return NewMessageResponse(result, err)
	case structs.PAT_UNINSTALL_TRAFFIC_COLLECTOR:
		msg, err := services.UninstallTrafficCollector()
		return NewMessageResponse(msg, err)
	case structs.PAT_UNINSTALL_POD_STATS_COLLECTOR:
		msg, err := services.UninstallPodStatsCollector()
		return NewMessageResponse(msg, err)
	case structs.PAT_UNINSTALL_METRICS_SERVER:
		msg, err := services.UninstallMetricsServer()
		return NewMessageResponse(msg, err)
	case structs.PAT_UNINSTALL_INGRESS_CONTROLLER_TREAFIK:
		msg, err := services.UninstallIngressControllerTreafik()
		return NewMessageResponse(msg, err)
	case structs.PAT_UNINSTALL_CERT_MANAGER:
		msg, err := services.UninstallCertManager()
		return NewMessageResponse(msg, err)
	case structs.PAT_UNINSTALL_CLUSTER_ISSUER:
		msg, err := services.UninstallClusterIssuer()
		return NewMessageResponse(msg, err)
	case structs.PAT_UNINSTALL_CONTAINER_REGISTRY:
		msg, err := services.UninstallContainerRegistry()
		return NewMessageResponse(msg, err)
	case structs.PAT_UNINSTALL_EXTERNAL_SECRETS:
		msg, err := services.UninstallExternalSecrets()
		return NewMessageResponse(msg, err)
	case structs.PAT_UNINSTALL_METALLB:
		msg, err := services.UninstallMetalLb()
		return NewMessageResponse(msg, err)
	case structs.PAT_UNINSTALL_KEPLER:
		msg, err := services.UninstallKepler()
		return NewMessageResponse(msg, err)
	case structs.PAT_UPGRADE_TRAFFIC_COLLECTOR:
		result, err := services.UpgradeTrafficCollector()
		return NewMessageResponse(result, err)
	case structs.PAT_UPGRADE_PODSTATS_COLLECTOR:
		result, err := services.UpgradePodStatsCollector()
		return NewMessageResponse(result, err)
	case structs.PAT_UPGRADE_METRICS_SERVER:
		result, err := services.UpgradeMetricsServer()
		return NewMessageResponse(result, err)
	case structs.PAT_UPGRADE_INGRESS_CONTROLLER_TREAFIK:
		result, err := services.UpgradeIngressControllerTreafik()
		return NewMessageResponse(result, err)
	case structs.PAT_UPGRADE_CERT_MANAGER:
		result, err := services.UpgradeCertManager()
		return NewMessageResponse(result, err)
	case structs.PAT_UPGRADE_CONTAINER_REGISTRY:
		result, err := services.UpgradeContainerRegistry()
		return NewMessageResponse(result, err)
	case structs.PAT_UPGRADE_METALLB:
		result, err := services.UpgradeMetalLb()
		return NewMessageResponse(result, err)
	case structs.PAT_UPGRADE_KEPLER:
		result, err := services.UpgradeKepler()
		return NewMessageResponse(result, err)

	case structs.PAT_STATS_PODSTAT_FOR_POD_ALL:
		data := services.StatsDataRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		ctrl := kubernetes.ControllerForPod(data.Namespace, data.PodName)
		if ctrl == nil {
			return fmt.Errorf("could not find controller for pod %s in namespace %s", data.PodName, data.Namespace)
		}
		return self.dbstats.GetPodStatsEntriesForController(*ctrl)
	case structs.PAT_STATS_PODSTAT_FOR_POD_LAST:
		data := services.StatsDataRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		ctrl := kubernetes.ControllerForPod(data.Namespace, data.PodName)
		if ctrl == nil {
			return fmt.Errorf("could not find controller for pod %s in namespace %s", data.PodName, data.Namespace)
		}
		return self.dbstats.GetLastPodStatsEntryForController(*ctrl)

	case structs.PAT_STATS_PODSTAT_FOR_CONTROLLER_ALL:
		data := kubernetes.K8sController{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return self.dbstats.GetPodStatsEntriesForController(data)
	case structs.PAT_STATS_PODSTAT_FOR_CONTROLLER_LAST:
		data := kubernetes.K8sController{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return self.dbstats.GetLastPodStatsEntryForController(data)
	case structs.PAT_STATS_TRAFFIC_FOR_CONTROLLER_ALL:
		data := kubernetes.K8sController{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return self.dbstats.GetTrafficStatsEntriesForController(data)
	case structs.PAT_STATS_TRAFFIC_FOR_CONTROLLER_SUM, structs.PAT_STATS_TRAFFIC_FOR_CONTROLLER_LAST:
		data := kubernetes.K8sController{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return self.dbstats.GetTrafficStatsEntrySumForController(data, false)
	case structs.PAT_STATS_TRAFFIC_FOR_CONTROLLER_SOCKET_CONNECTIONS:
		data := kubernetes.K8sController{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return self.dbstats.GetSocketConnectionsForController(data)

	case structs.PAT_STATS_TRAFFIC_FOR_POD_ALL:
		data := services.StatsDataRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		ctrl := kubernetes.ControllerForPod(data.Namespace, data.PodName)
		if ctrl == nil {
			return fmt.Errorf("could not find controller for pod %s in namespace %s", data.PodName, data.Namespace)
		}
		return self.dbstats.GetTrafficStatsEntriesForController(*ctrl)
	case structs.PAT_STATS_TRAFFIC_FOR_POD_SUM, structs.PAT_STATS_TRAFFIC_FOR_POD_LAST:
		data := services.StatsDataRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		ctrl := kubernetes.ControllerForPod(data.Namespace, data.PodName)
		if ctrl == nil {
			return fmt.Errorf("could not find controller for pod %s in namespace %s", data.PodName, data.Namespace)
		}
		return self.dbstats.GetTrafficStatsEntrySumForController(*ctrl, false)

	case structs.PAT_STATS_PODSTAT_FOR_NAMESPACE_ALL:
		data := services.NsStatsDataRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return self.dbstats.GetPodStatsEntriesForNamespace(data.Namespace)
	case structs.PAT_STATS_PODSTAT_FOR_NAMESPACE_LAST:
		data := services.NsStatsDataRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return self.dbstats.GetLastPodStatsEntriesForNamespace(data.Namespace)
	case structs.PAT_STATS_TRAFFIC_FOR_NAMESPACE_ALL:
		data := services.NsStatsDataRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return self.dbstats.GetTrafficStatsEntriesForNamespace(data.Namespace)
	case structs.PAT_STATS_TRAFFIC_FOR_NAMESPACE_SUM, structs.PAT_STATS_TRAFFIC_FOR_NAMESPACE_LAST:
		data := services.NsStatsDataRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return self.dbstats.GetTrafficStatsEntriesSumForNamespace(data.Namespace)

	case structs.PAT_METRICS_DEPLOYMENT_AVG_UTILIZATION:
		data := kubernetes.K8sController{}
		data.Kind = "Deployment"

		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return kubernetes.GetAverageUtilizationForDeployment(data)
	case structs.PAT_FILES_LIST:
		data := services.FilesListRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return services.List(data)
	case structs.PAT_FILES_CREATE_FOLDER:
		data := services.FilesCreateFolderRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return services.CreateFolder(data)
	case structs.PAT_FILES_RENAME:
		data := services.FilesRenameRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return services.Rename(data)
	case structs.PAT_FILES_CHOWN:
		data := services.FilesChownRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return services.Chown(data)
	case structs.PAT_FILES_CHMOD:
		data := services.FilesChmodRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return services.Chmod(data)
	case structs.PAT_FILES_DELETE:
		data := services.FilesDeleteRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return services.Delete(data)
	case structs.PAT_FILES_DOWNLOAD:
		data := services.FilesDownloadRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return services.Download(data)
	case structs.PAT_FILES_INFO:
		data := dtos.PersistentFileRequestDto{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return services.Info(data)

	case structs.PAT_CLUSTER_EXECUTE_HELM_CHART_TASK:
		data := services.ClusterHelmRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return services.InstallHelmChart(data)
	case structs.PAT_CLUSTER_UNINSTALL_HELM_CHART:
		data := services.ClusterHelmUninstallRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return services.DeleteHelmChart(data)
	case structs.PAT_CLUSTER_TCP_UDP_CONFIGURATION:
		return services.TcpUdpClusterConfiguration()
	case structs.PAT_CLUSTER_BACKUP:
		result, err := kubernetes.BackupNamespace("")
		if err != nil {
			return err.Error()
		}
		return result
	case structs.PAT_CLUSTER_READ_CONFIGMAP:
		data := services.ClusterGetConfigMap{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return kubernetes.GetConfigMapWR(data.Namespace, data.Name)
	case structs.PAT_CLUSTER_WRITE_CONFIGMAP:
		data := services.ClusterWriteConfigMap{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return kubernetes.WriteConfigMap(data.Namespace, data.Name, data.Data, data.Labels)
	case structs.PAT_CLUSTER_LIST_CONFIGMAPS:
		data := services.ClusterListWorkloads{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return kubernetes.ListConfigMapWithFieldSelector(data.Namespace, data.LabelSelector, data.Prefix)
	case structs.PAT_CLUSTER_READ_DEPLOYMENT:
		data := services.ClusterGetDeployment{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return kubernetes.GetDeploymentResult(data.Namespace, data.Name)
	// TODO
	// case structs.PAT_CLUSTER_WRITE_DEPLOYMENT:
	// 	data := ClusterWriteDeployment{}
	// 	structs.MarshalUnmarshal(&datagram, &data)
	// 	if err := utils.ValidateJSON(data); err != nil {
	// 		return err
	// 	}
	// 	return kubernetes.WriteConfigMap(data.Namespace, data.Name, data.Data, data.Labels)
	case structs.PAT_CLUSTER_LIST_DEPLOYMENTS:
		data := services.ClusterListWorkloads{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return kubernetes.ListDeploymentsWithFieldSelector(data.Namespace, data.LabelSelector, data.Prefix)
	case structs.PAT_CLUSTER_READ_PERSISTENT_VOLUME_CLAIM:
		data := services.ClusterGetPersistentVolume{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return NewMessageResponse(kubernetes.GetPersistentVolumeClaim(data.Namespace, data.Name))
	// TODO
	// case structs.PAT_CLUSTER_WRITE_PERSISTENT_VOLUME_CLAIM:
	// 	data := ClusterWritePersistentVolume{}
	// 	structs.MarshalUnmarshal(&datagram, &data)
	// 	if err := utils.ValidateJSON(data); err != nil {
	// 		return err
	// 	}
	// 	return kubernetes.WritePersistentVolume(data.Namespace, data.Name, data.Data, data.Labels)
	case structs.PAT_CLUSTER_LIST_PERSISTENT_VOLUME_CLAIMS:
		data := services.ClusterListWorkloads{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		// AllPersistentVolumes
		return kubernetes.ListPersistentVolumeClaimsWithFieldSelector(data.Namespace, data.LabelSelector, data.Prefix)

	case structs.PAT_CLUSTER_UPDATE_LOCAL_TLS_SECRET:
		data := services.ClusterUpdateLocalTlsSecret{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return kubernetes.CreateMogeniusContainerRegistryTlsSecret(data.LocalTlsCrt, data.LocalTlsKey)

	case structs.PAT_NAMESPACE_CREATE:
		data := services.NamespaceCreateRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		data.Project.AddSecretsToRedaction()
		return services.CreateNamespace(data)
	case structs.PAT_NAMESPACE_DELETE:
		data := services.NamespaceDeleteRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return services.DeleteNamespace(data)
	case structs.PAT_NAMESPACE_SHUTDOWN:
		data := services.NamespaceShutdownRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		data.Service.AddSecretsToRedaction()
		return services.ShutdownNamespace(data)
	case structs.PAT_NAMESPACE_POD_IDS:
		data := services.NamespacePodIdsRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return services.PodIds(data)
	case structs.PAT_NAMESPACE_VALIDATE_CLUSTER_PODS:
		data := services.NamespaceValidateClusterPodsRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return services.ValidateClusterPods(data)
	case structs.PAT_NAMESPACE_VALIDATE_PORTS:
		data := services.NamespaceValidatePortsRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return services.ValidateClusterPorts(data)
	case structs.PAT_NAMESPACE_LIST_ALL:
		return services.ListAllNamespaces()
	case structs.PAT_NAMESPACE_GATHER_ALL_RESOURCES:
		data := services.NamespaceGatherAllResourcesRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return services.ListAllResourcesForNamespace(data)
	case structs.PAT_NAMESPACE_BACKUP:
		data := services.NamespaceBackupRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		result, err := kubernetes.BackupNamespace(data.NamespaceName)
		if err != nil {
			return err.Error()
		}
		return result
	case structs.PAT_NAMESPACE_RESTORE:
		data := services.NamespaceRestoreRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		result, err := kubernetes.RestoreNamespace(data.YamlData, data.NamespaceName)
		if err != nil {
			return err.Error()
		}
		return result
	case structs.PAT_NAMESPACE_RESOURCE_YAML:
		data := services.NamespaceResourceYamlRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		result, err := kubernetes.AllResourcesFromToCombinedYaml(data.NamespaceName, data.Resources)
		if err != nil {
			return err.Error()
		}
		return result

	case structs.PAT_CLUSTER_HELM_REPO_ADD:
		data := helm.HelmRepoAddRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return NewMessageResponse(helm.HelmRepoAdd(data))
	case structs.PAT_CLUSTER_HELM_REPO_PATCH:
		data := helm.HelmRepoPatchRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return NewMessageResponse(helm.HelmRepoPatch(data))
	case structs.PAT_CLUSTER_HELM_REPO_UPDATE:
		return NewMessageResponse(helm.HelmRepoUpdate())
	case structs.PAT_CLUSTER_HELM_REPO_LIST:
		return NewMessageResponse(helm.HelmRepoList())
	case structs.PAT_CLUSTER_HELM_REPO_REMOVE:
		data := helm.HelmRepoRemoveRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return NewMessageResponse(helm.HelmRepoRemove(data))
	case structs.PAT_CLUSTER_HELM_CHART_SEARCH:
		data := helm.HelmChartSearchRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return NewMessageResponse(helm.HelmChartSearch(data))
	case structs.PAT_CLUSTER_HELM_CHART_INSTALL:
		data := helm.HelmChartInstallRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return NewMessageResponse(helm.HelmChartInstall(data))
	case structs.PAT_CLUSTER_HELM_CHART_SHOW:
		data := helm.HelmChartShowRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return NewMessageResponse(helm.HelmChartShow(data))
	case structs.PAT_CLUSTER_HELM_CHART_VERSIONS:
		data := helm.HelmChartVersionRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return NewMessageResponse(helm.HelmChartVersion(data))
	case structs.PAT_CLUSTER_HELM_RELEASE_UPGRADE:
		data := helm.HelmReleaseUpgradeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return NewMessageResponse(helm.HelmReleaseUpgrade(data))
	case structs.PAT_CLUSTER_HELM_RELEASE_UNINSTALL:
		data := helm.HelmReleaseUninstallRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return NewMessageResponse(helm.HelmReleaseUninstall(data))
	case structs.PAT_CLUSTER_HELM_RELEASE_LIST:
		data := helm.HelmReleaseListRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return NewMessageResponse(helm.HelmReleaseList(data))
	case structs.PAT_CLUSTER_HELM_RELEASE_STATUS:
		data := helm.HelmReleaseStatusRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return NewMessageResponse(helm.HelmReleaseStatus(data))
	case structs.PAT_CLUSTER_HELM_RELEASE_HISTORY:
		data := helm.HelmReleaseHistoryRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return NewMessageResponse(helm.HelmReleaseHistory(data))
	case structs.PAT_CLUSTER_HELM_RELEASE_ROLLBACK:
		data := helm.HelmReleaseRollbackRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return NewMessageResponse(helm.HelmReleaseRollback(data))
	case structs.PAT_CLUSTER_HELM_RELEASE_GET:
		data := helm.HelmReleaseGetRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return NewMessageResponse(helm.HelmReleaseGet(data))
	case structs.PAT_CLUSTER_HELM_RELEASE_GET_WORKLOADS:
		data := helm.HelmReleaseGetWorkloadsRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return NewMessageResponse(helm.HelmReleaseGetWorkloads(data))

	case structs.PAT_SERVICE_CREATE:
		data := services.ServiceUpdateRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		data.Service.AddSecretsToRedaction()
		data.Project.AddSecretsToRedaction()
		return services.UpdateService(data)
	case structs.PAT_SERVICE_DELETE:
		data := services.ServiceDeleteRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		data.Service.AddSecretsToRedaction()
		data.Project.AddSecretsToRedaction()
		return services.DeleteService(data)
	case structs.PAT_SERVICE_POD_IDS:
		data := services.ServiceGetPodIdsRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return services.ServicePodIds(data)
	case structs.PAT_SERVICE_POD_EXISTS:
		data := services.ServicePodExistsRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return services.ServicePodExists(data)
	case structs.PAT_SERVICE_PODS:
		data := services.ServicePodsRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return services.ServicePodStatus(data)
	// case structs.PAT_SERVICE_SET_IMAGE:
	// 	data := ServiceSetImageRequest{}
	// 	structs.MarshalUnmarshal(&datagram, &data)
	// 	if err := utils.ValidateJSON(data); err != nil {
	// 		return err
	// 	}
	// 	return SetImage(data)
	case structs.PAT_SERVICE_LOG:
		data := services.ServiceGetLogRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return services.PodLog(data)
	case structs.PAT_SERVICE_LOG_ERROR:
		data := services.ServiceGetLogRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return services.PodLogError(data)
	case structs.PAT_SERVICE_RESOURCE_STATUS:
		data := services.ServiceResourceStatusRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return services.PodStatus(data)
	case structs.PAT_SERVICE_RESTART:
		data := services.ServiceRestartRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		data.Service.AddSecretsToRedaction()
		return services.Restart(data)
	case structs.PAT_SERVICE_STOP:
		data := services.ServiceStopRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		data.Service.AddSecretsToRedaction()
		return services.StopService(data)
	case structs.PAT_SERVICE_START:
		data := services.ServiceStartRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		data.Service.AddSecretsToRedaction()
		return services.StartService(data)
	case structs.PAT_SERVICE_UPDATE_SERVICE:
		data := services.ServiceUpdateRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		data.Project.AddSecretsToRedaction()
		data.Service.AddSecretsToRedaction()
		return services.UpdateService(data)
	case structs.PAT_SERVICE_TRIGGER_JOB:
		data := services.ServiceTriggerJobRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return services.TriggerJobService(data)
	case structs.PAT_SERVICE_STATUS:
		data := services.ServiceStatusRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return services.StatusServiceDebounced(data)

	case structs.PAT_SERVICE_LOG_STREAM:
		data := services.ServiceLogStreamRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return self.logStream(data, datagram)

	case structs.PAT_SERVICE_EXEC_SH_CONNECTION_REQUEST:
		data := xterm.PodCmdConnectionRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		go self.execShConnection(data)
		return nil

	case structs.PAT_SERVICE_LOG_STREAM_CONNECTION_REQUEST:
		data := xterm.PodCmdConnectionRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		go self.logStreamConnection(data)
		return nil
	case structs.PAT_SERVICE_BUILD_LOG_STREAM_CONNECTION_REQUEST:
		data := xterm.BuildLogConnectionRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		go buildLogStreamConnection(data)
		return nil
	case structs.PAT_CLUSTER_COMPONENT_LOG_STREAM_CONNECTION_REQUEST:
		data := xterm.ComponentLogConnectionRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		go componentLogStreamConnection(data)
		return nil
	case structs.PAT_SERVICE_POD_EVENT_STREAM_CONNECTION_REQUEST:
		data := xterm.PodEventConnectionRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		go podEventStreamConnection(data)
		return nil
	case structs.PAT_SERVICE_SCAN_IMAGE_LOG_STREAM_CONNECTION_REQUEST:
		data := xterm.ScanImageLogConnectionRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		data.AddSecretsToRedaction()
		go scanImageLogStreamConnection(data)
		return nil
	case structs.PAT_SERVICE_CLUSTER_TOOL_STREAM_CONNECTION_REQUEST:
		data := xterm.ClusterToolConnectionRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		go services.XTermClusterToolStreamConnection(data)
		return nil

	case structs.PAT_LIST_ALL_WORKLOADS:
		resources, err := kubernetes.GetAvailableResources()
		return NewMessageResponse(resources, err)
	case structs.PAT_GET_WORKLOAD_LIST:
		data := utils.SyncResourceEntry{}
		structs.MarshalUnmarshal(&datagram, &data)
		return NewMessageResponse(kubernetes.GetUnstructuredResourceListFromStore(data.Group, data.Kind, data.Version, data.Name, data.Namespace))
	case structs.PAT_GET_NAMESPACE_WORKLOAD_LIST:
		data := kubernetes.GetUnstructuredNamespaceResourceListRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return NewMessageResponse(kubernetes.GetUnstructuredNamespaceResourceList(data.Namespace, data.Whitelist, data.Blacklist))
	case structs.PAT_GET_LABELED_WORKLOAD_LIST:
		data := kubernetes.GetUnstructuredLabeledResourceListRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		list, err := kubernetes.GetUnstructuredLabeledResourceList(data.Label, data.Whitelist, data.Blacklist)
		return NewMessageResponse(list, err)
	case structs.PAT_DESCRIBE_WORKLOAD:
		data := utils.SyncResourceItem{}
		structs.MarshalUnmarshal(&datagram, &data)
		describeStr, err := kubernetes.DescribeUnstructuredResource(data.Group, data.Version, data.Name, data.Namespace, data.ResourceName)
		return NewMessageResponse(describeStr, err)
	case structs.PAT_CREATE_NEW_WORKLOAD:
		data := utils.SyncResourceData{}
		structs.MarshalUnmarshal(&datagram, &data)
		newObj, err := kubernetes.CreateUnstructuredResource(data.Group, data.Version, data.Name, data.Namespace, data.YamlData)
		return NewMessageResponse(newObj, err)
	case structs.PAT_GET_WORKLOAD:
		data := utils.SyncResourceItem{}
		structs.MarshalUnmarshal(&datagram, &data)
		newObj, err := kubernetes.GetUnstructuredResource(data.Group, data.Version, data.Name, data.Namespace, data.ResourceName)
		return NewMessageResponse(newObj, err)
	case structs.PAT_GET_WORKLOAD_EXAMPLE:
		data := utils.SyncResourceItem{}
		structs.MarshalUnmarshal(&datagram, &data)
		return NewMessageResponse(kubernetes.GetResourceTemplateYaml(data.Group, data.Version, data.Name, data.Kind, data.Namespace, data.ResourceName), nil)
	case structs.PAT_UPDATE_WORKLOAD:
		data := utils.SyncResourceData{}
		structs.MarshalUnmarshal(&datagram, &data)
		updatedObj, err := kubernetes.UpdateUnstructuredResource(data.Group, data.Version, data.Name, data.Namespace, data.YamlData)
		return NewMessageResponse(updatedObj, err)
	case structs.PAT_DELETE_WORKLOAD:
		data := utils.SyncResourceItem{}
		structs.MarshalUnmarshal(&datagram, &data)
		err := kubernetes.DeleteUnstructuredResource(data.Group, data.Version, data.Name, data.Namespace, data.ResourceName)
		return NewMessageResponse(nil, err)

	case structs.PAT_GET_WORKSPACES:
		result, err := self.apiService.GetAllWorkspaces()
		return NewMessageResponse(result, err)
	case structs.PAT_CREATE_WORKSPACE:
		data := utils.WebsocketRequestCreateWorkspace{}
		structs.MarshalUnmarshal(&datagram, &data)
		result, err := self.apiService.CreateWorkspace(data.Name, v1alpha1.WorkspaceSpec{
			Name:      data.Name,
			Resources: data.Resources,
		})
		return NewMessageResponse(result, err)
	case structs.PAT_GET_WORKSPACE:
		data := utils.WebsocketRequestGetWorkspace{}
		structs.MarshalUnmarshal(&datagram, &data)
		result, err := self.apiService.GetWorkspace(data.Name)
		return NewMessageResponse(result, err)
	case structs.PAT_UPDATE_WORKSPACE:
		data := utils.WebsocketRequestUpdateWorkspace{}
		structs.MarshalUnmarshal(&datagram, &data)
		result, err := self.apiService.UpdateWorkspace(data.Name, v1alpha1.WorkspaceSpec{
			Name:      data.DisplayName,
			Resources: data.Resources,
		})
		return NewMessageResponse(result, err)
	case structs.PAT_DELETE_WORKSPACE:
		data := utils.WebsocketRequestDeleteWorkspace{}
		structs.MarshalUnmarshal(&datagram, &data)
		result, err := self.apiService.DeleteWorkspace(data.Name)
		return NewMessageResponse(result, err)

	case structs.PAT_GET_USERS:
		result, err := self.apiService.GetAllUsers()
		return NewMessageResponse(result, err)
	case structs.PAT_CREATE_USER:
		data := utils.WebsocketRequestCreateUser{}
		structs.MarshalUnmarshal(&datagram, &data)
		result, err := self.apiService.CreateUser(data.Name, v1alpha1.NewUserSpec(data.Name))
		return NewMessageResponse(result, err)
	case structs.PAT_GET_USER:
		data := utils.WebsocketRequestGetUser{}
		structs.MarshalUnmarshal(&datagram, &data)
		result, err := self.apiService.GetUser(data.Name)
		return NewMessageResponse(result, err)
	case structs.PAT_UPDATE_USER:
		data := utils.WebsocketRequestUpdateUser{}
		structs.MarshalUnmarshal(&datagram, &data)
		result, err := self.apiService.UpdateUser(data.Name, v1alpha1.NewUserSpec(data.Name))
		return NewMessageResponse(result, err)
	case structs.PAT_DELETE_USER:
		data := utils.WebsocketRequestDeleteUser{}
		structs.MarshalUnmarshal(&datagram, &data)
		result, err := self.apiService.DeleteUser(data.Name)
		return NewMessageResponse(result, err)

	case structs.PAT_GET_GROUPS:
		result, err := self.apiService.GetAllGroups()
		return NewMessageResponse(result, err)
	case structs.PAT_CREATE_GROUP:
		data := utils.WebsocketRequestCreateGroup{}
		structs.MarshalUnmarshal(&datagram, &data)
		result, err := self.apiService.CreateGroup(data.Name, v1alpha1.NewGroupSpec(data.Name, data.Users))
		return NewMessageResponse(result, err)
	case structs.PAT_GET_GROUP:
		data := utils.WebsocketRequestGetGroup{}
		structs.MarshalUnmarshal(&datagram, &data)
		result, err := self.apiService.GetGroup(data.Name)
		return NewMessageResponse(result, err)
	case structs.PAT_UPDATE_GROUP:
		data := utils.WebsocketRequestUpdateGroup{}
		structs.MarshalUnmarshal(&datagram, &data)
		result, err := self.apiService.UpdateGroup(data.Name, v1alpha1.NewGroupSpec(data.Name, data.Users))
		return NewMessageResponse(result, err)
	case structs.PAT_DELETE_GROUP:
		data := utils.WebsocketRequestDeleteGroup{}
		structs.MarshalUnmarshal(&datagram, &data)
		result, err := self.apiService.DeleteGroup(data.Name)
		return NewMessageResponse(result, err)

	case structs.PAT_GET_PERMISSIONS:
		result, err := self.apiService.GetAllPermissions()
		return NewMessageResponse(result, err)
	case structs.PAT_CREATE_PERMISSION:
		data := utils.WebsocketRequestCreatePermission{}
		structs.MarshalUnmarshal(&datagram, &data)
		result, err := self.apiService.CreatePermission(data.Name, v1alpha1.NewPermissionSpec(
			data.Group,
			data.Workspace,
			data.Read,
			data.Write,
			data.Delete,
		))
		return NewMessageResponse(result, err)
	case structs.PAT_GET_PERMISSION:
		data := utils.WebsocketRequestGetPermission{}
		structs.MarshalUnmarshal(&datagram, &data)
		result, err := self.apiService.GetPermission(data.Name)
		return NewMessageResponse(result, err)
	case structs.PAT_UPDATE_PERMISSION:
		data := utils.WebsocketRequestUpdatePermission{}
		structs.MarshalUnmarshal(&datagram, &data)
		result, err := self.apiService.UpdatePermission(data.Name, v1alpha1.NewPermissionSpec(
			data.Group,
			data.Workspace,
			data.Read,
			data.Write,
			data.Delete,
		))
		return NewMessageResponse(result, err)
	case structs.PAT_DELETE_PERMISSION:
		data := utils.WebsocketRequestDeletePermission{}
		structs.MarshalUnmarshal(&datagram, &data)
		result, err := self.apiService.DeletePermission(data.Name)
		return NewMessageResponse(result, err)

	case structs.PAT_BUILDER_STATUS:
		return kubernetes.GetDb().GetBuilderStatus()
	case structs.PAT_BUILD_INFOS:
		data := structs.BuildJobStatusRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return kubernetes.GetDb().GetBuildJobInfosFromDb(data.BuildId)
	case structs.PAT_BUILD_LAST_INFOS:
		data := structs.BuildTaskRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return kubernetes.GetDb().GetLastBuildJobInfosFromDb(data)
	case structs.PAT_BUILD_LIST_ALL:
		return services.ListAll()
	case structs.PAT_BUILD_LIST_BY_PROJECT:
		data := structs.ListBuildByProjectIdRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return services.ListByProjectId(data.ProjectId)
	case structs.PAT_BUILD_ADD:
		data := structs.BuildJob{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		data.Project.AddSecretsToRedaction()
		data.Service.AddSecretsToRedaction()
		return services.AddBuildJob(data)
	case structs.PAT_BUILD_CANCEL:
		data := structs.BuildJob{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		data.Project.AddSecretsToRedaction()
		data.Service.AddSecretsToRedaction()
		return services.Cancel(data.BuildId)
	case structs.PAT_BUILD_DELETE:
		data := structs.BuildJobStatusRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return services.DeleteBuild(data.BuildId)
	case structs.PAT_BUILD_LAST_JOB_OF_SERVICES:
		data := structs.BuildTaskListOfServicesRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return services.LastBuildInfosOfServices(data)
	case structs.PAT_BUILD_JOB_LIST_OF_SERVICE:
		data := structs.BuildTaskRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return kubernetes.GetDb().GetBuildJobInfosListFromDb(data.Namespace, data.Controller, data.Container)
	case structs.PAT_BUILD_DELETE_ALL_OF_SERVICE:
		data := structs.BuildTaskRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		kubernetes.GetDb().DeleteAllBuildData(data.Namespace, data.Controller, data.Container)
		return nil
	//case structs.PAT_BUILD_LAST_JOB_INFO_OF_SERVICE:
	//	data := structs.BuildServiceRequest{}
	//	structs.MarshalUnmarshal(&datagram, &data)
	//	if err := utils.ValidateJSON(data); err != nil {
	//		return err
	//	}
	//	return LastBuildForService(data.ServiceId)

	case structs.PAT_STORAGE_CREATE_VOLUME:
		data := services.NfsVolumeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return services.CreateMogeniusNfsVolume(data)
	case structs.PAT_STORAGE_DELETE_VOLUME:
		data := services.NfsVolumeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return services.DeleteMogeniusNfsVolume(data)
	// case structs.PAT_STORAGE_BACKUP_VOLUME:
	// 	data := NfsVolumeBackupRequest{}
	// 	structs.MarshalUnmarshal(&datagram, &data)
	// 	if err := utils.ValidateJSON(data); err != nil {
	// 		return err
	// 	}
	// 	data.AddSecretsToRedaction()
	// 	return BackupMogeniusNfsVolume(data)
	// case structs.PAT_STORAGE_RESTORE_VOLUME:
	// 	data := NfsVolumeRestoreRequest{}
	// 	structs.MarshalUnmarshal(&datagram, &data)
	// 	if err := utils.ValidateJSON(data); err != nil {
	// 		return err
	// 	}
	// 	data.AddSecretsToRedaction()
	// 	return RestoreMogeniusNfsVolume(data)
	case structs.PAT_STORAGE_STATS:
		data := services.NfsVolumeStatsRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return services.StatsMogeniusNfsVolume(data)
	case structs.PAT_STORAGE_NAMESPACE_STATS:
		data := services.NfsNamespaceStatsRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return services.StatsMogeniusNfsNamespace(data)
	case structs.PAT_STORAGE_STATUS:
		data := services.NfsStatusRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return services.StatusMogeniusNfs(data)

	case structs.PAT_LOG_LIST_ALL:
		return kubernetes.GetDb().ListLogFromDb()

	// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
	// External Secrets
	// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
	case structs.PAT_EXTERNAL_SECRET_STORE_CREATE:
		data := controllers.CreateSecretsStoreRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return controllers.CreateExternalSecretStore(data)
	case structs.PAT_EXTERNAL_SECRET_STORE_LIST:
		data := controllers.ListSecretStoresRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return controllers.ListExternalSecretsStores(data)
	case structs.PAT_EXTERNAL_SECRET_LIST_AVAILABLE_SECRETS:
		data := controllers.ListSecretsRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return controllers.ListAvailableExternalSecrets(data)
	case structs.PAT_EXTERNAL_SECRET_STORE_DELETE:
		data := controllers.DeleteSecretsStoreRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return controllers.DeleteExternalSecretsStore(data)
	// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
	// Labeled Network Policies
	// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
	case structs.PAT_ATTACH_LABELED_NETWORK_POLICY:
		data := controllers.AttachLabeledNetworkPolicyRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return NewMessageResponse(controllers.AttachLabeledNetworkPolicy(data))
	case structs.PAT_DETACH_LABELED_NETWORK_POLICY:
		data := controllers.DetachLabeledNetworkPolicyRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return NewMessageResponse(controllers.DetachLabeledNetworkPolicy(data))
	case structs.PAT_LIST_LABELED_NETWORK_POLICY_PORTS:
		return NewMessageResponse(controllers.ListLabeledNetworkPolicyPorts())
	case structs.PAT_LIST_CONFLICTING_NETWORK_POLICIES:
		data := controllers.ListConflictingNetworkPoliciesRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return NewMessageResponse(controllers.ListAllConflictingNetworkPolicies(data))
	case structs.PAT_REMOVE_CONFLICTING_NETWORK_POLICIES:
		data := controllers.RemoveConflictingNetworkPoliciesRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return NewMessageResponse(controllers.RemoveConflictingNetworkPolicies(data))
	case structs.PAT_LIST_CONTROLLER_NETWORK_POLICIES:
		data := controllers.ListControllerLabeledNetworkPoliciesRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return NewMessageResponse(controllers.ListControllerLabeledNetwork(data))
	case structs.PAT_UPDATE_NETWORK_POLICIES_TEMPLATE:
		data := []kubernetes.NetworkPolicy{}
		structs.MarshalUnmarshal(&datagram, &data)
		return NewMessageResponse(nil, controllers.UpdateNetworkPolicyTemplate(data))
	case structs.PAT_LIST_ALL_NETWORK_POLICIES:
		return NewMessageResponse(controllers.ListAllNetworkPolicies())
	case structs.PAT_LIST_NAMESPACE_NETWORK_POLICIES:
		data := controllers.ListNamespaceLabeledNetworkPoliciesRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return NewMessageResponse(controllers.ListNamespaceNetworkPolicies(data))
	case structs.PAT_ENFORCE_NETWORK_POLICY_MANAGER:
		data := controllers.EnforceNetworkPolicyManagerRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return NewMessageResponse(nil, controllers.EnforceNetworkPolicyManager(data.NamespaceName))
	case structs.PAT_DISABLE_NETWORK_POLICY_MANAGER:
		data := controllers.DisableNetworkPolicyManagerRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return NewMessageResponse(nil, controllers.DisableNetworkPolicyManager(data.NamespaceName))
	case structs.PAT_REMOVE_UNMANAGED_NETWORK_POLICIES:
		data := controllers.RemoveUnmanagedNetworkPoliciesRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return NewMessageResponse(nil, controllers.RemoveUnmanagedNetworkPolicies(data))
	case structs.PAT_LIST_ONLY_NAMESPACE_NETWORK_POLICIES:
		data := controllers.ListNamespaceLabeledNetworkPoliciesRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return NewMessageResponse(controllers.ListManagedAndUnmanagedNamespaceNetworkPolicies(data))
	// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
	// Cronjobs
	// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
	case structs.PAT_LIST_CRONJOB_JOBS:
		data := ListCronjobJobsRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return kubernetes.ListCronjobJobs(data.ControllerName, data.NamespaceName, data.ProjectId)

	case structs.PAT_LIVE_STREAM_NODES_TRAFFIC_REQUEST:
		data := xterm.WsConnectionRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		go self.xtermService.LiveStreamConnection(data, datagram, httpApi)
		return nil

	case structs.PAT_LIVE_STREAM_NODES_MEMORY_REQUEST:
		data := xterm.WsConnectionRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		go self.xtermService.LiveStreamConnection(data, datagram, httpApi)
		return nil

	case structs.PAT_LIVE_STREAM_NODES_CPU_REQUEST:
		data := xterm.WsConnectionRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		go self.xtermService.LiveStreamConnection(data, datagram, httpApi)
		return nil

	}

	return NewMessageResponse(nil, fmt.Errorf("Pattern not found"))
}

type MessageResponseStatus string

const (
	StatusSuccess MessageResponseStatus = "success"
	StatusError   MessageResponseStatus = "error"
)

type MessageResponse struct {
	Status  MessageResponseStatus `json:"status"` // success, error
	Message string                `json:"message,omitempty"`
	Data    interface{}           `json:"data,omitempty"`
}

func NewMessageResponse(result interface{}, err error) MessageResponse {
	if err != nil {
		return MessageResponse{
			Status:  StatusError,
			Message: err.Error(),
		}
	}
	if str, ok := result.(string); ok {
		return MessageResponse{
			Status:  StatusSuccess,
			Message: str,
		}
	}
	return MessageResponse{
		Status: StatusSuccess,
		Data:   result,
	}
}

type ClusterResourceInfoDto struct {
	LoadBalancerExternalIps []string              `json:"loadBalancerExternalIps"`
	NodeStats               []dtos.NodeStat       `json:"nodeStats"`
	Country                 *utils.CountryDetails `json:"country"`
	Provider                string                `json:"provider"`
	CniConfig               []structs.CniData     `json:"cniConfig"`
}

type K8sManagerUpgradeRequest struct {
	Command string `json:"command" validate:"required"` // complete helm command from platform ui
}

func UpgradeK8sManager(r K8sManagerUpgradeRequest) *structs.Job {
	var wg sync.WaitGroup

	job := structs.CreateJob("Upgrade mogenius platform", "UPGRADE", "", "")
	job.Start()
	kubernetes.UpgradeMyself(job, r.Command, &wg)
	wg.Wait()
	job.Finish()
	return job
}

type ListCronjobJobsRequest struct {
	ProjectId      string `json:"projectId" validate:"required"`
	NamespaceName  string `json:"namespaceName" validate:"required"`
	ControllerName string `json:"controllerName" validate:"required"`
}

func (self *socketApi) logStream(data services.ServiceLogStreamRequest, datagram structs.Datagram) services.ServiceLogStreamResult {
	_ = datagram
	result := services.ServiceLogStreamResult{}

	url, err := url.Parse(data.PostTo)
	if err != nil {
		result.Error = err.Error()
		result.Success = false
		self.logger.Error(result.Error)
		return result
	}

	pod := kubernetes.PodStatus(data.Namespace, data.PodId, false)
	terminatedState := kubernetes.LastTerminatedStateIfAny(pod)

	var previousResReq *rest.Request
	if terminatedState != nil {
		tmpPreviousResReq, err := services.PreviousPodLogStream(data.Namespace, data.PodId)
		if err != nil {
			self.logger.Error("failed to get previous pod log stream", "error", err)
		} else {
			previousResReq = tmpPreviousResReq
		}
	}

	restReq, err := services.PodLogStream(data)
	if err != nil {
		result.Error = err.Error()
		result.Success = false
		self.logger.Error(result.Error)
		return result
	}

	if terminatedState != nil {
		self.logger.Info("Logger try multiStreamData")
		go self.multiStreamData(previousResReq, restReq, terminatedState, url.String())
	} else {
		self.logger.Info("Logger try streamData")
		go self.streamData(restReq, url.String())
	}

	result.Success = true

	return result
}

func (self *socketApi) streamData(restReq *rest.Request, toServerUrl string) {
	ctx := context.Background()
	cancelCtx, endGofunc := context.WithCancel(ctx)
	stream, err := restReq.Stream(cancelCtx)
	if err != nil {
		self.logger.Error(err.Error())
	} else {
		structs.SendDataWs(toServerUrl, stream)
	}
	endGofunc()
}

func (self *socketApi) multiStreamData(previousRestReq *rest.Request, restReq *rest.Request, terminatedState *v1.ContainerStateTerminated, toServerUrl string) {
	ctx := context.Background()
	ctx, endGofunc := context.WithCancel(ctx)
	defer endGofunc()

	lastState := kubernetes.LastTerminatedStateToString(terminatedState)

	var previousStream io.ReadCloser
	if previousRestReq != nil {
		tmpPreviousStream, err := previousRestReq.Stream(ctx)
		if err != nil {
			self.logger.Error(err.Error())
			previousStream = io.NopCloser(strings.NewReader(fmt.Sprintln(err.Error())))
		} else {
			previousStream = tmpPreviousStream
		}
	}

	stream, err := restReq.Stream(ctx)
	if err != nil {
		self.logger.Error(err.Error())
		stream = io.NopCloser(strings.NewReader(fmt.Sprintln(err.Error())))
	}

	nl := strings.NewReader("\n")
	previousState := strings.NewReader(lastState)
	headlineLastLog := strings.NewReader("Last Log:\n")
	headlineCurrentLog := strings.NewReader("Current Log:\n")

	mergedStream := io.MultiReader(previousState, nl, headlineLastLog, nl, previousStream, nl, headlineCurrentLog, nl, stream)

	structs.SendDataWs(toServerUrl, io.NopCloser(mergedStream))
}

func (self *socketApi) execShConnection(podCmdConnectionRequest xterm.PodCmdConnectionRequest) {
	// allows to execute itself without being in $PATH (e.g. while developing locally)
	bin, err := os.Executable()
	if err != nil {
		self.logger.Error("failed to get current executable path", "error", err)
		return
	}

	cmd := exec.Command(
		bin,
		"exec",
		"--namespace",
		podCmdConnectionRequest.Namespace,
		"--pod",
		podCmdConnectionRequest.Pod,
		"--container",
		podCmdConnectionRequest.Container,
		"--",
		"sh",
	)

	xterm.XTermCommandStreamConnection(
		"exec-sh",
		podCmdConnectionRequest.WsConnection,
		podCmdConnectionRequest.Namespace,
		podCmdConnectionRequest.Controller,
		podCmdConnectionRequest.Pod,
		podCmdConnectionRequest.Container,
		cmd,
		nil,
	)
}

func (self *socketApi) logStreamConnection(podCmdConnectionRequest xterm.PodCmdConnectionRequest) {
	bin, err := os.Executable()
	if err != nil {
		self.logger.Error("failed to get current executable path", "error", err)
		return
	}

	cmd := exec.Command(
		bin,
		"logs",
		"--namespace",
		podCmdConnectionRequest.Namespace,
		"--pod",
		podCmdConnectionRequest.Pod,
		"--container",
		podCmdConnectionRequest.Container,
		"--tail-lines",
		podCmdConnectionRequest.LogTail,
	)

	xterm.XTermCommandStreamConnection(
		"log",
		podCmdConnectionRequest.WsConnection,
		podCmdConnectionRequest.Namespace,
		podCmdConnectionRequest.Controller,
		podCmdConnectionRequest.Pod,
		podCmdConnectionRequest.Container,
		cmd,
		services.GetPreviousLogContent(podCmdConnectionRequest),
	)
}

func buildLogStreamConnection(buildLogConnectionRequest xterm.BuildLogConnectionRequest) {
	xterm.XTermBuildLogStreamConnection(
		buildLogConnectionRequest.WsConnection,
		buildLogConnectionRequest.Namespace,
		buildLogConnectionRequest.Controller,
		buildLogConnectionRequest.Container,
		buildLogConnectionRequest.BuildTask,
		buildLogConnectionRequest.BuildId,
	)
}

func componentLogStreamConnection(componentLogConnectionRequest xterm.ComponentLogConnectionRequest) {
	xterm.XTermComponentStreamConnection(
		componentLogConnectionRequest.WsConnection,
		componentLogConnectionRequest.Component,
		componentLogConnectionRequest.Namespace,
		componentLogConnectionRequest.Controller,
		componentLogConnectionRequest.Release,
	)
}

func podEventStreamConnection(buildLogConnectionRequest xterm.PodEventConnectionRequest) {
	xterm.XTermPodEventStreamConnection(
		buildLogConnectionRequest.WsConnection,
		buildLogConnectionRequest.Namespace,
		buildLogConnectionRequest.Controller,
	)
}

func scanImageLogStreamConnection(buildLogConnectionRequest xterm.ScanImageLogConnectionRequest) {
	xterm.XTermScanImageLogStreamConnection(
		buildLogConnectionRequest.WsConnection,
		buildLogConnectionRequest.Namespace,
		buildLogConnectionRequest.Controller,
		buildLogConnectionRequest.Container,
		buildLogConnectionRequest.CmdType,
		buildLogConnectionRequest.ScanImageType,
		buildLogConnectionRequest.ContainerRegistryUrl,
		&buildLogConnectionRequest.ContainerRegistryUser,
		&buildLogConnectionRequest.ContainerRegistryPat,
	)
}

func ExecuteBinaryRequestUpload(datagram structs.Datagram) *services.FilesUploadRequest {
	data := services.FilesUploadRequest{}
	structs.MarshalUnmarshal(&datagram, &data)
	return &data
}
