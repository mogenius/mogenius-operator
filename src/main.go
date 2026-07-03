package main

import (
	"log"
	"mogenius-operator/src/cmd"
	"os"

	"github.com/joho/godotenv"

	// Align the Go runtime with container cgroup limits: GOMAXPROCS from the
	// CPU quota (avoids CFS throttling) and GOMEMLIMIT from the memory limit
	// (lets the GC react before the kernel OOM-kills the pod). Both respect
	// explicit GOMAXPROCS/GOMEMLIMIT env overrides and are no-ops without
	// cgroup limits.
	_ "github.com/KimMachineGun/automemlimit"
	_ "go.uber.org/automaxprocs"
)

func main() {
	if _, err := os.Stat(".env"); err == nil {
		err := godotenv.Load()
		if err != nil {
			log.Fatal(err)
		}
	}
	err := cmd.Run()
	if err != nil {
		log.Fatal(err)
	}
	os.Exit(0)
}
