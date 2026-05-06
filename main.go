package main

import (
	"os"

	"meow/cmd"
	"meow/internal/logo"
)

func main() {
	jsonMode := false
	for _, arg := range os.Args {
		if arg == "--json" || arg == "-h" || arg == "--help" || arg == "-help" {
			jsonMode = true
			break
		}
	}
	if !jsonMode {
		logo.Print()
	}
	cmd.Execute()
}
