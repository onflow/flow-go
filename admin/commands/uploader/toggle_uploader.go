package uploader

import (
	"context"
	"errors"

	"github.com/onflow/flow-go/admin"
	"github.com/onflow/flow-go/admin/commands"
	"github.com/onflow/flow-go/engine/execution/computation"
	"github.com/onflow/flow-go/engine/execution/ingestion"
)

var _ commands.AdminCommand = (*ToggleUploaderCommand)(nil)

type ToggleUploaderCommand struct{}

func (t *ToggleUploaderCommand) Handler(ctx context.Context, req *admin.CommandRequest) (interface{}, error) {
	enabled := req.ValidatorData.(bool)
	// TODO(tony.zhang): unify these duplicated flags
	computation.SetUploaderEnabled(enabled)
	ingestion.SetUploaderEnabled(enabled)
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
