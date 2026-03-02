package common

import (
	"context"

	"github.com/onflow/flow-go/admin"
	"github.com/onflow/flow-go/admin/commands"
	"github.com/onflow/flow-go/utils/logging"
)

var _ commands.AdminCommand = (*ResetComponentLogLevelCommand)(nil)

// ResetComponentLogLevelCommand removes runtime log level overrides for components matching
// the specified patterns, restoring them to static config or global default.
//
// Input is a JSON array of patterns. ["*"] resets all registered components.
// "*" may not be mixed with other patterns.
//
// Example input:
//
//	["hotstuff.voter", "hotstuff.*"]
//	["*"]
type ResetComponentLogLevelCommand struct {
	registry *logging.LogRegistry
}

// NewResetComponentLogLevelCommand constructs a ResetComponentLogLevelCommand.
func NewResetComponentLogLevelCommand(registry *logging.LogRegistry) *ResetComponentLogLevelCommand {
	return &ResetComponentLogLevelCommand{registry: registry}
}

// Validator validates that the input is a non-empty array of pattern strings. "*" must be
// the sole element if present.
//
// Returns [admin.InvalidAdminReqError] for invalid or malformed requests.
func (r *ResetComponentLogLevelCommand) Validator(req *admin.CommandRequest) error {
	raw, ok := req.Data.([]interface{})
	if !ok {
		return admin.NewInvalidAdminReqFormatError("input must be a JSON array of pattern strings")
	}
	if len(raw) == 0 {
		return admin.NewInvalidAdminReqFormatError("input must not be empty")
	}

	patterns := make([]string, 0, len(raw))
	for _, v := range raw {
		s, ok := v.(string)
		if !ok {
			return admin.NewInvalidAdminReqFormatError("each element must be a string")
		}
		patterns = append(patterns, s)
	}

	for _, p := range patterns {
		if p == "*" && len(patterns) > 1 {
			return admin.NewInvalidAdminReqErrorf("\"*\" must be the only element when resetting all components")
		}
	}

	req.ValidatorData = patterns
	return nil
}

// Handler removes the specified runtime overrides and returns "ok".
//
// No error returns are expected during normal operation.
func (r *ResetComponentLogLevelCommand) Handler(_ context.Context, req *admin.CommandRequest) (interface{}, error) {
	patterns := req.ValidatorData.([]string)
	r.registry.Reset(patterns)
	return "ok", nil
}
