package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/pprof"

	"github.com/rs/zerolog"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/module/updatable_configs"
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
	configManager     *updatable_configs.Manager
	commands          []string
}

// requireLoopback returns an http.HandlerFunc that only serves requests originating
// from the loopback interface. It is used to prevent sensitive profiling endpoints
// from being exposed when the admin server is bound to a publicly reachable address.
func requireLoopback(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		ip := net.ParseIP(host)
		if ip == nil || !ip.IsLoopback() {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next(w, r)
	}
}

// newAdminHandler creates a new admin HTTP handler.
func newAdminHandler(logger zerolog.Logger, triggerCheckpoint *atomic.Bool, configManager *updatable_configs.Manager) http.Handler {
	h := &adminHandler{
		logger:            logger.With().Str("component", "admin").Logger(),
		triggerCheckpoint: triggerCheckpoint,
		configManager:     configManager,
		commands:          []string{"ping", "list-commands", "trigger-checkpoint", "get-config", "set-config"},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/admin/run_command", h.handleCommand)

	// Register pprof handlers for profiling (CPU, heap, goroutine, etc.).
	// These endpoints are restricted to loopback to avoid exposing profiling
	// data when the admin server is bound to a public address.
	mux.HandleFunc("/debug/pprof/", requireLoopback(pprof.Index))
	mux.HandleFunc("/debug/pprof/cmdline", requireLoopback(pprof.Cmdline))
	mux.HandleFunc("/debug/pprof/profile", requireLoopback(pprof.Profile))
	mux.HandleFunc("/debug/pprof/symbol", requireLoopback(pprof.Symbol))
	mux.HandleFunc("/debug/pprof/trace", requireLoopback(pprof.Trace))

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

	case "get-config":
		var configName string
		if err := json.Unmarshal(req.Data, &configName); err != nil {
			h.writeError(w, http.StatusBadRequest, fmt.Sprintf("get-config data must be a string config name: %v", err))
			return
		}
		field, ok := h.configManager.GetField(configName)
		if !ok {
			h.writeError(w, http.StatusBadRequest, fmt.Sprintf("unknown config field: %s", configName))
			return
		}
		result = field.Get()

	case "set-config":
		var data map[string]any
		if err := json.Unmarshal(req.Data, &data); err != nil {
			h.writeError(w, http.StatusBadRequest, fmt.Sprintf("set-config data must be a JSON object: %v", err))
			return
		}
		if len(data) != 1 {
			h.writeError(w, http.StatusBadRequest, fmt.Sprintf("set-config data must have exactly one entry, got %d", len(data)))
			return
		}
		var configName string
		var configValue any
		for k, v := range data {
			configName = k
			configValue = v
		}
		field, ok := h.configManager.GetField(configName)
		if !ok {
			h.writeError(w, http.StatusBadRequest, fmt.Sprintf("unknown config field: %s", configName))
			return
		}
		oldValue := field.Get()
		if err := field.Set(configValue); err != nil {
			status := http.StatusInternalServerError
			if updatable_configs.IsValidationError(err) {
				status = http.StatusBadRequest
			}
			h.writeError(w, status, fmt.Sprintf("failed to set config %s: %v", configName, err))
			return
		}
		result = map[string]any{
			"oldValue": oldValue,
			"newValue": configValue,
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
