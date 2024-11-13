package structs

import (
	"fmt"
	"mogenius-k8s-manager/src/utils"
	"time"

	"github.com/fatih/color"
	punqStructs "github.com/mogenius/punq/structs"
	punqUtils "github.com/mogenius/punq/utils"
)

type Datagram punqStructs.Datagram

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
		Id:        punqUtils.NanoId(),
		Pattern:   "K8sNotificationDto",
		Payload:   data,
		CreatedAt: data.Started,
	}
	return datagram
}

func CreateDatagramFrom(pattern string, data interface{}) Datagram {
	datagram := Datagram{
		Id:        punqUtils.NanoId(),
		Pattern:   pattern,
		Payload:   data,
		CreatedAt: time.Now(),
	}
	return datagram
}

// func CreateDatagram(pattern string) Datagram {
// 	datagram := Datagram{
// 		Id:        punqUtils.NanoId(),
// 		Pattern:   pattern,
// 		CreatedAt: time.Now(),
// 	}
// 	return datagram
// }

func CreateDatagramBuildLogs(payload BuildJobInfo) Datagram {
	// func CreateDatagramBuildLogs(prefix string, namespace string, controllerName string, projectId string, line string, state punqStructs.JobStateEnum) Datagram {
	datagram := Datagram{
		Id:      punqUtils.NanoId(),
		Pattern: "build-logs-notification",
		Payload: payload,
		//Payload: map[string]interface{}{
		//	"logId":          prefix,
		//	"namespace":      namespace,
		//	"controllerName": controllerName,
		//	"projectId":      projectId,
		//	"line":           line,
		//	"state":          state,
		//},
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
		Id:        punqUtils.NanoId(),
		Username:  "",
		Pattern:   "",
		CreatedAt: time.Now(),
	}
	return datagram
}

func (d *Datagram) DisplayBeautiful() {
	IDCOLOR := color.New(color.FgWhite, color.BgBlue).SprintFunc()
	PATTERNCOLOR := color.New(color.FgBlack, color.BgYellow).SprintFunc()
	TIMECOLOR := color.New(color.FgWhite, color.BgRed).SprintFunc()
	SIZECOLOR := color.New(color.FgBlack, color.BgHiGreen).SprintFunc()
	PAYLOADCOLOR := color.New(color.FgBlack, color.BgHiGreen).SprintFunc()

	fmt.Printf("%s %s\n", IDCOLOR("ID:      "), d.Id)
	fmt.Printf("%s %s\n", PATTERNCOLOR("PATTERN: "), color.BlueString(d.Pattern))
	fmt.Printf("%s %s\n", TIMECOLOR("TIME:    "), time.Now().Format(time.RFC3339))
	fmt.Printf("%s %s\n", TIMECOLOR("Duration:"), punqStructs.DurationStrSince(d.CreatedAt))
	fmt.Printf("%s %s\n", SIZECOLOR("Size:    "), punqUtils.BytesToHumanReadable(d.GetSize()))
	fmt.Printf("%s %s\n\n", PAYLOADCOLOR("PAYLOAD: "), utils.PrettyPrintInterface(d.Payload))
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
