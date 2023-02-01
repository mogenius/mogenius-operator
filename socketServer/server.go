package socketServer

import (
	"encoding/json"
	"fmt"
	"log"
	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/services"
	"mogenius-k8s-manager/structs"
	"mogenius-k8s-manager/utils"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator"
	"github.com/gorilla/websocket"
	"github.com/mattn/go-tty"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

var validate = validator.New()
var connections = make(map[string]structs.ClusterConnection)

func Init(r *gin.Engine) {
	// r.Use(user.AuthUserMiddleware())
	r.GET("/ws", func(c *gin.Context) {
		clusterName := validateHeader(c)
		if clusterName != "" {
			wsHandler(c.Writer, c.Request, clusterName)
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

	addConnection(connection, clusterName)

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
			datagram := structs.Datagram{}
			_ = json.Unmarshal(msg, &datagram)
			datagramValidationError := validate.Struct(datagram)

			if datagramValidationError != nil {
				logger.Log.Errorf("Invalid datagram: %s", datagramValidationError.Error())
				return
			}

			if utils.Contains(services.ALL_REQUESTS, datagram.Pattern) {
				//services.ExecuteRequest(datagram, connection)
				datagram.DisplayBeautiful()
			} else {
				logger.Log.Errorf("Pattern not found: '%s'.", datagram.Pattern)
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
	if apiKey != utils.CONFIG.ApiServer.ApiKey {
		logger.Log.Errorf("Invalid x-authorization: '%s'", apiKey)
		return ""
	}

	clusterName := c.Request.Header.Get("x-clustername")
	if clusterName == "" {
		logger.Log.Errorf("Invalid x-clustername: '%s'", clusterName)
		return ""
	}

	logger.Log.Infof("New client connected %s/%s (Agent: %s)", clusterName, c.ClientIP(), userAgent)
	return clusterName
}

func addConnection(connection *websocket.Conn, clusterName string) {
	remoteAddr := connection.RemoteAddr().String()
	connections[remoteAddr] = structs.ClusterConnection{ClusterName: clusterName, Connection: connection, AddedAt: time.Now()}
}

func removeConnection(connection *websocket.Conn) {
	remoteAddr := connection.RemoteAddr().String()
	connection.Close()
	delete(connections, remoteAddr)
}

func printShortcuts() {
	logger.Log.Notice("Keyboard shortcusts: ")
	logger.Log.Notice("h:     help")
	logger.Log.Notice("l:     list clusters")
	logger.Log.Notice("s:     send command to cluster")
	logger.Log.Notice("q:     quit application")
	logger.Log.Notice("1-9:   request status from cluster")
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
		case "1", "2", "3", "4", "5", "6", "7", "8", "9":
			requestStatusFromCluster(string(r))
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
		case "q":
			os.Exit(0)
		default:
			logger.Log.Errorf("Unrecognized character '%s'.", r)
			printShortcuts()
		}
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

func requestStatusFromCluster(no string) {
	count := 0
	for _, value := range connections {
		count++
		if no == strconv.Itoa(count) {
			datagram := structs.CreateDatagramFrom("ClusterStatus", nil, value.Connection)
			datagram.Send()
			logger.Log.Infof("Requesting status for cluster '%s'.", value.ClusterName)
			return
		}
	}
	logger.Log.Errorf("Cluster number '%s' not found.", no)
}

func requestCmdFromCluster(pattern string) {
	if len(connections) > 0 {
		var payload interface{} = nil
		switch pattern {
		case "HeartBeat":
			payload = nil
		case "K8sNotification":
			payload = nil
		case "ClusterStatus":
			payload = nil
		case "files/storage-stats GET":
			payload = nil
		case "files/list POST":
			payload = services.FilesListRequestExampleData()
		case "files/download POST":
			payload = services.FilesDownloadRequestExampleData()
		case "files/upload POST":
			payload = services.FilesUploadRequestExampleData()
		case "files/update POST":
			payload = services.FilesUpdateRequestExampleData()
		case "files/create-folder POST":
			payload = services.FilesCreateFolderRequestExampleData()
		case "files/rename POST":
			payload = services.FilesRenameRequestExampleData()
		case "files/chown POST":
			payload = services.FilesChownRequestExampleData()
		case "files/chmod POST":
			payload = services.FilesChmodRequestExampleData()
		case "files/delete POST":
			payload = services.FilesDeleteRequestExampleData()
		case "namespace/create POST":
			payload = services.NamespaceCreateRequestExample()
		case "namespace/delete POST":
			payload = services.NamespaceDeleteRequestExample()
		case "namespace/shutdown POST":
			payload = services.NamespaceShutdownRequestExample()
		case "namespace/reboot POST":
			payload = services.NamespaceRebootRequestExample()
		case "namespace/ingress-state/:state GET":
			payload = services.NamespaceSetIngressStateRequestExample()
		case "namespace/pod-ids/:namespace GET":
			payload = services.NamespacePodIdsRequestExample()
		case "namespace/get-cluster-pods GET":
			payload = nil
		case "namespace/validate-cluster-pods POST":
			payload = services.NamespaceValidateClusterPodsRequestExample()
		case "namespace/validate-ports POST":
			payload = services.NamespaceValidatePortsRequestExample()
		case "namespace/storage-size POST":
			payload = services.NamespaceStorageSizeRequestExample()
		case "service/create POST":
			payload = services.ServiceCreateRequestExample()
		case "service/delete POST":
			payload = services.ServiceDeleteRequestExample()
		case "service/pod-ids/:namespace/:service GET":
			payload = services.ServiceGetPodIdsRequestExample()
		case "service/images/:imageName PATCH":
			payload = services.ServiceSetImageRequestExample()
		case "service/log/:namespace/:podId GET":
			payload = services.ServiceGetLogRequestExample()
		case "service/log-stream/:namespace/:podId/:sinceSeconds SSE":
			payload = services.ServiceLogStreamRequestExample()
		case "service/resource-status/:resource/:namespace/:name/:statusOnly GET":
			payload = services.ServiceResourceStatusRequestExample()
		case "service/restart POST":
			payload = services.ServiceRestartRequestExample()
		case "service/stop POST":
			payload = services.ServiceStopRequestExample()
		case "service/start POST":
			payload = services.ServiceStartRequestExample()
		case "service/update-service POST":
			payload = services.ServiceUpdateRequestExample()
		case "service/spectrum-bind POST":
			payload = services.ServiceBindSpectrumRequestExample()
		case "service/spectrum-unbind DELETE":
			payload = services.ServiceUnbindSpectrumRequestExample()
		case "service/spectrum-configmaps GET":
			payload = nil
		}
		firstConnection := selectRandomCluster()
		datagram := structs.CreateDatagramFrom(pattern, payload, firstConnection.Connection)
		datagram.DisplaySentSummary()
		datagram.Send()
		logger.Log.Infof("Requesting '%s' for cluster '%s'.", pattern, firstConnection.ClusterName)
		return
	}
	logger.Log.Error("Not connected to any cluster.")
}

func selectCommands() string {
	for index, patternName := range services.ALL_REQUESTS {
		fmt.Printf("%d: %s\n", index, patternName)
	}

	fmt.Println("input number:")
	var number int
	_, err := fmt.Scanf("%d", &number)
	if err != nil {
		logger.Log.Errorf("Unrecognized character '%s'. Please select 0-%d.", number, len(services.ALL_REQUESTS)-1)
		return ""
	}
	fmt.Println(number)

	if len(services.ALL_REQUESTS) >= number {
		return services.ALL_REQUESTS[number]
	} else {
		logger.Log.Errorf("Unrecognized character '%s'. Please select 0-%d.", number, len(services.ALL_REQUESTS)-1)
		return ""
	}
}

func selectRandomCluster() *structs.ClusterConnection {
	for _, v := range connections {
		return &v
	}
	return nil
}
