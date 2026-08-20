package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func newTestServer(in string, out *bytes.Buffer) *Server {
	return &Server{
		In:  strings.NewReader(in),
		Out: out,
		TriggerRun: func(context.Context) (RunInfo, error) {
			return RunInfo{}, nil
		},
		GetRun: func(context.Context, string) (RunInfo, error) {
			return RunInfo{}, nil
		},
	}
}

func decodeLines(t *testing.T, out *bytes.Buffer) []map[string]any {
	t.Helper()
	var lines []map[string]any
	sc := bytes.Split(bytes.TrimRight(out.Bytes(), "\n"), []byte("\n"))
	for _, l := range sc {
		if len(l) == 0 {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal(l, &m); err != nil {
			t.Fatalf("decoding line %q: %v", l, err)
		}
		lines = append(lines, m)
	}
	return lines
}

func TestServer_Initialize_ReturnsProtocolVersionAndCapabilities(t *testing.T) {
	var out bytes.Buffer
	s := newTestServer(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`+"\n", &out)

	if err := s.Serve(context.Background()); err != nil {
		t.Fatalf("Serve() error = %v", err)
	}

	lines := decodeLines(t, &out)
	if len(lines) != 1 {
		t.Fatalf("got %d response lines, want 1", len(lines))
	}
	result, ok := lines[0]["result"].(map[string]any)
	if !ok {
		t.Fatalf("response = %v, want a result object", lines[0])
	}
	if result["protocolVersion"] == "" || result["protocolVersion"] == nil {
		t.Errorf("result[protocolVersion] = %v, want non-empty", result["protocolVersion"])
	}
	if _, ok := result["capabilities"]; !ok {
		t.Errorf("result = %v, want a capabilities field", result)
	}
}

func TestServer_NotificationsInitialized_NoResponseWritten(t *testing.T) {
	var out bytes.Buffer
	s := newTestServer(`{"jsonrpc":"2.0","method":"notifications/initialized"}`+"\n", &out)

	if err := s.Serve(context.Background()); err != nil {
		t.Fatalf("Serve() error = %v", err)
	}

	if out.Len() != 0 {
		t.Fatalf("out = %q, want no response written for a notification", out.String())
	}
}

func TestServer_Ping_ReturnsEmptyResult(t *testing.T) {
	var out bytes.Buffer
	s := newTestServer(`{"jsonrpc":"2.0","id":2,"method":"ping"}`+"\n", &out)

	if err := s.Serve(context.Background()); err != nil {
		t.Fatalf("Serve() error = %v", err)
	}

	lines := decodeLines(t, &out)
	if len(lines) != 1 {
		t.Fatalf("got %d response lines, want 1", len(lines))
	}
	if _, ok := lines[0]["result"]; !ok {
		t.Fatalf("response = %v, want a result field", lines[0])
	}
	if _, ok := lines[0]["error"]; ok {
		t.Fatalf("response = %v, want no error field", lines[0])
	}
}

func TestServer_UnknownMethod_ReturnsMethodNotFoundError(t *testing.T) {
	var out bytes.Buffer
	s := newTestServer(`{"jsonrpc":"2.0","id":3,"method":"bogus"}`+"\n", &out)

	if err := s.Serve(context.Background()); err != nil {
		t.Fatalf("Serve() error = %v", err)
	}

	lines := decodeLines(t, &out)
	if len(lines) != 1 {
		t.Fatalf("got %d response lines, want 1", len(lines))
	}
	errObj, ok := lines[0]["error"].(map[string]any)
	if !ok {
		t.Fatalf("response = %v, want an error object", lines[0])
	}
	if code, _ := errObj["code"].(float64); code != CodeMethodNotFound {
		t.Errorf("error.code = %v, want %d", errObj["code"], CodeMethodNotFound)
	}
}

func TestServer_MalformedJSON_ReturnsParseError(t *testing.T) {
	var out bytes.Buffer
	s := newTestServer("{not json"+"\n", &out)

	if err := s.Serve(context.Background()); err != nil {
		t.Fatalf("Serve() error = %v", err)
	}

	lines := decodeLines(t, &out)
	if len(lines) != 1 {
		t.Fatalf("got %d response lines, want 1", len(lines))
	}
	errObj, ok := lines[0]["error"].(map[string]any)
	if !ok {
		t.Fatalf("response = %v, want an error object", lines[0])
	}
	if code, _ := errObj["code"].(float64); code != CodeParseError {
		t.Errorf("error.code = %v, want %d", errObj["code"], CodeParseError)
	}
}

func TestServer_MultipleRequests_EachGetsOwnResponseLine(t *testing.T) {
	var out bytes.Buffer
	in := `{"jsonrpc":"2.0","id":1,"method":"ping"}` + "\n" +
		`{"jsonrpc":"2.0","id":2,"method":"ping"}` + "\n"
	s := newTestServer(in, &out)

	if err := s.Serve(context.Background()); err != nil {
		t.Fatalf("Serve() error = %v", err)
	}

	lines := decodeLines(t, &out)
	if len(lines) != 2 {
		t.Fatalf("got %d response lines, want 2", len(lines))
	}
	if lines[0]["id"] != float64(1) || lines[1]["id"] != float64(2) {
		t.Errorf("response ids = %v, %v, want 1, 2", lines[0]["id"], lines[1]["id"])
	}
}
