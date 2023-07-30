package main

import (
	"os"
	"os/exec"
)

func main() {
	// Create an *exec.Cmd
	cmd := exec.Command("kubectl", "exec", "--stdin", "--tty", "default-backend-5bdc897df6-zzjzf", "--", "/bin/sh")

	// Assign os.Stdin, os.Stdout, and os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Run the command
	err := cmd.Run()

	if err != nil {
		panic(err)
	}
}
