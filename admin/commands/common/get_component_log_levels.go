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

// Handler returns a snapshot of all registered component log levels and the global default.
//
// No error returns are expected during normal operation.
func (g *GetComponentLogLevelsCommand) Handler(_ context.Context, _ *admin.CommandRequest) (interface{}, error) {
	defaultLevel, levels := g.registry.Levels()

	components := make(map[string]interface{}, len(levels))
	for id, cl := range levels {
		components[id] = map[string]string{
			"level":  zerolog.Level(cl.Level).String(),
			"source": string(cl.Source),
		}
	}

	return map[string]interface{}{
		"default":    defaultLevel.String(),
		"components": components,
	}, nil
}
