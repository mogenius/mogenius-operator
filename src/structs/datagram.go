package structs

import (
	"fmt"
	"mogenius-k8s-manager/src/shell"
	"mogenius-k8s-manager/src/utils"
	"time"
)

type Datagram struct {
	Id        string      `json:"id" validate:"required"`
	Pattern   string      `json:"pattern" validate:"required"`
	Payload   interface{} `json:"payload,omitempty"`
	Username  string      `json:"username,omitempty"`
	Err       string      `json:"err,omitempty"`
	CreatedAt time.Time   `json:"-"`
}

func CreateDatagramRequest(request Datagram, data interface{}) Datagram {
	datagram := Datagram{
		Id:        request.Id,
		Pattern:   request.Pattern,
		Payload:   data,
		CreatedAt: request.CreatedAt,
	}
	return datagram
}

func CreateDatagramNotificationFromJob(data *Job) Datagram {
	// delay for timing issue caused by events being triggered too closely together
	time.Sleep(100 * time.Millisecond)
	datagram := Datagram{
		Id:        utils.NanoId(),
		Pattern:   "K8sNotificationDto",
		Payload:   data,
		CreatedAt: data.Started,
	}
	return datagram
}

func CreateDatagramFrom(pattern string, data interface{}) Datagram {
	datagram := Datagram{
		Id:        utils.NanoId(),
		Pattern:   pattern,
		Payload:   data,
		CreatedAt: time.Now(),
	}
	return datagram
}

func CreateDatagramBuildLogs(payload BuildJobInfo) Datagram {
	datagram := Datagram{
		Id:        utils.NanoId(),
		Pattern:   "build-logs-notification",
		Payload:   payload,
		CreatedAt: time.Now(),
	}
	return datagram
}

func CreateDatagramAck(pattern string, id string) Datagram {
	datagram := Datagram{
		Id:        id,
		Pattern:   pattern,
		CreatedAt: time.Now(),
	}
	return datagram
}

func CreateEmptyDatagram() Datagram {
	datagram := Datagram{
		Id:        utils.NanoId(),
		Username:  "",
		Pattern:   "",
		CreatedAt: time.Now(),
	}
	return datagram
}

func (d *Datagram) DisplayBeautiful() {
	fmt.Printf("%s %s\n", shell.Colorize("ID:      ", shell.White, shell.BgBlue), d.Id)
	fmt.Printf("%s %s\n", shell.Colorize("PATTERN: ", shell.White, shell.BgYellow), shell.Colorize(d.Pattern, shell.Blue))
	fmt.Printf("%s %s\n", shell.Colorize("TIME:    ", shell.White, shell.BgRed), time.Now().Format(time.RFC3339))
	fmt.Printf("%s %s\n", shell.Colorize("Duration:", shell.White, shell.BgRed), utils.DurationStrSince(d.CreatedAt))
	fmt.Printf("%s %s\n", shell.Colorize("Size:    ", shell.Black, shell.BgGreen), utils.BytesToHumanReadable(d.GetSize()))
	fmt.Printf("%s %s\n\n", shell.Colorize("PAYLOAD: ", shell.Black, shell.Green), utils.PrettyPrintInterface(d.Payload))
}

func (d *Datagram) DisplayReceiveSummary() {
	structsLogger.Debug("RECEIVED",
		"pattern", d.Pattern,
		"id", d.Id,
		"username", d.Username,
		"size", d.GetSize(),
	)
}

func (d *Datagram) DisplaySentSummary(queuePosition int, queueLen int) {
	structsLogger.Debug("SENT",
		"pattern", d.Pattern,
		"id", d.Id,
		"username", d.Username,
		"size", d.GetSize(),
		"createdAt", d.CreatedAt,
		"queuePosition", queuePosition,
		"queueLen", queueLen,
	)
}

func (d *Datagram) DisplaySentSummaryEvent(kind string, reason string, msg string, count int32) {
	structsLogger.Debug("SENT",
		"pattern", d.Pattern,
		"kind", kind,
		"reason", reason,
		"msg", msg,
		"count", count,
	)
}

func (d *Datagram) DisplayStreamSummary() {
	structsLogger.Debug("STREAMING",
		"pattern", d.Pattern,
		"id", d.Id,
	)
}

func (d *Datagram) Send() {
	JobServerSendData(*d)
}

func (d *Datagram) GetSize() int64 {
	return int64(len(utils.PrettyPrintInterface(d)))
}
