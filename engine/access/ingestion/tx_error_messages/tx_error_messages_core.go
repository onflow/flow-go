package tx_error_messages

import (
	"context"
	"fmt"

	execproto "github.com/onflow/flow/protobuf/go/flow/execution"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/access/rpc/backend"
	commonrpc "github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// TxErrorMessagesCore is responsible for managing transaction result error messages
// It handles both storage and retrieval of error messages
// from execution nodes.
type TxErrorMessagesCore struct {
	log   zerolog.Logger // used to log relevant actions with context
	state protocol.State // used to access the  protocol state

	backend *backend.Backend

	executionReceipts              storage.ExecutionReceipts
	transactionResultErrorMessages storage.TransactionResultErrorMessages

	preferredExecutionNodeIDs flow.IdentifierList
	fixedExecutionNodeIDs     flow.IdentifierList
}

// NewTxErrorMessagesCore creates a new instance of TxErrorMessagesCore.
func NewTxErrorMessagesCore(
	log zerolog.Logger,
	state protocol.State,
	backend *backend.Backend,
	executionReceipts storage.ExecutionReceipts,
	transactionResultErrorMessages storage.TransactionResultErrorMessages,
	preferredExecutionNodeIDs flow.IdentifierList,
	fixedExecutionNodeIDs flow.IdentifierList,
) *TxErrorMessagesCore {
	return &TxErrorMessagesCore{
		log:                            log.With().Str("module", "tx_error_messages_core").Logger(),
		state:                          state,
		backend:                        backend,
		executionReceipts:              executionReceipts,
		transactionResultErrorMessages: transactionResultErrorMessages,
		preferredExecutionNodeIDs:      preferredExecutionNodeIDs,
		fixedExecutionNodeIDs:          fixedExecutionNodeIDs,
	}
}

// HandleTransactionResultErrorMessages processes transaction result error messages for a given block ID.
// It retrieves error messages from the backend if they do not already exist in storage.
//
// The function first checks if error messages for the given block ID are already present in storage.
// If they are not, it fetches the messages from execution nodes and stores them.
//
// Parameters:
// - ctx: The context for managing cancellation and deadlines during the operation.
// - blockID: The identifier of the block for which transaction result error messages need to be processed.
//
// No errors are expected during normal operation.
func (c *TxErrorMessagesCore) HandleTransactionResultErrorMessages(ctx context.Context, blockID flow.Identifier) error {
	exists, err := c.transactionResultErrorMessages.Exists(blockID)
	if err != nil {
		return fmt.Errorf("could not check existance of transaction result error messages: %w", err)
	}

	if exists {
		return nil
	}

	// retrieves error messages from the backend if they do not already exist in storage
	execNodes, err := commonrpc.ExecutionNodesForBlockID(
		ctx,
		blockID,
		c.executionReceipts,
		c.state,
		c.log,
		c.preferredExecutionNodeIDs,
		c.fixedExecutionNodeIDs,
	)
	if err != nil {
		c.log.Error().Err(err).Msg(fmt.Sprintf("failed to find execution nodes for block id: %s", blockID))
		return fmt.Errorf("could not find execution nodes for block: %w", err)
	}

	req := &execproto.GetTransactionErrorMessagesByBlockIDRequest{
		BlockId: convert.IdentifierToMessage(blockID),
	}

	c.log.Debug().
		Msgf("transaction error messages for block %s are being downloaded", blockID)

	resp, execNode, err := c.backend.GetTransactionErrorMessagesFromAnyEN(ctx, execNodes, req)
	if err != nil {
		c.log.Error().Err(err).Msg("failed to get transaction error messages from execution nodes")
		return err
	}

	if len(resp) > 0 {
		err = c.storeTransactionResultErrorMessages(blockID, resp, execNode)
		if err != nil {
			return fmt.Errorf("could not store error messages (block: %s): %w", blockID, err)
		}
	}

	return nil
}

func (c *TxErrorMessagesCore) HandleTransactionResultErrorMessagesByENs(
	ctx context.Context,
	blockID flow.Identifier,
	execNodes flow.IdentityList,
) error {
	exists, err := c.transactionResultErrorMessages.Exists(blockID)
	if err != nil {
		return fmt.Errorf("could not check existance of transaction result error messages: %w", err)
	}
	if exists {
		return nil
	}

	req := &execproto.GetTransactionErrorMessagesByBlockIDRequest{
		BlockId: convert.IdentifierToMessage(blockID),
	}
	c.log.Debug().Msgf("transaction error messages for block %s are being downloaded", blockID)

	resp, execNode, err := c.backend.GetTransactionErrorMessagesFromAnyEN(ctx, execNodes, req)
	if err != nil {
		c.log.Error().Err(err).Msg("failed to get transaction error messages from execution nodes")
		return err
	}

	if len(resp) > 0 {
		err = c.storeTransactionResultErrorMessages(blockID, resp, execNode)
		if err != nil {
			return fmt.Errorf("could not store error messages (block: %s): %w", blockID, err)
		}
	}

	return nil
}

// storeTransactionResultErrorMessages stores the transaction result error messages for a given block ID.
//
// Parameters:
// - blockID: The identifier of the block for which the error messages are to be stored.
// - errorMessagesResponses: A slice of responses containing the error messages to be stored.
// - execNode: The execution node associated with the error messages.
//
// No errors are expected during normal operation.
func (c *TxErrorMessagesCore) storeTransactionResultErrorMessages(
	blockID flow.Identifier,
	errorMessagesResponses []*execproto.GetTransactionErrorMessagesResponse_Result,
	execNode *flow.Identity, //TODO(illia): the type was *flow.IdentitySkeleton
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

	err := c.transactionResultErrorMessages.Store(blockID, errorMessages)
	if err != nil {
		return fmt.Errorf("failed to store transaction error messages: %w", err)
	}

	return nil
}
