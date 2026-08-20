// Package httpapi implements the on-demand backup trigger HTTP API.
package httpapi

import (
	"encoding/json"
	"net/http"
)

// ErrorResponse is the JSON body returned for any non-2xx response.
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
