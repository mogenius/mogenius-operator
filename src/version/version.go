package version

import (
	"log/slog"
	"mogenius-k8s-manager/src/interfaces"

	"github.com/mogenius/punq/version"
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
		"Version", version.Ver,
		"Branch", version.Branch,
		"GitCommitHash", version.GitCommitHash,
		"BuildTimestamp", version.BuildTimestamp,
	)
}
