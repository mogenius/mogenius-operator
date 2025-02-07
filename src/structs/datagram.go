package structs

import (
	"encoding/json"
	"fmt"
	"mogenius-k8s-manager/src/assert"
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
	User      User        `json:"user,omitempty"`
}

type User struct {
	FirstName string `json:"firstName,omitempty"`
	LastName  string `json:"lastName,omitempty"`
	Email     string `json:"email,omitempty"`
	Source    string `json:"source,omitempty"`
}

type UserSource string

const (
	SourceUser              UserSource = "user"
	SourceDemoController    UserSource = "demo-controller"
	SourceTaskService       UserSource = "task-service"
	SourceQueueService      UserSource = "queue-service"
	SourceK8sManagerService UserSource = "k8s-manager-service"
	SourceGitService        UserSource = "git-service"
)

var UserSourceToString = map[UserSource]string{
	SourceUser:              "user",
	SourceDemoController:    "demo-controller",
	SourceTaskService:       "task-service",
	SourceQueueService:      "queue-service",
	SourceK8sManagerService: "k8s-manager-service",
	SourceGitService:        "git-service",
}

var UserSourceFromString = map[string]UserSource{
	"user":                SourceUser,
	"demo-controller":     SourceDemoController,
	"task-service":        SourceTaskService,
	"queue-service":       SourceQueueService,
	"k8s-manager-service": SourceK8sManagerService,
	"git-service":         SourceGitService,
}

func (self UserSource) MarshalJSON() ([]byte, error) {
	val, ok := UserSourceToString[self]
	assert.Assert(ok, "unhandled enum variant", self)
	return []byte(`"` + val + `"`), nil
}

func (self *UserSource) UnmarshalJSON(data []byte) error {
	var dataString *string
	err := json.Unmarshal(data, &dataString)
	if err != nil {
		return err
	}
	userSource, ok := UserSourceFromString[*dataString]
	if !ok {
		return fmt.Errorf("unknown user source: %s", *dataString)
	}
	*self = userSource
	return nil
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

func (d *Datagram) GetSize() int64 {
	return int64(len(utils.PrettyPrintInterface(d)))
}
