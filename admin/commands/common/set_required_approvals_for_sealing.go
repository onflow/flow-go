package common

import (
	"context"
	"errors"

	"github.com/rs/zerolog/log"

	"github.com/onflow/flow-go/admin"
	"github.com/onflow/flow-go/admin/commands"
	"github.com/onflow/flow-go/module/updatable_configs"
)

var _ commands.AdminCommand = (*SetRequiredApprovalsForSealingCommand)(nil)

type SetRequiredApprovalsForSealingCommand struct{}

func (s *SetRequiredApprovalsForSealingCommand) Handler(ctx context.Context, req *admin.CommandRequest) (interface{}, error) {
	val := req.ValidatorData.(uint)

	oldVal := updatable_configs.AcquireRequiredApprovalsForSealConstructionSetter().SetValue(val)

	log.Info().Msgf("admintool: required approvals for sealing is changed from %v to %v", oldVal, val)

	return "ok", nil
}

func (s *SetRequiredApprovalsForSealingCommand) Validator(req *admin.CommandRequest) error {
	val, ok := req.Data.(uint)
	if !ok {
		return errors.New("the data field must be a uint")
	}

	req.ValidatorData = val

	return nil
}
