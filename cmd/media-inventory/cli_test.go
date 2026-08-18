package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRealMain_NoSubcommand_PrintsUsageAndExits2(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := realMain(nil, &stdout, &stderr)

	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "usage") {
		t.Fatalf("stderr = %q, want it to contain usage", stderr.String())
	}
}

func TestRealMain_UnknownSubcommand_PrintsUsageAndExits2(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := realMain([]string{"bogus"}, &stdout, &stderr)

	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "usage") {
		t.Fatalf("stderr = %q, want it to contain usage", stderr.String())
	}
}

func TestRealMain_Version_PrintsVersionAndExits0(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := realMain([]string{"version"}, &stdout, &stderr)

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

	code := realMain([]string{"run"}, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("exit code = %d, want 1, stderr = %q", code, stderr.String())
	}
	if stderr.Len() == 0 {
		t.Fatal("stderr = empty, want an error message")
	}
}
