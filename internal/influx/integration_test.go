//go:build integration

package influx

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestIntegration_RealInfluxDB writes one real point to a live InfluxDB
// v2 instance. Opt-in only: requires MEDIA_INVENTORY_IT=1 plus
// INFLUX_URL/INFLUX_TOKEN/INFLUX_ORG/INFLUX_BUCKET, and is excluded from
// the default `go test ./...` run by the integration build tag.
func TestIntegration_RealInfluxDB(t *testing.T) {
	if os.Getenv("MEDIA_INVENTORY_IT") != "1" {
		t.Skip("set MEDIA_INVENTORY_IT=1 to run real-InfluxDB integration tests")
	}
	url := os.Getenv("INFLUX_URL")
	token := os.Getenv("INFLUX_TOKEN")
	org := os.Getenv("INFLUX_ORG")
	bucket := os.Getenv("INFLUX_BUCKET")
	if url == "" || token == "" || org == "" || bucket == "" {
		t.Skip("INFLUX_URL, INFLUX_TOKEN, INFLUX_ORG, and INFLUX_BUCKET must be set for this test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	c := New(url, token, org, bucket)

	line := Encode(Metrics{Movies: 1}, Tags{Host: "integration-test", Job: "media-inventory", SchemaVersion: 1}, time.Now())
	if err := c.Write(ctx, line); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
}
