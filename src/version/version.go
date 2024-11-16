package version

import (
	"log/slog"
	"mogenius-k8s-manager/src/interfaces"
)

type Version struct {
	logger *slog.Logger
}

func NewVersion(logManager interfaces.LogManagerModule) *Version {
	return &Version{
		logger: logManager.CreateLogger("version"),
	}
}

func (self *Version) PrintVersionInfo() {
	self.logger.Info(
		"mogenius-k8s-manager",
		"Version", Ver,
		"Branch", Branch,
		"GitCommitHash", GitCommitHash,
		"BuildTimestamp", BuildTimestamp,
	)
}
