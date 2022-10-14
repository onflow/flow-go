package common

import (
	"context"
	"fmt"

	"github.com/onflow/flow-go/admin"
	"github.com/onflow/flow-go/admin/commands"
	"github.com/onflow/flow-go/module/updatable_configs"
)

var _ commands.AdminCommand = (*SetConfigCommand)(nil)

// SetConfigCommand is an admin command which enables setting any config field which
// has registered as dynamically updatable with the config Manager.
type SetConfigCommand struct {
	configs *updatable_configs.Manager
}

func NewSetConfigCommand(configs *updatable_configs.Manager) *SetConfigCommand {
	return &SetConfigCommand{
		configs: configs,
	}
}

// validatedSetConfigData represents a set-config admin request which has passed basic validation.
// It contains the config field to update, and the config value.
type validatedSetConfigData struct {
	field updatable_configs.Field
	value any
}

func (s *SetConfigCommand) Handler(_ context.Context, req *admin.CommandRequest) (interface{}, error) {
	validatedReq := req.ValidatorData.(validatedSetConfigData)

	oldValue := validatedReq.field.Get()

	err := validatedReq.field.Set(validatedReq.value)
	if err != nil {
		if updatable_configs.IsValidationError(err) {
			return nil, fmt.Errorf("config update failed due to invalid input: %w", err)
		}
		return nil, fmt.Errorf("unexpected error setting config field %s: %w", validatedReq.field.Name, err)
	}

	res := map[string]any{
		"oldValue": oldValue,
		"newValue": validatedReq.value,
	}

	return res, nil
}

// Validator validates the request.
// Returns admin.InvalidAdminReqError for invalid/malformed requests.
func (s *SetConfigCommand) Validator(req *admin.CommandRequest) error {
	mval, ok := req.Data.(map[string]any)
	if !ok {
		return admin.NewInvalidAdminReqFormatError("expected map[string]any")
	}

	if len(mval) != 1 {
		return admin.NewInvalidAdminReqErrorf("data field must have 1 entry, got %d", len(mval))
	}

	var (
		configName  string
		configValue any
	)
	// select the single name/value pair from the map
	for k, v := range mval {
		configName = k
		configValue = v
		break
	}

	field, ok := s.configs.GetField(configName)
	if !ok {
		return admin.NewInvalidAdminReqErrorf("unknown config field: %s", configName)
	}

	// we have found a corresponding updatable config field, set it in the ValidatorData
	// field - we will attempt to apply the change in Handler
	req.ValidatorData = validatedSetConfigData{
		field: field,
		value: configValue,
	}
	return nil
}
