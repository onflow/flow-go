package common

import (
	"context"

	"github.com/onflow/flow-go/admin"
	"github.com/onflow/flow-go/admin/commands"
	"github.com/onflow/flow-go/module/updatable_configs"
)

var _ commands.AdminCommand = (*ListConfigCommand)(nil)

// ListConfigCommand is an admin command which lists all config fields which may
// by dynamically modified via admin command.
type ListConfigCommand struct {
	configs *updatable_configs.Manager
}

func NewListConfigCommand(configs *updatable_configs.Manager) *ListConfigCommand {
	return &ListConfigCommand{
		configs: configs,
	}
}

func (s *ListConfigCommand) Handler(_ context.Context, _ *admin.CommandRequest) (interface{}, error) {
	fields := s.configs.AllFields()

	// create a response
	res := make(map[string]any, len(fields))
	for _, field := range fields {
		res[field.Name] = map[string]any{
			"type": field.TypeName,
		}
	}
	return res, nil
}

func (s *ListConfigCommand) Validator(req *admin.CommandRequest) error {
	return nil
}
