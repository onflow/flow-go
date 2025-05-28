package tx_error_messages

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/access/rpc/backend"
	commonrpc "github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"

	execproto "github.com/onflow/flow/protobuf/go/flow/execution"
)

// TxErrorMessagesCore is responsible for managing transaction result error messages
// It handles both storage and retrieval of error messages
// from execution nodes.
type TxErrorMessagesCore struct {
	log zerolog.Logger // used to log relevant actions with context

	backend                        *backend.Backend
	transactionResultErrorMessages storage.TransactionResultErrorMessages
	execNodeIdentitiesProvider     *commonrpc.ExecutionNodeIdentitiesProvider
}

// NewTxErrorMessagesCore creates a new instance of TxErrorMessagesCore.
func NewTxErrorMessagesCore(
	log zerolog.Logger,
	backend *backend.Backend,
	transactionResultErrorMessages storage.TransactionResultErrorMessages,
	execNodeIdentitiesProvider *commonrpc.ExecutionNodeIdentitiesProvider,
) *TxErrorMessagesCore {
	return &TxErrorMessagesCore{
		log:                            log.With().Str("module", "tx_error_messages_core").Logger(),
		backend:                        backend,
		transactionResultErrorMessages: transactionResultErrorMessages,
		execNodeIdentitiesProvider:     execNodeIdentitiesProvider,
	}
}

// FetchTransactionResultErrorMessages retrieves transaction result for a given block ID
// if they do not already exist in storage.
//
// The function first checks if error messages for the given block ID are already present in storage.
// If they are not, it fetches the messages from execution nodes and stores them.
//
// Parameters:
// - ctx: The context for managing cancellation and deadlines during the operation.
// - blockID: The identifier of the block for which transaction result error messages need to be processed.
//
// Expected errors during normal operation:
//   - status.Error - GRPC call failed, some of possible codes are:
//   - codes.NotFound - request cannot be served by EN because of absence of data.
//   - codes.Unavailable - remote node is not unavailable.
func (c *TxErrorMessagesCore) FetchTransactionResultErrorMessages(ctx context.Context, blockID flow.Identifier) error {
	execNodes, err := c.execNodeIdentitiesProvider.ExecutionNodesForBlockID(ctx, blockID)
	if err != nil {
		c.log.Error().Err(err).Msg(fmt.Sprintf("failed to find execution nodes for block id: %s", blockID))
		return fmt.Errorf("could not find execution nodes for block: %w", err)
	}

	return c.FetchTransactionResultErrorMessagesFromENs(ctx, blockID, execNodes)
}

// FetchTransactionResultErrorMessagesByResultID retrieves tx result error messages for a given execution result ID
// from execution nodes that generated receipts within a specific block.
//
// Parameters:
// - ctx: The context for managing cancellation and deadlines during the operation.
// - blockID: The identifier of the block containing the execution result.
// - resultID: The identifier of the specific execution result for which to fetch error messages.
//
// Expected errors during normal operation:
//   - storage.ErrNotFound - if no execution result is found for the given block in the AN's storage.
//   - status.Error - GRPC call failed, some of possible codes are:
//   - codes.NotFound - request cannot be served by EN because of absence of data.
//   - codes.Unavailable - remote node is not unavailable.
func (c *TxErrorMessagesCore) FetchTransactionResultErrorMessagesByResultID(
	ctx context.Context,
	blockID flow.Identifier,
	resultID flow.Identifier,
) error {
	execNodes, err := c.execNodeIdentitiesProvider.ExecutionNodesForResultID(blockID, resultID)
	if err != nil {
		c.log.Error().Err(err).
			Str("block_id", blockID.String()).
			Str("result_id", resultID.String()).
			Msg("failed to find execution nodes for specific result ID")
		return fmt.Errorf("could not find execution nodes for result %v in block %v: %w", resultID, blockID, err)
	}

	return c.FetchTransactionResultErrorMessagesFromENs(ctx, blockID, execNodes)
}

// FetchTransactionResultErrorMessagesFromENs fetches transaction result error messages via provided list
// of execution nodes.
//
// Expected errors during normal operation:
//   - status.Error - GRPC call failed, some of possible codes are:
//   - codes.NotFound - request cannot be served by EN because of absence of data.
//   - codes.Unavailable - remote node is not unavailable.
func (c *TxErrorMessagesCore) FetchTransactionResultErrorMessagesFromENs(
	ctx context.Context,
	blockID flow.Identifier,
	execNodes flow.IdentitySkeletonList,
) error {
	exists, err := c.transactionResultErrorMessages.Exists(blockID)
	if err != nil {
		return fmt.Errorf("could not check existance of transaction result error messages: %w", err)
	}

	if exists {
		return nil
	}

	// retrieves error messages from the backend if they do not already exist in storage
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

	err := c.transactionResultErrorMessages.Store(blockID, errorMessages)
	if err != nil {
		return fmt.Errorf("failed to store transaction error messages: %w", err)
	}

	return nil
}
