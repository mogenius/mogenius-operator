package main

import (
	"log"
	"mogenius-operator/src/cmd"
	"os"

	"github.com/joho/godotenv"
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
