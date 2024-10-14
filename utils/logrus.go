package utils

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
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

var MainLogBytesCounter uint64 = 0

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

	MainLogBytesCounter += uint64(len(entry.Message))

	component := "main"

	if MainLogBytesCounter > uint64(CONFIG.Misc.LogRotationSizeInBytes) {
		RotateLog(MainLogPath(), component)
	} else {
		DeleteFilesLogRetention(component)
	}

	return nil
}

func RotateLog(sourceFilePath string, component string) {
	MainLogBytesCounter = 0

	rotatedLogfilePath := fmt.Sprintf("%s/%s-%s.log", CONFIG.Kubernetes.LogDataPath, component, time.Now().Format("2006-01-02-15-04-05.000"))
	err := os.MkdirAll(CONFIG.Kubernetes.LogDataPath, os.ModePerm)
	if err != nil {
		logrus.Errorf("Failed to create parent directories for rotation: %v", err)
	}

	sourceFile, err := os.OpenFile(sourceFilePath, os.O_RDWR|os.O_CREATE, 0666)
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

	err = os.Truncate(sourceFilePath, 0)
	if err != nil {
		logrus.Errorf("Failed to truncate log file: %v", err)
	}

	deleteFilesOlderThanLogRetention(component)
	DeleteFilesLogRetention(component)
}

func deleteFilesOlderThanLogRetention(component string) {
	//searchPattern := fmt.Sprintf("%s.log", time.Now().Format("2006-01-02-15-04-05.000"))
	pattern := fmt.Sprintf("%s-\\d{4}-\\d{2}-\\d{2}-\\d{2}-\\d{2}-\\d{2}\\.\\d{3}", component)
	regex, err := regexp.Compile(pattern)
	if err != nil {
		logrus.Errorf("Failed to compile regex: %v", err)
		return
	}

	files, err := os.ReadDir(CONFIG.Kubernetes.LogDataPath)
	if err != nil {
		logrus.Errorf("Failed to read directory: %v", err)
	}

	for _, file := range files {
		if file.IsDir() || !regex.MatchString(file.Name()) || strings.HasPrefix(file.Name(), fmt.Sprintf("%s.log", component)) {
			continue
		}
		//if file.IsDir() {
		//	continue
		//}
		//
		//// Skip files that don't match the search pattern
		//if !strings.HasSuffix(file.Name(), searchPattern) {
		//	continue
		//}
		//
		//if strings.HasPrefix(file.Name(), "main.log") {
		//	continue
		//}

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

func DeleteFilesLogRetention(component string) {
	pattern := fmt.Sprintf("%s-\\d{4}-\\d{2}-\\d{2}-\\d{2}-\\d{2}-\\d{2}\\.\\d{3}", component)
	regex, err := regexp.Compile(pattern)
	if err != nil {
		logrus.Errorf("Failed to compile regex: %v", err)
		return
	}

	files, err := os.ReadDir(CONFIG.Kubernetes.LogDataPath)
	if err != nil {
		logrus.Errorf("Failed to read directory: %v", err)
	}

	var totalSize uint64 = 0
	var filteredFiles []os.FileInfo
	for _, file := range files {
		if file.IsDir() || !regex.MatchString(file.Name()) || strings.HasPrefix(file.Name(), fmt.Sprintf("%s.log", component)) {
			continue
		}
		fileInfo, err := file.Info()
		if err != nil {
			logrus.Errorf("Failed to get file info: %v", err)
			continue
		}
		filteredFiles = append(filteredFiles, fileInfo)
		totalSize += uint64(fileInfo.Size())
	}
	sort.Slice(filteredFiles, func(i, j int) bool {
		return filteredFiles[i].ModTime().Before(filteredFiles[j].ModTime())
	})

	for totalSize > uint64(CONFIG.Misc.LogRotationMaxSizeInBytes) && len(files) > 0 {
		oldestFile := files[0]
		filePath := filepath.Join(CONFIG.Kubernetes.LogDataPath, oldestFile.Name())

		oldestFileFileInfo, err := oldestFile.Info()
		if err != nil {
			logrus.Errorf("Failed to get file info: %v", err)
			continue
		}

		err = os.Remove(filePath)
		if err != nil {
			logrus.Errorf("Failed to delete file: %v", err)
			continue
		}
		totalSize -= uint64(oldestFileFileInfo.Size())
		files = files[1:]
	}
}

func MainLogPath() string {
	logFilePath := fmt.Sprintf("%s/main.log", CONFIG.Kubernetes.LogDataPath)
	return logFilePath
}
