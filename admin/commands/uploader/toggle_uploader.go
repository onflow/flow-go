package uploader

import (
	"context"
	"errors"

	"github.com/onflow/flow-go/admin"
	"github.com/onflow/flow-go/admin/commands"
	"github.com/onflow/flow-go/engine/execution/computation"
)

var _ commands.AdminCommand = (*ToggleUploaderCommand)(nil)

type ToggleUploaderCommand struct{}

func (t *ToggleUploaderCommand) Handler(ctx context.Context, req *admin.CommandRequest) (interface{}, error) {
	enabled := req.ValidatorData.(bool)
	computation.SetUploaderEnabled(enabled)
	return "ok", nil
}

func (t *ToggleUploaderCommand) Validator(req *admin.CommandRequest) error {
	enabled, ok := req.Data.(bool)
	if !ok {
		return errors.New("the input must be a boolean")
	}

	req.ValidatorData = enabled
	return nil
}

func NewToggleUploaderCommand() commands.AdminCommand {
	return &ToggleUploaderCommand{}
}
