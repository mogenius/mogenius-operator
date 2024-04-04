package structs

import (
	"fmt"
	"mogenius-k8s-manager/dtos"
	"mogenius-k8s-manager/utils"
	"time"

	"github.com/fatih/color"
	punqStructs "github.com/mogenius/punq/structs"
	punqUtils "github.com/mogenius/punq/utils"
	log "github.com/sirupsen/logrus"
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

func CreateDatagramFromNotification(data *dtos.K8sNotificationDto) Datagram {
	created, err := time.Parse(time.RFC3339, data.StartedAt)
	if err != nil {
		created = time.Now()
	}
	datagram := Datagram{
		Id:        punqUtils.NanoId(),
		Pattern:   "K8sNotificationDto",
		Payload:   data,
		CreatedAt: created,
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

func CreateDatagram(pattern string) Datagram {
	datagram := Datagram{
		Id:        punqUtils.NanoId(),
		Pattern:   pattern,
		CreatedAt: time.Now(),
	}
	return datagram
}

func CreateDatagramBuildLogs(prefix string, namespace string, controllerName string, projectId string, line string, state punqStructs.JobStateEnum) Datagram {
	datagram := Datagram{
		Id:      punqUtils.NanoId(),
		Pattern: "build-logs-notification",
		Payload: map[string]interface{}{
			"logId":          prefix,
			"namespace":      namespace,
			"controllerName": controllerName,
			"projectId":      projectId,
			"line":           line,
			"state":          state,
		},
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

	log.Infof("%s %s\n", IDCOLOR("ID:      "), d.Id)
	log.Infof("%s %s\n", PATTERNCOLOR("PATTERN: "), color.BlueString(d.Pattern))
	log.Infof("%s %s\n", TIMECOLOR("TIME:    "), time.Now().Format(time.RFC3339))
	log.Infof("%s %s\n", TIMECOLOR("Duration:"), punqStructs.DurationStrSince(d.CreatedAt))
	log.Infof("%s %s\n", SIZECOLOR("Size:    "), punqUtils.BytesToHumanReadable(d.GetSize()))
	log.Infof("%s %s\n\n", PAYLOADCOLOR("PAYLOAD: "), utils.PrettyPrintInterface(d.Payload))
}

func (d *Datagram) DisplayReceiveSummary() {
	log.Infof("%s%s%s%s     %s %s\n", punqUtils.FillWith("RECEIVED", 23, " "), punqUtils.FillWith(d.Pattern, 40, " "), color.BlueString(d.Id), punqUtils.FillWith(fmt.Sprint(d.Username+"   "[:3]), 10, " "), punqUtils.FillWith(punqUtils.BytesToHumanReadable(d.GetSize()), 12, " "), punqUtils.FillWith(punqStructs.DurationStrSince(d.CreatedAt), 12, " "))
}

func (d *Datagram) DisplaySentSummary(queuePosition int, queueLen int) {
	log.Infof("%s%s%s%s     %s %s %s\n", punqUtils.FillWith("SENT", 23, " "), punqUtils.FillWith(d.Pattern, 40, " "), color.BlueString(d.Id), punqUtils.FillWith(fmt.Sprint(d.Username+"   "[:3]), 10, " "), punqUtils.FillWith(punqUtils.BytesToHumanReadable(d.GetSize()), 12, " "), punqUtils.FillWith(punqStructs.DurationStrSince(d.CreatedAt), 12, " "), color.YellowString(fmt.Sprintf("[Queue: %d/%d]", queuePosition, queueLen)))
}

func (d *Datagram) DisplaySentSummaryEvent(kind string, reason string, msg string, count int32) {
	log.Infof("%s%s: %s/%s -> %s (Count: %d)\n", punqUtils.FillWith("SENT", 23, " "), d.Pattern, kind, reason, msg, count)
}

func (d *Datagram) DisplayStreamSummary() {
	log.Infof("%s%s%s\n", punqUtils.FillWith("STREAMING", 23, " "), punqUtils.FillWith(d.Pattern, 60, " "), color.BlueString(d.Id))
}

func (d *Datagram) Send() {
	JobServerSendData(*d)
}

func (d *Datagram) GetSize() int64 {
	return int64(len(utils.PrettyPrintInterface(d)))
}
