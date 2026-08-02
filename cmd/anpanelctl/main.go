package main

import (
	"github.com/matthewlu070111/anpanel/internal/cli"
	"os"
)

func main() {
	os.Exit(cli.Run(os.Args[1:]))
}
