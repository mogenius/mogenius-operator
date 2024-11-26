package version

import (
	"fmt"
	"mogenius-k8s-manager/src/shell"
)

type Version struct{}

func NewVersion() *Version {
	return &Version{}
}

func (self *Version) PrintVersionInfo() {
	fmt.Printf(
		"███╗░░░███╗░█████╗░░██████╗░███████╗███╗░░██╗██╗██╗░░░██╗░██████╗\n" +
			"████╗░████║██╔══██╗██╔════╝░██╔════╝████╗░██║██║██║░░░██║██╔════╝\n" +
			"██╔████╔██║██║░░██║██║░░██╗░█████╗░░██╔██╗██║██║██║░░░██║╚█████╗░\n" +
			"██║╚██╔╝██║██║░░██║██║░░╚██╗██╔══╝░░██║╚████║██║██║░░░██║░╚═══██╗\n" +
			"██║░╚═╝░██║╚█████╔╝╚██████╔╝███████╗██║░╚███║██║╚██████╔╝██████╔╝\n" +
			"╚═╝░░░░░╚═╝░╚════╝░░╚═════╝░╚══════╝╚═╝░░╚══╝╚═╝░╚═════╝░╚═════╝░\n\n",
	)

	versionInfo := ""
	versionInfo = versionInfo + fmt.Sprintf("CLI:       %s\n", shell.Colorize(Ver, shell.Yellow))
	versionInfo = versionInfo + fmt.Sprintf("Container: %s\n", shell.Colorize(Ver, shell.Yellow))
	versionInfo = versionInfo + fmt.Sprintf("Branch:    %s\n", shell.Colorize(Branch, shell.Yellow))
	versionInfo = versionInfo + fmt.Sprintf("Commit:    %s\n", shell.Colorize(GitCommitHash, shell.Yellow))
	versionInfo = versionInfo + fmt.Sprintf("Timestamp: %s\n", shell.Colorize(BuildTimestamp, shell.Yellow))

	fmt.Print(versionInfo)
}
