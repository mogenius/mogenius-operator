package socketServer

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	mokubernetes "mogenius-k8s-manager/kubernetes"
	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/services"
	"mogenius-k8s-manager/structs"
	"mogenius-k8s-manager/utils"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/schollz/progressbar/v3"

	"github.com/gorilla/websocket"
)

const heartBeatSeconds = 30

var sendMutex sync.Mutex

func StartClient() {
	fmt.Println(utils.FillWith("", 60, "#"))
	fmt.Printf("###   CURRENT CONTEXT: %s   ###\n", utils.FillWith(mokubernetes.CurrentContextName(), 31, " "))
	fmt.Println(utils.FillWith("", 60, "#"))

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
			heartBeat := structs.CreateDatagram("HeartBeat", c)
			heartBeat.DisplaySentSummary()
			err := heartBeat.Send()
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
				rawJson := string(message)
				datagram := structs.Datagram{}

				jsonErr := json.Unmarshal([]byte(rawJson), &datagram)
				if jsonErr != nil {
					logger.Log.Errorf("%s", jsonErr.Error())
				}

				if utils.Contains(services.ALL_REQUESTS, datagram.Pattern) {
					//log.Printf("recv: %s (%s)", datagram.Pattern, datagram.Id)
					datagram.DisplayReceiveSummary()
					responsePayload, reader, totalSize := services.ExecuteRequest(datagram, c)
					if reader == nil {
						result := structs.CreateDatagramRequest(datagram, responsePayload, c)
						sendMutex.Lock()
						result.Send()
						sendMutex.Unlock()
					} else {
						buf := make([]byte, 512)
						bar := progressbar.DefaultBytes(*totalSize)
						sendMutex.Lock()
						c.WriteMessage(websocket.TextMessage, []byte("######START######;"+structs.PrettyPrintString(datagram)))
						for {
							chunk, err := reader.Read(buf)
							if err != nil {
								if err != io.EOF {
									fmt.Println(err)
								} else {
									fmt.Printf("%s transmitted.\n", utils.BytesToHumanReadable(*totalSize))
									bar.Finish()
								}
								break
							}
							c.WriteMessage(websocket.BinaryMessage, buf)
							bar.Add(chunk)
							//fmt.Print(".")
						}
						if err != nil {
							logger.Log.Errorf("reading bytes error: %s", err.Error())
						}
						c.WriteMessage(websocket.TextMessage, []byte("######END######;"+structs.PrettyPrintString(datagram)))
						sendMutex.Unlock()
					}
					//log.Printf("sent: %s (%s)", result.Pattern, result.Id)
				} else {
					logger.Log.Errorf("Pattern not found: '%s'.", datagram.Pattern)
				}
			}
		}
	}()
}
