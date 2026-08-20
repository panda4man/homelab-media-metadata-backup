package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRequireToken_NoAuthorizationHeader_401Unauthorized(t *testing.T) {
	called := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true })

	req := httptest.NewRequest("GET", "/v1/anything", nil)
	rec := httptest.NewRecorder()
	requireToken("correct-token", inner).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"error":"unauthorized"`) {
		t.Errorf("body = %q, want it to contain unauthorized error", rec.Body.String())
	}
	if called {
		t.Error("inner handler was called, want it skipped on missing auth")
	}
}

func TestRequireToken_WrongToken_401Unauthorized(t *testing.T) {
	called := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true })

	req := httptest.NewRequest("GET", "/v1/anything", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	rec := httptest.NewRecorder()
	requireToken("correct-token", inner).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	if called {
		t.Error("inner handler was called, want it skipped on wrong token")
	}
}

func TestRequireToken_CorrectToken_ReachesInnerHandler(t *testing.T) {
	called := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/v1/anything", nil)
	req.Header.Set("Authorization", "Bearer correct-token")
	rec := httptest.NewRecorder()
	requireToken("correct-token", inner).ServeHTTP(rec, req)

	if !called {
		t.Fatal("inner handler was not called, want it reached with correct token")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}
