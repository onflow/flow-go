package state_synchronization

import (
	"context"
	"encoding/hex"
	"fmt"

	"github.com/onflow/flow-go/admin"
	"github.com/onflow/flow-go/admin/commands"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
)

var _ commands.AdminCommand = (*ReadExecutionDataCommand)(nil)

type requestData struct {
	rootID flow.Identifier
}

type ReadExecutionDataCommand struct {
	executionDataStore execution_data.ExecutionDataStore
}

func (r *ReadExecutionDataCommand) Handler(ctx context.Context, req *admin.CommandRequest) (interface{}, error) {
	data := req.ValidatorData.(*requestData)

	ed, err := r.executionDataStore.Get(ctx, data.rootID)

	if err != nil {
		return nil, fmt.Errorf("failed to get execution data: %w", err)
	}

	return commands.ConvertToMap(ed)
}

// Validator validates the request.
// Returns admin.InvalidAdminReqError for invalid/malformed requests.
func (r *ReadExecutionDataCommand) Validator(req *admin.CommandRequest) error {
	input, ok := req.Data.(map[string]interface{})
	if !ok {
		return admin.NewInvalidAdminReqFormatError("expected map[string]any")
	}

	id, ok := input["execution_data_id"]
	if !ok {
		return admin.NewInvalidAdminReqErrorf("missing required field 'execution_data_id")
	}

	data := &requestData{}

	idStr, ok := id.(string)
	if !ok {
		return admin.NewInvalidAdminReqParameterError("execution_data_id", "must be a string", id)
	}

	if len(idStr) == 2*flow.IdentifierLen {
		b, err := hex.DecodeString(idStr)
		if err != nil {
			return admin.NewInvalidAdminReqParameterError("execution_data_id", "must be 64-char hex string", id)
		}
		data.rootID = flow.HashToID(b)
	} else {
		return admin.NewInvalidAdminReqParameterError("execution_data_id", "must be 64-char hex string", id)
	}

	req.ValidatorData = data

	return nil
}

func NewReadExecutionDataCommand(store execution_data.ExecutionDataStore) commands.AdminCommand {
	return &ReadExecutionDataCommand{
		store,
	}
}
