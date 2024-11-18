package main

import (
	"fmt"
	"mogenius-k8s-manager/src/cmd"
	"os"
)

func main() {
	err := cmd.Run()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	os.Exit(0)
}
