package httpapi

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNew_Health_ReturnsOKWithoutAuth(t *testing.T) {
	handler := New(Config{Token: "some-token"})

	req := httptest.NewRequest("GET", "/v1/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"status":"ok"`) {
		t.Errorf("body = %q, want it to contain status ok", rec.Body.String())
	}
}

func TestNew_UnknownAuthenticatedPath_404(t *testing.T) {
	handler := New(Config{Token: "some-token"})

	req := httptest.NewRequest("GET", "/v1/nope", nil)
	req.Header.Set("Authorization", "Bearer some-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestNew_HealthWrongMethod_405(t *testing.T) {
	handler := New(Config{Token: "some-token"})

	req := httptest.NewRequest("POST", "/v1/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
}

func TestNew_AuthenticatedPathWithoutToken_401(t *testing.T) {
	handler := New(Config{Token: "some-token"})

	req := httptest.NewRequest("GET", "/v1/nope", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestServe_ContextCancelled_ReturnsNilAndClosesListener(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- Serve(ctx, ln, http.NewServeMux()) }()

	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Serve() error = %v, want nil", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Serve() did not return within grace period")
	}

	if _, err := ln.Accept(); err == nil {
		t.Error("Accept() on listener succeeded, want it closed")
	}
}
