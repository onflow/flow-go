package common

import (
	"context"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/admin"
	"github.com/onflow/flow-go/admin/commands"
	"github.com/onflow/flow-go/utils/logging"
)

var _ commands.AdminCommand = (*GetComponentLogLevelsCommand)(nil)

// GetComponentLogLevelsCommand returns the current log level for every registered component
// and the global default.
type GetComponentLogLevelsCommand struct {
	registry *logging.LogRegistry
}

// NewGetComponentLogLevelsCommand constructs a GetComponentLogLevelsCommand.
func NewGetComponentLogLevelsCommand(registry *logging.LogRegistry) *GetComponentLogLevelsCommand {
	return &GetComponentLogLevelsCommand{registry: registry}
}

// Validator performs no validation — this command takes no input.
//
// No error returns are expected during normal operation.
func (g *GetComponentLogLevelsCommand) Validator(_ *admin.CommandRequest) error {
	return nil
}

// Handler returns a snapshot of all registered component log levels and the registry config.
//
// No error returns are expected during normal operation.
func (g *GetComponentLogLevelsCommand) Handler(_ context.Context, _ *admin.CommandRequest) (interface{}, error) {
	_, levels := g.registry.Levels()
	cfg := g.registry.Config()

	components := make(map[string]interface{}, len(levels))
	for id, cl := range levels {
		components[id] = map[string]string{
			"level":  zerolog.Level(cl.Level).String(),
			"source": string(cl.Source),
		}
	}

	staticOverrides := make(map[string]string, len(cfg.StaticOverrides))
	for pattern, level := range cfg.StaticOverrides {
		staticOverrides[pattern] = level.String()
	}

	dynamicOverrides := make(map[string]string, len(cfg.DynamicOverrides))
	for pattern, level := range cfg.DynamicOverrides {
		dynamicOverrides[pattern] = level.String()
	}

	return map[string]interface{}{
		"config": map[string]interface{}{
			"default":           cfg.Default.String(),
			"static_overrides":  staticOverrides,
			"dynamic_overrides": dynamicOverrides,
		},
		"components": components,
	}, nil
}
