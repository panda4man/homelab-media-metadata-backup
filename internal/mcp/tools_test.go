package mcp

import (
	"bytes"
	"context"
	"errors"
	"testing"
)

func TestServer_ToolsList_ReturnsBothTools(t *testing.T) {
	var out bytes.Buffer
	s := newTestServer(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`+"\n", &out)

	if err := s.Serve(context.Background()); err != nil {
		t.Fatalf("Serve() error = %v", err)
	}

	lines := decodeLines(t, &out)
	result := lines[0]["result"].(map[string]any)
	toolList := result["tools"].([]any)
	if len(toolList) != 2 {
		t.Fatalf("got %d tools, want 2", len(toolList))
	}
	names := map[string]bool{}
	for _, raw := range toolList {
		tool := raw.(map[string]any)
		names[tool["name"].(string)] = true
	}
	if !names["trigger_backup"] || !names["get_backup_run"] {
		t.Errorf("tool names = %v, want trigger_backup and get_backup_run", names)
	}
}

func TestServer_ToolsCall_TriggerBackup_Success_ReturnsRunAsTextContent(t *testing.T) {
	var out bytes.Buffer
	s := newTestServer(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"trigger_backup","arguments":{}}}`+"\n", &out)
	s.TriggerRun = func(context.Context) (RunInfo, error) {
		return RunInfo{ID: "abc123", Status: "running"}, nil
	}

	if err := s.Serve(context.Background()); err != nil {
		t.Fatalf("Serve() error = %v", err)
	}

	lines := decodeLines(t, &out)
	result, ok := lines[0]["result"].(map[string]any)
	if !ok {
		t.Fatalf("response = %v, want a result object", lines[0])
	}
	if isErr, _ := result["isError"].(bool); isErr {
		t.Fatalf("result.isError = true, want false: %v", result)
	}
	content := result["content"].([]any)[0].(map[string]any)
	if content["type"] != "text" {
		t.Errorf("content.type = %v, want text", content["type"])
	}
	text, _ := content["text"].(string)
	if !bytes.Contains([]byte(text), []byte("abc123")) {
		t.Errorf("content.text = %q, want it to contain the run id", text)
	}
}

func TestServer_ToolsCall_TriggerBackup_Error_ReturnsIsErrorResult(t *testing.T) {
	var out bytes.Buffer
	s := newTestServer(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"trigger_backup","arguments":{}}}`+"\n", &out)
	s.TriggerRun = func(context.Context) (RunInfo, error) {
		return RunInfo{}, errors.New("apiclient: already running")
	}

	if err := s.Serve(context.Background()); err != nil {
		t.Fatalf("Serve() error = %v", err)
	}

	lines := decodeLines(t, &out)
	if _, hasProtocolError := lines[0]["error"]; hasProtocolError {
		t.Fatalf("response = %v, want a successful JSON-RPC response with isError:true, not a protocol error", lines[0])
	}
	result := lines[0]["result"].(map[string]any)
	if isErr, _ := result["isError"].(bool); !isErr {
		t.Errorf("result.isError = %v, want true", result["isError"])
	}
	content := result["content"].([]any)[0].(map[string]any)
	text, _ := content["text"].(string)
	if !bytes.Contains([]byte(text), []byte("already running")) {
		t.Errorf("content.text = %q, want it to describe the error", text)
	}
}

func TestServer_ToolsCall_GetBackupRun_Success_ReturnsRunAsTextContent(t *testing.T) {
	var out bytes.Buffer
	s := newTestServer(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"get_backup_run","arguments":{"id":"abc123"}}}`+"\n", &out)
	var gotID string
	s.GetRun = func(_ context.Context, id string) (RunInfo, error) {
		gotID = id
		return RunInfo{ID: id, Status: "completed", State: "valid"}, nil
	}

	if err := s.Serve(context.Background()); err != nil {
		t.Fatalf("Serve() error = %v", err)
	}

	if gotID != "abc123" {
		t.Errorf("GetRun called with id = %q, want abc123", gotID)
	}
	lines := decodeLines(t, &out)
	result := lines[0]["result"].(map[string]any)
	content := result["content"].([]any)[0].(map[string]any)
	text, _ := content["text"].(string)
	if !bytes.Contains([]byte(text), []byte("completed")) {
		t.Errorf("content.text = %q, want it to contain the run status", text)
	}
}

func TestServer_ToolsCall_GetBackupRun_MissingID_ReturnsInvalidParamsProtocolError(t *testing.T) {
	var out bytes.Buffer
	s := newTestServer(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"get_backup_run","arguments":{}}}`+"\n", &out)

	if err := s.Serve(context.Background()); err != nil {
		t.Fatalf("Serve() error = %v", err)
	}

	lines := decodeLines(t, &out)
	errObj, ok := lines[0]["error"].(map[string]any)
	if !ok {
		t.Fatalf("response = %v, want a protocol-level error (missing required argument)", lines[0])
	}
	if code, _ := errObj["code"].(float64); code != CodeInvalidParams {
		t.Errorf("error.code = %v, want %d", errObj["code"], CodeInvalidParams)
	}
}

func TestServer_ToolsCall_UnknownTool_ReturnsInvalidParamsProtocolError(t *testing.T) {
	var out bytes.Buffer
	s := newTestServer(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"bogus_tool","arguments":{}}}`+"\n", &out)

	if err := s.Serve(context.Background()); err != nil {
		t.Fatalf("Serve() error = %v", err)
	}

	lines := decodeLines(t, &out)
	errObj, ok := lines[0]["error"].(map[string]any)
	if !ok {
		t.Fatalf("response = %v, want a protocol-level error", lines[0])
	}
	if code, _ := errObj["code"].(float64); code != CodeInvalidParams {
		t.Errorf("error.code = %v, want %d", errObj["code"], CodeInvalidParams)
	}
}
