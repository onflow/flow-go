package common

import (
	"context"
	"fmt"
	"strings"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/admin"
	"github.com/onflow/flow-go/admin/commands"
	"github.com/onflow/flow-go/utils/logging"
)

const (
	maxPatternLength = 1024
)

var _ commands.AdminCommand = (*SetComponentLogLevelCommand)(nil)

// SetComponentLogLevelCommand sets the log level for one or more components identified by
// exact or wildcard patterns. Input is a JSON object mapping pattern to level string.
//
// Example input:
//
//	{"hotstuff.voter": "debug", "network.*": "warn"}
type SetComponentLogLevelCommand struct {
	registry *logging.LogRegistry
}

// NewSetComponentLogLevelCommand constructs a SetComponentLogLevelCommand.
func NewSetComponentLogLevelCommand(registry *logging.LogRegistry) *SetComponentLogLevelCommand {
	return &SetComponentLogLevelCommand{registry: registry}
}

type parsedComponentLevel struct {
	pattern string
	level   zerolog.Level
}

// Validator validates that the input is a non-empty map of pattern → level string with
// recognisable level values.
//
// Returns [admin.InvalidAdminReqError] for invalid or malformed requests.
func (s *SetComponentLogLevelCommand) Validator(req *admin.CommandRequest) error {
	raw, ok := req.Data.(map[string]interface{})
	if !ok {
		return admin.NewInvalidAdminReqFormatError("input must be a JSON object mapping component pattern to level string")
	}
	if len(raw) == 0 {
		return admin.NewInvalidAdminReqFormatError("input must not be empty")
	}

	parsed := make([]parsedComponentLevel, 0, len(raw))
	for pattern, val := range raw {
		levelStr, ok := val.(string)
		if !ok {
			return admin.NewInvalidAdminReqErrorf("level for %q must be a string", pattern)
		}
		level, err := zerolog.ParseLevel(levelStr)
		if err != nil {
			return admin.NewInvalidAdminReqErrorf("invalid level %q for component %q: %w", levelStr, pattern, err)
		}
		if len(pattern) > maxPatternLength {
			return admin.NewInvalidAdminReqErrorf("pattern %q is too long (max %d characters)", pattern, maxPatternLength)
		}
		pattern = logging.NormalizePattern(strings.TrimSpace(pattern))
		if pattern == "*" {
			return admin.NewInvalidAdminReqErrorf("global wildcard \"*\" is not a valid when setting component level logging. use set-log-level instead")
		}
		if err := logging.ValidatePattern(logging.NormalizePattern(pattern)); err != nil {
			return admin.NewInvalidAdminReqErrorf("invalid pattern %q: %w", pattern, err)
		}

		parsed = append(parsed, parsedComponentLevel{pattern: pattern, level: level})
	}

	req.ValidatorData = parsed
	return nil
}

// Handler applies the validated component level overrides and returns the updated patterns.
//
// No error returns are expected during normal operation.
func (s *SetComponentLogLevelCommand) Handler(_ context.Context, req *admin.CommandRequest) (interface{}, error) {
	entries := req.ValidatorData.([]parsedComponentLevel)

	result := make(map[string]string, len(entries))
	for _, e := range entries {
		if err := s.registry.SetLevel(e.pattern, e.level); err != nil {
			return nil, fmt.Errorf("failed to set level for pattern %q: %w", e.pattern, err)
		}
		result[e.pattern] = fmt.Sprintf("set to %s", e.level)
	}
	return result, nil
}
