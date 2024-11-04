package structs

import (
	"bytes"
	"fmt"
	"io"
	"mogenius-k8s-manager/utils"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	jsoniter "github.com/json-iterator/go"
	punqUtils "github.com/mogenius/punq/utils"
)

const PingSeconds = 3

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

func UnmarshalBuildScanImageEntry(dst *BuildScanImageEntry, data []byte) error {
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

func UnmarshalLog(dst *Log, data []byte) error {
	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	err := json.Unmarshal(data, dst)
	if err != nil {
		return err
	}
	return nil
}

func UnmarshalJobListEntry(dst *BuildJob, data []byte) error {
	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	err := json.Unmarshal(data, dst)
	if err != nil {
		return err
	}
	if dst != nil {
		for index, container := range dst.Service.Containers {
			if container.GitRepository != nil {
				u, err := url.Parse(*container.GitRepository)
				if err != nil {
					dst.Service.Containers[index].GitRepository = punqUtils.Pointer("")
				} else {
					dst.Service.Containers[index].GitRepository = punqUtils.Pointer(fmt.Sprintf("%s%s", u.Host, u.Path))
				}
			}
		}
	}
	return nil
}

func SendData(sendToServer string, data []byte) {
	resp, err := http.Post(sendToServer, "application/json", bytes.NewBuffer(data))
	if err != nil {
		structsLogger.Error("Error occurred during sending the request.", "error", err)
	} else {
		defer resp.Body.Close()
	}
}

func SendDataWs(sendToServer string, reader io.ReadCloser) {
	header := utils.HttpHeader("-logs")
	connection, _, err := websocket.DefaultDialer.Dial(sendToServer, header)
	if err != nil {
		structsLogger.Error("Connection to stream endpoint failed", "sendToServer", sendToServer, "error", err)
	} else {
		// API send ack when it is ready to receive messages.
		err = connection.SetReadDeadline(time.Now().Add(2 * time.Second))
		if err != nil {
			structsLogger.Error("Error setting read deadline.", "error", err)
			return
		}
		_, ack, err := connection.ReadMessage()
		if err != nil {
			structsLogger.Error("Error reading ack message.", "error", err)
			return
		}

		structsLogger.Info("Ready ack from stream endpoint.", "ack", string(ack))

		buf := make([]byte, 1024)
		for {
			if reader != nil {
				n, err := reader.Read(buf)
				if err != nil {
					if err != io.EOF {
						structsLogger.Error("Unexpected stop of stream.", "sendToServer", sendToServer)
					}
					return
				}
				if connection != nil {
					// debugging
					// str := string(buf[:n])
					// StructsLogger.Info("Send data ws.", "data", str)

					err = connection.WriteMessage(websocket.BinaryMessage, buf[:n])
					if err != nil {
						structsLogger.Error("Error sending data", "sendToServer", sendToServer, "error", err)
						return
					}

					// if conn, ok := connection.UnderlyingConn().(*net.TCPConn); ok {
					// 	err := conn.SetWriteBuffer(0)
					// 	if err != nil {
					// 		StructsLogger.Error("Error flushing connection", "error", err)
					// 	}
					// }
				} else {
					structsLogger.Error("connection cannot be nil", "sendToServer", sendToServer)
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
	pingTicker := time.NewTicker(time.Second * PingSeconds)

	// TODO: handle shutdown with shutdown.Add(func() {}) -> https://github.com/mogenius/mogenius-k8s-manager/commit/d41220c211b158fcbe17d3638327753169be19ef#diff-9c67221ec2c7a8e91c0c4275e64b83ba5a67be0efd8eec7db6e9b08b4476c7a4L187-L199
	for {
		<-pingTicker.C
		sendMutex.Lock()
		err := c.WriteMessage(websocket.PingMessage, nil)
		sendMutex.Unlock()
		if err != nil {
			structsLogger.Error("pingTicker", "error", err)
			return err
		}
	}
}
