package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/anpanel/anpanel/internal/agent"
	"github.com/anpanel/anpanel/internal/app"
	"github.com/anpanel/anpanel/internal/cli"
	"github.com/anpanel/anpanel/internal/config"
)

var buildVersion = "dev"

func main() {
	if filepath.Base(os.Args[0]) == "anpanelctl" {
		os.Exit(cli.Run(os.Args[1:]))
	}
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "configuration error:", err)
		os.Exit(1)
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	switch os.Args[1] {
	case "web":
		err = app.Run(ctx, cfg, logger)
	case "agent":
		err = agent.Run(ctx, cfg, logger)
	case "version":
		fmt.Println("anpanel " + buildVersion)
		return
	case "ctl":
		os.Exit(cli.Run(os.Args[2:]))
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		logger.Error("service stopped", "error", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: anpanel <web|agent|version>")
}
