package uploader

import (
	"context"

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

// Validator validates the request.
// Returns admin.InvalidAdminReqError for invalid/malformed requests.
func (t *ToggleUploaderCommand) Validator(req *admin.CommandRequest) error {
	enabled, ok := req.Data.(bool)
	if !ok {
		return admin.NewInvalidAdminReqFormatError("expected bool")
	}

	req.ValidatorData = enabled
	return nil
}

func NewToggleUploaderCommand() commands.AdminCommand {
	return &ToggleUploaderCommand{}
}
