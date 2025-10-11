package tx_error_messages

import (
	"context"
	"fmt"

	"github.com/jordanschalm/lockctx"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/access/rpc/backend/transactions/error_messages"
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

	txErrorMessageProvider         error_messages.Provider
	transactionResultErrorMessages storage.TransactionResultErrorMessages
	execNodeIdentitiesProvider     *commonrpc.ExecutionNodeIdentitiesProvider
	lockManager                    storage.LockManager
}

// NewTxErrorMessagesCore creates a new instance of TxErrorMessagesCore.
func NewTxErrorMessagesCore(
	log zerolog.Logger,
	txErrorMessageProvider error_messages.Provider,
	transactionResultErrorMessages storage.TransactionResultErrorMessages,
	execNodeIdentitiesProvider *commonrpc.ExecutionNodeIdentitiesProvider,
	lockManager storage.LockManager,
) *TxErrorMessagesCore {
	return &TxErrorMessagesCore{
		log:                            log.With().Str("module", "tx_error_messages_core").Logger(),
		txErrorMessageProvider:         txErrorMessageProvider,
		transactionResultErrorMessages: transactionResultErrorMessages,
		execNodeIdentitiesProvider:     execNodeIdentitiesProvider,
		lockManager:                    lockManager,
	}
}

// FetchErrorMessages processes transaction result error messages for a given block ID.
// It retrieves error messages from the txErrorMessageProvider if they do not already exist in storage.
//
// The function first checks if error messages for the given block ID are already present in storage.
// If they are not, it fetches the messages from execution nodes and stores them.
//
// Parameters:
// - ctx: The context for managing cancellation and deadlines during the operation.
// - blockID: The identifier of the block for which transaction result error messages need to be processed.
//
// No errors are expected during normal operation.
func (c *TxErrorMessagesCore) FetchErrorMessages(ctx context.Context, blockID flow.Identifier) error {
	execNodes, err := c.execNodeIdentitiesProvider.ExecutionNodesForBlockID(ctx, blockID)
	if err != nil {
		c.log.Error().Err(err).Msg(fmt.Sprintf("failed to find execution nodes for block id: %s", blockID))
		return fmt.Errorf("could not find execution nodes for block: %w", err)
	}

	return c.FetchErrorMessagesByENs(ctx, blockID, execNodes)
}

// FetchErrorMessagesByENs requests the transaction result error messages for the specified block ID from
// any of the given execution nodes and persists them once retrieved. This function blocks until ingesting
// the tx error messages is completed or failed.
// It returns [storage.ErrAlreadyExists] if tx result error messages for the block already exist.
func (c *TxErrorMessagesCore) FetchErrorMessagesByENs(
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

	// retrieves error messages from the txErrorMessageProvider if they do not already exist in storage
	req := &execproto.GetTransactionErrorMessagesByBlockIDRequest{
		BlockId: convert.IdentifierToMessage(blockID),
	}

	c.log.Debug().
		Msgf("transaction error messages for block %s are being downloaded", blockID)

	resp, execNode, err := c.txErrorMessageProvider.ErrorMessageByBlockIDFromAnyEN(ctx, execNodes, req)
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

// storeTransactionResultErrorMessages persists and indexes all transaction result error messages for the given blockID.
// The caller must acquire [storage.LockInsertTransactionResultErrMessage] and hold it until the write batch has been committed.
//
// Note that transaction error messages are auxiliary data provided by the Execution Nodes on a goodwill basis and
// not protected by the protocol. Execution Error messages might be non-deterministic, i.e. potentially different
// for different execution nodes. Hence, we also persist which execution node (`execNode) provided the error message.
//
// It returns [storage.ErrAlreadyExists] if tx result error messages for the block already exist.
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

	err := storage.WithLock(c.lockManager, storage.LockInsertTransactionResultErrMessage, func(lctx lockctx.Context) error {
		return c.transactionResultErrorMessages.Store(lctx, blockID, errorMessages)
	})
	if err != nil {
		return fmt.Errorf("failed to store transaction error messages: %w", err)
	}

	return nil
}
