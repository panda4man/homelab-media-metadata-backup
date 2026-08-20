package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/panda4man/homelab-media-metadata-backup/internal/config"
	"github.com/panda4man/homelab-media-metadata-backup/internal/orchestrator"
	"github.com/panda4man/homelab-media-metadata-backup/internal/runlock"
	"github.com/panda4man/homelab-media-metadata-backup/internal/validation"
)

// Exit codes:
//
//	0 success (snapshot valid or warning, off-site sync succeeded or was disabled)
//	1 failed run (snapshot state is failed)
//	2 usage / config error
//	3 run already in progress
//	4 off-site backup failure (snapshot itself is valid/warning, but the off-site copy failed)
//	5 serve command failed to listen or the API server stopped with an error
const (
	exitOK            = 0
	exitFailed        = 1
	exitUsage         = 2
	exitAlreadyLocked = 3
	exitOffsiteFailed = 4
	exitServeFailed   = 5
)

const version = "0.1.0-dev"

const usage = `usage: media-inventory <command>

commands:
  run       execute a full inventory scan
  serve     run the on-demand backup trigger HTTP API
  mcp       run an MCP stdio server exposing the backup trigger as tools
  config    print the resolved configuration and exit
  version   print the version and exit
`

func realMain(args []string, stdin io.Reader, stdout, stderr io.Writer, getenv func(string) string) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return exitUsage
	}

	switch args[0] {
	case "version":
		fmt.Fprintln(stdout, version)
		return exitOK
	case "config":
		cfg, err := config.Load(getenv)
		if err != nil {
			fmt.Fprintln(stderr, "error:", err)
			return exitUsage
		}
		fmt.Fprint(stdout, cfg.String())
		return exitOK
	case "run":
		return runCommand(stdout, stderr, getenv)
	case "serve":
		return serveCommand(stdout, stderr, getenv)
	case "mcp":
		return mcpCommand(stdin, stdout, stderr, getenv)
	default:
		fmt.Fprint(stderr, usage)
		return exitUsage
	}
}

// runCommand loads configuration, wires every package into a real
// orchestrator.Deps, and executes one full inventory run.
func runCommand(stdout, stderr io.Writer, getenv func(string) string) int {
	cfg, err := config.Load(getenv)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return exitUsage
	}

	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	logger := slog.New(slog.NewTextHandler(stdout, nil))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	deps := buildDeps(cfg, logger, hostname)

	result, runErr := orchestrator.Run(ctx, cfg, deps)

	switch {
	case errors.Is(runErr, runlock.ErrLocked):
		fmt.Fprintln(stderr, "error:", runErr)
		return exitAlreadyLocked
	case result.State == validation.StateFailed:
		if runErr != nil {
			fmt.Fprintln(stderr, "error:", runErr)
		}
		return exitFailed
	case runErr != nil:
		fmt.Fprintln(stderr, "error:", runErr)
		return exitOffsiteFailed
	default:
		return exitOK
	}
}
