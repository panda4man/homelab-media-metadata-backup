//go:build integration

package radarr

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestIntegration_RealRadarr exercises Ping and Movies against a real
// Radarr instance. Opt-in only: requires MEDIA_INVENTORY_IT=1 plus
// RADARR_URL/RADARR_API_KEY, and is excluded from the default `go test
// ./...` run by the integration build tag.
func TestIntegration_RealRadarr(t *testing.T) {
	if os.Getenv("MEDIA_INVENTORY_IT") != "1" {
		t.Skip("set MEDIA_INVENTORY_IT=1 to run real-Radarr integration tests")
	}
	url := os.Getenv("RADARR_URL")
	key := os.Getenv("RADARR_API_KEY")
	if url == "" || key == "" {
		t.Skip("RADARR_URL and RADARR_API_KEY must be set for this test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	c := New(url, key)

	if err := c.Ping(ctx); err != nil {
		t.Fatalf("Ping() error = %v", err)
	}
	if _, err := c.Movies(ctx); err != nil {
		t.Fatalf("Movies() error = %v", err)
	}
}
