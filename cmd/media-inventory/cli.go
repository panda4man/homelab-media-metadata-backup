package main

import (
	"errors"
	"fmt"
	"io"

	"github.com/panda4man/homelab-media-metadata-backup/internal/config"
)

// Exit codes:
//
//	0 success (snapshot valid or warning)
//	1 failed run
//	2 usage / config error
//	3 run already in progress (reserved, see internal/runlock)
const (
	exitOK            = 0
	exitFailed        = 1
	exitUsage         = 2
	exitAlreadyLocked = 3
)

const version = "0.1.0-dev"

const usage = `usage: media-inventory <command>

commands:
  run       execute a full inventory scan
  config    print the resolved configuration and exit
  version   print the version and exit
`

func realMain(args []string, stdout, stderr io.Writer, getenv func(string) string) int {
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
		if err := runCommand(args[1:], stdout, stderr, getenv); err != nil {
			fmt.Fprintln(stderr, "error:", err)
			return exitFailed
		}
		return exitOK
	default:
		fmt.Fprint(stderr, usage)
		return exitUsage
	}
}

// runCommand is a placeholder until the orchestrator (slice 17) exists.
func runCommand(_ []string, _, _ io.Writer, _ func(string) string) error {
	return errors.New("run: not yet implemented")
}
