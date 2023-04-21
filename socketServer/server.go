package socketServer

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
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
	"github.com/schollz/progressbar/v3"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

var validate = validator.New()
var connections = make(map[string]*structs.ClusterConnection)
var sendMutex sync.Mutex

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
	r.POST(utils.CONFIG.ApiServer.StreamPath, func(c *gin.Context) {
		clusterName := validateHeader(c)
		if clusterName != "" {
			ctx := context.Background()
			cancelCtx, _ := context.WithCancel(ctx)

			reader := bufio.NewScanner(c.Request.Body)
			for {
				select {
				case <-cancelCtx.Done():
					fmt.Println("done")
					return
				default:
					for reader.Scan() {
						lastBytes := reader.Bytes()
						fmt.Println(string(lastBytes))
					}
				}
			}
			// data, err := ioutil.ReadAll(c.Request.Body)
			// if err != nil {
			// 	fmt.Println(err.Error())
			// }
			fmt.Println(c.Request.Header)
			// fmt.Println(string(data))
		}
	})
}

// should handle more errors
func wsHandler(w http.ResponseWriter, r *http.Request, clusterName string) {

	connection, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Log.Error("websocket connection err:", err)
		return
	}

	defer removeConnection(connection)

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
				if utils.Contains(services.COMMAND_REQUESTS, datagram.Pattern) ||
					utils.Contains(services.BINARY_REQUEST_UPLOAD, datagram.Pattern) {
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
						fmt.Printf("%s\n", RECEIVCOLOR(utils.FillWith("RECEIVED", 22, " ")))
						datagram.DisplayBeautiful()
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

	logger.Log.Infof("New client connected %s/%s (Agent: %s)", clusterName, c.Request.RemoteAddr, userAgent)
	return clusterName
}

func addConnection(connection *websocket.Conn, clusterName string) {
	sendMutex.Lock()
	defer sendMutex.Unlock()
	remoteAddr := connection.RemoteAddr().String()
	connections[remoteAddr] = &structs.ClusterConnection{ClusterName: clusterName, Connection: connection, AddedAt: time.Now()}
}

func removeConnection(connection *websocket.Conn) {
	sendMutex.Lock()
	defer sendMutex.Unlock()
	remoteAddr := connection.RemoteAddr().String()
	connection.Close()
	delete(connections, remoteAddr)
}

func printShortcuts() {
	logger.Log.Notice("Keyboard shortcusts: ")
	logger.Log.Notice("h:     help")
	logger.Log.Notice("l:     list clusters")
	logger.Log.Notice("s:     send command to cluster")
	logger.Log.Notice("c:     close blocked connection")
	logger.Log.Notice("k:     close all connections")
	logger.Log.Notice("q:     quit application")
}

func ReadInput() {
	printShortcuts()

	tty, err := tty.Open()
	if err != nil {
		log.Fatal(err)
	}
	defer tty.Close()

	for {
		r, err := tty.ReadRune()
		if err != nil {
			log.Fatal(err)
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
		case "l":
			listClusters()
		case "c":
			closeBlockedConnection()
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

func closeBlockedConnection() {
	cluster := selectBlockedCluster()
	if cluster != nil {
		cluster.Connection.Close()
	}
}

func closeAllConnections() {
	for _, cluster := range connections {
		cluster.Connection.Close()
	}
}

func listClusters() []string {
	var result []string = make([]string, 0)
	logger.Log.Noticef("Listing %d connected clusters:", len(connections))
	count := 0
	for _, value := range connections {
		count++
		logger.Log.Noticef("%d: %s/%s", count, value.ClusterName, value.Connection.RemoteAddr().String())
		result = append(result, value.ClusterName)
	}
	return result
}

func requestCmdFromCluster(pattern string) {
	var blockConnection = false

	if len(connections) > 0 {
		var payload interface{} = nil
		switch pattern {
		case "K8sNotification":
			payload = nil
		case "ClusterStatus":
			payload = nil
		case "ClusterResourceInfo":
			payload = nil
		case "cluster/execute-helm-chart-task":
			payload = services.ClusterHelmRequestExample()
		case "cluster/uninstall-helm-chart":
			payload = services.ClusterHelmUninstallRequestExample()
		case "cluster/tcp-udp-configuration":
			payload = nil

		case "UpgradeK8sManager":
			payload = services.K8sManagerUpgradeRequestExample()

		case "files/list":
			payload = services.FilesListRequestExampleData()
		case "files/download":
			payload = services.FilesDownloadRequestExampleData()
		case "files/upload":
			payload = services.FilesUploadRequestExampleData()
		case "files/create-folder":
			payload = services.FilesCreateFolderRequestExampleData()
		case "files/rename":
			payload = services.FilesRenameRequestExampleData()
		case "files/chown":
			payload = services.FilesChownRequestExampleData()
		case "files/chmod":
			payload = services.FilesChmodRequestExampleData()
		case "files/delete":
			payload = services.FilesDeleteRequestExampleData()

		case "namespace/create":
			payload = services.NamespaceCreateRequestExample()
		case "namespace/delete":
			payload = services.NamespaceDeleteRequestExample()
		case "namespace/shutdown":
			payload = services.NamespaceShutdownRequestExample()
		case "namespace/pod-ids":
			payload = services.NamespacePodIdsRequestExample()
		case "namespace/validate-cluster-pods":
			payload = services.NamespaceValidateClusterPodsRequestExample()
		case "namespace/validate-ports":
			payload = services.NamespaceValidatePortsRequestExample()
		case "namespace/storage-size":
			payload = services.NamespaceStorageSizeRequestExample()
		case "namespace/list-all":
			payload = nil
		case "namespace/gather-all-resources":
			payload = services.NamespaceGatherAllResourcesRequestExample()
		case "namespace/backup":
			payload = services.NamespaceBackupRequestExample()
		case "namespace/restore":
			payload = services.NamespaceRestoreRequestExample()

		case "service/create":
			payload = services.ServiceCreateRequestExample()
		case "service/delete":
			payload = services.ServiceDeleteRequestExample()
		case "service/pod-ids":
			payload = services.ServiceGetPodIdsRequestExample()
		case "SERVICE_POD_EXISTS":
			payload = services.ServicePodExistsRequestExample()
		case "service/set-image":
			payload = services.ServiceSetImageRequestExample()
		case "service/log":
			payload = services.ServiceGetLogRequestExample()
		case "service/log-error":
			payload = services.ServiceGetLogRequestExample()
		case "service/log-stream":
			payload = services.ServiceLogStreamRequestExample()
			blockConnection = true
		case "service/resource-status":
			payload = services.ServiceResourceStatusRequestExample()
		case "service/restart":
			payload = services.ServiceRestartRequestExample()
		case "service/stop":
			payload = services.ServiceStopRequestExample()
		case "service/start":
			payload = services.ServiceStartRequestExample()
		case "service/update-service":
			payload = services.ServiceUpdateRequestExample()

		case "list/namespaces":
			payload = services.K8sListRequestExample()
		case "list/deployments":
			payload = services.K8sListRequestExample()
		case "list/services":
			payload = services.K8sListRequestExample()
		case "list/pods":
			payload = services.K8sListRequestExample()
		case "list/ingresses":
			payload = services.K8sListRequestExample()
		case "list/configmaps":
			payload = services.K8sListRequestExample()
		case "list/secrets":
			payload = services.K8sListRequestExample()
		case "list/nodes":
			payload = services.K8sListRequestExample()
		case "list/daemonsets":
			payload = services.K8sListRequestExample()
		case "list/statefulsets":
			payload = services.K8sListRequestExample()
		case "list/jobs":
			payload = services.K8sListRequestExample()
		case "list/cronjobs":
			payload = services.K8sListRequestExample()
		case "list/replicasets":
			payload = services.K8sListRequestExample()
		case "list/persistentvolume":
			payload = services.K8sListRequestExample()
		case "list/persistentvolumeclaim":
			payload = services.K8sListRequestExample()

		case "update/deployment":
			payload = services.K8sUpdateDeploymentRequestExample()
		case "update/service":
			payload = services.K8sUpdateServiceRequestExample()
		case "update/pod":
			payload = services.K8sUpdatePodRequestExample()
		case "update/ingress":
			payload = services.K8sUpdateIngressRequestExample()
		case "update/configmap":
			payload = services.K8sUpdateConfigmapRequestExample()
		case "update/secret":
			payload = services.K8sUpdateSecretRequestExample()
		case "update/daemonset":
			payload = services.K8sUpdateDaemonsetRequestExample()
		case "update/statefulset":
			payload = services.K8sUpdateStatefulSetRequestExample()
		case "update/job":
			payload = services.K8sUpdateJobRequestExample()
		case "update/cronjob":
			payload = services.K8sUpdateCronJobRequestExample()
		case "update/replicaset":
			payload = services.K8sUpdateReplicaSetRequestExample()
		case "update/persistentvolume":
			payload = services.K8sUpdatePersistentVolumeRequestExample()
		case "update/persistentvolumeclaim":
			payload = services.K8sUpdatePersistentVolumeClaimRequestExample()

		case "delete/namespace":
			payload = services.K8sDeleteNamespaceRequestExample()
		case "delete/deployment":
			payload = services.K8sDeleteDeploymentRequestExample()
		case "delete/service":
			payload = services.K8sDeleteServiceRequestExample()
		case "delete/pod":
			payload = services.K8sDeletePodRequestExample()
		case "delete/ingress":
			payload = services.K8sDeleteIngressRequestExample()
		case "delete/configmap":
			payload = services.K8sDeleteConfigmapRequestExample()
		case "delete/secret":
			payload = services.K8sDeleteSecretRequestExample()
		case "delete/daemonset":
			payload = services.K8sDeleteDaemonsetRequestExample()
		case "delete/statefulset":
			payload = services.K8sDeleteStatefulsetRequestExample()
		case "delete/job":
			payload = services.K8sDeleteJobRequestExample()
		case "delete/cronjob":
			payload = services.K8sDeleteCronjobRequestExample()
		case "delete/replicaset":
			payload = services.K8sDeleteReplicaSetRequestExample()
		case "delete/persistentvolume":
			payload = services.K8sDeletePersistentVolumeRequestExample()
		case "delete/persistentvolumeclaim":
			payload = services.K8sDeletePersistentVolumeClaimRequestExample()

		case "storage/enable":
			payload = services.NfsStorageInstallRequestExample()
		case "storage/disable":
			payload = services.NfsStorageInstallRequestExample()
		case "storage/create-volume":
			payload = services.NfsVolumeRequestExample()
		case "storage/delete-volume":
			payload = services.NfsVolumeRequestExample()
		case "storage/backup-volume":
			payload = services.NfsVolumeBackupRequestExample()
		case "storage/restore-volume":
			payload = services.NfsVolumeRestoreRequestExample()
		case "storage/stats":
			payload = services.NfsVolumeRequestExample()
		}
		firstConnection := selectRandomCluster(blockConnection)
		datagram := structs.CreateDatagramFrom(pattern, payload, firstConnection.Connection)
		datagram.Send()
		// send file after pattern
		if pattern == "files/upload" {
			sendFile()
		}
		return
	}
	logger.Log.Error("Not connected to any cluster.")
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

func selectRandomCluster(blockConnection bool) *structs.ClusterConnection {
	for _, v := range connections {
		if !v.Blocked {
			v.Blocked = blockConnection
			return v
		}
	}
	fmt.Println("All connections are blocked.")
	return nil
}

func selectBlockedCluster() *structs.ClusterConnection {
	for _, v := range connections {
		if v.Blocked {
			return v
		}
	}
	fmt.Println("No blocked connection available.")
	return nil
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
		cluster := selectRandomCluster(false)
		buf := make([]byte, 512)
		bar := progressbar.DefaultBytes(totalSize)

		sendMutex.Lock()
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
		sendMutex.Unlock()
	} else {
		logger.Log.Error("reader cannot be nil")
		logger.Log.Error("file size cannot be nil")
	}
}
