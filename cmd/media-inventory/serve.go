package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/panda4man/homelab-media-metadata-backup/internal/config"
	"github.com/panda4man/homelab-media-metadata-backup/internal/httpapi"
)

const minAPITokenLength = 16

// serveCommand loads configuration and runs the on-demand backup trigger
// HTTP API until it's told to stop.
func serveCommand(stdout, stderr io.Writer, getenv func(string) string) int {
	cfg, err := config.Load(getenv)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return exitUsage
	}

	if len(cfg.APIToken) < minAPITokenLength {
		fmt.Fprintf(stderr, "error: API_TOKEN must be at least %d characters\n", minAPITokenLength)
		return exitUsage
	}

	ln, err := net.Listen("tcp", cfg.APIAddr)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return exitServeFailed
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	handler := httpapi.New(httpapi.Config{Token: cfg.APIToken})
	if err := httpapi.Serve(ctx, ln, handler); err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return exitServeFailed
	}
	return exitOK
}
