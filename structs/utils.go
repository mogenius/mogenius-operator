package structs

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/utils"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	jsoniter "github.com/json-iterator/go"
)

const PingSeconds = 10

func MarshalUnmarshal(datagram *Datagram, data interface{}) {
	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	bytes, err := json.Marshal(datagram.Payload)
	if err != nil {
		datagram.Err = err.Error()
		return
	}
	err = json.Unmarshal(bytes, data)
	if err != nil {
		datagram.Err = err.Error()
	}
}

func UnmarshalJob(dst *BuildJob, data []byte) error {
	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	err := json.Unmarshal(data, dst)
	if err != nil {
		return err
	}
	return nil
}

func UnmarshalScan(dst *BuildScanResult, data []byte) error {
	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	err := json.Unmarshal(data, dst)
	if err != nil {
		return err
	}
	return nil
}

func UnmarshalJobListEntry(dst *BuildJobListEntry, data []byte) error {
	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	err := json.Unmarshal(data, dst)
	if err != nil {
		return err
	}
	if dst != nil {
		u, err := url.Parse(dst.GitRepo)
		if err != nil {
			dst.GitRepo = ""
		} else {
			dst.GitRepo = fmt.Sprintf("%s%s", u.Host, u.Path)
		}
	}
	return nil
}

func SendData(sendToServer string, data []byte) {
    resp, err := http.Post(sendToServer, "application/json", bytes.NewBuffer(data))
    if err != nil {
		logger.Log.Errorf("Error occurred during sending the request. Error: %s", err)
    } else {
		defer resp.Body.Close()
	}
}

func SendDataWs(sendToServer string, reader io.ReadCloser) {
	header := utils.HttpHeader("-logs")
	connection, _, err := websocket.DefaultDialer.Dial(sendToServer, header)
	if err != nil {
		logger.Log.Errorf("Connection to stream endpoint (%s) failed: %s\n", sendToServer, err.Error())
	} else {
		// API send ack when it is ready to receive messages.
		connection.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, ack, err := connection.ReadMessage()
		if err != nil {
			logger.Log.Errorf("Error reading ack message: %s.", err)
			return
		}

		logger.Log.Infof("Ready ack from stream endpoint: %s.", string(ack))

		buf := make([]byte, 1024)
		for {
			if reader != nil {
				n, err := reader.Read(buf)
				if err != nil {
					if err != io.EOF {
						logger.Log.Errorf("Unexpected stop of stream: %s.", sendToServer)
					}
					return
				}
				if connection != nil {
					// debugging
					// str := string(buf[:n])
					// logger.Log.Infof("Send data ws: %s.", str)

					err = connection.WriteMessage(websocket.BinaryMessage, buf[:n])
					if err != nil {
						logger.Log.Errorf("Error sending data to '%s': %s\n", sendToServer, err.Error())
						return
					}

					// if conn, ok := connection.UnderlyingConn().(*net.TCPConn); ok {
					// 	err := conn.SetWriteBuffer(0)
					// 	if err != nil {
					// 		log.Println("Error flushing connection:", err)
					// 	}
					// }
				} else {
					logger.Log.Errorf("%s - connection cannot be nil.", sendToServer)
					return
				}
			} else {
				return
			}
		}
	}

	defer func() {
		// reset everything if connection dies
		if connection != nil {
			connection.Close()
		}
		if reader != nil {
			reader.Close()
		}
	}()
}

func Ping(c *websocket.Conn, sendMutex *sync.Mutex) error {
	interrupt := make(chan os.Signal, 1)
	defer close(interrupt)
	signal.Notify(interrupt, os.Interrupt)

	pingTicker := time.NewTicker(time.Second * PingSeconds)

	for {
		select {
		case <-pingTicker.C:
			sendMutex.Lock()
			err := c.WriteMessage(websocket.PingMessage, nil)
			sendMutex.Unlock()
			if err != nil {
				log.Println("pingTicker ERROR:", err)
				return err
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
				return err
			}
			select {
			case <-time.After(time.Second):
			}
			return nil
		}
	}
}
