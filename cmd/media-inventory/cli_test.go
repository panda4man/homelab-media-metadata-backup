package main

import (
	"bytes"
	"strings"
	"testing"
)

func noEnv(string) string { return "" }

func validConfigEnv() map[string]string {
	return map[string]string{
		"MEDIA_MOVIES_PATH": "/media/movies",
		"MEDIA_TV_PATH":     "/media/tv",
		"RADARR_URL":        "http://radarr:7878",
		"RADARR_API_KEY":    "radarr-secret",
		"SONARR_URL":        "http://sonarr:8989",
		"SONARR_API_KEY":    "sonarr-secret",
	}
}

// runnableConfigEnv is validConfigEnv plus a real, writable SNAPSHOT_PATH
// (required even for a run that fails fast, since the lock file lives
// there) and Radarr/Sonarr URLs pointed at a port nothing listens on, so
// "unreachable" fails instantly via connection-refused rather than a slow
// DNS lookup or timeout.
func runnableConfigEnv(t *testing.T) map[string]string {
	t.Helper()
	env := validConfigEnv()
	env["SNAPSHOT_PATH"] = t.TempDir()
	env["RADARR_URL"] = "http://127.0.0.1:1"
	env["SONARR_URL"] = "http://127.0.0.1:1"
	return env
}

func getenvFrom(m map[string]string) func(string) string {
	return func(key string) string { return m[key] }
}

func TestRealMain_NoSubcommand_PrintsUsageAndExits2(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := realMain(nil, strings.NewReader(""), &stdout, &stderr, noEnv)

	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "usage") {
		t.Fatalf("stderr = %q, want it to contain usage", stderr.String())
	}
}

func TestRealMain_UnknownSubcommand_PrintsUsageAndExits2(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := realMain([]string{"bogus"}, strings.NewReader(""), &stdout, &stderr, noEnv)

	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "usage") {
		t.Fatalf("stderr = %q, want it to contain usage", stderr.String())
	}
}

func TestRealMain_Version_PrintsVersionAndExits0(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := realMain([]string{"version"}, strings.NewReader(""), &stdout, &stderr, noEnv)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0, stderr = %q", code, stderr.String())
	}
	if strings.TrimSpace(stdout.String()) != version {
		t.Fatalf("stdout = %q, want %q", stdout.String(), version)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRealMain_Run_UnreachableServices_ExitsFailed(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := realMain([]string{"run"}, strings.NewReader(""), &stdout, &stderr, getenvFrom(runnableConfigEnv(t)))

	if code != 1 {
		t.Fatalf("exit code = %d, want 1 (failed state - Radarr/Sonarr unreachable), stderr = %q", code, stderr.String())
	}
}

func TestRealMain_Run_MissingConfig_ExitsUsageError(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := realMain([]string{"run"}, strings.NewReader(""), &stdout, &stderr, noEnv)

	if code != 2 {
		t.Fatalf("exit code = %d, want 2, stderr = %q", code, stderr.String())
	}
	if stderr.Len() == 0 {
		t.Fatal("stderr = empty, want a config error message")
	}
}

func TestRealMain_Config_ValidEnv_PrintsResolvedConfigAndExits0(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := realMain([]string{"config"}, strings.NewReader(""), &stdout, &stderr, getenvFrom(validConfigEnv()))

	if code != 0 {
		t.Fatalf("exit code = %d, want 0, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "MEDIA_MOVIES_PATH=/media/movies") {
		t.Errorf("stdout = %q, want it to contain resolved MEDIA_MOVIES_PATH", stdout.String())
	}
	if strings.Contains(stdout.String(), "radarr-secret") || strings.Contains(stdout.String(), "sonarr-secret") {
		t.Errorf("stdout = %q, contains unredacted secret", stdout.String())
	}
}

func TestRealMain_Config_MissingRequiredVars_ExitsUsageError(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := realMain([]string{"config"}, strings.NewReader(""), &stdout, &stderr, noEnv)

	if code != 2 {
		t.Fatalf("exit code = %d, want 2, stdout = %q", code, stdout.String())
	}
	if !strings.Contains(stderr.String(), "MEDIA_MOVIES_PATH") {
		t.Errorf("stderr = %q, want it to name the missing var", stderr.String())
	}
}

func TestRealMain_Serve_MissingAPIToken_ExitsUsageError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	env := validConfigEnv()

	code := realMain([]string{"serve"}, strings.NewReader(""), &stdout, &stderr, getenvFrom(env))

	if code != 2 {
		t.Fatalf("exit code = %d, want 2, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "API_TOKEN") {
		t.Errorf("stderr = %q, want it to name API_TOKEN", stderr.String())
	}
}

func TestRealMain_Serve_ShortAPIToken_ExitsUsageError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	env := validConfigEnv()
	env["API_TOKEN"] = "short"

	code := realMain([]string{"serve"}, strings.NewReader(""), &stdout, &stderr, getenvFrom(env))

	if code != 2 {
		t.Fatalf("exit code = %d, want 2, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "API_TOKEN") {
		t.Errorf("stderr = %q, want it to name API_TOKEN", stderr.String())
	}
}

func TestRealMain_NoSubcommand_UsageListsServe(t *testing.T) {
	var stdout, stderr bytes.Buffer

	realMain(nil, strings.NewReader(""), &stdout, &stderr, noEnv)

	if !strings.Contains(stderr.String(), "serve") {
		t.Errorf("stderr = %q, want usage to list serve", stderr.String())
	}
}

func TestRealMain_NoSubcommand_UsageListsMcp(t *testing.T) {
	var stdout, stderr bytes.Buffer

	realMain(nil, strings.NewReader(""), &stdout, &stderr, noEnv)

	if !strings.Contains(stderr.String(), "mcp") {
		t.Errorf("stderr = %q, want usage to list mcp", stderr.String())
	}
}

func TestRealMain_Mcp_ShortAPIToken_ExitsUsageError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	env := map[string]string{
		"API_URL":   "http://backup-host:8080",
		"API_TOKEN": "short",
	}

	code := realMain([]string{"mcp"}, strings.NewReader(""), &stdout, &stderr, getenvFrom(env))

	if code != 2 {
		t.Fatalf("exit code = %d, want 2, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "API_TOKEN") {
		t.Errorf("stderr = %q, want it to name API_TOKEN", stderr.String())
	}
	if !strings.Contains(stderr.String(), "API_TOKEN") {
		t.Errorf("stderr = %q, want it to name API_TOKEN", stderr.String())
	}
}
