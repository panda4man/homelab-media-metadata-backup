package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
)

// RunInfo is the mcp package's own view of a backup run, decoupled from
// internal/httpapi.Run so this package never depends on internal/httpapi
// (and transitively internal/orchestrator). It reaches a run's status only
// through the injected TriggerRun/GetRun funcs.
type RunInfo struct {
	ID             string `json:"id"`
	Status         string `json:"status"`
	Reason         string `json:"reason,omitempty"`
	State          string `json:"state,omitempty"`
	SnapshotPath   string `json:"snapshot_path,omitempty"`
	OffsiteSuccess bool   `json:"offsite_success,omitempty"`
	Error          string `json:"error,omitempty"`
}

// Server speaks the MCP stdio JSON-RPC protocol, exposing backup-trigger
// tools through the injected TriggerRun/GetRun funcs.
type Server struct {
	In         io.Reader
	Out        io.Writer
	TriggerRun func(context.Context) (RunInfo, error)
	GetRun     func(context.Context, string) (RunInfo, error)
	Logger     *slog.Logger
}

const maxLineBytes = 10 << 20 // 10MiB, generous for a tool call payload

// Serve reads newline-delimited JSON-RPC requests from In until EOF or ctx
// cancellation, dispatching each to the matching handler and writing its
// response (if any) to Out.
func (s *Server) Serve(ctx context.Context) error {
	scanner := bufio.NewScanner(s.In)
	scanner.Buffer(make([]byte, 0, 64*1024), maxLineBytes)

	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return err
		}
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		s.handle(ctx, line)
	}
	return scanner.Err()
}

func (s *Server) handle(ctx context.Context, line []byte) {
	var req request
	if err := json.Unmarshal(line, &req); err != nil {
		s.write(newError(nil, CodeParseError, "parse error: "+err.Error()))
		return
	}

	isNotification := len(req.ID) == 0

	var resp response
	switch req.Method {
	case "initialize":
		resp = s.handleInitialize(req)
	case "notifications/initialized":
		return
	case "ping":
		resp = newResult(req.ID, struct{}{})
	case "tools/list":
		resp = s.handleToolsList(req)
	case "tools/call":
		resp = s.handleToolsCall(ctx, req)
	default:
		resp = newError(req.ID, CodeMethodNotFound, "method not found: "+req.Method)
	}

	if isNotification {
		return
	}
	s.write(resp)
}

func (s *Server) write(resp response) {
	if err := encodeLine(s.Out, resp); err != nil && s.Logger != nil {
		s.Logger.Error("mcp: failed to write response", "error", err)
	}
}

const protocolVersion = "2024-11-05"

type initializeResult struct {
	ProtocolVersion string         `json:"protocolVersion"`
	Capabilities    map[string]any `json:"capabilities"`
	ServerInfo      serverInfo     `json:"serverInfo"`
}

type serverInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

func (s *Server) handleInitialize(req request) response {
	return newResult(req.ID, initializeResult{
		ProtocolVersion: protocolVersion,
		Capabilities:    map[string]any{"tools": map[string]any{}},
		ServerInfo:      serverInfo{Name: "media-inventory", Version: "0.1.0-dev"},
	})
}
