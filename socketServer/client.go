package socketServer

import (
	"encoding/json"
	"log"
	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/structs"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"time"

	"github.com/google/uuid"

	"github.com/gorilla/websocket"
)

const apiKey = "94E23575-A689-4F88-8D67-215A274F4E6E"
const serverAddress = "127.0.0.1:8080"
const heartBeatSeconds = 3
const clusterName = "BenesTestCluster"

func StartClient() {
	u := url.URL{Scheme: "ws", Host: serverAddress, Path: "/ws"}
	log.Printf("connecting to %s", u.String())

	c, _, err := websocket.DefaultDialer.Dial(u.String(), http.Header{
		"x-authorization": []string{apiKey},
		"x-clustername":   []string{clusterName}})
	if err != nil {
		logger.Log.Error("dial:", err)
		return
	} else {
		log.Printf("Connected to %s", u.String())
	}
	defer c.Close()

	done := make(chan struct{})

	parseMessage(done, c)

	// KEEP THE CONNECTION OPEN
	heartbeat(done, c)
}

func heartbeat(done chan struct{}, c *websocket.Conn) {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	heartBeatTicker := time.NewTicker(time.Second * heartBeatSeconds)
	defer heartBeatTicker.Stop()

	for {
		select {
		case <-done:
			return
		case <-heartBeatTicker.C:
			logger.Log.Info("HeartBeat ...")
			heartBeat := structs.TCPRequest{Pattern: "HeartBeat", Id: uuid.New().String()}
			err := c.WriteJSON(heartBeat)
			if err != nil {
				log.Println("HEARTBEAT ERROR:", err)
				return
			}
		case <-interrupt:
			log.Println("interrupt")

			// Cleanly close the connection by sending a close message and then
			// waiting (with timeout) for the server to close the connection.
			err := c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				log.Println("write close:", err)
				return
			}
			select {
			case <-done:
				log.Fatal("CTRL + C pressed. Terminating.")
			case <-time.After(time.Second):
			}
			return
		}
	}
}

func parseMessage(done chan struct{}, c *websocket.Conn) {
	go func() {
		defer close(done)
		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				log.Println("read:", err)
				return
			} else {
				rowJson := string(message)
				request := structs.TCPRequest{}

				_ = json.Unmarshal([]byte(rowJson), &request)

				responseMessage := structs.TCPResponse{
					Id: request.Id,
				}

				switch request.Pattern {
				case "namespace-service-pod-traffic-time-series":
					//TCPTrafficNamespaceServicePod(rowJson, &responseMessage)
					break
				case "namespace-service-compute-time-series":
					//TCPComputeNamespaceService(rowJson, &responseMessage)
					break
				case "tcp-test":
					jsonStr, err := json.Marshal(map[string]interface{}{
						"message": "tcp-test",
					})
					if err == nil {
						responseMessage.Response = string(jsonStr)
					}
					responseMessage.Response = string(jsonStr)
				default:
					responseMessage.Err = "Pattern not found"
				}
				//c.WriteJSON(responseMessage)
			}
			log.Printf("recv: %s", message)
		}
	}()
}
