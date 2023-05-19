package structs

import (
	"fmt"
	"log"
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
