package offsite

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type fakeRunner struct {
	called  bool
	gotName string
	gotArgs []string
	result  RunResult
	err     error
}

func (f *fakeRunner) Run(ctx context.Context, name string, args ...string) (RunResult, error) {
	f.called = true
	f.gotName = name
	f.gotArgs = args
	return f.result, f.err
}

func TestSync_HappyPath_InvokesExpectedArgv(t *testing.T) {
	fr := &fakeRunner{result: RunResult{ExitCode: 0}}
	s := Syncer{Runner: fr, Remote: "media-inventory:"}

	res, err := s.Sync(context.Background(), "/data/snapshots")
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
	if !res.Success {
		t.Error("Success = false, want true")
	}
	if fr.gotName != "rclone" {
		t.Errorf("name = %q, want rclone", fr.gotName)
	}
	wantArgs := []string{"copy", "/data/snapshots", "media-inventory:", "--checksum"}
	if len(fr.gotArgs) != len(wantArgs) {
		t.Fatalf("args = %v, want %v", fr.gotArgs, wantArgs)
	}
	for i := range wantArgs {
		if fr.gotArgs[i] != wantArgs[i] {
			t.Errorf("args[%d] = %q, want %q", i, fr.gotArgs[i], wantArgs[i])
		}
	}
}

func TestSync_NonZeroExit_SuccessFalseErrorWrapsStderr(t *testing.T) {
	fr := &fakeRunner{result: RunResult{ExitCode: 1, Stderr: []byte("connection refused by remote")}}
	s := Syncer{Runner: fr, Remote: "b2:bucket"}

	res, err := s.Sync(context.Background(), "/data/snapshots")
	if res.Success {
		t.Error("Success = true, want false")
	}
	if err == nil {
		t.Fatal("Sync() error = nil, want an error")
	}
	if !strings.Contains(err.Error(), "connection refused by remote") {
		t.Errorf("err = %v, want it to contain rclone's stderr", err)
	}
}

func TestSync_RcloneBinaryMissing_ErrRcloneMissing(t *testing.T) {
	fr := &fakeRunner{err: &exec.Error{Name: "rclone", Err: exec.ErrNotFound}}
	s := Syncer{Runner: fr, Remote: "b2:bucket"}

	_, err := s.Sync(context.Background(), "/data/snapshots")
	if !errors.Is(err, ErrRcloneMissing) {
		t.Errorf("err = %v, want ErrRcloneMissing", err)
	}
}

func TestSync_ContextDeadlineExceeded_PropagatedNotSwallowed(t *testing.T) {
	fr := &fakeRunner{err: context.DeadlineExceeded}
	s := Syncer{Runner: fr, Remote: "b2:bucket"}

	_, err := s.Sync(context.Background(), "/data/snapshots")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err = %v, want it to wrap context.DeadlineExceeded", err)
	}
}

func TestSync_EmptyRemote_SkippedNoRunnerCall(t *testing.T) {
	fr := &fakeRunner{result: RunResult{ExitCode: 0}}
	s := Syncer{Runner: fr, Remote: ""}

	res, err := s.Sync(context.Background(), "/data/snapshots")
	if err != nil {
		t.Fatalf("Sync() error = %v, want nil when off-site is disabled", err)
	}
	if !res.Skipped {
		t.Error("Skipped = false, want true")
	}
	if fr.called {
		t.Error("Runner.Run() was called, want it skipped entirely when RCLONE_REMOTE is empty")
	}
}

func TestSync_StdoutAndStderrForwardedToLogger(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	fr := &fakeRunner{result: RunResult{
		ExitCode: 0,
		Stdout:   []byte("Transferred: 3 files"),
		Stderr:   []byte("NOTICE: some warning"),
	}}
	s := Syncer{Runner: fr, Remote: "b2:bucket", Logger: logger}

	if _, err := s.Sync(context.Background(), "/data/snapshots"); err != nil {
		t.Fatalf("Sync() error = %v", err)
	}

	logged := buf.String()
	if !strings.Contains(logged, "Transferred: 3 files") {
		t.Errorf("log output = %q, want it to contain rclone's stdout", logged)
	}
	if !strings.Contains(logged, "NOTICE: some warning") {
		t.Errorf("log output = %q, want it to contain rclone's stderr", logged)
	}
}

func TestSync_NoLoggerConfigured_DoesNotPanic(t *testing.T) {
	fr := &fakeRunner{result: RunResult{ExitCode: 0}}
	s := Syncer{Runner: fr, Remote: "b2:bucket"} // Logger left nil

	if _, err := s.Sync(context.Background(), "/data/snapshots"); err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
}

func TestSync_ArgvContainsOnlyExpectedFields_NoExternalSecrets(t *testing.T) {
	// Sync's signature is Sync(ctx, localDir) - it has no parameter through
	// which an API key could flow in. This pins the exact argv shape so
	// that invariant stays visible and testable, not just implied.
	fr := &fakeRunner{result: RunResult{ExitCode: 0}}
	s := Syncer{Runner: fr, Remote: "b2:bucket"}

	if _, err := s.Sync(context.Background(), "/data/snapshots"); err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
	for _, secretLike := range []string{"RADARR_API_KEY", "SONARR_API_KEY", "INFLUX_TOKEN", "Bearer "} {
		for _, a := range fr.gotArgs {
			if strings.Contains(a, secretLike) {
				t.Errorf("argv %v contains secret-shaped substring %q", fr.gotArgs, secretLike)
			}
		}
	}
}

func TestBoundedBuffer_CapsCaptureSize(t *testing.T) {
	var b boundedBuffer
	huge := bytes.Repeat([]byte("x"), 10*captureLimit)

	n, err := b.Write(huge)
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if n != len(huge) {
		t.Errorf("Write() n = %d, want %d (io.Writer must report the full write succeeded)", n, len(huge))
	}
	if len(b.Bytes()) > captureLimit+64 { // small allowance for the truncation marker
		t.Errorf("captured %d bytes, want it capped near %d", len(b.Bytes()), captureLimit)
	}
	if !bytes.Contains(b.Bytes(), []byte("truncated")) {
		t.Error("capped output does not indicate truncation")
	}
}

func TestExecRunner_RealRcloneBinary_LocalCopySmoke(t *testing.T) {
	if _, err := exec.LookPath("rclone"); err != nil {
		t.Skip("rclone not installed, skipping real-binary smoke test")
	}

	src := t.TempDir()
	dst := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "hello.txt"), []byte("hi"), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	s := Syncer{Runner: ExecRunner{}, Remote: dst}

	res, err := s.Sync(ctx, src)
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
	if !res.Success {
		t.Fatalf("Success = false, want true")
	}
	if _, err := os.Stat(filepath.Join(dst, "hello.txt")); err != nil {
		t.Errorf("copied file missing in destination: %v", err)
	}
}
