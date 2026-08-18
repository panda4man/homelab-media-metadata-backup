package influx

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func newTestServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv
}

func TestWrite_HappyPath_PostsExpectedRequest(t *testing.T) {
	var gotMethod, gotPath, gotQuery, gotAuth, gotBody string
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		gotAuth = r.Header.Get("Authorization")
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		w.WriteHeader(http.StatusNoContent)
	})
	c := New(srv.URL, "my-token", "homelab", "media")

	line := "media_inventory,host=unraid movies=1i 123"
	if err := c.Write(context.Background(), line); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Errorf("method = %s, want POST", gotMethod)
	}
	if gotPath != "/api/v2/write" {
		t.Errorf("path = %s, want /api/v2/write", gotPath)
	}
	if gotQuery != "org=homelab&bucket=media&precision=ns" {
		t.Errorf("query = %s, want org/bucket/precision=ns", gotQuery)
	}
	if gotAuth != "Token my-token" {
		t.Errorf("Authorization = %q, want %q", gotAuth, "Token my-token")
	}
	if gotBody != line {
		t.Errorf("body = %q, want %q", gotBody, line)
	}
}

func TestWrite_204_ReturnsNil(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	c := New(srv.URL, "t", "o", "b")

	if err := c.Write(context.Background(), "line"); err != nil {
		t.Errorf("Write() error = %v, want nil", err)
	}
}

func TestWrite_Unauthorized_ReturnsErrorWithBody(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("invalid token"))
	})
	c := New(srv.URL, "bad-token", "o", "b")

	err := c.Write(context.Background(), "line")
	if err == nil {
		t.Fatal("Write() error = nil, want an error")
	}
	if !strings.Contains(err.Error(), "invalid token") {
		t.Errorf("err = %v, want it to contain the response body", err)
	}
}

func TestWrite_ServerError_ReturnsErrorWithBody(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	})
	c := New(srv.URL, "t", "o", "b")

	err := c.Write(context.Background(), "line")
	if err == nil || !strings.Contains(err.Error(), "internal error") {
		t.Errorf("err = %v, want it to contain the response body", err)
	}
}

func TestWrite_ConnectionRefused_ReturnsError(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	addr := l.Addr().String()
	l.Close()

	c := New("http://"+addr, "t", "o", "b")
	if err := c.Write(context.Background(), "line"); err == nil {
		t.Error("Write() error = nil, want an error")
	}
}

func TestWrite_Timeout_ReturnsError(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusNoContent)
	})
	c := New(srv.URL, "t", "o", "b", WithTimeout(10*time.Millisecond))

	if err := c.Write(context.Background(), "line"); err == nil {
		t.Error("Write() error = nil, want a timeout error")
	}
}

func TestPublisher_NilClient_SkipsEntirelyNoPanic(t *testing.T) {
	p := Publisher{Client: nil}
	// Must not panic and must not block - Publish returns nothing, by design.
	p.Publish(context.Background(), fullMetrics())
}

func TestPublisher_Success_NoLogWarning(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	p := Publisher{
		Client: New(srv.URL, "t", "o", "b"),
		Tags:   fullTags(),
		Logger: logger,
	}

	p.Publish(context.Background(), fullMetrics())

	if strings.Contains(buf.String(), "WARN") {
		t.Errorf("log output = %q, want no warning on success", buf.String())
	}
}

func TestPublisher_Failure_LogsWarningDoesNotPanic(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	p := Publisher{
		Client: New(srv.URL, "t", "o", "b"),
		Tags:   fullTags(),
		Logger: logger,
	}

	p.Publish(context.Background(), fullMetrics()) // must not panic, returns nothing

	if !strings.Contains(buf.String(), "WARN") {
		t.Errorf("log output = %q, want a WARN on publish failure", buf.String())
	}
}
