package structs

import (
	"encoding/json"
	"fmt"
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
	Id         string `json:"id" validate:"required"`
	Pattern    string `json:"pattern" validate:"required"`
	Payload    string `json:"payload,omitempty"`
	Err        string `json:"err,omitempty"`
	Connection *websocket.Conn
}

func CreateDatagramRequest(request Datagram, data interface{}, c *websocket.Conn) Datagram {
	datagram := Datagram{
		Id:         request.Id,
		Pattern:    request.Pattern,
		Payload:    PrettyPrintString(data),
		Connection: c,
	}
	return datagram
}

func CreateDatagramFrom(pattern string, data interface{}, c *websocket.Conn) Datagram {
	datagram := Datagram{
		Id:         uuid.New().String(),
		Pattern:    pattern,
		Payload:    PrettyPrintString(data),
		Connection: c,
	}
	return datagram
}

func CreateDatagram(pattern string, c *websocket.Conn) Datagram {
	datagram := Datagram{
		Id:         uuid.New().String(),
		Pattern:    pattern,
		Connection: c,
	}
	return datagram
}

func (d *Datagram) DisplayBeautiful() {

	IDCOLOR := color.New(color.FgWhite, color.BgBlue).SprintFunc()
	PATTERNCOLOR := color.New(color.FgWhite, color.BgYellow).SprintFunc()
	TIMECOLOR := color.New(color.FgWhite, color.BgRed).SprintFunc()
	PAYLOADCOLOR := color.New(color.FgWhite, color.BgHiGreen).SprintFunc()

	fmt.Printf("%s %s\n", IDCOLOR("ID:      "), d.Id)
	fmt.Printf("%s %s\n", PATTERNCOLOR("PATTERN: "), color.BlueString(d.Pattern))
	fmt.Printf("%s %s\n", TIMECOLOR("TIME:    "), time.Now().Format(time.RFC3339))

	var obj map[string]interface{}
	json.Unmarshal([]byte(d.Payload), &obj)

	f := colorjson.NewFormatter()
	f.Indent = 2

	s, _ := f.Marshal(obj)

	fmt.Printf("%s %s\n", PAYLOADCOLOR("PAYLOAD: "), string(s))
}

func (d *Datagram) DisplayReceiveSummary() {

	RECEIVCOLOR := color.New(color.FgWhite, color.BgHiGreen).SprintFunc()
	PATTERNCOLOR := color.New(color.FgWhite, color.BgYellow).SprintFunc()
	IDCOLOR := color.New(color.FgWhite, color.BgBlue).SprintFunc()

	fmt.Printf("%s%s%s\n", RECEIVCOLOR("RECEIVED        "), PATTERNCOLOR(utils.FillWith(d.Pattern, 50, " ")), IDCOLOR(d.Id))
}

func (d *Datagram) DisplaySentSummary() {

	RECEIVCOLOR := color.New(color.FgWhite, color.BgHiGreen).SprintFunc()
	PATTERNCOLOR := color.New(color.FgWhite, color.BgYellow).SprintFunc()
	IDCOLOR := color.New(color.FgWhite, color.BgBlue).SprintFunc()

	fmt.Printf("%s%s%s\n", RECEIVCOLOR("SENT            "), PATTERNCOLOR(utils.FillWith(d.Pattern, 50, " ")), IDCOLOR(d.Id))
}

func (d *Datagram) Send() error {
	if d.Connection != nil {
		sendMutex.Lock()
		defer sendMutex.Unlock()
		return d.Connection.WriteJSON(d)

	} else {
		return fmt.Errorf("Connection cannot be nil.")
	}
}
