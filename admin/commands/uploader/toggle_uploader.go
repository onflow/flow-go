package uploader

import (
	"context"

	"github.com/onflow/flow-go/admin"
	"github.com/onflow/flow-go/admin/commands"
	"github.com/onflow/flow-go/engine/execution/ingestion/uploader"
)

var _ commands.AdminCommand = (*ToggleUploaderCommand)(nil)

type ToggleUploaderCommand struct {
	uploadManager *uploader.Manager
}

func (t *ToggleUploaderCommand) Handler(ctx context.Context, req *admin.CommandRequest) (interface{}, error) {
	enabled := req.ValidatorData.(bool)
	t.uploadManager.SetEnabled(enabled)
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

func NewToggleUploaderCommand(uploadManager *uploader.Manager) commands.AdminCommand {
	return &ToggleUploaderCommand{
		uploadManager: uploadManager,
	}
}
