package socketserver

import (
	"bufio"
	"fmt"
	"io"
	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/services"
	"mogenius-k8s-manager/structs"
	"mogenius-k8s-manager/utils"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator"
	"github.com/gorilla/websocket"
	jsoniter "github.com/json-iterator/go"
	"github.com/mattn/go-tty"
	punqUtils "github.com/mogenius/punq/utils"
	"github.com/schollz/progressbar/v3"
)

var loadTestStartTime time.Time
var loadTestPattern string = "list/pods"
var loadTestTotalBytes int64 = 0
var loadTestRequests int = 10000
var loadTestReceived int = 0

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

var validate = validator.New()
var cluster *structs.ClusterConnection
var serverSendMutex sync.Mutex

func Init(r *gin.Engine) {
	// r.Use(user.AuthUserMiddleware())
	r.GET(utils.CONFIG.ApiServer.WS_Path, func(c *gin.Context) {
		clusterName := validateHeader(c)
		if clusterName != "" {
			wsHandler(c.Writer, c.Request, clusterName)
		}
	})
	r.GET(utils.CONFIG.EventServer.Path, func(c *gin.Context) {
		clusterName := validateHeader(c)
		if clusterName != "" {
			wsHandler(c.Writer, c.Request, clusterName)
		}
	})
	r.GET(utils.CONFIG.ShellServer.Path, func(c *gin.Context) {
		clusterName := validateHeader(c)
		if clusterName != "" {
			wsShellHandler(c.Writer, c.Request, clusterName)
		}
	})
}

func wsShellHandler(w http.ResponseWriter, r *http.Request, clusterName string) {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Log.Error("websocket connection err:", err)
		return
	}
	defer c.Close()

	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			text := scanner.Text()
			if err := c.WriteMessage(websocket.TextMessage, []byte(text)); err != nil {
				fmt.Println("Error writing message:", err)
				return
			}
		}
		if err := scanner.Err(); err != nil {
			fmt.Println("Error reading from stdin:", err)
			return
		}
	}()

	for {
		_, msg, err := c.ReadMessage()
		if err != nil {
			fmt.Println("Error reading from WebSocket:", err)
			break
		}

		fmt.Println(string(msg))
	}
}

// should handle more errors
func wsHandler(w http.ResponseWriter, r *http.Request, clusterName string) {

	connection, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Log.Error("websocket connection err:", err)
		return
	}

	defer connection.Close()

	if r.RequestURI == utils.CONFIG.ApiServer.WS_Path {
		addConnection(connection, clusterName)
	}

	for {
		msgType, msg, err := connection.ReadMessage()
		if err != nil {
			logger.Log.Error("websocket read err:", err)
			break
		}

		switch msgType {
		case websocket.BinaryMessage:
			fmt.Print(string(msg))
		case websocket.TextMessage:
			recvText := string(msg)
			if strings.HasPrefix(recvText, "######START######;") || strings.HasPrefix(recvText, "######END######;") {
				currentMsg := string(msg)
				currentMsg = strings.Replace(currentMsg, "######START######;", "", 1)
				currentMsg = strings.Replace(currentMsg, "######END######;", "", 1)
				msg = []byte(currentMsg)
			}

			datagram := structs.CreateEmptyDatagram()
			var json = jsoniter.ConfigCompatibleWithStandardLibrary
			_ = json.Unmarshal(msg, &datagram)
			datagramValidationError := validate.Struct(datagram)

			if datagramValidationError != nil {
				logger.Log.Errorf("Invalid datagram: %s", datagramValidationError.Error())
				continue
			} else {
				if punqUtils.Contains(services.COMMAND_REQUESTS, datagram.Pattern) ||
					punqUtils.Contains(services.BINARY_REQUEST_UPLOAD, datagram.Pattern) {
					if datagram.Pattern == "namespace/backup" {
						backupData := datagram.Payload.(map[string]interface{})["data"].(string)
						name := datagram.Payload.(map[string]interface{})["namespaceName"].(string)
						messages := datagram.Payload.(map[string]interface{})["messages"].([]interface{})
						fmt.Printf("Backuped '%s'. Saved to 'backup.yaml'. Bytes=%d", name, len(backupData))
						fmt.Println("Messages:")
						for _, msg := range messages {
							fmt.Println(msg)
						}
						err := os.WriteFile("backup.yaml", []byte(backupData), os.ModePerm)
						if err != nil {
							logger.Log.Error(err.Error())
						}
					} else if datagram.Pattern != "KubernetesEvent" {
						RECEIVCOLOR := color.New(color.FgBlack, color.BgBlue).SprintFunc()
						fmt.Printf("%s\n", RECEIVCOLOR(punqUtils.FillWith("RECEIVED", 22, " ")))
						datagram.DisplayBeautiful()

						if datagram.Pattern == loadTestPattern {
							loadTestTotalBytes += datagram.GetSize()
							loadTestReceived++
						}
						if loadTestReceived > 0 {
							fmt.Printf("Result (%d): %s / %s \n", loadTestReceived, time.Since(loadTestStartTime), punqUtils.BytesToHumanReadable(loadTestTotalBytes))
						}
					}
				} else {
					logger.Log.Errorf("Pattern not found: '%s'.", datagram.Pattern)
				}
			}
		case websocket.CloseMessage:
			logger.Log.Warning("Received websocket.CloseMessage.")
		case websocket.PingMessage:
			logger.Log.Warning("Received websocket.PingMessage.")
		case websocket.PongMessage:
			logger.Log.Warning("Received websocket.PongMessage.")
		default:
			logger.Log.Warningf("Received unknown messageType '%d' via websocket.", msgType)
		}
	}
}

func validateHeader(c *gin.Context) string {
	userAgent := c.Request.Header.Get("User-Agent")
	if userAgent == "" {
		userAgent = "unknown"
	}

	apiKey := c.Request.Header.Get("x-authorization")
	if apiKey != utils.CONFIG.Kubernetes.ApiKey {
		logger.Log.Errorf("Invalid x-authorization: '%s'", apiKey)
		return ""
	}

	clusterName := c.Request.Header.Get("x-cluster-name")
	if clusterName == "" {
		logger.Log.Errorf("Invalid x-cluster-name: '%s'", clusterName)
		return ""
	}

	logger.Log.Infof("New client connected %s -> %s (Agent: %s)", c.Request.RequestURI, c.Request.RemoteAddr, userAgent)
	return clusterName
}

func addConnection(wsconnection *websocket.Conn, clusterName string) {
	serverSendMutex.Lock()
	defer serverSendMutex.Unlock()
	// remoteAddr := connection.RemoteAddr().String()
	cluster = &structs.ClusterConnection{ClusterName: clusterName, Connection: wsconnection, AddedAt: time.Now()}
}

func printShortcuts() {
	logger.Log.Notice("Keyboard shortcusts: ")
	logger.Log.Notice("h:     help")
	logger.Log.Notice("l:     list clusters")
	logger.Log.Notice("s:     send command to cluster")
	logger.Log.Notice("c:     close blocked connection")
	logger.Log.Notice("k:     close all connections")
	logger.Log.Notice("x:     perform load test")
	logger.Log.Notice("q:     quit application")
}

func ReadInput() {
	printShortcuts()

	tty, err := tty.Open()
	if err != nil {
		logger.Log.Fatalf("Error opening terminal: %s", err.Error())
	}
	defer tty.Close()

	for {
		r, err := tty.ReadRune()
		if err != nil {
			logger.Log.Fatalf("Error reading from terminal: %s", err.Error())
		}
		switch string(r) {
		case "h":
			printShortcuts()
		case "s":
			cmd := selectCommands()
			if cmd != "" {
				requestCmdFromCluster(cmd)
			} else {
				printShortcuts()
			}
		case "x":
			loadTestStartTime = time.Now()
			loadTestReceived = 0
			for i := 0; i < loadTestRequests; i++ {
				go func() {
					datagram := requestCmdFromCluster(services.PAT_LIST_PODS)
					loadTestTotalBytes = datagram.GetSize()
				}()
			}
		case "k":
			closeAllConnections()
		case "q":
			os.Exit(0)
		default:
			logger.Log.Errorf("Unrecognized character '%s'.", r)
			printShortcuts()
		}
	}
}

func closeAllConnections() {
	cluster.Connection.Close()
	cluster = nil
}

func requestCmdFromCluster(pattern string) *structs.Datagram {
	if cluster.Connection != nil {
		var payload interface{} = nil
		switch pattern {
		case services.PAT_K8SNOTIFICATION:
			payload = nil
		case services.PAT_CLUSTERSTATUS:
			payload = nil
		case services.PAT_CLUSTERRESOURCEINFO:
			payload = nil
		case services.PAT_CLUSTER_EXECUTE_HELM_CHART_TASK:
			payload = services.ClusterHelmRequestExample()
		case services.PAT_CLUSTER_UNINSTALL_HELM_CHART:
			payload = services.ClusterHelmUninstallRequestExample()
		case services.PAT_CLUSTER_TCP_UDP_CONFIGURATION:
			payload = nil

		case services.PAT_CLUSTER_WRITE_CONFIGMAP:
			payload = services.ClusterWriteConfigMapExample()
		case services.PAT_CLUSTER_READ_CONFIGMAP:
			payload = services.ClusterGetConfigMapExample()
		case services.PAT_CLUSTER_LIST_CONFIGMAPS:
			payload = services.ClusterListWorkloadsExample()
		case services.PAT_CLUSTER_LIST_DEPLOYMENTS:
			payload = services.ClusterListWorkloadsExample()
		case services.PAT_INSTALL_CLUSTER_ISSUER:
			payload = services.ClusterIssuerInstallRequestExample()

		case services.PAT_UPGRADEK8SMANAGER:
			payload = services.K8sManagerUpgradeRequestExample()

		case services.PAT_INSTALL_LOCAL_DEV_COMPONENTS:
			payload = services.ClusterIssuerInstallRequestExample()

		case services.PAT_FILES_LIST:
			payload = services.FilesListRequestExampleData()
		case services.PAT_FILES_DOWNLOAD:
			payload = services.FilesDownloadRequestExampleData()
		case services.PAT_FILES_UPLOAD:
			payload = services.FilesUploadRequestExampleData()
		case services.PAT_FILES_CREATE_FOLDER:
			payload = services.FilesCreateFolderRequestExampleData()
		case services.PAT_FILES_RENAME:
			payload = services.FilesRenameRequestExampleData()
		case services.PAT_FILES_CHOWN:
			payload = services.FilesChownRequestExampleData()
		case services.PAT_FILES_CHMOD:
			payload = services.FilesChmodRequestExampleData()
		case services.PAT_FILES_DELETE:
			payload = services.FilesDeleteRequestExampleData()

		case services.PAT_NAMESPACE_CREATE:
			payload = services.NamespaceCreateRequestExample()
		case services.PAT_NAMESPACE_DELETE:
			payload = services.NamespaceDeleteRequestExample()
		case services.PAT_NAMESPACE_SHUTDOWN:
			payload = services.NamespaceShutdownRequestExample()
		case services.PAT_NAMESPACE_POD_IDS:
			payload = services.NamespacePodIdsRequestExample()
		case services.PAT_NAMESPACE_VALIDATE_CLUSTER_PODS:
			payload = services.NamespaceValidateClusterPodsRequestExample()
		case services.PAT_NAMESPACE_VALIDATE_PORTS:
			payload = services.NamespaceValidatePortsRequestExample()
		case services.PAT_NAMESPACE_LIST_ALL:
			payload = nil
		case services.PAT_NAMESPACE_GATHER_ALL_RESOURCES:
			payload = services.NamespaceGatherAllResourcesRequestExample()
		case services.PAT_NAMESPACE_BACKUP:
			payload = services.NamespaceBackupRequestExample()
		case services.PAT_NAMESPACE_RESTORE:
			payload = services.NamespaceRestoreRequestExample()
		case services.PAT_NAMESPACE_RESOURCE_YAML:
			payload = services.NamespaceResourceYamlRequestExample()

		case services.PAT_SERVICE_CREATE:
			payload = services.ServiceCreateRequestExample()
		case services.PAT_SERVICE_DELETE:
			payload = services.ServiceDeleteRequestExample()
		case services.PAT_SERVICE_POD_IDS:
			payload = services.ServiceGetPodIdsRequestExample()
		case services.PAT_SERVICE_POD_EXISTS:
			payload = services.ServicePodExistsRequestExample()
		case services.PAT_SERVICE_PODS:
			payload = services.ServicePodsRequestExample()
		case services.PAT_SERVICE_SET_IMAGE:
			payload = services.ServiceSetImageRequestExample()
		case services.PAT_SERVICE_LOG:
			payload = services.ServiceGetLogRequestExample()
		case services.PAT_SERVICE_LOG_ERROR:
			payload = services.ServiceGetLogRequestExample()
		case services.PAT_SERVICE_LOG_STREAM:
			payload = services.ServiceLogStreamRequestExample()
		case services.PAT_SERVICE_RESOURCE_STATUS:
			payload = services.ServiceResourceStatusRequestExample()
		case services.PAT_SERVICE_RESTART:
			payload = services.ServiceRestartRequestExample()
		case services.PAT_SERVICE_STOP:
			payload = services.ServiceStopRequestExample()
		case services.PAT_SERVICE_START:
			payload = services.ServiceStartRequestExample()
		case services.PAT_SERVICE_UPDATE_SERVICE:
			payload = services.ServiceUpdateRequestExample()
		case services.PAT_SERVICE_STATUS:
			payload = services.ServiceStatusRequestExample()
		case services.PAT_SERVICE_TRIGGER_JOB:
			payload = services.ServiceTriggerJobRequestExample()

		case services.PAT_LIST_CREATE_TEMPLATES:
			payload = nil

		case services.PAT_LIST_NAMESPACES:
			payload = services.K8sListRequestExample()
		case services.PAT_LIST_DEPLOYMENTS:
			payload = services.K8sListRequestExample()
		case services.PAT_LIST_SERVICES:
			payload = services.K8sListRequestExample()
		case services.PAT_LIST_PODS:
			payload = services.K8sListRequestExample()
		case services.PAT_LIST_INGRESSES:
			payload = services.K8sListRequestExample()
		case services.PAT_LIST_CONFIGMAPS:
			payload = services.K8sListRequestExample()
		case services.PAT_LIST_SECRETS:
			payload = services.K8sListRequestExample()
		case services.PAT_LIST_NODES:
			payload = services.K8sListRequestExample()
		case services.PAT_LIST_DAEMONSETS:
			payload = services.K8sListRequestExample()
		case services.PAT_LIST_STATEFULSETS:
			payload = services.K8sListRequestExample()
		case services.PAT_LIST_JOBS:
			payload = services.K8sListRequestExample()
		case services.PAT_LIST_CRONJOBS:
			payload = services.K8sListRequestExample()
		case services.PAT_LIST_REPLICASETS:
			payload = services.K8sListRequestExample()
		case services.PAT_LIST_PERSISTENT_VOLUMES:
			payload = services.K8sListRequestExample()
		case services.PAT_LIST_PERSISTENT_VOLUME_CLAIMS:
			payload = services.K8sListRequestExample()
		case services.PAT_LIST_VOLUME_ATTACHMENT:
			payload = services.K8sListRequestExample()
		case services.PAT_LIST_STORAGE_CLASS:
			payload = services.K8sListRequestExample()
		case services.PAT_LIST_NETWORK_POLICY:
			payload = services.K8sListRequestExample()

		case services.PAT_DESCRIBE_NAMESPACE:
			payload = services.K8sDescribeRequestExample()
		case services.PAT_DESCRIBE_DEPLOYMENT:
			payload = services.K8sDescribeRequestExample()
		case services.PAT_DESCRIBE_SERVICE:
			payload = services.K8sDescribeRequestExample()
		case services.PAT_DESCRIBE_POD:
			payload = services.K8sDescribeRequestExample()
		case services.PAT_DESCRIBE_INGRESS:
			payload = services.K8sDescribeRequestExample()
		case services.PAT_DESCRIBE_CONFIGMAP:
			payload = services.K8sDescribeRequestExample()
		case services.PAT_DESCRIBE_SECRET:
			payload = services.K8sDescribeRequestExample()
		case services.PAT_DESCRIBE_NODE:
			payload = services.K8sDescribeRequestExample()
		case services.PAT_DESCRIBE_DAEMONSET:
			payload = services.K8sDescribeRequestExample()
		case services.PAT_DESCRIBE_STATEFULSET:
			payload = services.K8sDescribeRequestExample()
		case services.PAT_DESCRIBE_JOB:
			payload = services.K8sDescribeRequestExample()
		case services.PAT_DESCRIBE_CRONJOB:
			payload = services.K8sDescribeRequestExample()
		case services.PAT_DESCRIBE_REPLICASET:
			payload = services.K8sDescribeRequestExample()
		case services.PAT_DESCRIBE_PERSISTENT_VOLUME:
			payload = services.K8sDescribeRequestExample()
		case services.PAT_DESCRIBE_PERSISTENT_VOLUME_CLAIM:
			payload = services.K8sDescribeRequestExample()
		case services.PAT_DESCRIBE_VOLUME_ATTACHMENT:
			payload = services.K8sDescribeRequestExample()
		case services.PAT_DESCRIBE_STORAGE_CLASS:
			payload = services.K8sDescribeRequestExample()
		case services.PAT_DESCRIBE_NETWORK_POLICY:
			payload = services.K8sDescribeRequestExample()

		case services.PAT_UPDATE_DEPLOYMENT:
			payload = services.K8sUpdateDeploymentRequestExample()
		case services.PAT_UPDATE_SERVICE:
			payload = services.K8sUpdateServiceRequestExample()
		case services.PAT_UPDATE_POD:
			payload = services.K8sUpdatePodRequestExample()
		case services.PAT_UPDATE_INGRESS:
			payload = services.K8sUpdateIngressRequestExample()
		case services.PAT_UPDATE_CONFIGMAP:
			payload = services.K8sUpdateConfigmapRequestExample()
		case services.PAT_UPDATE_SECRET:
			payload = services.K8sUpdateSecretRequestExample()
		case services.PAT_UPDATE_DAEMONSET:
			payload = services.K8sUpdateDaemonsetRequestExample()
		case services.PAT_UPDATE_STATEFULSET:
			payload = services.K8sUpdateStatefulSetRequestExample()
		case services.PAT_UPDATE_JOB:
			payload = services.K8sUpdateJobRequestExample()
		case services.PAT_UPDATE_CRONJOB:
			payload = services.K8sUpdateCronJobRequestExample()
		case services.PAT_UPDATE_REPLICASET:
			payload = services.K8sUpdateReplicaSetRequestExample()
		case services.PAT_UPDATE_PERSISTENT_VOLUME:
			payload = services.K8sUpdatePersistentVolumeRequestExample()
		case services.PAT_UPDATE_PERSISTENT_VOLUME_CLAIM:
			payload = services.K8sUpdatePersistentVolumeClaimRequestExample()
		case services.PAT_UPDATE_STORAGE_CLASS:
			payload = services.K8sUpdateStorageClassExample()
		case services.PAT_UPDATE_NETWORK_POLICY:
			payload = services.K8sUpdateNetworkPolicyExample()

		case services.PAT_DELETE_NAMESPACE:
			payload = services.K8sDeleteNamespaceRequestExample()
		case services.PAT_DELETE_DEPLOYMENT:
			payload = services.K8sDeleteDeploymentRequestExample()
		case services.PAT_DELETE_SERVICE:
			payload = services.K8sDeleteServiceRequestExample()
		case services.PAT_DELETE_POD:
			payload = services.K8sDeletePodRequestExample()
		case services.PAT_DELETE_INGRESS:
			payload = services.K8sDeleteIngressRequestExample()
		case services.PAT_DELETE_CONFIGMAP:
			payload = services.K8sDeleteConfigmapRequestExample()
		case services.PAT_DELETE_SECRET:
			payload = services.K8sDeleteSecretRequestExample()
		case services.PAT_DELETE_DAEMONSET:
			payload = services.K8sDeleteDaemonsetRequestExample()
		case services.PAT_DELETE_STATEFULSET:
			payload = services.K8sDeleteStatefulsetRequestExample()
		case services.PAT_DELETE_JOB:
			payload = services.K8sDeleteJobRequestExample()
		case services.PAT_DELETE_CRONJOB:
			payload = services.K8sDeleteCronjobRequestExample()
		case services.PAT_DELETE_REPLICASET:
			payload = services.K8sDeleteReplicaSetRequestExample()
		case services.PAT_DELETE_PERSISTENT_VOLUME:
			payload = services.K8sDeletePersistentVolumeRequestExample()
		case services.PAT_DELETE_PERSISTENT_VOLUME_CLAIM:
			payload = services.K8sDeletePersistentVolumeClaimRequestExample()
		case services.PAT_DELETE_NETWORK_POLICY:
			payload = services.K8sDeleteNetworkPolicyExample()
		case services.PAT_DELETE_STORAGE_CLASS:
			payload = services.K8sDeleteStorageClassExample()

		case services.PAT_BUILDER_STATUS:
			payload = nil
		case services.PAT_BUILD_INFOS:
			payload = structs.BuildJobExample()
		case services.PAT_BUILD_LIST_ALL:
			payload = nil
		case services.PAT_BUILD_LIST_BY_PROJECT:
			payload = structs.ListBuildByProjectIdRequestExample()
		case services.PAT_BUILD_ADD:
			payload = structs.BuildJobExample()
		case services.PAT_BUILD_SCAN:
			payload = structs.ScanImageRequestExample()
		case services.PAT_BUILD_CANCEL:
			payload = structs.BuildJobExample()
		case services.PAT_BUILD_DELETE:
			payload = structs.BuildJobExample()
		case services.PAT_BUILD_LAST_JOB_OF_SERVICES:
			payload = structs.BuildServicesStatusRequestExample()
		case services.PAT_BUILD_JOB_LIST_OF_SERVICE:
			payload = structs.BuildServiceRequestExample()
		case services.PAT_BUILD_LAST_JOB_INFO_OF_SERVICE:
			payload = structs.BuildServiceRequestExample()

		case services.PAT_STORAGE_CREATE_VOLUME:
			payload = services.NfsVolumeRequestExample()
		case services.PAT_STORAGE_DELETE_VOLUME:
			payload = services.NfsVolumeRequestExample()
		case services.PAT_STORAGE_BACKUP_VOLUME:
			payload = services.NfsVolumeBackupRequestExample()
		case services.PAT_STORAGE_RESTORE_VOLUME:
			payload = services.NfsVolumeRestoreRequestExample()
		case services.PAT_STORAGE_STATS:
			payload = services.NfsVolumeRequestExample()
		case services.PAT_STORAGE_NAMESPACE_STATS:
			payload = services.NfsNamespaceStatsRequestExample()

		case services.PAT_EXEC_SHELL:
			payload = nil

		case services.PAT_POPEYE_CONSOLE:
			payload = nil
		}

		datagram := structs.CreateDatagramFrom(pattern, payload)
		serverSendMutex.Lock()
		err := cluster.Connection.WriteJSON(datagram)
		serverSendMutex.Unlock()
		if err != nil {
			logger.Log.Error(err.Error())
		}
		datagram.DisplayBeautiful()

		// send file after pattern
		if pattern == services.PAT_FILES_UPLOAD {
			sendFile()
		}
		return &datagram
	}
	logger.Log.Error("Not connected to any cluster.")
	return nil
}

func selectCommands() string {
	allCommands := append([]string{}, services.COMMAND_REQUESTS...)
	allCommands = append(allCommands, services.BINARY_REQUEST_UPLOAD...)
	for index, patternName := range allCommands {
		fmt.Printf("%d: %s\n", index, patternName)
	}

	fmt.Println("input number:")
	var number int
	_, err := fmt.Scanf("%d", &number)
	if err != nil {
		logger.Log.Errorf("Unrecognized character '%s'. Please select 0-%d.", number, len(allCommands)-1)
		return ""
	}
	fmt.Println(number)

	if len(allCommands) >= number {
		return allCommands[number]
	} else {
		logger.Log.Errorf("Unrecognized character '%s'. Please select 0-%d.", number, len(allCommands)-1)
		return ""
	}
}

func sendFile() {
	err := utils.ZipSource("./video.mp4", "test.zip")
	if err != nil {
		logger.Log.Error(err)
		return
	}

	file, err := os.Open("./test.zip")
	if err != nil {
		logger.Log.Error(err)
		return
	}
	info, err := file.Stat()
	var totalSize int64 = 0
	if err == nil {
		totalSize = info.Size()
	} else {
		logger.Log.Error(err)
		return
	}

	reader := bufio.NewReader(file)
	if reader != nil && totalSize > 0 {
		buf := make([]byte, 512)
		bar := progressbar.DefaultBytes(totalSize)

		serverSendMutex.Lock()
		cluster.Connection.WriteMessage(websocket.TextMessage, []byte("######START_UPLOAD######;"))
		for {
			chunk, err := reader.Read(buf)
			if err != nil {
				if err != io.EOF {
					fmt.Println(err)
				}
				bar.Finish()
				break
			}
			cluster.Connection.WriteMessage(websocket.BinaryMessage, buf)
			bar.Add(chunk)
		}
		if err != nil {
			logger.Log.Errorf("reading bytes error: %s", err.Error())
		}
		cluster.Connection.WriteMessage(websocket.TextMessage, []byte("######END_UPLOAD######;"))
		serverSendMutex.Unlock()
	} else {
		logger.Log.Error("reader cannot be nil")
		logger.Log.Error("file size cannot be nil")
	}
}
