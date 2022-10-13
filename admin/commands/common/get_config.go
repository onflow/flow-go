package common

import (
	"context"
	"errors"
	"fmt"

	"github.com/onflow/flow-go/admin"
	"github.com/onflow/flow-go/admin/commands"
	"github.com/onflow/flow-go/module/updatable_configs"
)

var _ commands.AdminCommand = (*GetConfigCommand)(nil)

type GetConfigCommand struct {
	configs *updatable_configs.Manager
}

func NewGetConfigCommand(configs *updatable_configs.Manager) *GetConfigCommand {
	return &GetConfigCommand{
		configs: configs,
	}
}

// validatedGetConfigData represents a validated get-config request,
// and includes the requested config field.
type validatedGetConfigData struct {
	field updatable_configs.Field
}

func (s *GetConfigCommand) Handler(_ context.Context, req *admin.CommandRequest) (interface{}, error) {
	validatedReq := req.ValidatorData.(validatedGetConfigData)
	curValue := validatedReq.field.Get()
	return curValue, nil
}

func (s *GetConfigCommand) Validator(req *admin.CommandRequest) error {
	configName, ok := req.Data.(string)
	if !ok {
		return errors.New("the data field must be a string")
	}

	field, ok := s.configs.GetField(configName)
	if !ok {
		return fmt.Errorf("unknown config field: %s", configName)
	}

	// we have found a corresponding updatable config field, set it in the ValidatorData
	// field - we will read it in the Handler
	req.ValidatorData = validatedGetConfigData{
		field: field,
	}

	return nil
}
