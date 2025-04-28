package structs

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
