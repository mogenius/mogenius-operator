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

	// var obj interface{}
	// json.Unmarshal([]byte(d.Payload), &obj)
	// if err != nil {
	// 	logger.Log.Errorf("Error Marshaling Payload: %s", err.Error())
	// }

	f := colorjson.NewFormatter()
	f.Indent = 2
	s, _ := f.Marshal(d.Payload)

	fmt.Printf("%s %s\n\n", PAYLOADCOLOR("PAYLOAD: "), string(s))
}

func (d *Datagram) DisplayReceiveSummary() {
	RECEIVCOLOR := color.New(color.FgBlack, color.BgBlue).SprintFunc()
	PATTERNCOLOR := color.New(color.FgBlack, color.BgHiRed).SprintFunc()
	IDCOLOR := color.New(color.FgWhite, color.BgCyan).SprintFunc()

	fmt.Printf("%s%s%s (%s)\n", RECEIVCOLOR("RECEIVED        "), PATTERNCOLOR(utils.FillWith(d.Pattern, 60, " ")), IDCOLOR(d.Id), DurationStrSince(d.CreatedAt))
}

func (d *Datagram) DisplaySentSummary() {
	RECEIVCOLOR := color.New(color.FgBlack, color.BgBlue).SprintFunc()
	PATTERNCOLOR := color.New(color.FgBlack, color.BgHiRed).SprintFunc()
	IDCOLOR := color.New(color.FgWhite, color.BgCyan).SprintFunc()

	fmt.Printf("%s%s%s (%s)\n", RECEIVCOLOR("SENT            "), PATTERNCOLOR(utils.FillWith(d.Pattern, 60, " ")), IDCOLOR(d.Id), DurationStrSince(d.CreatedAt))
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
