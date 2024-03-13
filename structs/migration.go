package structs

import (
	"time"

	jsoniter "github.com/json-iterator/go"

	log "github.com/sirupsen/logrus"
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
		log.Errorf("MigrationBytes ERR: %s", err.Error())
	}
	return bytes
}
