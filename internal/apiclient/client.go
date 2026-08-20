// Package apiclient is a client for the on-demand backup trigger HTTP API
// added in slices 19-20.
package apiclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/panda4man/homelab-media-metadata-backup/internal/httpapi"
)

const bearerPrefix = "Bearer "

// ErrUnauthorized means the configured token was rejected.
var ErrUnauthorized = errors.New("apiclient: unauthorized")

// ErrAlreadyRunning means a run was already in progress when TriggerRun was called.
var ErrAlreadyRunning = errors.New("apiclient: already running")

// ErrRunNotFound means GetRun was called with an id the server doesn't know about.
var ErrRunNotFound = errors.New("apiclient: run not found")

const defaultTimeout = 10 * time.Second

// Client is a client for the on-demand backup trigger API.
type Client struct {
	baseURL    string
	token      string
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

// New builds a Client for the given base URL and bearer token.
func New(baseURL, token string, opts ...Option) *Client {
	c := &Client{
		baseURL:    strings.TrimSuffix(baseURL, "/"),
		token:      token,
		httpClient: &http.Client{Timeout: defaultTimeout},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// TriggerRun starts a new backup run and returns the run record the server
// created for it.
func (c *Client) TriggerRun(ctx context.Context) (httpapi.Run, error) {
	resp, err := c.post(ctx, "/v1/runs")
	if err != nil {
		return httpapi.Run{}, err
	}
	defer resp.Body.Close()
	if err := checkStatus(resp, http.StatusAccepted); err != nil {
		return httpapi.Run{}, err
	}

	var run httpapi.Run
	if err := json.NewDecoder(resp.Body).Decode(&run); err != nil {
		return httpapi.Run{}, fmt.Errorf("apiclient: parsing trigger response: %w", err)
	}
	return run, nil
}

// GetRun fetches the current record for a previously triggered run.
func (c *Client) GetRun(ctx context.Context, id string) (httpapi.Run, error) {
	resp, err := c.get(ctx, "/v1/runs/"+url.PathEscape(id))
	if err != nil {
		return httpapi.Run{}, err
	}
	defer resp.Body.Close()
	if err := checkStatus(resp, http.StatusOK); err != nil {
		return httpapi.Run{}, err
	}

	var run httpapi.Run
	if err := json.NewDecoder(resp.Body).Decode(&run); err != nil {
		return httpapi.Run{}, fmt.Errorf("apiclient: parsing run response: %w", err)
	}
	return run, nil
}

func (c *Client) get(ctx context.Context, path string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	return c.do(req)
}

func (c *Client) post(ctx context.Context, path string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	return c.do(req)
}

func (c *Client) do(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", bearerPrefix+c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("apiclient: request failed: %w", err)
	}
	return resp, nil
}

func checkStatus(resp *http.Response, okStatus int) error {
	if resp.StatusCode == okStatus {
		return nil
	}
	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return ErrUnauthorized
	case http.StatusConflict:
		return ErrAlreadyRunning
	case http.StatusNotFound:
		return ErrRunNotFound
	default:
		return fmt.Errorf("apiclient: unexpected status %d", resp.StatusCode)
	}
}
