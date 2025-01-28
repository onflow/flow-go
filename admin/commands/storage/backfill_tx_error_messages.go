package storage

import (
	"context"
	"fmt"

	"github.com/onflow/flow-go/admin"
	"github.com/onflow/flow-go/admin/commands"
	"github.com/onflow/flow-go/engine/access/index"
	"github.com/onflow/flow-go/engine/access/rpc/backend"
	commonrpc "github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"

	execproto "github.com/onflow/flow/protobuf/go/flow/execution"
)

var _ commands.AdminCommand = (*BackfillTxErrorMessagesCommand)(nil)

// backfillTxErrorMessagesRequest represents the input parameters for
// backfilling transaction error messages.
type backfillTxErrorMessagesRequest struct {
	startHeight      uint64            // Start height from which to begin backfilling.
	endHeight        uint64            // End height up to which backfilling is performed.
	executionNodeIds flow.IdentityList // List of execution node IDs to be used for backfilling.
}

// BackfillTxErrorMessagesCommand executes a command to backfill
// transaction error messages by fetching them from execution nodes.
type BackfillTxErrorMessagesCommand struct {
	state           protocol.State
	txResultsIndex  *index.TransactionResultsIndex
	txErrorMessages storage.TransactionResultErrorMessages
	backend         *backend.Backend
}

// NewBackfillTxErrorMessagesCommand creates a new instance of BackfillTxErrorMessagesCommand
func NewBackfillTxErrorMessagesCommand(
	state protocol.State,
	txResultsIndex *index.TransactionResultsIndex,
	txErrorMessages storage.TransactionResultErrorMessages,
	backend *backend.Backend,
) commands.AdminCommand {
	return &BackfillTxErrorMessagesCommand{
		state:           state,
		txResultsIndex:  txResultsIndex,
		txErrorMessages: txErrorMessages,
		backend:         backend,
	}
}

// Validator validates the input for the backfill command. The input is validated
// for field types, boundaries, and coherence of start and end heights.
//
// Expected errors during normal operation:
//   - admin.InvalidAdminReqError - if start-height is greater than end-height or
//     if the input format is invalid, if an invalid execution node ID is provided.
func (b *BackfillTxErrorMessagesCommand) Validator(request *admin.CommandRequest) error {
	input, ok := request.Data.(map[string]interface{})
	if !ok {
		return admin.NewInvalidAdminReqFormatError("expected map[string]any")
	}

	data := &backfillTxErrorMessagesRequest{}

	rootHeight := b.state.Params().SealedRoot().Height
	data.startHeight = rootHeight // Default value

	if startHeightIn, ok := input["start-height"]; ok {
		if startHeight, err := parseN(startHeightIn); err != nil {
			return admin.NewInvalidAdminReqErrorf("invalid 'start-height' field: %w", err)
		} else if startHeight > rootHeight {
			data.startHeight = startHeight
		}
	}

	sealed, err := b.state.Sealed().Head()
	if err != nil {
		return fmt.Errorf("failed to lookup sealed header: %w", err)
	}
	data.endHeight = sealed.Height // Default value

	if endHeightIn, ok := input["end-height"]; ok {
		if endHeight, err := parseN(endHeightIn); err != nil {
			return admin.NewInvalidAdminReqErrorf("invalid 'end-height' field: %w", err)
		} else if endHeight < sealed.Height {
			data.endHeight = endHeight
		}
	}

	if data.endHeight < data.startHeight {
		return admin.NewInvalidAdminReqErrorf("start-height %d should not be smaller than end-height %d", data.startHeight, data.endHeight)
	}

	identities, err := b.state.Final().Identities(filter.HasRole[flow.Identity](flow.RoleExecution))
	if err != nil {
		return fmt.Errorf("failed to retreive execution IDs: %w", err)
	}

	if executionNodeIdsIn, ok := input["execution-node-ids"]; ok {
		executionNodeIds, err := b.parseExecutionNodeIds(executionNodeIdsIn, identities)
		if err != nil {
			return err
		}
		data.executionNodeIds = executionNodeIds
	} else {
		// in case no execution node ids provided, the command will use any valid execution node
		data.executionNodeIds = identities
	}

	request.ValidatorData = data

	return nil
}

// Handler performs the backfilling operation by fetching missing transaction
// error messages for blocks within the specified height range. Uses execution nodes
// from data.executionNodeIds if available, otherwise defaults to valid execution nodes.
//
// No errors are expected during normal operation.
func (b *BackfillTxErrorMessagesCommand) Handler(ctx context.Context, request *admin.CommandRequest) (interface{}, error) {
	if b.txErrorMessages == nil {
		return nil, fmt.Errorf("failed to backfill, could not get transaction error messages storage")
	}

	data := request.ValidatorData.(*backfillTxErrorMessagesRequest)

	for height := data.startHeight; height <= data.endHeight; height++ {
		header, err := b.state.AtHeight(height).Head()
		if err != nil {
			return nil, fmt.Errorf("failed to get block header: %w", err)
		}

		blockID := header.ID()

		exists, err := b.txErrorMessages.Exists(blockID)
		if err != nil {
			return nil, fmt.Errorf("could not check existance of transaction result error messages: %w", err)
		}

		if exists {
			continue
		}

		results, err := b.txResultsIndex.ByBlockID(blockID, height)
		if err != nil {
			return nil, fmt.Errorf("failed to get result by block ID: %w", err)
		}

		fetchTxErrorMessages := false
		for _, txResult := range results {
			if txResult.Failed {
				fetchTxErrorMessages = true
			}
		}

		if !fetchTxErrorMessages {
			continue
		}

		req := &execproto.GetTransactionErrorMessagesByBlockIDRequest{
			BlockId: convert.IdentifierToMessage(blockID),
		}

		resp, execNode, err := b.backend.GetTransactionErrorMessagesFromAnyEN(ctx, data.executionNodeIds.ToSkeleton(), req)
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve transaction error messages for block id %#v: %w", blockID, err)
		}

		err = b.storeTransactionResultErrorMessages(blockID, resp, execNode)
		if err != nil {
			return nil, fmt.Errorf("could not store error messages: %w", err)
		}
	}

	return nil, nil
}

// parseExecutionNodeIds converts a list of node IDs from input to flow.IdentityList.
// Returns an error if the IDs are invalid or empty.
//
// Expected errors during normal operation:
// - admin.InvalidAdminReqParameterError - if execution-node-ids is empty or has an invalid format.
func (b *BackfillTxErrorMessagesCommand) parseExecutionNodeIds(executionNodeIdsIn interface{}, allIdentities flow.IdentityList) (flow.IdentityList, error) {
	var ids flow.IdentityList

	switch executionNodeIds := executionNodeIdsIn.(type) {
	case []string:
		if len(executionNodeIds) == 0 {
			return nil, admin.NewInvalidAdminReqParameterError("execution-node-ids", "must be a non empty list of string", executionNodeIdsIn)
		}
		requestedENIdentifiers, err := commonrpc.IdentifierList(executionNodeIds)
		if err != nil {
			return nil, admin.NewInvalidAdminReqParameterError("execution-node-ids", err.Error(), executionNodeIdsIn)
		}

		for _, en := range requestedENIdentifiers {
			id, exists := allIdentities.ByNodeID(en)
			if !exists {
				return nil, admin.NewInvalidAdminReqParameterError("execution-node-ids", "could not found execution nodes by provided ids", executionNodeIdsIn)
			}
			ids = append(ids, id)
		}
	default:
		return nil, admin.NewInvalidAdminReqParameterError("execution-node-ids", "must be a list of string", executionNodeIdsIn)
	}

	return ids, nil
}

// storeTransactionResultErrorMessages saves retrieved error messages for a given block ID.
//
// No errors are expected during normal operation.
func (b *BackfillTxErrorMessagesCommand) storeTransactionResultErrorMessages(
	blockID flow.Identifier,
	errorMessagesResponses []*execproto.GetTransactionErrorMessagesResponse_Result,
	execNode *flow.IdentitySkeleton,
) error {
	errorMessages := make([]flow.TransactionResultErrorMessage, 0, len(errorMessagesResponses))
	for _, value := range errorMessagesResponses {
		errorMessage := flow.TransactionResultErrorMessage{
			ErrorMessage:  value.ErrorMessage,
			TransactionID: convert.MessageToIdentifier(value.TransactionId),
			Index:         value.Index,
			ExecutorID:    execNode.NodeID,
		}
		errorMessages = append(errorMessages, errorMessage)
	}

	err := b.txErrorMessages.Store(blockID, errorMessages)
	if err != nil {
		return fmt.Errorf("failed to store transaction error messages: %w", err)
	}

	return nil
}
