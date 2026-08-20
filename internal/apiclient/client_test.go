package apiclient

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/panda4man/homelab-media-metadata-backup/internal/httpapi"
)

func newTestServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv
}

func TestTriggerRun_HappyPath_SendsBearerHeaderAndReturnsDecodedRun(t *testing.T) {
	var gotMethod, gotPath, gotAuth string
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(httpapi.Run{ID: "run-1", Status: "running"})
	})
	c := New(srv.URL, "test-token")

	run, err := c.TriggerRun(context.Background())
	if err != nil {
		t.Fatalf("TriggerRun() error = %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %s, want POST", gotMethod)
	}
	if gotPath != "/v1/runs" {
		t.Errorf("path = %s, want /v1/runs", gotPath)
	}
	if gotAuth != "Bearer test-token" {
		t.Errorf("Authorization header = %q, want %q", gotAuth, "Bearer test-token")
	}
	if run.ID != "run-1" || run.Status != "running" {
		t.Errorf("run = %+v, want decoded ID/Status", run)
	}
}

func TestTriggerRun_AlreadyRunning_ReturnsErrAlreadyRunning(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(httpapi.ErrorResponse{Error: "already_running"})
	})
	c := New(srv.URL, "k")

	_, err := c.TriggerRun(context.Background())
	if !errors.Is(err, ErrAlreadyRunning) {
		t.Errorf("err = %v, want ErrAlreadyRunning", err)
	}
}

func TestTriggerRun_Unauthorized_ReturnsErrUnauthorized(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(httpapi.ErrorResponse{Error: "unauthorized"})
	})
	c := New(srv.URL, "wrong-token")

	_, err := c.TriggerRun(context.Background())
	if !errors.Is(err, ErrUnauthorized) {
		t.Errorf("err = %v, want ErrUnauthorized", err)
	}
}

func TestTriggerRun_ServerError_MessageContainsStatusCode(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	c := New(srv.URL, "k")

	_, err := c.TriggerRun(context.Background())
	if err == nil {
		t.Fatal("TriggerRun() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("err = %v, want message to contain 500", err)
	}
}

func TestTriggerRun_MalformedBody_ReturnsError(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte("{not valid json"))
	})
	c := New(srv.URL, "k")

	_, err := c.TriggerRun(context.Background())
	if err == nil {
		t.Fatal("TriggerRun() error = nil, want decode error")
	}
}

func TestGetRun_HappyPath_SendsExactPathAndReturnsDecodedRun(t *testing.T) {
	var gotPath string
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(httpapi.Run{ID: "abc", Status: "completed"})
	})
	c := New(srv.URL, "k")

	run, err := c.GetRun(context.Background(), "abc")
	if err != nil {
		t.Fatalf("GetRun() error = %v", err)
	}
	if gotPath != "/v1/runs/abc" {
		t.Errorf("path = %s, want /v1/runs/abc", gotPath)
	}
	if run.ID != "abc" || run.Status != "completed" {
		t.Errorf("run = %+v, want decoded ID/Status", run)
	}
}

func TestGetRun_NotFound_ReturnsErrRunNotFound(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(httpapi.ErrorResponse{Error: "not_found"})
	})
	c := New(srv.URL, "k")

	_, err := c.GetRun(context.Background(), "missing")
	if !errors.Is(err, ErrRunNotFound) {
		t.Errorf("err = %v, want ErrRunNotFound", err)
	}
}

func TestNew_TrailingSlashOnBaseURL_ProducesSameRequestPath(t *testing.T) {
	var gotPath string
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(httpapi.Run{ID: "abc"})
	})

	withSlash := New(srv.URL+"/", "k")
	if _, err := withSlash.GetRun(context.Background(), "abc"); err != nil {
		t.Fatalf("GetRun() error = %v", err)
	}
	pathWithSlash := gotPath

	withoutSlash := New(srv.URL, "k")
	if _, err := withoutSlash.GetRun(context.Background(), "abc"); err != nil {
		t.Fatalf("GetRun() error = %v", err)
	}
	pathWithoutSlash := gotPath

	if pathWithSlash != pathWithoutSlash {
		t.Errorf("path with trailing slash = %q, path without = %q, want equal", pathWithSlash, pathWithoutSlash)
	}
	if strings.Contains(pathWithSlash, "//") {
		t.Errorf("path = %q, contains double slash", pathWithSlash)
	}
	if pathWithSlash != "/v1/runs/abc" {
		t.Errorf("path = %q, want /v1/runs/abc", pathWithSlash)
	}
}

func TestTriggerRun_CancelledContext_ReturnsErrorNotHang(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	})
	c := New(srv.URL, "k")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := c.TriggerRun(ctx)
	if err == nil {
		t.Fatal("TriggerRun() error = nil, want error for cancelled context")
	}
}

func TestGetRun_CancelledContext_ReturnsErrorNotHang(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	c := New(srv.URL, "k")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := c.GetRun(ctx, "abc")
	if err == nil {
		t.Fatal("GetRun() error = nil, want error for cancelled context")
	}
}
