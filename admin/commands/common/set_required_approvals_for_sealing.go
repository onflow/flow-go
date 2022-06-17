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
	setter module.RequiredApprovalsForSealConstructionInstanceSetter
}

func NewSetRequiredApprovalsForSealingCommand(setter module.RequiredApprovalsForSealConstructionInstanceSetter) *SetRequiredApprovalsForSealingCommand {
	return &SetRequiredApprovalsForSealingCommand{
		setter: setter,
	}
}

func (s *SetRequiredApprovalsForSealingCommand) Handler(ctx context.Context, req *admin.CommandRequest) (interface{}, error) {
	val := req.ValidatorData.(uint)

	// SetValue will validate and only set it if the value is valid.
	oldVal, err := s.setter.SetValue(val)
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

	// since the validation is stateful, so we rely on the SetValue to validate the value.
	// the response will include the error if the value is invalid.
	req.ValidatorData = val

	return nil
}
