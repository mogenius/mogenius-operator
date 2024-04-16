package utils

import (
	"strings"

	punqStructs "github.com/mogenius/punq/structs"
	"github.com/sirupsen/logrus"
)

var secrets = map[string]bool{}

const REDACTED = "***[REDACTED]***"

func AddSecret(secret *string) {
	if secret == nil || len(*secret) < 4 {
		return
	}
	secrets[*secret] = true
}

func SecretArray() []string {
	var result []string
	for secret := range secrets {
		result = append(result, secret)
	}
	return result
}

type SecretRedactionHook struct{}

func (hook *SecretRedactionHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

func (hook *SecretRedactionHook) Fire(entry *logrus.Entry) error {
	for _, secret := range SecretArray() {
		// Iterate over all fields looking for secrets to redact.
		for key, value := range entry.Data {
			if strVal, ok := value.(string); ok {
				entry.Data[key] = strings.ReplaceAll(strVal, secret, REDACTED)
			}
		}

		// Optionally, check the message as well
		entry.Message = strings.ReplaceAll(entry.Message, secret, REDACTED)
	}
	return nil
}

func RedactString(targetSring string) string {
	for _, secret := range SecretArray() {
		targetSring = strings.ReplaceAll(targetSring, secret, REDACTED)
	}
	return targetSring
}

func PrettyPrintInterface(i interface{}) string {
	str := punqStructs.PrettyPrintString(i)
	return RedactString(str)
}

func PrettyPrintInterfaceLog(data []byte) {
	str := RedactString(string(data))
	punqStructs.PrettyPrintJSON([]byte(str))
}
