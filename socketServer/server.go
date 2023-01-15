package socketServer

import (
	"encoding/json"
	"errors"
	"log"
	"mogenius-k8s-manager/dtos"
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
		_, msg, err := connection.ReadMessage()
		if err != nil {
			logger.Log.Error("websocket read err:", err)
			break
		}

		datagram := structs.Datagram{}
		_ = json.Unmarshal(msg, &datagram)
		datagramValidationError := validate.Struct(datagram)

		if datagramValidationError != nil {
			logger.Log.Errorf("Invalid datagram: %s", datagramValidationError.Error())
			return
		}

		if utils.Contains(services.ALL_REQUESTS, datagram.Pattern) || utils.Contains(services.ALL_TESTS, datagram.Pattern) {
			services.ExecuteRequest(datagram, connection)
			structs.PrettyPrint(datagram)
		} else {
			logger.Log.Errorf("Pattern not found: '%s'.", datagram.Pattern)
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
	logger.Log.Notice("t:     send test-cmd to cluster")
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
			cluster, selectErr := selectCluster(tty)
			if selectErr == nil {
				cmd := selectCommand(tty)
				sendCmdToCluster(cluster, cmd)
				logger.Log.Noticef("Selected cluster: %s", cluster.ClusterName)
			} else {
				logger.Log.Notice(selectErr.Error())
			}
		case "t":
			testCmd := selectTestCommands(tty)
			requestTestFromCluster(testCmd)
		case "l":
			listClusters()
		case "q":
			os.Exit(0)
		default:
			logger.Log.Errorf("Unrecognized character '%s'.", r)
		}
	}
}

func selectCommand(tty *tty.TTY) string {
	for innerLoop := true; innerLoop; {
		r, err := tty.ReadRune()
		if err != nil {
			log.Fatal(err)
		}
		cmds := listCommands()
		inputInt, _ := strconv.Atoi(string(r))
		if len(cmds) >= inputInt {
			innerLoop = false
			return cmds[inputInt-1]
		} else {
			logger.Log.Errorf("Unrecognized character '%s'. Please select 1-%d.", string(r), len(cmds))
			innerLoop = false
		}
	}
	return ""
}

func listCommands() []string {
	cmds := []string{"status", "version"}
	logger.Log.Noticef("Select from (%d) Commands:", len(cmds))
	for i, cmd := range cmds {
		logger.Log.Noticef("%d: %s", i+1, cmd)
	}
	return cmds
}

func selectCluster(tty *tty.TTY) (structs.ClusterConnection, error) {
	clusters := listClusters()
	if len(clusters) > 0 {
		for innerLoop := true; innerLoop; {
			r, err := tty.ReadRune()
			if err != nil {
				log.Fatal(err)
			}

			inputInt, _ := strconv.Atoi(string(r))
			if len(clusters) >= inputInt {
				innerLoop = false
				return connectionFromNo(string(r))
			} else {
				logger.Log.Errorf("Unrecognized character '%s'. Please select 1-%d.", string(r), len(clusters))
				innerLoop = false
			}
		}
	}
	return structs.ClusterConnection{}, errors.New("No clusters available for selection.")
}

func connectionFromNo(no string) (structs.ClusterConnection, error) {
	count := 0
	for _, value := range connections {
		count++
		if no == strconv.Itoa(count) {
			return value, nil
		}
	}
	return structs.ClusterConnection{}, errors.New("No connection found")
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
			datagram := structs.CreateDatagramFrom("ClusterStatus", nil)
			value.Connection.WriteJSON(datagram)
			logger.Log.Infof("Requesting status for cluster '%s'.", value.ClusterName)
			return
		}
	}
	logger.Log.Errorf("Cluster number '%s' not found.", no)
}

func requestTestFromCluster(pattern string) {
	if len(connections) > 0 {
		var payload interface{} = nil
		switch pattern {
		case "TestCreateNamespace":
			payload = services.NamespaceCreateRequest{
				Namespace: dtos.K8sNamespaceDtoExampleData(),
				Stage:     dtos.K8sStageDtoExampleData(),
			}
		}
		firstConnection := selectRandomCluster()
		datagram := structs.CreateDatagramFrom(pattern, payload)
		firstConnection.Connection.WriteJSON(datagram)
		logger.Log.Infof("Requesting '%s' for cluster '%s'.", pattern, firstConnection.ClusterName)
		return
	}
	logger.Log.Error("Not connected to any cluster.")
}

func selectTestCommands(tty *tty.TTY) string {
	for index, testPatternName := range services.ALL_TESTS {
		logger.Log.Infof("%d: %s", index, testPatternName)
	}

	for innerLoop := true; innerLoop; {
		r, err := tty.ReadRune()
		if err != nil {
			log.Fatal(err)
		}
		inputInt, _ := strconv.Atoi(string(r))
		if len(services.ALL_TESTS) >= inputInt {
			innerLoop = false
			return services.ALL_TESTS[inputInt]
		} else {
			logger.Log.Errorf("Unrecognized character '%s'. Please select 0-%d.", string(r), len(services.ALL_TESTS))
			innerLoop = false
		}
	}
	return ""
}

func sendCmdToCluster(cluster structs.ClusterConnection, cmd string) {
	datagram := structs.CreateDatagramFrom(cmd, nil)
	cluster.Connection.WriteJSON(datagram)
	logger.Log.Infof("Sending CMD '%s' to cluster '%s'.", cmd, cluster.ClusterName)
}

func selectRandomCluster() *structs.ClusterConnection {
	for _, v := range connections {
		return &v
	}
	return nil
}
