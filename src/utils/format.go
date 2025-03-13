package utils

import (
	"fmt"
	"hash/fnv"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/jaevor/go-nanoid"
)

type ValidationError struct {
	Errors []string `json:"errors"`
}

func createEmptyValidationErr() *ValidationError {
	return &ValidationError{
		Errors: []string{},
	}
}

func (self *ValidationError) Error() string {
	return strings.Join(self.Errors, " | ")
}

func ValidateJSON(obj interface{}) *ValidationError {
	result := createEmptyValidationErr()
	err := validate.Struct(obj)
	if err != nil {
		if validationErrors, ok := err.(validator.ValidationErrors); ok {
			for _, fieldError := range validationErrors {
				errorMessage := fmt.Sprintf("Field '%s' failed validation, Condition: %s", fieldError.Field(), fieldError.Tag())
				result.Errors = append(result.Errors, errorMessage)
			}
		}
		utilsLogger.Error("struct validation failed", "result", result)
		return result
	}
	return nil
}

func FormatJsonTimePretty(jsonTimestamp string) string {
	t, err := time.Parse(time.RFC3339, jsonTimestamp)
	if err != nil {
		utilsLogger.Error("Failed to parse timestamp", "timestamp", jsonTimestamp, "expectedFormat", "RFC3339", "error", err)
		return jsonTimestamp
	}
	return t.Format("2006-01-02 15:04:05")
}
func FormatJsonTimePrettyFromTime(t time.Time) string {
	return t.Format("2006-01-02 15:04:05")
}

func NanoId() string {
	id, err := nanoid.Custom("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890", 21)
	if err != nil {
		utilsLogger.Error("NanoId() failed ", "error", err)
	}
	return id()
}

func NanoIdSmallLowerCase() string {
	id, err := nanoid.Custom("abcdefghijklmnopqrstuvwxyz1234567890", 10)
	if err != nil {
		utilsLogger.Error("NanoIdSmallLowerCase() failed", "error", err)
	}
	return id()
}

func NanoIdExtraLong() string {
	id, err := nanoid.Custom("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890", 21)
	if err != nil {
		utilsLogger.Error("NanoIdExtraLong() failed", "error", err)
	}
	return id()
}

func UnixTimeSeconds() string {
	return fmt.Sprint(time.Now().Unix())
}

func QuickHash(s string) string {
	h := fnv.New32a()
	h.Write([]byte(s))
	return fmt.Sprint(h.Sum32())
}

func BytesToHumanReadable(b int64) string {
	const unit = 1000
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "kMGTPE"[exp])
}

func NumberToHumanReadable(b uint64) string {
	const unit = 1000
	if b < unit {
		return fmt.Sprintf("%d", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %c",
		float64(b)/float64(div), "kMGTPE"[exp])
}

func FillWith(s string, targetLength int, chars string) string {
	if len(s) >= targetLength {
		return TruncateText(s, targetLength)
	}
	for i := 0; len(s) < targetLength; i++ {
		s = s + chars
	}

	return s
}

func TruncateText(s string, max int) string {
	if max < 4 || max > len(s) {
		return s
	}
	return s[:max-4] + " ..."
}
