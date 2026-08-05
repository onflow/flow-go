package common

import (
	"context"

	"github.com/onflow/flow-go/admin"
	"github.com/onflow/flow-go/admin/commands"
	"github.com/onflow/flow-go/module/updatable_configs"
)

var _ commands.AdminCommand = (*GetConfigCommand)(nil)

// GetConfigCommand is an admin command which retrieves the current value of a
// dynamically updatable config.
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

// Validator validates the request.
// Returns admin.InvalidAdminReqError for invalid/malformed requests.
func (s *GetConfigCommand) Validator(req *admin.CommandRequest) error {
	configName, ok := req.Data.(string)
	if !ok {
		return admin.NewInvalidAdminReqFormatError("the data field must be a string")
	}

	field, ok := s.configs.GetField(configName)
	if !ok {
		return admin.NewInvalidAdminReqErrorf("unknown config field: %s", configName)
	}

	// we have found a corresponding updatable config field, set it in the ValidatorData
	// field - we will read it in the Handler
	req.ValidatorData = validatedGetConfigData{
		field: field,
	}

	return nil
}
