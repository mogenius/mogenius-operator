package structs

import (
	"fmt"
	"log"
	"time"

	jsoniter "github.com/json-iterator/go"
)

func PrettyPrint(i interface{}) {
	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	iJson, err := json.MarshalIndent(i, "", "  ")
	if err != nil {
		log.Fatalf(err.Error())
	}
	fmt.Printf("%s\n", string(iJson))
}

func PrettyPrintString(i interface{}) string {
	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	iJson, err := json.MarshalIndent(i, "", "  ")
	if err != nil {
		log.Fatalf(err.Error())
	}
	return string(iJson)
}

func MilliSecSince(since time.Time) int64 {
	return time.Since(since).Milliseconds()
}

func MicroSecSince(since time.Time) int64 {
	return time.Since(since).Microseconds()
}

func DurationStrSince(since time.Time) string {
	duration := MilliSecSince(since)
	durationStr := fmt.Sprintf("%d ms", duration)
	if duration <= 0 {
		duration = MicroSecSince(since)
		durationStr = fmt.Sprintf("%d μs", duration)
	}
	return durationStr
}
