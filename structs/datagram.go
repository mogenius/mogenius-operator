package structs

import (
	"fmt"
	"mogenius-k8s-manager/dtos"
	"time"

	"github.com/fatih/color"
	"github.com/google/uuid"
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

func CreateDatagramFromNotification(data *dtos.K8sNotificationDto) Datagram {
	created, err := time.Parse(time.RFC3339, data.StartedAt)
	if err != nil {
		created = time.Now()
	}
	datagram := Datagram{
		Id:        uuid.New().String(),
		Pattern:   "K8sNotificationDto",
		Payload:   data,
		CreatedAt: created,
	}
	return datagram
}

func CreateDatagramFrom(pattern string, data interface{}) Datagram {
	datagram := Datagram{
		Id:        uuid.New().String(),
		Pattern:   pattern,
		Payload:   data,
		CreatedAt: time.Now(),
	}
	return datagram
}

func CreateDatagram(pattern string) Datagram {
	datagram := Datagram{
		Id:        uuid.New().String(),
		Pattern:   pattern,
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
		Id:        uuid.New().String(),
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
	//PAYLOADCOLOR := color.New(color.FgBlack, color.BgHiGreen).SprintFunc()

	fmt.Printf("%s %s\n", IDCOLOR("ID:      "), d.Id)
	fmt.Printf("%s %s\n", PATTERNCOLOR("PATTERN: "), color.BlueString(d.Pattern))
	fmt.Printf("%s %s\n", TIMECOLOR("TIME:    "), time.Now().Format(time.RFC3339))
	fmt.Printf("%s %s\n", TIMECOLOR("Duration:"), punqStructs.DurationStrSince(d.CreatedAt))
	fmt.Printf("%s %s\n", SIZECOLOR("Size:    "), punqUtils.BytesToHumanReadable(d.GetSize()))
	//fmt.Printf("%s %s\n\n", PAYLOADCOLOR("PAYLOAD: "), punqStructs.PrettyPrintString(d.Payload))
}

func (d *Datagram) DisplayReceiveSummary() {
	fmt.Println()
	fmt.Printf("%s%s%s (%s / %s)\n", punqUtils.FillWith("RECEIVED", 23, " "), punqUtils.FillWith(d.Pattern, 40, " "), color.BlueString(d.Id), punqUtils.BytesToHumanReadable(d.GetSize()), punqStructs.DurationStrSince(d.CreatedAt))
}

func (d *Datagram) DisplaySentSummary(queuePosition int, queueLen int) {
	fmt.Printf("%s%s%s (%s / %s) [Queue: %d/%d]\n", punqUtils.FillWith("SENT", 23, " "), punqUtils.FillWith(d.Pattern, 40, " "), color.BlueString(d.Id), punqUtils.BytesToHumanReadable(d.GetSize()), punqStructs.DurationStrSince(d.CreatedAt), queuePosition, queueLen)
}

func (d *Datagram) DisplaySentSummaryEvent(kind string, reason string, msg string, count int32) {
	fmt.Printf("%s%s: %s/%s -> %s (Count: %d)\n", punqUtils.FillWith("SENT", 23, " "), d.Pattern, kind, reason, msg, count)
}

func (d *Datagram) DisplayStreamSummary() {
	fmt.Printf("%s%s%s\n", punqUtils.FillWith("STREAMING", 23, " "), punqUtils.FillWith(d.Pattern, 60, " "), color.BlueString(d.Id))
}

func (d *Datagram) Send() {
	JobServerSendData(*d)
}

func (d *Datagram) GetSize() int64 {
	return int64(len(punqStructs.PrettyPrintString(d)))
}
