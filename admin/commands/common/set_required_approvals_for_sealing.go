package common

import (
	"context"
	"errors"
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/onflow/flow-go/admin"
	"github.com/onflow/flow-go/admin/commands"
	"github.com/onflow/flow-go/module/updatable_configs"
	"github.com/onflow/flow-go/module/updatable_configs/validation"
)

var _ commands.AdminCommand = (*SetRequiredApprovalsForSealingCommand)(nil)

type SetRequiredApprovalsForSealingCommand struct{}

func (s *SetRequiredApprovalsForSealingCommand) Handler(ctx context.Context, req *admin.CommandRequest) (interface{}, error) {
	val := req.ValidatorData.(uint)

	oldVal, err := updatable_configs.AcquireRequiredApprovalsForSealConstructionSetter().SetValue(val)
	if err != nil {
		log.Fatal().Err(err).Msgf("Validator should have validated the data field, but it's still invalid, val: %v", val)
	}

	log.Info().Msgf("admintool: required approvals for sealing is changed from %v to %v", oldVal, val)

	return "ok", nil
}

func (s *SetRequiredApprovalsForSealingCommand) Validator(req *admin.CommandRequest) error {
	val, ok := req.Data.(uint)
	if !ok {
		return errors.New("the data field must be a uint")
	}

	err := validation.ValidateRequireApprovals(val)
	if err != nil {
		return fmt.Errorf("the data field contains invalid value: %w", err)
	}

	req.ValidatorData = val

	return nil
}
