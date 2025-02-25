package storage

import (
	"context"
	"fmt"

	"github.com/onflow/flow-go/admin"
	"github.com/onflow/flow-go/admin/commands"
	"github.com/onflow/flow-go/engine/access/ingestion/tx_error_messages"
	commonrpc "github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/state/protocol"
)

var _ commands.AdminCommand = (*BackfillTxErrorMessagesCommand)(nil)

// backfillTxErrorMessagesRequest represents the input parameters for
// backfilling transaction error messages.
type backfillTxErrorMessagesRequest struct {
	startHeight      uint64                    // Start height from which to begin backfilling.
	endHeight        uint64                    // End height up to which backfilling is performed.
	executionNodeIds flow.IdentitySkeletonList // List of execution node IDs to be used for backfilling.
}

// BackfillTxErrorMessagesCommand executes a command to backfill
// transaction error messages by fetching them from execution nodes.
type BackfillTxErrorMessagesCommand struct {
	state               protocol.State
	txErrorMessagesCore *tx_error_messages.TxErrorMessagesCore
}

// NewBackfillTxErrorMessagesCommand creates a new instance of BackfillTxErrorMessagesCommand
func NewBackfillTxErrorMessagesCommand(
	state protocol.State,
	txErrorMessagesCore *tx_error_messages.TxErrorMessagesCore,
) commands.AdminCommand {
	return &BackfillTxErrorMessagesCommand{
		state:               state,
		txErrorMessagesCore: txErrorMessagesCore,
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

	sealed, err := b.state.Sealed().Head()
	if err != nil {
		return fmt.Errorf("failed to lookup sealed header: %w", err)
	}

	lastSealedHeight := sealed.Height
	if startHeightIn, ok := input["start-height"]; ok {
		startHeight, err := parseN(startHeightIn)
		if err != nil {
			return admin.NewInvalidAdminReqErrorf("invalid 'start-height' field: %w", err)
		}

		if startHeight > lastSealedHeight {
			return admin.NewInvalidAdminReqErrorf(
				"'start-height' %d must not be greater than latest sealed block %d",
				startHeight,
				lastSealedHeight,
			)
		}

		if startHeight < rootHeight {
			return admin.NewInvalidAdminReqErrorf(
				"'start-height' %d must not be less than root block %d",
				startHeight,
				rootHeight,
			)
		}

		data.startHeight = startHeight
	}

	data.endHeight = lastSealedHeight // Default value
	if endHeightIn, ok := input["end-height"]; ok {
		endHeight, err := parseN(endHeightIn)
		if err != nil {
			return admin.NewInvalidAdminReqErrorf("invalid 'end-height' field: %w", err)
		}

		if endHeight > lastSealedHeight {
			return admin.NewInvalidAdminReqErrorf(
				"'end-height' %d must not be greater than latest sealed block %d",
				endHeight,
				lastSealedHeight,
			)
		}

		data.endHeight = endHeight
	}

	if data.endHeight < data.startHeight {
		return admin.NewInvalidAdminReqErrorf(
			"'start-height' %d must not be less than 'end-height' %d",
			data.startHeight,
			data.endHeight,
		)
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
		data.executionNodeIds = identities.ToSkeleton()
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
	if b.txErrorMessagesCore == nil {
		return nil, fmt.Errorf("failed to backfill, could not get transaction error messages storage")
	}

	data := request.ValidatorData.(*backfillTxErrorMessagesRequest)

	for height := data.startHeight; height <= data.endHeight; height++ {
		header, err := b.state.AtHeight(height).Head()
		if err != nil {
			return nil, fmt.Errorf("failed to get block header: %w", err)
		}

		blockID := header.ID()
		err = b.txErrorMessagesCore.HandleTransactionResultErrorMessagesByENs(ctx, blockID, data.executionNodeIds)
		if err != nil {
			return nil, fmt.Errorf("error encountered while processing transaction result error message for block: %d, %w", height, err)
		}
	}

	return nil, nil
}

// parseExecutionNodeIds converts a list of node IDs from input to flow.IdentitySkeletonList.
// Returns an error if the IDs are invalid or empty.
//
// Expected errors during normal operation:
// - admin.InvalidAdminReqParameterError - if execution-node-ids is empty or has an invalid format.
func (b *BackfillTxErrorMessagesCommand) parseExecutionNodeIds(executionNodeIdsIn interface{}, allIdentities flow.IdentityList) (flow.IdentitySkeletonList, error) {
	var ids flow.IdentityList

	executionNodeIds, err := convertToExecutionStringList(executionNodeIdsIn)
	if err != nil {
		return nil, err
	}

	requestedENIdentifiers, err := commonrpc.IdentifierList(executionNodeIds)
	if err != nil {
		return nil, admin.NewInvalidAdminReqParameterError("execution-node-ids", err.Error(), executionNodeIdsIn)
	}

	for _, en := range requestedENIdentifiers {
		id, exists := allIdentities.ByNodeID(en)
		if !exists {
			return nil, admin.NewInvalidAdminReqParameterError("execution-node-ids", "could not find execution node by provided id", en)
		}
		ids = append(ids, id)
	}

	return ids.ToSkeleton(), nil
}

func convertToExecutionStringList(executionNodeIdsIn interface{}) ([]string, error) {
	var executionNodeIds []string
	switch executionNodeIdsConverted := executionNodeIdsIn.(type) {
	case []interface{}:
		for _, executionNodeIdIn := range executionNodeIdsConverted {
			executionNodeId, ok := executionNodeIdIn.(string)
			if !ok {
				return nil, admin.NewInvalidAdminReqParameterError("execution-node-ids", "must be a list of strings", executionNodeIdsIn)
			}
			executionNodeIds = append(executionNodeIds, executionNodeId)
		}
	default:
		return nil, admin.NewInvalidAdminReqParameterError("execution-node-ids", "must be a list of strings", executionNodeIdsIn)
	}

	if len(executionNodeIds) == 0 {
		return nil, admin.NewInvalidAdminReqParameterError("execution-node-ids", "must be a non empty list of strings", executionNodeIdsIn)
	}

	return executionNodeIds, nil
}
