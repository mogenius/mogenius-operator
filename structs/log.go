package structs

import (
	"time"

	jsoniter "github.com/json-iterator/go"
)

type LogType string
type Category string

const (
	Debug   LogType = "DEBUG"
	Verbose LogType = "VERBOSE"
	Info    LogType = "INFO"
	Warning LogType = "WARNING"
	Error   LogType = "ERROR"
)

const (
	Misc         Category = "MISC"
	Installation Category = "INSTALLATION"
	Kubernetes   Category = "KUBERNETES"
	Storage      Category = "STORAGE"
)

type Log struct {
	Id        uint64   `json:"id"`
	Title     string   `json:"title"`
	Message   string   `json:"message"`
	Type      LogType  `json:"type"`
	Category  Category `json:"category"`
	CreatedAt string   `json:"createdAt"`
}

func CreateLog(id uint64, title string, message string, category Category, logType LogType) Log {
	return Log{
		Id:        id,
		Title:     title,
		Message:   message,
		Type:      logType,
		Category:  category,
		CreatedAt: time.Now().Format(time.RFC3339),
	}
}

func LogBytes(logEntry Log) []byte {
	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	bytes, err := json.Marshal(logEntry)
	if err != nil {
		StructsLogger.Error("LogBytes", "error", err)
	}
	return bytes
}
