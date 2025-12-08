package structs

import (
	"log/slog"
	"mogenius-operator/src/utils"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type Datagram struct {
	Id        string    `json:"id" validate:"required"`
	Pattern   string    `json:"pattern" validate:"required"`
	Payload   any       `json:"payload,omitempty"`
	Username  string    `json:"username,omitempty"`
	Err       string    `json:"err,omitempty"`
	CreatedAt time.Time `json:"-"`
	User      User      `json:"user,omitempty"`
	Workspace string    `json:"workspace,omitempty"`
	Zlib      bool      `json:"zlib,omitempty"`
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

func CreateDatagramForClusterEvent(pattern, apiVersion, kind, name, eventType string, obj *unstructured.Unstructured) Datagram {

	var status any
	if kind == "Application" {
		status, _, _ = unstructured.NestedFieldNoCopy(obj.Object, "status", "resources")
	}

	datagram := Datagram{
		Id:      utils.NanoId(),
		Pattern: pattern,
		Payload: map[string]any{
			"eventType": eventType,
			"resource": map[string]any{
				"apiVersion":      apiVersion,
				"kind":            kind,
				"namespace":       obj.GetNamespace(),
				"name":            name,
				"resourceName":    obj.GetName(),
				"uid":             string(obj.GetUID()),
				"resourceVersion": obj.GetResourceVersion(),

				"status": status,
			},
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
		Id:        utils.NanoId(),
		Username:  "",
		Pattern:   "",
		CreatedAt: time.Now(),
	}
	return datagram
}

func (d *Datagram) DisplayReceiveSummary(logger *slog.Logger) {
	logger.Debug("RECEIVED",
		"pattern", d.Pattern,
		"id", d.Id,
		"username", d.Username,
		"size", d.GetSize(),
	)
}

func (d *Datagram) GetSize() int64 {
	return int64(len(utils.PrettyPrintInterface(d)))
}
