package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/panda4man/homelab-media-metadata-backup/internal/apiclient"
	"github.com/panda4man/homelab-media-metadata-backup/internal/httpapi"
	"github.com/panda4man/homelab-media-metadata-backup/internal/mcp"
)

// mcpCommand runs an MCP stdio server exposing the on-demand backup trigger
// API as tools, reaching it over apiclient. It reads API_URL/API_TOKEN
// directly rather than through config.Load: this process runs on the
// operator's workstation, not the backup host, and has none of the app's
// required config vars (MEDIA_MOVIES_PATH, RADARR_API_KEY, etc).
func mcpCommand(stdin io.Reader, stdout, stderr io.Writer, getenv func(string) string) int {
	apiURL := getenv("API_URL")
	if apiURL == "" {
		fmt.Fprintln(stderr, "error: API_URL is required")
		return exitUsage
	}

	apiToken := getenv("API_TOKEN")
	if len(apiToken) < minAPITokenLength {
		fmt.Fprintf(stderr, "error: API_TOKEN must be at least %d characters\n", minAPITokenLength)
		return exitUsage
	}

	client := apiclient.New(apiURL, apiToken)

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
