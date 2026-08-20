package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/panda4man/homelab-media-metadata-backup/internal/httpapi"
)

func TestMcpCommand_MissingAPIToken_ExitsUsageError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	env := validConfigEnv()

	code := mcpCommand(strings.NewReader(""), &stdout, &stderr, getenvFrom(env))

	if code != exitUsage {
		t.Fatalf("exit code = %d, want %d, stderr = %q", code, exitUsage, stderr.String())
	}
	if !strings.Contains(stderr.String(), "API_TOKEN") {
		t.Errorf("stderr = %q, want it to name API_TOKEN", stderr.String())
	}
}

func TestMcpCommand_ShortAPIToken_ExitsUsageError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	env := validConfigEnv()
	env["API_TOKEN"] = "short"

	code := mcpCommand(strings.NewReader(""), &stdout, &stderr, getenvFrom(env))

	if code != exitUsage {
		t.Fatalf("exit code = %d, want %d, stderr = %q", code, exitUsage, stderr.String())
	}
	if !strings.Contains(stderr.String(), "API_TOKEN") {
		t.Errorf("stderr = %q, want it to name API_TOKEN", stderr.String())
	}
}

func TestMcpCommand_MissingConfig_ExitsUsageError(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := mcpCommand(strings.NewReader(""), &stdout, &stderr, noEnv)

	if code != exitUsage {
		t.Fatalf("exit code = %d, want %d", code, exitUsage)
	}
}

func TestMcpCommand_EmptyStdin_ServesUntilEOFAndExitsOK(t *testing.T) {
	var stdout, stderr bytes.Buffer
	env := validConfigEnv()
	env["API_TOKEN"] = "a-token-at-least-16-chars"

	code := mcpCommand(strings.NewReader(""), &stdout, &stderr, getenvFrom(env))

	if code != exitOK {
		t.Fatalf("exit code = %d, want %d, stderr = %q", code, exitOK, stderr.String())
	}
}

func TestToRunInfo_CopiesAllFields(t *testing.T) {
	run := httpapi.Run{
		ID:             "abc123",
		Status:         "completed",
		Reason:         "manual trigger",
		State:          "valid",
		SnapshotPath:   "/data/snapshots/2026-08-20",
		OffsiteSuccess: true,
		Error:          "",
	}

	got := toRunInfo(run)

	if got.ID != run.ID || got.Status != run.Status || got.Reason != run.Reason ||
		got.State != run.State || got.SnapshotPath != run.SnapshotPath ||
		got.OffsiteSuccess != run.OffsiteSuccess || got.Error != run.Error {
		t.Errorf("toRunInfo(%+v) = %+v, want a direct field-by-field copy", run, got)
	}
}

func TestAPIBaseURL_ColonPrefixedAddr_PrependsLocalhost(t *testing.T) {
	got := apiBaseURL(":8080")
	want := "http://localhost:8080"
	if got != want {
		t.Errorf("apiBaseURL(%q) = %q, want %q", ":8080", got, want)
	}
}

func TestAPIBaseURL_HostPortAddr_PrependsHTTP(t *testing.T) {
	got := apiBaseURL("127.0.0.1:9000")
	want := "http://127.0.0.1:9000"
	if got != want {
		t.Errorf("apiBaseURL(%q) = %q, want %q", "127.0.0.1:9000", got, want)
	}
}
