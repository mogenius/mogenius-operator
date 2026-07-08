package main

import (
	"log"
	"mogenius-operator/src/cmd"
	"os"

	"github.com/joho/godotenv"

	// Derive GOMEMLIMIT from the container's cgroup memory limit (90% for
	// non-heap headroom) so the GC reacts before the kernel OOM-kills the
	// pod. Respects an explicit GOMEMLIMIT override and is a no-op without
	// a limit. GOMAXPROCS needs no equivalent: since Go 1.25 the runtime
	// derives it from the cgroup CPU quota natively and keeps it updated.
	_ "github.com/KimMachineGun/automemlimit"
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
