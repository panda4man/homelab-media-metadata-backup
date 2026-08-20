package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/panda4man/homelab-media-metadata-backup/internal/config"
	"github.com/panda4man/homelab-media-metadata-backup/internal/httpapi"
	"github.com/panda4man/homelab-media-metadata-backup/internal/orchestrator"
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

	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}
	logger := slog.New(slog.NewTextHandler(stdout, nil))
	deps := buildDeps(cfg, logger, hostname)

	handler := httpapi.New(httpapi.Config{
		Token:    cfg.APIToken,
		Ctx:      ctx,
		Cfg:      cfg,
		LockPath: orchestrator.LockPath(cfg),
		RunFn: func(runCtx context.Context, runCfg config.Config) (orchestrator.Result, error) {
			return orchestrator.Run(runCtx, runCfg, deps)
		},
		NewID:  newRunID,
		Now:    time.Now,
		Logger: logger,
	})
	if err := httpapi.Serve(ctx, ln, handler); err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return exitServeFailed
	}
	return exitOK
}

// newRunID returns a random 16-hex-character run identifier for the
// on-demand trigger API.
func newRunID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
