package socketServer

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
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

	"github.com/fatih/color"
	"github.com/schollz/progressbar/v3"

	"github.com/gorilla/websocket"
)

const PingSeconds = 10

func StartClient(connectionCounter int) {
	host := fmt.Sprintf("%s:%d", utils.CONFIG.ApiServer.WebsocketServer, utils.CONFIG.ApiServer.WebsocketPort)
	connectionUrl := url.URL{Scheme: "ws", Host: host, Path: "/ws"}

	connection, _, err := websocket.DefaultDialer.Dial(connectionUrl.String(), http.Header{
		"x-authorization": []string{utils.CONFIG.ApiServer.ApiKey},
		"x-clustername":   []string{utils.CONFIG.Kubernetes.ClusterName}})
	if err != nil {
		logger.Log.Infof("Connection%d %s ... %s -> %s\n", connectionCounter, color.BlueString(connectionUrl.String()), color.RedString("FAIL 💥"), color.HiRedString(err.Error()))
		return
	} else {
		logger.Log.Infof("Connection%d %s ... %s\n", connectionCounter, color.BlueString(connectionUrl.String()), color.GreenString("SUCCESS 🚀"))
	}
	defer connection.Close()

	done := make(chan struct{})

	parseMessage(done, connection)
}

func parseMessage(done chan struct{}, c *websocket.Conn) {
	var sendMutex sync.Mutex

	go func() {
		defer close(done)
		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				log.Println("read:", err)
				return
			} else {
				rawJson := string(message)
				datagram := structs.CreateEmptyDatagram()

				jsonErr := json.Unmarshal([]byte(rawJson), &datagram)
				if jsonErr != nil {
					logger.Log.Errorf("%s", jsonErr.Error())
				}

				datagram.DisplayReceiveSummary()

				if utils.Contains(services.COMMAND_REQUESTS, datagram.Pattern) {
					// ####### COMMAND
					responsePayload := services.ExecuteCommandRequest(datagram, c)
					result := structs.CreateDatagramRequest(datagram, responsePayload, c)
					sendMutex.Lock()
					result.Send()
					sendMutex.Unlock()
				} else if utils.Contains(services.STREAM_REQUESTS, datagram.Pattern) {
					// ####### STREAM
					responsePayload, restReq := services.ExecuteStreamRequest(datagram, c)
					result := structs.CreateDatagramRequest(datagram, responsePayload, c)

					stream, err := restReq.Stream(context.TODO())
					if err != nil {
						result.Err = err.Error()
					}
					defer stream.Close()

					logger.Log.Noticef("Start streaming logs: %s ...", structs.PrettyPrintString(responsePayload))
					sendMutex.Lock()
					c.WriteMessage(websocket.TextMessage, []byte("######START######;"+structs.PrettyPrintString(datagram)))
					for {
						buf := make([]byte, 512)
						numBytes, err := stream.Read(buf)
						if numBytes == 0 {
							continue
						}
						if err != nil {
							if err == io.EOF {
								// DONE
							}
							break
						}
						logger.Log.Info(string(buf[:numBytes]))
						c.WriteMessage(websocket.BinaryMessage, buf)
					}
					c.WriteMessage(websocket.TextMessage, []byte("######END######;"+structs.PrettyPrintString(datagram)))
					sendMutex.Unlock()
				} else if utils.Contains(services.BINARY_REQUESTS, datagram.Pattern) {
					// ####### BINARY
					responsePayload, reader, totalSize := services.ExecuteBinaryRequest(datagram, c)
					result := structs.CreateDatagramRequest(datagram, responsePayload, c)
					if reader != nil && *totalSize > 0 && result.Err == "" {
						buf := make([]byte, 512)
						bar := progressbar.DefaultBytes(*totalSize)

						sendMutex.Lock()
						c.WriteMessage(websocket.TextMessage, []byte("######START######;"+structs.PrettyPrintString(datagram)))
						for {
							chunk, err := reader.Read(buf)
							if err != nil {
								if err != io.EOF {
									fmt.Println(err)
								}
								bar.Finish()
								break
							}
							c.WriteMessage(websocket.BinaryMessage, buf)
							bar.Add(chunk)
						}
						if err != nil {
							logger.Log.Errorf("reading bytes error: %s", err.Error())
						}
						c.WriteMessage(websocket.TextMessage, []byte("######END######;"+structs.PrettyPrintString(datagram)))
						sendMutex.Unlock()
					} else {
						// something went wrong. send error message instead of stream
						result.Send()
					}
				} else {
					logger.Log.Errorf("Pattern not found: '%s'.", datagram.Pattern)
				}
			}
		}
	}()

	// KEEP THE CONNECTION OPEN
	Ping(done, c, &sendMutex)
}

func Ping(done chan struct{}, c *websocket.Conn, sendMutex *sync.Mutex) {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	pingTicker := time.NewTicker(time.Second * PingSeconds)
	defer pingTicker.Stop()

	for {
		select {
		case <-done:
			return
		case <-pingTicker.C:
			err := c.WriteMessage(websocket.PingMessage, nil)
			if err != nil {
				log.Println("pingTicker ERROR:", err)
				return
			}
		case <-interrupt:
			log.Println("interrupt")

			// Cleanly close the connection by sending a close message and then
			// waiting (with timeout) for the server to close the connection.
			sendMutex.Lock()
			err := c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			sendMutex.Unlock()
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
