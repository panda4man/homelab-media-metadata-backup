// Package radarr is a minimal, read-only client for the Radarr v3 API: just
// enough to check reachability and fetch the movie list with the fields the
// disaster-recovery manifest needs. It never issues anything but GET
// requests - the application must never modify Radarr.
package radarr

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// ErrUnauthorized means the configured API key was rejected.
var ErrUnauthorized = errors.New("radarr: unauthorized")

// ErrUnavailable means Radarr could not be reached or returned a server
// error. The orchestrator treats this as fatal for the run.
var ErrUnavailable = errors.New("radarr: unavailable")

const defaultTimeout = 10 * time.Second

// Client is a Radarr v3 API client.
type Client struct {
	baseURL    string
	apiKey     string
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

// New builds a Radarr client for the given base URL and API key.
func New(baseURL, apiKey string, opts ...Option) *Client {
	c := &Client{
		baseURL:    strings.TrimSuffix(baseURL, "/"),
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: defaultTimeout},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Ping checks that Radarr is reachable and the API key is accepted.
func (c *Client) Ping(ctx context.Context) error {
	resp, err := c.get(ctx, "/api/v3/system/status")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return checkStatus(resp)
}

// Movies fetches every movie Radarr knows about, mapped to the fields the
// manifest needs. A movie with no file on disk is still returned - the
// filesystem, not Radarr, decides what actually exists.
func (c *Client) Movies(ctx context.Context) ([]Movie, error) {
	resp, err := c.get(ctx, "/api/v3/movie")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err := checkStatus(resp); err != nil {
		return nil, err
	}

	var dtos []movieDTO
	if err := json.NewDecoder(resp.Body).Decode(&dtos); err != nil {
		return nil, fmt.Errorf("radarr: parsing movies response: %w", err)
	}

	movies := make([]Movie, 0, len(dtos))
	for _, d := range dtos {
		movies = append(movies, d.toMovie())
	}
	return movies, nil
}

func (c *Client) get(ctx context.Context, path string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Api-Key", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	return resp, nil
}

func checkStatus(resp *http.Response) error {
	if resp.StatusCode == http.StatusOK {
		return nil
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return ErrUnauthorized
	}
	return fmt.Errorf("%w: status %d", ErrUnavailable, resp.StatusCode)
}
