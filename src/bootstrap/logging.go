package bootstrap

import (
	"log/slog"
	"mogenius-operator/src/assert"
	"mogenius-operator/src/config"
	"mogenius-operator/src/logging"
	"mogenius-operator/src/secrets"
	"os"
	"strings"

	"github.com/mattn/go-isatty"
	"k8s.io/klog/v2"
)

// Logging bundles the logger setup derived from MO_LOG_LEVEL and
// MO_LOG_FILTER.
type Logging struct {
	Manager logging.SlogManager
	// CmdLogger is the logger of the command entrypoint.
	CmdLogger *slog.Logger
	// RecordChannel receives every record at info level and above (debug
	// with MO_LOG_LEVEL=debug); its consumer must keep draining it.
	RecordChannel chan logging.LogLine
}

// NewLogging creates the slog manager with the pretty-print handler on
// stderr and the record channel handler, and routes klog through it.
func NewLogging(configModule *config.Config) Logging {
	logLevel, err := logging.ParseLogLevel(configModule.Get("MO_LOG_LEVEL"))
	if configModule.Get("MO_LOG_LEVEL") == "mo" {
		logLevel = slog.LevelInfo
		err = nil
	}
	assert.Assert(err == nil, "failed to parse log level", err)
	logFilter := []string{}
	moLogFilter := strings.SplitSeq(configModule.Get("MO_LOG_FILTER"), ",")
	for f := range moLogFilter {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}
		logFilter = append(logFilter, f)
	}
	prettyPrintHandler := logging.NewPrettyPrintHandler(
		os.Stderr,
		isatty.IsTerminal(os.Stderr.Fd()),
		logLevel,
		logFilter,
		secrets.EraseSecrets,
	)
	recordChannelLogLevel := slog.LevelInfo
	if logLevel == slog.LevelDebug {
		recordChannelLogLevel = slog.LevelDebug
	}
	channelHandler := logging.NewRecordChannelHandler(
		512,
		recordChannelLogLevel,
		secrets.EraseSecrets,
	)
	slogManager := logging.NewSlogManager(
		logLevel,
		[]slog.Handler{
			channelHandler,
			prettyPrintHandler,
		},
	)
	klog.SetSlogLogger(slogManager.CreateLogger("klog"))

	return Logging{
		Manager:       slogManager,
		CmdLogger:     slogManager.CreateLogger("cmd"),
		RecordChannel: channelHandler.GetRecordChannel(),
	}
}
