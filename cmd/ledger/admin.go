package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/rs/zerolog"
	"go.uber.org/atomic"
)

// adminRequest represents the JSON request body for admin commands.
// This matches the format used by the execution node's admin framework.
type adminRequest struct {
	CommandName string          `json:"commandName"`
	Data        json.RawMessage `json:"data,omitempty"`
}

// adminResponse represents the JSON response for admin commands.
// This matches the format used by the execution node's admin framework.
type adminResponse struct {
	Output any    `json:"output,omitempty"`
	Error  string `json:"error,omitempty"`
}

// adminHandler is a simple HTTP-only admin server for the ledger service.
// Unlike the execution node's admin framework (which uses gRPC + HTTP gateway),
// this directly handles HTTP requests without the gRPC proxy layer.
type adminHandler struct {
	logger            zerolog.Logger
	triggerCheckpoint *atomic.Bool
	commands          []string
}

// newAdminHandler creates a new admin HTTP handler.
func newAdminHandler(logger zerolog.Logger, triggerCheckpoint *atomic.Bool) http.Handler {
	h := &adminHandler{
		logger:            logger.With().Str("component", "admin").Logger(),
		triggerCheckpoint: triggerCheckpoint,
		commands:          []string{"ping", "list-commands", "trigger-checkpoint"},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/admin/run_command", h.handleCommand)
	return mux
}

func (h *adminHandler) handleCommand(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		h.writeError(w, http.StatusMethodNotAllowed, "method not allowed, use POST")
		return
	}

	var req adminRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid JSON: %v", err))
		return
	}

	h.logger.Info().Str("command", req.CommandName).Msg("received admin command")

	var result any

	switch req.CommandName {
	case "ping":
		result = "pong"

	case "list-commands":
		result = h.commands

	case "trigger-checkpoint":
		if h.triggerCheckpoint.CompareAndSwap(false, true) {
			h.logger.Info().Msg("trigger checkpoint as soon as finishing writing the current segment file")
			result = "ok"
		} else {
			result = "checkpoint already triggered"
		}

	default:
		h.writeError(w, http.StatusBadRequest, fmt.Sprintf("unknown command: %s", req.CommandName))
		return
	}

	h.writeSuccess(w, result)
}

func (h *adminHandler) writeError(w http.ResponseWriter, status int, msg string) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(adminResponse{Error: msg})
}

func (h *adminHandler) writeSuccess(w http.ResponseWriter, output any) {
	_ = json.NewEncoder(w).Encode(adminResponse{Output: output})
}
