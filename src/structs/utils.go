package structs

import (
	"bytes"
	"fmt"
	"io"
	"mogenius-k8s-manager/src/shutdown"
	"mogenius-k8s-manager/src/utils"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	jsoniter "github.com/json-iterator/go"
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
					dst.Service.Containers[index].GitRepository = utils.Pointer("")
				} else {
					dst.Service.Containers[index].GitRepository = utils.Pointer(fmt.Sprintf("%s%s", u.Host, u.Path))
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
	defer func() {
		if reader != nil {
			err := reader.Close()
			if err != nil {
				structsLogger.Debug("failed to close reader", "error", err)
			}
		}
	}()

	header := utils.HttpHeader("-logs")
	conn, _, err := websocket.DefaultDialer.Dial(sendToServer, header)
	if err != nil {
		structsLogger.Error("Connection to stream endpoint failed", "sendToServer", sendToServer, "error", err)
		return
	}

	defer func() {
		err := conn.Close()
		if err != nil {
			structsLogger.Debug("failed to close connection", "error", err)
		}
	}()

	// API send ack when it is ready to receive messages.
	err = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if err != nil {
		structsLogger.Error("Error setting read deadline.", "error", err)
		return
	}
	_, ack, err := conn.ReadMessage()
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
			// debugging
			// str := string(buf[:n])
			// StructsLogger.Info("Send data ws.", "data", str)

			err = conn.WriteMessage(websocket.BinaryMessage, buf[:n])
			if err != nil {
				structsLogger.Error("Error sending data", "sendToServer", sendToServer, "error", err)
				return
			}

			// if conn, ok := conn.UnderlyingConn().(*net.TCPConn); ok {
			// 	err := conn.SetWriteBuffer(0)
			// 	if err != nil {
			// 		StructsLogger.Error("Error flushing connection", "error", err)
			// 	}
			// }
		} else {
			return
		}
	}
}

func Ping(conn *websocket.Conn, connWriteLock *sync.Mutex) error {
	cancel := make(chan struct{})
	cancelFinished := make(chan struct{})
	pingTicker := time.NewTicker(time.Second * PingSeconds)

	shutdown.Add(func() {
		defer close(cancel)
		cancel <- struct{}{}
		select {
		case <-cancelFinished: // block for clean shutdown
		case <-time.After(5 * time.Second): // cancel blocking in case something went wrong
		}
	})

	for {
		select {
		case <-pingTicker.C:
			connWriteLock.Lock()
			err := conn.WriteMessage(websocket.PingMessage, nil)
			connWriteLock.Unlock()
			if err != nil {
				structsLogger.Error("pingTicker", "error", err)
				return err
			}
		case <-cancel:
			connWriteLock.Lock()
			structsLogger.Debug("shutting down websocket connection")
			err := conn.WriteMessage(
				websocket.CloseMessage,
				websocket.FormatCloseMessage(
					websocket.CloseNormalClosure,
					"",
				),
			)
			connWriteLock.Unlock()
			if err != nil {
				structsLogger.Error("failed to shut down websocket connection", "error", err)
				return err
			}
			structsLogger.Debug("websocket connection was shut down")
			cancelFinished <- struct{}{}
			return nil
		}
	}
}
