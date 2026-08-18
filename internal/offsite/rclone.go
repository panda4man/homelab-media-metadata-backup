// Package offsite syncs the local snapshot directory to an off-site
// destination via rclone, run as a subprocess - never as a Go SDK
// dependency, so the destination provider can change without touching
// application code.
package offsite

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
)

// ErrRcloneMissing means the rclone binary could not be found on PATH.
var ErrRcloneMissing = errors.New("backup: rclone binary not found")

// captureLimit bounds how much of rclone's stdout/stderr is held in
// memory, so a runaway or chatty process can't exhaust it.
const captureLimit = 64 * 1024

// RunResult is what a subprocess run produced.
type RunResult struct {
	Stdout   []byte
	Stderr   []byte
	ExitCode int
}

// Runner abstracts subprocess execution so Syncer is testable without a
// real rclone binary.
type Runner interface {
	Run(ctx context.Context, name string, args ...string) (RunResult, error)
}

// ExecRunner is the real Runner, backed by os/exec.
type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, name string, args ...string) (RunResult, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var stdout, stderr boundedBuffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	exitCode := -1
	if cmd.ProcessState != nil {
		exitCode = cmd.ProcessState.ExitCode()
	}
	return RunResult{Stdout: stdout.Bytes(), Stderr: stderr.Bytes(), ExitCode: exitCode}, runErr
}

// boundedBuffer is an io.Writer that keeps only the first captureLimit
// bytes written to it, appending a truncation marker if more arrives. It
// always reports the full write as successful so it never causes the
// underlying command to fail on a "short write".
type boundedBuffer struct {
	buf bytes.Buffer
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	remaining := captureLimit - b.buf.Len()
	if remaining > 0 {
		if len(p) > remaining {
			b.buf.Write(p[:remaining])
			b.buf.WriteString("...(truncated)")
		} else {
			b.buf.Write(p)
		}
	}
	return len(p), nil
}

func (b *boundedBuffer) Bytes() []byte { return b.buf.Bytes() }

// Result is the outcome of a Sync call.
type Result struct {
	// Skipped is true when off-site sync is disabled (no remote
	// configured) - not an error, just nothing to do.
	Skipped bool
	Success bool
}

// Syncer copies the local snapshot directory to Remote via rclone.
type Syncer struct {
	Runner Runner
	Remote string
	Logger *slog.Logger
}

// Sync runs `rclone copy <localDir> <remote> --checksum`. An empty Remote
// disables the step entirely: Sync returns Result{Skipped: true} without
// invoking Runner at all. Any failure - missing binary, non-zero exit,
// or a context deadline - is returned as a non-fatal-to-the-snapshot
// backup failure; the caller decides what that means for the run.
func (s Syncer) Sync(ctx context.Context, localDir string) (Result, error) {
	if s.Remote == "" {
		return Result{Skipped: true}, nil
	}

	logger := s.logger()
	args := []string{"copy", localDir, s.Remote, "--checksum"}
	logger.Info("rclone sync starting", "remote", s.Remote, "local_dir", localDir)

	res, err := s.runner().Run(ctx, "rclone", args...)
	if len(res.Stdout) > 0 {
		logger.Info("rclone stdout", "output", string(res.Stdout))
	}
	if len(res.Stderr) > 0 {
		logger.Warn("rclone stderr", "output", string(res.Stderr))
	}

	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return Result{Success: false}, fmt.Errorf("%w: %w", ErrRcloneMissing, err)
		}
		return Result{Success: false}, fmt.Errorf("backup: rclone sync failed: %w", err)
	}
	if res.ExitCode != 0 {
		return Result{Success: false}, fmt.Errorf("backup: rclone exited %d: %s", res.ExitCode, truncateForError(string(res.Stderr)))
	}

	logger.Info("rclone sync completed")
	return Result{Success: true}, nil
}

func (s Syncer) logger() *slog.Logger {
	if s.Logger != nil {
		return s.Logger
	}
	return slog.Default()
}

func (s Syncer) runner() Runner {
	if s.Runner != nil {
		return s.Runner
	}
	return ExecRunner{}
}

const errMessageLimit = 500

func truncateForError(s string) string {
	if len(s) <= errMessageLimit {
		return s
	}
	return s[:errMessageLimit] + "...(truncated)"
}
