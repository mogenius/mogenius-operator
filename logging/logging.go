package logging

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"io"
	"mogenius-k8s-manager/structs"
	"mogenius-k8s-manager/utils"
	"os"
	"strings"
)

type FileHook struct {
	Components []FileHookComponent
}

type FileHookComponent struct {
	Component       structs.ComponentEnum
	Filename        string
	Logfile         *os.File
	logBytesCounter uint64
}

var fileHookComponents = []FileHookComponent{}

func NewFileHook(components []structs.ComponentEnum) error {
	for _, component := range components {
		filename := fmt.Sprintf("%s/%s.log", utils.CONFIG.Kubernetes.LogDataPath, component)

		file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			return err
		}

		logBytesCounter := uint64(0)
		fileInfo, err := file.Stat()
		if err != nil {
			log.Errorf("Failed to get file info: %v", err)
		} else {
			logBytesCounter = uint64(fileInfo.Size())
		}

		fileHookComponents = append(fileHookComponents, FileHookComponent{
			Component:       component,
			Filename:        filename,
			Logfile:         file,
			logBytesCounter: logBytesCounter,
		})

	}

	fileHook := &FileHook{
		Components: fileHookComponents,
	}

	log.AddHook(fileHook)

	return nil
}

func (hook *FileHook) Fire(entry *log.Entry) error {
	component, ok := entry.Data["component"].(structs.ComponentEnum)
	fileHookComponent := hook.isComponentToLog(component)
	if ok && fileHookComponent != nil {
		line, err := entry.String()
		if err != nil {
			return err
		}

		fileHookComponent.logBytesCounter += uint64(len(entry.Message))

		if fileHookComponent.logBytesCounter > uint64(utils.CONFIG.Misc.LogRotationSizeInBytes) {
			utils.RotateLog(fileHookComponent.Filename, string(component))
		} else {
			utils.DeleteFilesLogRetention(string(component))
		}

		_, err = fileHookComponent.Logfile.Write([]byte(line))
		return err
	}
	return nil
}

func (hook *FileHook) Levels() []log.Level {
	return log.AllLevels
}

func (hook *FileHook) isComponentToLog(component structs.ComponentEnum) *FileHookComponent {
	for i := range hook.Components {
		if hook.Components[i].Component == component {
			return &hook.Components[i]
		}
	}
	return nil
}

func SetupLogging() {
	// Create a log file
	err := os.MkdirAll(utils.CONFIG.Kubernetes.LogDataPath, os.ModePerm)
	if err != nil {
		log.Fatalf("Failed to create parent directories: %v", err)
	}
	file, err := os.OpenFile(utils.MainLogPath(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}

	mw := io.MultiWriter(os.Stdout, file)

	fileInfo, err := file.Stat()
	if err != nil {
		log.Fatalf("Failed to get file info: %v", err)
	}
	utils.MainLogBytesCounter = uint64(fileInfo.Size())

	log.SetOutput(mw)
	log.SetLevel(log.TraceLevel)

	log.AddHook(&utils.SecretRedactionHook{})
	log.AddHook(&utils.LogRotationHook{})

	log.SetFormatter(&log.TextFormatter{
		ForceColors:      true,
		DisableTimestamp: false,
		DisableQuote:     true,
	})

	log.SetReportCaller(utils.CONFIG.Misc.DebugLogCaller)
	logLevel, err := log.ParseLevel(utils.CONFIG.Misc.LogLevel)
	if err != nil {
		logLevel = log.InfoLevel
		log.Error("Error parsing log level. Using default log level: info")
	}
	log.SetLevel(logLevel)

	if strings.ToLower(utils.CONFIG.Misc.LogFormat) == "json" {
		log.SetFormatter(&log.JSONFormatter{})
	} else if strings.ToLower(utils.CONFIG.Misc.LogFormat) == "text" {
		log.SetFormatter(&log.TextFormatter{
			ForceColors:      true,
			DisableTimestamp: false,
			DisableQuote:     true,
		})
	} else {
		log.SetFormatter(&log.TextFormatter{})
	}

	// Create a log file for each component
	components := []structs.ComponentEnum{
		structs.ComponentIacManager,
		structs.ComponentDb,
		structs.Store,
		structs.ComponentDbStats,
		structs.ComponentCrds,
		structs.ComponentKubernetes,
		structs.ComponentHelm,
		structs.ComponentServices,
	}

	err = NewFileHook(components)
	if err != nil {
		log.Fatal(err)
	}
}
