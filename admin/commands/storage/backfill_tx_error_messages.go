package storage

import (
	"context"
	"fmt"
	"strings"

	"github.com/onflow/flow-go/admin"
	"github.com/onflow/flow-go/admin/commands"
	commonrpc "github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

const (
	ANY = "any"
)

var _ commands.AdminCommand = (*BackfillTxErrorMessagesCommand)(nil)

type backfillTxErrorMessagesRequest struct {
	startHeight      uint64
	endHeight        uint64
	executionNodeIds flow.IdentifierList
}

type BackfillTxErrorMessagesCommand struct {
	state           protocol.State
	txResults       storage.TransactionResults
	txErrorMessages storage.TransactionResultErrorMessages
}

func NewBackfillTxErrorMessagesCommand(
	state protocol.State,
	txResults storage.TransactionResults,
	txErrorMessages storage.TransactionResultErrorMessages,
) commands.AdminCommand {
	return &BackfillTxErrorMessagesCommand{
		state:           state,
		txResults:       txResults,
		txErrorMessages: txErrorMessages,
	}
}

// Validator validates the request.
// Returns admin.InvalidAdminReqError for invalid/malformed requests.
func (b *BackfillTxErrorMessagesCommand) Validator(request *admin.CommandRequest) error {
	input, ok := request.Data.(map[string]interface{})
	if !ok {
		return admin.NewInvalidAdminReqFormatError("expected map[string]any")
	}

	data := &backfillTxErrorMessagesRequest{}

	if startHeight, ok := input["start-height"]; ok {
		if startHeight, err := parseN(startHeight); err != nil {
			return admin.NewInvalidAdminReqErrorf("invalid 'startHeight' field: %w", err)
		} else {
			data.startHeight = startHeight
		}
	} else {
		data.startHeight = b.state.Params().SealedRoot().Height
	}

	if endHeight, ok := input["end-height"]; ok {
		if endHeight, err := parseN(endHeight); err != nil {
			return admin.NewInvalidAdminReqErrorf("invalid 'endHeight' field: %w", err)
		} else {
			data.endHeight = endHeight
		}
	} else {
		header, err := b.state.Sealed().Head()
		if err != nil {
			return fmt.Errorf("failed to lookup sealed header: %w", err)
		}

		data.endHeight = header.Height
	}

	if data.endHeight < data.startHeight {
		return admin.NewInvalidAdminReqErrorf("end-height %v should not be smaller than start-height %v", data.endHeight, data.startHeight)
	}

	if inExecutionNodeIds, ok := input["execution-node-ids"]; ok {
		switch executionNodeIds := inExecutionNodeIds.(type) {
		case string:
			executionNodeIds = strings.ToLower(strings.TrimSpace(executionNodeIds))
			if executionNodeIds == ANY {
				data.executionNodeIds = flow.IdentifierList{}
			} else {
				return admin.NewInvalidAdminReqErrorf("invalid 'execution-node-ids' field: should be 'any' or list of ids")
			}
		case []string:
			ids, err := commonrpc.IdentifierList(executionNodeIds)
			if err != nil {
				return admin.NewInvalidAdminReqErrorf("invalid 'execution-node-ids' field: %w", err)
			}
			data.executionNodeIds = ids
		default:
			return admin.NewInvalidAdminReqParameterError("execution-node-ids", "must be string or number", inExecutionNodeIds)
		}
	} else {
		return fmt.Errorf("'execution-node-ids' field should be provided")
	}

	request.ValidatorData = data

	return nil
}

func (b *BackfillTxErrorMessagesCommand) Handler(ctx context.Context, req *admin.CommandRequest) (interface{}, error) {
	return nil, nil
}
