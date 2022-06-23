package common

import (
	"context"
	"errors"
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/onflow/flow-go/admin"
	"github.com/onflow/flow-go/admin/commands"
	"github.com/onflow/flow-go/module"
)

var _ commands.AdminCommand = (*SetRequiredApprovalsForSealingCommand)(nil)

type SetRequiredApprovalsForSealingCommand struct {
	setter module.SealingConfigsSetter
}

func NewSetRequiredApprovalsForSealingCommand(setter module.SealingConfigsSetter) *SetRequiredApprovalsForSealingCommand {
	return &SetRequiredApprovalsForSealingCommand{
		setter: setter,
	}
}

func (s *SetRequiredApprovalsForSealingCommand) Handler(ctx context.Context, req *admin.CommandRequest) (interface{}, error) {
	val := req.ValidatorData.(uint)

	// SetValue will validate and only set it if the value is valid.
	oldVal, err := s.setter.SetRequiredApprovalsForSealingConstruction(val)
	if err != nil {
		return "fail", fmt.Errorf("fail to set required approvals for sealing %v: %w", val, err)
	}

	log.Info().Msgf("admintool: required approvals for sealing is changed from %v to %v", oldVal, val)

	return "ok", nil
}

func (s *SetRequiredApprovalsForSealingCommand) Validator(req *admin.CommandRequest) error {
	value, ok := req.Data.(float64)
	if !ok {
		return errors.New("the data field must be a uint")
	}

	val := uint(value)

	if value != float64(val) {
		return fmt.Errorf("required approvals for sealing must be whole integer value (%v != %v)",
			value, val)
	}

	// since the validation is stateful, so we rely on the SetValue to validate the value.
	// the response will include the error if the value is invalid.
	req.ValidatorData = val

	return nil
}
