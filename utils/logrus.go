package utils

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

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

var logbytesCounter uint64 = 0

type SecretRedactionHook struct{}
type LogRotationHook struct{}

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

func (hook *LogRotationHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

func (hook *LogRotationHook) Fire(entry *logrus.Entry) error {
	logbytesCounter += uint64(len(entry.Message))

	if logbytesCounter > uint64(CONFIG.Misc.LogRotationSizeInBytes) {
		rotateLog()
	}

	return nil
}

func rotateLog() {
	logbytesCounter = 0

	rotatedLogfilePath := fmt.Sprintf("%s/%s.log", CONFIG.Kubernetes.LogDataPath, time.Now().Format("2006-01-02-15-04-05.000"))
	err := os.MkdirAll(CONFIG.Kubernetes.LogDataPath, os.ModePerm)
	if err != nil {
		logrus.Errorf("Failed to create parent directories for rotation: %v", err)
	}

	sourceFile, err := os.OpenFile(MainLogPath(), os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		logrus.Errorf("Failed to open main log file: %v", err)
	}
	defer sourceFile.Close()

	destFile, err := os.Create(rotatedLogfilePath)
	if err != nil {
		logrus.Errorf("Failed to open rotated log file: %v", err)
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		logrus.Errorf("Failed to copy log file: %v", err)
	}

	err = os.Truncate(MainLogPath(), 0)
	if err != nil {
		logrus.Errorf("Failed to truncate log file: %v", err)
	}

	deleteFilesOlderThanLogRetention()
}

func deleteFilesOlderThanLogRetention() {
	files, err := os.ReadDir(CONFIG.Kubernetes.LogDataPath)
	if err != nil {
		logrus.Errorf("Failed to read directory: %v", err)
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}
		if strings.HasPrefix(file.Name(), "main.log") {
			continue
		}

		fileInfo, err := file.Info()
		if err != nil {
			logrus.Errorf("Failed to get file info: %v", err)
			continue
		}

		if time.Since(fileInfo.ModTime()).Hours() > float64(CONFIG.Misc.LogRetentionDays*24) {
			err := os.Remove(fmt.Sprintf("%s/%s", CONFIG.Kubernetes.LogDataPath, file.Name()))
			if err != nil {
				logrus.Errorf("Failed to delete file: %v", err)
			}
		}
	}
}

func MainLogPath() string {
	logFilePath := fmt.Sprintf("%s/main.log", CONFIG.Kubernetes.LogDataPath)
	return logFilePath
}
