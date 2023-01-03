package structs

import (
	"encoding/json"
	"fmt"
	"log"
)

func PrettyPrint(i interface{}) {
	iJson, err := json.MarshalIndent(i, "", "  ")
	if err != nil {
		log.Fatalf(err.Error())
	}
	fmt.Printf("%s\n", string(iJson))
}
