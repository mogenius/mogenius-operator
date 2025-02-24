package xterm

import (
	"context"
	"os"
	"regexp"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

func cmdScanImageLogOutputToWebsocket(ctx context.Context, cancel context.CancelFunc, scanImageType string, conn *websocket.Conn, connWriteLock *sync.Mutex, tty *os.File) {
	toolLoadingCtx, toolLoadingCancel := context.WithTimeout(context.Background(), time.Second*time.Duration(600))

	defer func() {
		toolLoadingCancel()
		cancel()
	}()

	for {
		select {
		case <-ctx.Done():
			toolLoadingCancel()
			return
		default:
			// loading
			streamBeginning := false
			if scanImageType == "grype" {
				go func() {
					for {
						select {
						case <-toolLoadingCtx.Done():
							return
						default:
							time.Sleep(1 * time.Second)
							connWriteLock.Lock()
							err := conn.WriteMessage(websocket.TextMessage, []byte("."))
							connWriteLock.Unlock()
							if err != nil {
								xtermLogger.Error("WriteMessage", "error", err)
							}
							continue
						}
					}
				}()
			}

			buf := make([]byte, 1024)
			for {
				read, err := tty.Read(buf)
				if err != nil {
					// XtermLogger.Errorf("1 Unable to read from pty/cmd: %s", err.Error())
					return
				}
				if conn != nil {
					// loading
					if !streamBeginning {
						if len(string(buf[:read])) > 0 {
							re := regexp.MustCompile(`Vulnerability`)
							matches := re.FindAllString(string(buf[:read]), -1)

							if len(matches) > 0 {
								toolLoadingCancel()
								streamBeginning = true
							}
						}
					}
					connWriteLock.Lock()
					err := conn.WriteMessage(websocket.BinaryMessage, buf[:read])
					connWriteLock.Unlock()
					if err != nil {
						xtermLogger.Error("WriteMessage", "error", err)
					}
					continue
				}
			}
		}
	}
}
