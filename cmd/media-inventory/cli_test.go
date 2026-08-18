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

func getenvFrom(m map[string]string) func(string) string {
	return func(key string) string { return m[key] }
}

func TestRealMain_NoSubcommand_PrintsUsageAndExits2(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := realMain(nil, &stdout, &stderr, noEnv)

	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "usage") {
		t.Fatalf("stderr = %q, want it to contain usage", stderr.String())
	}
}

func TestRealMain_UnknownSubcommand_PrintsUsageAndExits2(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := realMain([]string{"bogus"}, &stdout, &stderr, noEnv)

	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "usage") {
		t.Fatalf("stderr = %q, want it to contain usage", stderr.String())
	}
}

func TestRealMain_Version_PrintsVersionAndExits0(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := realMain([]string{"version"}, &stdout, &stderr, noEnv)

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

func TestRealMain_Run_NotYetImplemented_Exits1(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := realMain([]string{"run"}, &stdout, &stderr, getenvFrom(validConfigEnv()))

	if code != 1 {
		t.Fatalf("exit code = %d, want 1, stderr = %q", code, stderr.String())
	}
	if stderr.Len() == 0 {
		t.Fatal("stderr = empty, want an error message")
	}
}

func TestRealMain_Config_ValidEnv_PrintsResolvedConfigAndExits0(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := realMain([]string{"config"}, &stdout, &stderr, getenvFrom(validConfigEnv()))

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

	code := realMain([]string{"config"}, &stdout, &stderr, noEnv)

	if code != 2 {
		t.Fatalf("exit code = %d, want 2, stdout = %q", code, stdout.String())
	}
	if !strings.Contains(stderr.String(), "MEDIA_MOVIES_PATH") {
		t.Errorf("stderr = %q, want it to name the missing var", stderr.String())
	}
}
