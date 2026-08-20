package mcp

import (
	"context"
	"encoding/json"
)

// Tool describes one MCP tool as advertised by tools/list.
type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

func tools() []Tool {
	return []Tool{
		{
			Name:        "trigger_backup",
			Description: "Trigger a new media inventory backup run.",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
		{
			Name:        "get_backup_run",
			Description: "Get the status of a previously triggered backup run.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"id": map[string]any{
						"type":        "string",
						"description": "the run id returned by trigger_backup",
					},
				},
				"required": []string{"id"},
			},
		},
	}
}

type toolsListResult struct {
	Tools []Tool `json:"tools"`
}

func (s *Server) handleToolsList(req request) response {
	return newResult(req.ID, toolsListResult{Tools: tools()})
}

type contentItem struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type callToolResult struct {
	Content []contentItem `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

type toolCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// handleToolsCall dispatches a tools/call request. An unknown tool or a
// missing argument is a JSON-RPC protocol error, while a TriggerRun/GetRun
// failure comes back as an isError result so the calling model can react to it.
func (s *Server) handleToolsCall(ctx context.Context, req request) response {
	var params toolCallParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return newError(req.ID, CodeInvalidParams, "invalid params: "+err.Error())
	}

	switch params.Name {
	case "trigger_backup":
		return s.callTriggerBackup(ctx, req.ID)
	case "get_backup_run":
		return s.callGetBackupRun(ctx, req.ID, params.Arguments)
	default:
		return newError(req.ID, CodeInvalidParams, "unknown tool: "+params.Name)
	}
}

func (s *Server) callTriggerBackup(ctx context.Context, id json.RawMessage) response {
	run, err := s.TriggerRun(ctx)
	if err != nil {
		return newResult(id, errorResult(err.Error()))
	}
	return newResult(id, textResult(run))
}

type getBackupRunArgs struct {
	ID string `json:"id"`
}

func (s *Server) callGetBackupRun(ctx context.Context, id json.RawMessage, rawArgs json.RawMessage) response {
	var args getBackupRunArgs
	if len(rawArgs) > 0 {
		if err := json.Unmarshal(rawArgs, &args); err != nil {
			return newError(id, CodeInvalidParams, "invalid arguments: "+err.Error())
		}
	}
	if args.ID == "" {
		return newError(id, CodeInvalidParams, "missing required argument: id")
	}

	run, err := s.GetRun(ctx, args.ID)
	if err != nil {
		return newResult(id, errorResult(err.Error()))
	}
	return newResult(id, textResult(run))
}

func textResult(run RunInfo) callToolResult {
	b, err := json.Marshal(run)
	if err != nil {
		return errorResult(err.Error())
	}
	return callToolResult{Content: []contentItem{{Type: "text", Text: string(b)}}}
}

func errorResult(message string) callToolResult {
	return callToolResult{
		Content: []contentItem{{Type: "text", Text: message}},
		IsError: true,
	}
}
