//go:build integration

package httpapi_test

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/panda4man/homelab-media-metadata-backup/internal/apiclient"
	"github.com/panda4man/homelab-media-metadata-backup/internal/httpapi"
)

// TestIntegration_RealAPI exercises a live serve instance end to end: a
// health check, a rejected unauthenticated trigger, and a full triggered
// run polled through to a terminal status. Requires MEDIA_INVENTORY_IT=1
// plus API_URL/API_TOKEN, and is skipped otherwise.
func TestIntegration_RealAPI(t *testing.T) {
	if os.Getenv("MEDIA_INVENTORY_IT") != "1" {
		t.Skip("set MEDIA_INVENTORY_IT=1 to run real-API integration tests")
	}
	apiURL := os.Getenv("API_URL")
	apiToken := os.Getenv("API_TOKEN")
	if apiURL == "" || apiToken == "" {
		t.Skip("API_URL and API_TOKEN must be set for this test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	healthReq, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL+"/v1/health", nil)
	if err != nil {
		t.Fatalf("building health request: %v", err)
	}
	healthResp, err := http.DefaultClient.Do(healthReq)
	if err != nil {
		t.Fatalf("GET /v1/health: %v", err)
	}
	healthResp.Body.Close()
	if healthResp.StatusCode != http.StatusOK {
		t.Fatalf("GET /v1/health status = %d, want %d", healthResp.StatusCode, http.StatusOK)
	}

	unauthReq, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL+"/v1/runs", nil)
	if err != nil {
		t.Fatalf("building unauthenticated trigger request: %v", err)
	}
	unauthResp, err := http.DefaultClient.Do(unauthReq)
	if err != nil {
		t.Fatalf("POST /v1/runs (no token): %v", err)
	}
	unauthResp.Body.Close()
	if unauthResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("POST /v1/runs (no token) status = %d, want %d", unauthResp.StatusCode, http.StatusUnauthorized)
	}

	client := apiclient.New(apiURL, apiToken)
	run, err := client.TriggerRun(ctx)
	if err != nil {
		t.Fatalf("TriggerRun() error = %v", err)
	}

	final := waitForTerminalStatus(t, client, run.ID)
	if final.Status != "completed" && final.Status != "rejected" {
		t.Fatalf("run %s reached non-terminal status %q", run.ID, final.Status)
	}
}

// waitForTerminalStatus polls GetRun until the run reaches "completed" or
// "rejected", bounded by a deadline channel rather than a single guessed
// sleep, mirroring handlers_test.go's waitForRunStatus.
func waitForTerminalStatus(t *testing.T, client *apiclient.Client, id string) httpapi.Run {
	t.Helper()
	deadline := time.After(2 * time.Minute)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		run, err := client.GetRun(ctx, id)
		cancel()
		if err != nil {
			t.Fatalf("GetRun(%q): %v", id, err)
		}
		if run.Status == "completed" || run.Status == "rejected" {
			return run
		}
		select {
		case <-ticker.C:
		case <-deadline:
			t.Fatalf("run %s did not reach a terminal status in time; last status = %q", id, run.Status)
		}
	}
}
