package common

import (
	"context"
	"errors"
	"fmt"

	"github.com/onflow/flow-go/admin"
	"github.com/onflow/flow-go/admin/commands"
	"github.com/onflow/flow-go/utils/debug"
)

var _ commands.AdminCommand = (*SetProfilerEnabledCommand)(nil)

type SetProfilerEnabledCommand struct{}

func (s *SetProfilerEnabledCommand) Handler(ctx context.Context, req *admin.CommandRequest) (interface{}, error) {
	enabled := req.ValidatorData.(bool)
	debug.SetProfilerEnabled(enabled)
	return "ok", nil
}

func (s *SetProfilerEnabledCommand) Validator(req *admin.CommandRequest) error {
	enabled, ok := req.Data.(string)
	if !ok {
		return errors.New("the input must be true or false")
	}
	if enabled == "true" {
		req.ValidatorData = true
	} else if enabled == "false" {
		req.ValidatorData = false
	} else {
		return fmt.Errorf("unknown value, the input must be true or false")
	}

	return nil
}
