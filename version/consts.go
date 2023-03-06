package version

import "mogenius-k8s-manager/logger"

var (
	Ver            = "1.0.5"
	Branch         = ""
	GitCommitHash  = "" // ldflags
	BuildTimestamp = "" // ldflags
)

func PrintVersionInfo() {
	logger.Log.Infof("Ver:         %s", Ver)
	logger.Log.Infof("Branch:      %s", Branch)
	logger.Log.Infof("Hash:        %t", GitCommitHash)
	logger.Log.Infof("BuildAt:     %s", BuildTimestamp)
}
