package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/panda4man/homelab-media-metadata-backup/internal/apiclient"
	"github.com/panda4man/homelab-media-metadata-backup/internal/config"
	"github.com/panda4man/homelab-media-metadata-backup/internal/httpapi"
	"github.com/panda4man/homelab-media-metadata-backup/internal/mcp"
)

// mcpCommand loads configuration and runs an MCP stdio server exposing the
// on-demand backup trigger API as tools, reaching it over apiclient against
// the "serve" command's HTTP API.
func mcpCommand(stdin io.Reader, stdout, stderr io.Writer, getenv func(string) string) int {
	cfg, err := config.Load(getenv)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return exitUsage
	}

	if len(cfg.APIToken) < minAPITokenLength {
		fmt.Fprintf(stderr, "error: API_TOKEN must be at least %d characters\n", minAPITokenLength)
		return exitUsage
	}

	client := apiclient.New(apiBaseURL(cfg.APIAddr), cfg.APIToken)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	srv := &mcp.Server{
		In:  stdin,
		Out: stdout,
		TriggerRun: func(ctx context.Context) (mcp.RunInfo, error) {
			run, err := client.TriggerRun(ctx)
			return toRunInfo(run), err
		},
		GetRun: func(ctx context.Context, id string) (mcp.RunInfo, error) {
			run, err := client.GetRun(ctx, id)
			return toRunInfo(run), err
		},
		Logger: slog.New(slog.NewTextHandler(stderr, nil)),
	}

	if err := srv.Serve(ctx); err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return exitServeFailed
	}
	return exitOK
}

// apiBaseURL turns a net.Listen-style address (e.g. ":8080" or
// "127.0.0.1:9000") into the HTTP base URL apiclient dials.
func apiBaseURL(addr string) string {
	if strings.HasPrefix(addr, ":") {
		return "http://localhost" + addr
	}
	return "http://" + addr
}

// toRunInfo is the chokepoint where httpapi.Run's fields cross into mcp's
// own decoupled RunInfo: a direct field-by-field copy, no logic needed.
func toRunInfo(run httpapi.Run) mcp.RunInfo {
	return mcp.RunInfo{
		ID:             run.ID,
		Status:         run.Status,
		Reason:         run.Reason,
		State:          run.State,
		SnapshotPath:   run.SnapshotPath,
		OffsiteSuccess: run.OffsiteSuccess,
		Error:          run.Error,
	}
}
