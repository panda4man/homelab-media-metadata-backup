package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/panda4man/homelab-media-metadata-backup/internal/httpapi"
)

// mcpEnv returns the minimal env mcpCommand needs: API_URL and a
// long-enough API_TOKEN. Deliberately excludes every app-config var
// (MEDIA_MOVIES_PATH, RADARR_API_KEY, etc) that config.Load would require —
// mcpCommand must never call config.Load, since it runs on the operator's
// workstation, not the backup host.
func mcpEnv() map[string]string {
	return map[string]string{
		"API_URL":   "http://backup-host:8080",
		"API_TOKEN": "a-token-at-least-16-chars",
	}
}

func TestMcpCommand_MissingAPIURL_ExitsUsageError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	env := mcpEnv()
	delete(env, "API_URL")

	code := mcpCommand(strings.NewReader(""), &stdout, &stderr, getenvFrom(env))

	if code != exitUsage {
		t.Fatalf("exit code = %d, want %d, stderr = %q", code, exitUsage, stderr.String())
	}
	if !strings.Contains(stderr.String(), "API_URL") {
		t.Errorf("stderr = %q, want it to name API_URL", stderr.String())
	}
}

func TestMcpCommand_MissingAPIToken_ExitsUsageError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	env := mcpEnv()
	delete(env, "API_TOKEN")

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
	env := mcpEnv()
	env["API_TOKEN"] = "short"

	code := mcpCommand(strings.NewReader(""), &stdout, &stderr, getenvFrom(env))

	if code != exitUsage {
		t.Fatalf("exit code = %d, want %d, stderr = %q", code, exitUsage, stderr.String())
	}
	if !strings.Contains(stderr.String(), "API_TOKEN") {
		t.Errorf("stderr = %q, want it to name API_TOKEN", stderr.String())
	}
}

// TestMcpCommand_NoAppConfigVarsSet_StillRuns is the regression guard for
// the bug this test replaced: mcpCommand must succeed on an env that
// supplies ONLY API_URL/API_TOKEN, proving config.Load is never invoked on
// this path (it would fail here, demanding MEDIA_MOVIES_PATH and friends,
// none of which exist on the operator's workstation).
func TestMcpCommand_NoAppConfigVarsSet_StillRuns(t *testing.T) {
	var stdout, stderr bytes.Buffer
	env := mcpEnv()

	code := mcpCommand(strings.NewReader(""), &stdout, &stderr, getenvFrom(env))

	if code != exitOK {
		t.Fatalf("exit code = %d, want %d, stderr = %q", code, exitOK, stderr.String())
	}
}

func TestMcpCommand_EmptyStdin_ServesUntilEOFAndExitsOK(t *testing.T) {
	var stdout, stderr bytes.Buffer
	env := mcpEnv()

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
