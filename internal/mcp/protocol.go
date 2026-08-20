// Package mcp implements the server side of the Model Context Protocol
// stdio transport: newline-delimited JSON-RPC 2.0 over an io.Reader/io.Writer
// pair, with tool definitions for triggering and inspecting backup runs.
package mcp

import (
	"encoding/json"
	"io"
)

const jsonRPCVersion = "2.0"

// Standard JSON-RPC 2.0 error codes (https://www.jsonrpc.org/specification#error_object).
const (
	CodeParseError     = -32700
	CodeInvalidParams  = -32602
	CodeMethodNotFound = -32601
)

// request is one JSON-RPC 2.0 request or notification read from the wire.
// A missing ID marks a notification, which never gets a response.
type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// response is one JSON-RPC 2.0 response: exactly one of Result or Error is set.
type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func newResult(id json.RawMessage, result any) response {
	return response{JSONRPC: jsonRPCVersion, ID: id, Result: result}
}

func newError(id json.RawMessage, code int, message string) response {
	return response{JSONRPC: jsonRPCVersion, ID: id, Error: &rpcError{Code: code, Message: message}}
}

// encodeLine writes v to w as a single line of JSON terminated by "\n", the
// framing the MCP stdio transport expects.
func encodeLine(w io.Writer, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	b = append(b, '\n')
	_, err = w.Write(b)
	return err
}
