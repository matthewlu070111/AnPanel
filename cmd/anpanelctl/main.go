package main

import (
	"github.com/anpanel/anpanel/internal/cli"
	"os"
)

func main() {
	os.Exit(cli.Run(os.Args[1:]))
}
