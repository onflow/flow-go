package common

import (
	"context"
	"errors"

	"github.com/onflow/flow-go/admin"
	"github.com/onflow/flow-go/admin/commands"
	"github.com/onflow/flow-go/module/profiler"
)

var _ commands.AdminCommand = (*SetProfilerEnabledCommand)(nil)

type SetProfilerEnabledCommand struct{}

func (s *SetProfilerEnabledCommand) Handler(ctx context.Context, req *admin.CommandRequest) (interface{}, error) {
	enabled := req.ValidatorData.(bool)
	profiler.SetProfilerEnabled(enabled)
	return "ok", nil
}

func (s *SetProfilerEnabledCommand) Validator(req *admin.CommandRequest) error {
	enabled, ok := req.Data.(bool)
	if !ok {
		return errors.New("the data field must be a bool, either true or false")
	}

	req.ValidatorData = enabled

	return nil
}
