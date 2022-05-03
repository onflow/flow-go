package state_synchronization

import (
	"context"
	"encoding/hex"
	"errors"
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
	executionDataGetter execution_data.ExecutionDataGetter
}

func (r *ReadExecutionDataCommand) Handler(ctx context.Context, req *admin.CommandRequest) (interface{}, error) {
	data := req.ValidatorData.(*requestData)

	ed, err := r.executionDataGetter.GetExecutionData(ctx, data.rootID)

	if err != nil {
		return nil, fmt.Errorf("failed to get execution data: %w", err)
	}

	return commands.ConvertToMap(ed)
}

func (r *ReadExecutionDataCommand) Validator(req *admin.CommandRequest) error {
	input, ok := req.Data.(map[string]interface{})
	if !ok {
		return errors.New("wrong input format")
	}

	id, ok := input["execution_data_id"]
	if !ok {
		return errors.New("the \"execution_data_id\" field is required")
	}

	errInvalidIDValue := fmt.Errorf("invalid value for \"execution_data_id\": %v", id)
	data := &requestData{}

	idStr, ok := id.(string)

	if !ok {
		return errInvalidIDValue
	}

	if len(idStr) == 2*flow.IdentifierLen {
		b, err := hex.DecodeString(idStr)
		if err != nil {
			return errInvalidIDValue
		}
		data.rootID = flow.HashToID(b)
	} else {
		return errInvalidIDValue
	}

	req.ValidatorData = data

	return nil
}

func NewReadExecutionDataCommand(getter execution_data.ExecutionDataGetter) commands.AdminCommand {
	return &ReadExecutionDataCommand{
		getter,
	}
}
