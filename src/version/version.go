package version

import (
	"fmt"
	"mogenius-operator/src/shell"
	"runtime"
)

type Version struct {
	Version        string `json:"version"`
	Branch         string `json:"branch"`
	GitCommitHash  string `json:"gitCommitHash"`
	BuildTimestamp string `json:"buildTimestamp"`
	Os             string `json:"os"`
	Arch           string `json:"arch"`
}

func NewVersion() *Version {
	return &Version{
		Version:        Ver,
		Branch:         Branch,
		GitCommitHash:  GitCommitHash,
		BuildTimestamp: BuildTimestamp,
		Os:             runtime.GOOS,
		Arch:           runtime.GOARCH,
	}
}

func (self *Version) PrintVersionInfo() {
	version := NewVersion()

	fmt.Printf(
		"███╗░░░███╗░█████╗░░██████╗░███████╗███╗░░██╗██╗██╗░░░██╗░██████╗\n" +
			"████╗░████║██╔══██╗██╔════╝░██╔════╝████╗░██║██║██║░░░██║██╔════╝\n" +
			"██╔████╔██║██║░░██║██║░░██╗░█████╗░░██╔██╗██║██║██║░░░██║╚█████╗░\n" +
			"██║╚██╔╝██║██║░░██║██║░░╚██╗██╔══╝░░██║╚████║██║██║░░░██║░╚═══██╗\n" +
			"██║░╚═╝░██║╚█████╔╝╚██████╔╝███████╗██║░╚███║██║╚██████╔╝██████╔╝\n" +
			"╚═╝░░░░░╚═╝░╚════╝░░╚═════╝░╚══════╝╚═╝░░╚══╝╚═╝░╚═════╝░╚═════╝░\n\n",
	)

	versionInfo := ""
	versionInfo = versionInfo + fmt.Sprintf("OS:         %s\n", shell.Colorize(version.Os, shell.Yellow))
	versionInfo = versionInfo + fmt.Sprintf("Arch:       %s\n", shell.Colorize(version.Arch, shell.Yellow))
	versionInfo = versionInfo + fmt.Sprintf("Version:    %s\n", shell.Colorize(version.Version, shell.Yellow))
	versionInfo = versionInfo + fmt.Sprintf("Branch:     %s\n", shell.Colorize(version.Branch, shell.Yellow))
	versionInfo = versionInfo + fmt.Sprintf("Commit:     %s\n", shell.Colorize(version.GitCommitHash, shell.Yellow))
	versionInfo = versionInfo + fmt.Sprintf("Timestamp:  %s\n", shell.Colorize(version.BuildTimestamp, shell.Yellow))
	fmt.Print(versionInfo)
}
