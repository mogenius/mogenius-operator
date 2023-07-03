package structs

import (
	"fmt"
	"io"
	"log"
	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/utils"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	jsoniter "github.com/json-iterator/go"
)

const PingSeconds = 10

func PrettyPrint(i interface{}) {
	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	iJson, err := json.MarshalIndent(i, "", "  ")
	if err != nil {
		log.Fatalf(err.Error())
	}
	fmt.Printf("%s\n", string(iJson))
}

func PrettyPrintString(i interface{}) string {
	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	iJson, err := json.MarshalIndent(i, "", "  ")
	if err != nil {
		log.Fatalf(err.Error())
	}
	return string(iJson)
}

func MilliSecSince(since time.Time) int64 {
	return time.Since(since).Milliseconds()
}

func MicroSecSince(since time.Time) int64 {
	return time.Since(since).Microseconds()
}

func DurationStrSince(since time.Time) string {
	duration := MilliSecSince(since)
	durationStr := fmt.Sprintf("%d ms", duration)
	if duration <= 0 {
		duration = MicroSecSince(since)
		durationStr = fmt.Sprintf("%d μs", duration)
	}
	return durationStr
}

func SendDataWs(sendToServer string, reader io.ReadCloser) {
	connection, _, err := websocket.DefaultDialer.Dial(sendToServer, utils.HttpHeader())
	if err != nil {
		logger.Log.Errorf("Connection to Stream-Endpoint (%s) failed: %s\n", sendToServer, err.Error())
	} else {

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
					err = connection.WriteMessage(websocket.BinaryMessage, buf[:n])
					if err != nil {
						logger.Log.Errorf("Error sending data to '%s': %s\n", sendToServer, err.Error())
						return
					}
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

func Ping(done chan struct{}, c *websocket.Conn, sendMutex *sync.Mutex) {
	interrupt := make(chan os.Signal, 1)
	defer close(interrupt)
	signal.Notify(interrupt, os.Interrupt)

	pingTicker := time.NewTicker(time.Second * PingSeconds)

	for {
		select {
		case <-done:
			pingTicker.Stop()
			return
		case <-pingTicker.C:
			sendMutex.Lock()
			err := c.WriteMessage(websocket.PingMessage, nil)
			sendMutex.Unlock()
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
