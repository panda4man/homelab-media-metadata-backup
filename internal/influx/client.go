// Package influx publishes aggregate operational metrics to InfluxDB v2.
// InfluxDB is observability infrastructure, not part of the
// disaster-recovery contract: a publish failure is always logged and
// swallowed by Publisher, never allowed to invalidate an otherwise-good
// snapshot.
package influx

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/panda4man/homelab-media-metadata-backup/internal/clockx"
)

const defaultTimeout = 10 * time.Second

// Client is a minimal InfluxDB v2 write-API client.
type Client struct {
	baseURL    string
	token      string
	org        string
	bucket     string
	httpClient *http.Client
}

// Option configures a Client constructed by New.
type Option func(*Client)

// WithHTTPClient overrides the default HTTP client (and its timeout).
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) { c.httpClient = hc }
}

// WithTimeout overrides the default per-request timeout.
func WithTimeout(d time.Duration) Option {
	return func(c *Client) { c.httpClient.Timeout = d }
}

// New builds an InfluxDB v2 client for the given URL, token, org, and
// bucket.
func New(baseURL, token, org, bucket string, opts ...Option) *Client {
	c := &Client{
		baseURL:    strings.TrimSuffix(baseURL, "/"),
		token:      token,
		org:        org,
		bucket:     bucket,
		httpClient: &http.Client{Timeout: defaultTimeout},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Write posts one line-protocol line to /api/v2/write at nanosecond
// precision.
func (c *Client) Write(ctx context.Context, line string) error {
	endpoint := fmt.Sprintf("%s/api/v2/write?org=%s&bucket=%s&precision=ns",
		c.baseURL, url.QueryEscape(c.org), url.QueryEscape(c.bucket))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(line))
	if err != nil {
		return fmt.Errorf("influx: building request: %w", err)
	}
	req.Header.Set("Authorization", "Token "+c.token)
	req.Header.Set("Content-Type", "text/plain; charset=utf-8")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("influx: write request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusOK {
		return nil
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	return fmt.Errorf("influx: write failed: status %d: %s", resp.StatusCode, body)
}

// Publisher wraps a Client with the "never fail the run" contract: a
// nil Client (InfluxDB not configured) or a failed write are both
// swallowed, logged at WARN, and Publish returns nothing - there is no
// error for a caller to even consider propagating.
type Publisher struct {
	Client *Client
	Tags   Tags
	Clock  clockx.Clock
	Logger *slog.Logger
}

// Publish encodes m and writes it, logging (not failing) on any problem.
func (p Publisher) Publish(ctx context.Context, m Metrics) {
	if p.Client == nil {
		return
	}
	now := time.Now()
	if p.Clock != nil {
		now = p.Clock.Now()
	}
	line := Encode(m, p.Tags, now)
	if err := p.Client.Write(ctx, line); err != nil {
		p.logger().Warn("influx: failed to publish metrics", "error", err)
	}
}

func (p Publisher) logger() *slog.Logger {
	if p.Logger != nil {
		return p.Logger
	}
	return slog.Default()
}
