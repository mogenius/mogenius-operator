package socketServer

import (
	"bufio"
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
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"
	"github.com/google/uuid"
	"github.com/schollz/progressbar/v3"

	"github.com/gorilla/websocket"

	mokubernetes "mogenius-k8s-manager/kubernetes"
)

const PingSeconds = 10

func StartK8sManager() {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	fmt.Println(utils.FillWith("", 60, "#"))
	fmt.Printf("###   CURRENT CONTEXT: %s   ###\n", utils.FillWith(mokubernetes.CurrentContextName(), 31, " "))
	fmt.Println(utils.FillWith("", 60, "#"))

	var connectionCounter int
	maxGoroutines := utils.CONFIG.Misc.ConcurrentConnections
	connectionGuard := make(chan struct{}, maxGoroutines)

	for {
		select {
		case <-interrupt:
			log.Fatal("CTRL + C pressed. Terminating.")
		case <-time.After(1000 * time.Millisecond):
		}

		connectionGuard <- struct{}{} // would block if guard channel is already filled
		go func() {
			connectionCounter++
			startClient(connectionCounter)
			<-connectionGuard
		}()

	}
}

func startClient(connectionCounter int) {
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
	defer func() {
		connection.Close()
	}()

	done := make(chan struct{})

	parseMessage(done, connection)
}

func parseMessage(done chan struct{}, c *websocket.Conn) {
	var sendMutex sync.Mutex
	var preparedFileName *string
	var preparedFileRequest *services.FilesUploadRequest
	var openFile *os.File
	bar := progressbar.DefaultSilent(0)

	go func() {
		defer func() {
			close(done)
		}()
		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				log.Println("read:", err)
				return
			} else {
				rawDataStr := string(message)
				if rawDataStr == "" {
					continue
				}
				if strings.HasPrefix(rawDataStr, "######START_UPLOAD######;") {
					preparedFileName = utils.Pointer(fmt.Sprintf("%s.zip", uuid.New().String()))
					rawDataStr = strings.Replace(rawDataStr, "######START_UPLOAD######;", "", 1)
					openFile, err = os.OpenFile(*preparedFileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
					if preparedFileRequest != nil {
						bar = progressbar.DefaultBytes(preparedFileRequest.SizeInBytes)
					} else {
						progressbar.DefaultBytes(0)
					}
				}
				if strings.HasPrefix(rawDataStr, "######END_UPLOAD######;") {
					openFile.Close()
					if preparedFileName != nil && preparedFileRequest != nil {
						services.Uploaded(*preparedFileName, *preparedFileRequest)
					}
					bar.Finish()
					os.Remove(*preparedFileName)
					preparedFileName = nil
					preparedFileRequest = nil
					continue
				}
				if preparedFileName != nil {
					openFile.Write([]byte(rawDataStr))
					bar.Add(len(rawDataStr))
				} else {
					datagram := structs.CreateEmptyDatagram()

					jsonErr := json.Unmarshal([]byte(rawDataStr), &datagram)
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
					} else if utils.Contains(services.BINARY_REQUEST_UPLOAD, datagram.Pattern) {
						preparedFileRequest = services.ExecuteBinaryRequestUpload(datagram, c)
					} else if utils.Contains(services.STREAM_REQUESTS, datagram.Pattern) {
						// ####### STREAM
						responsePayload, restReq := services.ExecuteStreamRequest(datagram, c)
						result := structs.CreateDatagramRequest(datagram, responsePayload, c)
						result.DisplayStreamSummary()

						ctx := context.Background()
						cancelCtx, endGofunc := context.WithCancel(ctx)
						stream, err := restReq.Stream(context.TODO())
						if err != nil {
							result.Err = err.Error()
						}
						defer func() {
							stream.Close()
							endGofunc()
							sendMutex.Unlock()
						}()

						go startClient(1234)

						sendMutex.Lock()
						c.WriteMessage(websocket.TextMessage, []byte("######START######;"+structs.PrettyPrintString(datagram)))
						reader := bufio.NewScanner(stream)
						for {
							select {
							case <-cancelCtx.Done():
								c.WriteMessage(websocket.TextMessage, []byte("######END######;"+structs.PrettyPrintString(datagram)))
								return
							default:
								for reader.Scan() {
									lastBytes := reader.Bytes()
									c.WriteMessage(websocket.BinaryMessage, lastBytes)
								}
							}
						}
					} else if utils.Contains(services.BINARY_REQUESTS_DOWNLOAD, datagram.Pattern) {
						responsePayload, reader, totalSize := services.ExecuteBinaryRequestDownload(datagram, c)
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
		}
	}()

	// KEEP THE CONNECTION OPEN
	ping(done, c, &sendMutex)
}

func ping(done chan struct{}, c *websocket.Conn, sendMutex *sync.Mutex) {
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
