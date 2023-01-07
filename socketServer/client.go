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
	"net/url"
	"os"
	"os/signal"
	"time"

	"github.com/gorilla/websocket"
)

const heartBeatSeconds = 30

func StartClient() {
	host := fmt.Sprintf("%s:%d", utils.CONFIG.ApiServer.WebsocketServer, utils.CONFIG.ApiServer.WebsocketPort)
	u := url.URL{Scheme: "ws", Host: host, Path: "/ws"}
	log.Printf("connecting to %s", u.String())

	c, _, err := websocket.DefaultDialer.Dial(u.String(), http.Header{
		"x-authorization": []string{utils.CONFIG.ApiServer.ApiKey},
		"x-clustername":   []string{utils.CONFIG.Kubernetes.ClusterName}})
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
			heartBeat := structs.CreateDatagram("HeartBeat")
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
				datagram := structs.Datagram{}

				_ = json.Unmarshal([]byte(rowJson), &datagram)

				if utils.Contains(services.ALL_REQUESTS, datagram.Pattern) {
					log.Printf("recv: %s (%s)", datagram.Pattern, datagram.Id)
					payload := services.ExecuteRequest(datagram)
					result := structs.CreateDatagramRequest(datagram, payload)
					c.WriteJSON(result)
					log.Printf("sent: %s (%s)", result.Pattern, result.Id)
				} else {
					logger.Log.Errorf("Pattern not found: '%s'.", datagram.Pattern)
				}
			}
		}
	}()
}
