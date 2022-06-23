package common

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/onflow/flow-go/admin"
	"github.com/onflow/flow-go/admin/commands"
	"github.com/onflow/flow-go/module"
)

var _ commands.AdminCommand = (*GetRequiredApprovalsForSealingCommand)(nil)

type GetRequiredApprovalsForSealingCommand struct {
	getter module.SealingConfigsGetter
}

func NewGetRequiredApprovalsForSealingCommand(getter module.SealingConfigsGetter) *GetRequiredApprovalsForSealingCommand {
	return &GetRequiredApprovalsForSealingCommand{
		getter: getter,
	}
}

func (s *GetRequiredApprovalsForSealingCommand) Handler(ctx context.Context, req *admin.CommandRequest) (interface{}, error) {
	val := s.getter.RequireApprovalsForSealConstructionDynamicValue()

	log.Info().Msgf("admintool: required approvals for sealing is %v", val)

	return fmt.Sprintf("%v", val), nil
}

func (s *GetRequiredApprovalsForSealingCommand) Validator(req *admin.CommandRequest) error {
	return nil
}
