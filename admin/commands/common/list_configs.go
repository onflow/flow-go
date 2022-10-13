package common

import (
	"context"
	"fmt"
	"strings"

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

// Handler returns a human-readable list string of available updatable config fields.
func (s *ListConfigCommand) Handler(_ context.Context, _ *admin.CommandRequest) (interface{}, error) {
	fields := s.configs.AllFields()

	res := strings.Builder{}
	res.WriteString("Configurable Fields:\n")
	for _, field := range fields {
		res.WriteString(fmt.Sprintf("- %s (%s)\n", field.Name, field.TypeName))
	}
	return res.String(), nil
}

func (s *ListConfigCommand) Validator(_ *admin.CommandRequest) error {
	return nil
}
