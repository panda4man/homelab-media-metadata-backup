package main

import (
	"errors"
	"fmt"
	"io"
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
  version   print the version and exit
`

func realMain(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return exitUsage
	}

	switch args[0] {
	case "version":
		fmt.Fprintln(stdout, version)
		return exitOK
	case "run":
		if err := runCommand(args[1:], stdout, stderr); err != nil {
			fmt.Fprintln(stderr, "error:", err)
			return exitFailed
		}
		return exitOK
	default:
		fmt.Fprint(stderr, usage)
		return exitUsage
	}
}

// runCommand is a placeholder until config loading (slice 2) and the
// orchestrator (slice 17) exist.
func runCommand(_ []string, _, _ io.Writer) error {
	return errors.New("run: not yet implemented")
}
