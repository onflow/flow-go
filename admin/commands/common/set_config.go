package common

import (
	"context"
	"errors"
	"fmt"

	"github.com/onflow/flow-go/admin"
	"github.com/onflow/flow-go/admin/commands"
	"github.com/onflow/flow-go/module/updatable_configs"
)

var _ commands.AdminCommand = (*SetConfigCommand)(nil)

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

type setConfigResponse struct {
	OldValue any
	NewValue any
}

func (s *SetConfigCommand) Handler(_ context.Context, req *admin.CommandRequest) (interface{}, error) {
	validatedReq := req.ValidatorData.(validatedSetConfigData)

	oldValue := validatedReq.field.Get()

	err := validatedReq.field.Set(validatedReq.value)
	if err != nil {
		return nil, fmt.Errorf("could not set new config value for %s: %w", validatedReq.field)
	}

	res := map[string]any{
		"oldValue": oldValue,
		"newValue": validatedReq.value,
	}

	return res, nil
}

func (s *SetConfigCommand) Validator(req *admin.CommandRequest) error {
	mval, ok := req.Data.(map[string]any)
	if !ok {
		return errors.New("the data field must be a map[string]any")
	}

	if len(mval) != 1 {
		return fmt.Errorf("data field must have 1 entry, got %d", len(mval))
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
		return fmt.Errorf("unknown config field: %s", configName)
	}

	// we have found a corresponding updatable config field, set it in the ValidatorData
	// field - we will attempt
	req.ValidatorData = validatedSetConfigData{
		field: field,
		value: configValue,
	}
	return nil
}
