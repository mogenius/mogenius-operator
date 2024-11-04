package main

import (
	"mogenius-k8s-manager/cmd"
	"mogenius-k8s-manager/shutdown"
	"mogenius-k8s-manager/utils"
)

func main() {
	utils.PrintLogo()
	go cmd.Execute()
	shutdown.Listen()
}
