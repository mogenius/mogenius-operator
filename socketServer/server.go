package socketServer

import (
	"encoding/json"
	"log"
	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/structs"
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

		switch datagram.Pattern {
		case "HeartBeat":
			// sendConnect(connection.RemoteAddr().String(), request.ClusterName)
			// logger.Log.Infof("HeartBeat '%s' ...", clusterName)
		case "ClusterStatus":
			structs.PrettyPrint(datagram.Payload)
		default:
			logger.Log.Errorf("Unknown pattern '%s'.", datagram.Pattern)
			logger.Log.Error(string(msg))
		}
	}
}

func validateHeader(c *gin.Context) string {
	userAgent := c.Request.Header.Get("User-Agent")
	if userAgent == "" {
		userAgent = "unknown"
	}

	apiKey := c.Request.Header.Get("x-authorization")
	if apiKey != os.Getenv("API_KEY") {
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
	logger.Log.Notice("q:     quit application")
	logger.Log.Notice("1-9:   send status request to cluster")
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
			sendStatusRequestToCluster(string(r))
		case "h":
			printShortcuts()
		case "l":
			listClusters()
		case "q":
			os.Exit(0)
		default:
			logger.Log.Errorf("Unrecognized character '%s'.", r)
		}
	}
}

func listClusters() {
	logger.Log.Noticef("Listing %d connected clusters:", len(connections))
	count := 0
	for _, value := range connections {
		count++
		logger.Log.Noticef("%d: %s/%s", count, value.ClusterName, value.Connection.RemoteAddr().String())
	}
}

func sendStatusRequestToCluster(no string) {
	count := 0
	for _, value := range connections {
		count++
		if no == strconv.Itoa(count) {
			conResponse := structs.CreateDatagramFrom("ClusterStatus", nil)
			value.Connection.WriteJSON(conResponse)
			logger.Log.Infof("Requesting status for cluster '%s'.", value.ClusterName)
			return
		}
	}
	logger.Log.Errorf("Cluster number '%s' not found.", no)
}
