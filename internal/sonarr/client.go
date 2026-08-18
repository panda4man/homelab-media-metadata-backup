// Package sonarr is a minimal, read-only client for the Sonarr v3 API: just
// enough to check reachability and fetch series and episode metadata with
// the fields the disaster-recovery manifest needs. It never issues
// anything but GET requests - the application must never modify Sonarr.
package sonarr

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// ErrUnauthorized means the configured API key was rejected.
var ErrUnauthorized = errors.New("sonarr: unauthorized")

// ErrUnavailable means Sonarr could not be reached or returned a server
// error. The orchestrator treats this as fatal for the run.
var ErrUnavailable = errors.New("sonarr: unavailable")

const defaultTimeout = 10 * time.Second

// Client is a Sonarr v3 API client.
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

// New builds a Sonarr client for the given base URL and API key.
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

// Ping checks that Sonarr is reachable and the API key is accepted.
func (c *Client) Ping(ctx context.Context) error {
	resp, err := c.get(ctx, "/api/v3/system/status", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return checkStatus(resp)
}

// Series fetches every series Sonarr knows about, mapped to the fields the
// manifest needs.
func (c *Client) Series(ctx context.Context) ([]Series, error) {
	resp, err := c.get(ctx, "/api/v3/series", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err := checkStatus(resp); err != nil {
		return nil, err
	}

	var dtos []seriesDTO
	if err := json.NewDecoder(resp.Body).Decode(&dtos); err != nil {
		return nil, fmt.Errorf("sonarr: parsing series response: %w", err)
	}

	series := make([]Series, 0, len(dtos))
	for _, d := range dtos {
		series = append(series, d.toSeries())
	}
	return series, nil
}

// Episodes fetches every episode for a series, joined with its file
// (relative path and size) when one exists. An episode with no file on
// disk is still returned - the filesystem, not Sonarr, decides what
// actually exists.
func (c *Client) Episodes(ctx context.Context, seriesID int) ([]Episode, error) {
	query := url.Values{"seriesId": {strconv.Itoa(seriesID)}}

	episodeResp, err := c.get(ctx, "/api/v3/episode", query)
	if err != nil {
		return nil, err
	}
	defer episodeResp.Body.Close()
	if err := checkStatus(episodeResp); err != nil {
		return nil, err
	}
	var episodeDTOs []episodeDTO
	if err := json.NewDecoder(episodeResp.Body).Decode(&episodeDTOs); err != nil {
		return nil, fmt.Errorf("sonarr: parsing episode response: %w", err)
	}

	fileResp, err := c.get(ctx, "/api/v3/episodefile", query)
	if err != nil {
		return nil, err
	}
	defer fileResp.Body.Close()
	if err := checkStatus(fileResp); err != nil {
		return nil, err
	}
	var fileDTOs []episodeFileDTO
	if err := json.NewDecoder(fileResp.Body).Decode(&fileDTOs); err != nil {
		return nil, fmt.Errorf("sonarr: parsing episodefile response: %w", err)
	}

	return joinEpisodes(episodeDTOs, fileDTOs), nil
}

func (c *Client) get(ctx context.Context, path string, query url.Values) (*http.Response, error) {
	target := c.baseURL + path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
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
