package structs

import (
	"mogenius-k8s-manager/logger"
	"time"

	jsoniter "github.com/json-iterator/go"
)

type Migration struct {
	Id        uint64 `json:"id"`
	Name      string `json:"name"`
	AppliedAt string `json:"appliedAt"`
}

func CreateMigration(id uint64, name string) Migration {
	return Migration{
		Id:        id,
		Name:      name,
		AppliedAt: time.Now().Format(time.RFC3339),
	}
}

func MigrationBytes(migration Migration) []byte {
	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	bytes, err := json.Marshal(migration)
	if err != nil {
		logger.Log.Errorf("MigrationBytes ERR: %s", err.Error())
	}
	return bytes
}
