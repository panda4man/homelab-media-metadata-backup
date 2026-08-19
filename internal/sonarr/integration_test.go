//go:build integration

package sonarr

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestIntegration_RealSonarr exercises Ping, Series, and Episodes against
// a real Sonarr instance. Opt-in only: requires MEDIA_INVENTORY_IT=1 plus
// SONARR_URL/SONARR_API_KEY, and is excluded from the default `go test
// ./...` run by the integration build tag.
func TestIntegration_RealSonarr(t *testing.T) {
	if os.Getenv("MEDIA_INVENTORY_IT") != "1" {
		t.Skip("set MEDIA_INVENTORY_IT=1 to run real-Sonarr integration tests")
	}
	url := os.Getenv("SONARR_URL")
	key := os.Getenv("SONARR_API_KEY")
	if url == "" || key == "" {
		t.Skip("SONARR_URL and SONARR_API_KEY must be set for this test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	c := New(url, key)

	if err := c.Ping(ctx); err != nil {
		t.Fatalf("Ping() error = %v", err)
	}
	series, err := c.Series(ctx)
	if err != nil {
		t.Fatalf("Series() error = %v", err)
	}
	if len(series) > 0 {
		if _, err := c.Episodes(ctx, series[0].ID); err != nil {
			t.Fatalf("Episodes() error = %v", err)
		}
	}
}
