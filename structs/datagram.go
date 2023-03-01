package structs

import (
	"fmt"
	"mogenius-k8s-manager/dtos"
	"mogenius-k8s-manager/utils"
	"sync"
	"time"

	"github.com/TylerBrock/colorjson"
	"github.com/fatih/color"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

var sendMutex sync.Mutex

type Datagram struct {
	Id         string          `json:"id" validate:"required"`
	Pattern    string          `json:"pattern" validate:"required"`
	Payload    interface{}     `json:"payload,omitempty"`
	Err        string          `json:"err,omitempty"`
	CreatedAt  time.Time       `json:"-"`
	Connection *websocket.Conn `json:"-"`
}

func CreateDatagramRequest(request Datagram, data interface{}, c *websocket.Conn) Datagram {
	datagram := Datagram{
		Id:         request.Id,
		Pattern:    request.Pattern,
		Payload:    data,
		CreatedAt:  request.CreatedAt,
		Connection: c,
	}
	return datagram
}

func CreateDatagramFromNotification(data *dtos.K8sNotificationDto, c *websocket.Conn) Datagram {
	created, err := time.Parse(time.RFC3339, data.StartedAt)
	if err != nil {
		created = time.Now()
	}
	datagram := Datagram{
		Id:         uuid.New().String(),
		Pattern:    "K8sNotificationDto",
		Payload:    data,
		CreatedAt:  created,
		Connection: c,
	}
	return datagram
}

func CreateDatagramFrom(pattern string, data interface{}, c *websocket.Conn) Datagram {
	datagram := Datagram{
		Id:         uuid.New().String(),
		Pattern:    pattern,
		Payload:    data,
		CreatedAt:  time.Now(),
		Connection: c,
	}
	return datagram
}

func CreateDatagram(pattern string, c *websocket.Conn) Datagram {
	datagram := Datagram{
		Id:         uuid.New().String(),
		Pattern:    pattern,
		CreatedAt:  time.Now(),
		Connection: c,
	}
	return datagram
}

func CreateEmptyDatagram() Datagram {
	datagram := Datagram{
		Id:         uuid.New().String(),
		Pattern:    "",
		CreatedAt:  time.Now(),
		Connection: nil,
	}
	return datagram
}

func (d *Datagram) DisplayBeautiful() {
	IDCOLOR := color.New(color.FgWhite, color.BgBlue).SprintFunc()
	PATTERNCOLOR := color.New(color.FgBlack, color.BgYellow).SprintFunc()
	TIMECOLOR := color.New(color.FgWhite, color.BgRed).SprintFunc()
	PAYLOADCOLOR := color.New(color.FgBlack, color.BgHiGreen).SprintFunc()

	fmt.Printf("%s %s\n", IDCOLOR("ID:      "), d.Id)
	fmt.Printf("%s %s\n", PATTERNCOLOR("PATTERN: "), color.BlueString(d.Pattern))
	fmt.Printf("%s %s\n", TIMECOLOR("TIME:    "), time.Now().Format(time.RFC3339))
	fmt.Printf("%s %s\n", TIMECOLOR("Duration:"), DurationStrSince(d.CreatedAt))

	f := colorjson.NewFormatter()
	f.Indent = 2
	s, _ := f.Marshal(d.Payload)

	fmt.Printf("%s %s\n\n", PAYLOADCOLOR("PAYLOAD: "), string(s))
}

func (d *Datagram) DisplayReceiveSummary() {
	fmt.Println()
	fmt.Printf("%s%s%s (%s)\n", utils.FillWith("RECEIVED", 23, " "), utils.FillWith(d.Pattern, 60, " "), color.BlueString(d.Id), DurationStrSince(d.CreatedAt))
}

func (d *Datagram) DisplaySentSummary() {
	fmt.Printf("%s%s%s (%s)\n", utils.FillWith("SENT", 23, " "), utils.FillWith(d.Pattern, 60, " "), color.BlueString(d.Id), DurationStrSince(d.CreatedAt))
}

func (d *Datagram) DisplaySentSummaryEvent(msg string) {
	fmt.Printf("%s%s%s (%s)\n", utils.FillWith("SENT", 23, " "), utils.FillWith(d.Pattern+": "+msg, 60, " "), color.BlueString(d.Id), DurationStrSince(d.CreatedAt))
}

func (d *Datagram) DisplayStreamSummary() {
	fmt.Printf("%s%s%s\n", utils.FillWith("STREAMING", 23, " "), utils.FillWith(d.Pattern, 60, " "), color.BlueString(d.Id))
}

func (d *Datagram) Send() error {
	if d.Connection != nil {
		sendMutex.Lock()
		defer sendMutex.Unlock()
		err := d.Connection.WriteJSON(d)
		d.DisplaySentSummary()
		return err
	} else {
		return fmt.Errorf("connection cannot be nil")
	}
}

func (d *Datagram) SendEvent(msg string) error {
	if d.Connection != nil {
		sendMutex.Lock()
		defer sendMutex.Unlock()
		err := d.Connection.WriteJSON(d)
		d.DisplaySentSummaryEvent(msg)
		return err
	} else {
		return fmt.Errorf("connection cannot be nil")
	}
}
