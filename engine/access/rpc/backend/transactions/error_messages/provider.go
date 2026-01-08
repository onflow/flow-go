package error_messages

import (
	"context"
	"errors"

	execproto "github.com/onflow/flow/protobuf/go/flow/execution"
	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/engine/access/index"
	"github.com/onflow/flow-go/engine/access/rpc/backend/node_communicator"
	"github.com/onflow/flow-go/engine/access/rpc/connection"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"

	"github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

const DefaultFailedErrorMessage = "failed"

// Provider declares the lookup transaction error methods by different input parameters.
type Provider interface {
	// ErrorMessageByTransactionID is a function type for getting transaction error message by block ID and transaction ID.
	// Expected errors during normal operation:
	//   - InsufficientExecutionReceipts - found insufficient receipts for given block ID.
	//   - status.Error - remote GRPC call to EN has failed.
	ErrorMessageByTransactionID(ctx context.Context, blockID flow.Identifier, height uint64, transactionID flow.Identifier) (string, error)

	// ErrorMessageByIndex is a function type for getting transaction error message by index.
	// Expected errors during normal operation:
	//   - InsufficientExecutionReceipts - found insufficient receipts for given block ID.
	//   - status.Error - remote GRPC call to EN has failed.
	ErrorMessageByIndex(ctx context.Context, blockID flow.Identifier, height uint64, index uint32) (string, error)

	// ErrorMessagesByBlockID is a function type for getting transaction error messages by block ID.
	// Expected errors during normal operation:
	//   - InsufficientExecutionReceipts - found insufficient receipts for given block ID.
	//   - status.Error - remote GRPC call to EN has failed.
	ErrorMessagesByBlockID(ctx context.Context, blockID flow.Identifier, height uint64) (map[flow.Identifier]string, error)

	// ErrorMessageFromAnyEN performs an RPC call using available nodes passed as argument.
	// List of nodes must be non-empty otherwise an error will be returned.
	// Expected errors during normal operation:
	//   - status.Error - GRPC call failed, some of possible codes are:
	//   - codes.NotFound - request cannot be served by EN because of absence of data.
	//   - codes.Unavailable - remote node is not unavailable.
	ErrorMessageFromAnyEN(
		ctx context.Context,
		execNodes flow.IdentitySkeletonList,
		req *execproto.GetTransactionErrorMessageRequest,
	) (*execproto.GetTransactionErrorMessageResponse, error)

	// ErrorMessageByIndexFromAnyEN performs an RPC call using available nodes passed as argument.
	// List of nodes must be non-empty otherwise an error will be returned.
	// Expected errors during normal operation:
	//   - status.Error - GRPC call failed, some of possible codes are:
	//   - codes.NotFound - request cannot be served by EN because of absence of data.
	//   - codes.Unavailable - remote node is not unavailable.
	ErrorMessageByIndexFromAnyEN(
		ctx context.Context,
		execNodes flow.IdentitySkeletonList,
		req *execproto.GetTransactionErrorMessageByIndexRequest,
	) (*execproto.GetTransactionErrorMessageResponse, error)

	// ErrorMessageByBlockIDFromAnyEN performs an RPC call using available nodes passed as argument.
	// List of nodes must be non-empty otherwise an error will be returned.
	// Expected errors during normal operation:
	//   - status.Error - GRPC call failed, some of possible codes are:
	//   - codes.NotFound - request cannot be served by EN because of absence of data.
	//   - codes.Unavailable - remote node is not unavailable.
	ErrorMessageByBlockIDFromAnyEN(
		ctx context.Context,
		execNodes flow.IdentitySkeletonList,
		req *execproto.GetTransactionErrorMessagesByBlockIDRequest,
	) ([]*execproto.GetTransactionErrorMessagesResponse_Result, *flow.IdentitySkeleton, error)
}

type ProviderImpl struct {
	log zerolog.Logger

	txResultErrorMessages storage.TransactionResultErrorMessages
	txResultsIndex        *index.TransactionResultsIndex

	connFactory                connection.ConnectionFactory
	nodeCommunicator           node_communicator.Communicator
	execNodeIdentitiesProvider *rpc.ExecutionNodeIdentitiesProvider
}

var _ Provider = (*ProviderImpl)(nil)

func NewTxErrorMessageProvider(
	log zerolog.Logger,
	txResultErrorMessages storage.TransactionResultErrorMessages,
	txResultsIndex *index.TransactionResultsIndex,
	connFactory connection.ConnectionFactory,
	nodeCommunicator node_communicator.Communicator,
	execNodeIdentitiesProvider *rpc.ExecutionNodeIdentitiesProvider,
) *ProviderImpl {
	return &ProviderImpl{
		log:                        log,
		txResultErrorMessages:      txResultErrorMessages,
		txResultsIndex:             txResultsIndex,
		connFactory:                connFactory,
		nodeCommunicator:           nodeCommunicator,
		execNodeIdentitiesProvider: execNodeIdentitiesProvider,
	}
}

// ErrorMessageByTransactionID returns transaction error message for specified transaction.
// If transaction error messages are stored locally, they will be checked first in local storage.
// If error messages are not stored locally, an RPC call will be made to the EN to fetch message.
//
// Expected errors during normal operation:
//   - InsufficientExecutionReceipts - found insufficient receipts for the given block ID.
//   - status.Error - remote GRPC call to EN has failed.
func (e *ProviderImpl) ErrorMessageByTransactionID(
	ctx context.Context,
	blockID flow.Identifier,
	height uint64,
	transactionID flow.Identifier,
) (string, error) {
	if e.txResultErrorMessages != nil {
		res, err := e.txResultErrorMessages.ByBlockIDTransactionID(blockID, transactionID)
		if err == nil {
			return res.ErrorMessage, nil
		}
	}

	execNodes, err := e.execNodeIdentitiesProvider.ExecutionNodesForBlockID(
		ctx,
		blockID,
	)
	if err != nil {
		if optimistic_sync.IsExecutionResultNotReadyError(err) {
			return "", status.Error(codes.NotFound, err.Error())
		}
		return "", rpc.ConvertError(err, "failed to select execution nodes", codes.Internal)
	}
	req := &execproto.GetTransactionErrorMessageRequest{
		BlockId:       convert.IdentifierToMessage(blockID),
		TransactionId: convert.IdentifierToMessage(transactionID),
	}

	resp, err := e.ErrorMessageFromAnyEN(ctx, execNodes, req)
	if err != nil {
		// If no execution nodes return a valid response,
		// return a static message "failed".
		txResult, err := e.txResultsIndex.ByBlockIDTransactionID(blockID, height, transactionID)
		if err != nil {
			return "", rpc.ConvertStorageError(err)
		}

		if txResult.Failed {
			return DefaultFailedErrorMessage, nil
		}

		// in case tx result is not failed
		return "", nil
	}

	return resp.ErrorMessage, nil
}

// ErrorMessageByIndex returns the transaction error message for a specified transaction using its index.
// If transaction error messages are stored locally, they will be checked first in local storage.
// If error messages are not stored locally, an RPC call will be made to the EN to fetch message.
//
// Expected errors during normal operation:
//   - InsufficientExecutionReceipts - found insufficient receipts for the given block ID.
//   - status.Error - remote GRPC call to EN has failed.
func (e *ProviderImpl) ErrorMessageByIndex(
	ctx context.Context,
	blockID flow.Identifier,
	height uint64,
	index uint32,
) (string, error) {
	if e.txResultErrorMessages != nil {
		res, err := e.txResultErrorMessages.ByBlockIDTransactionIndex(blockID, index)
		if err == nil {
			return res.ErrorMessage, nil
		}
	}

	execNodes, err := e.execNodeIdentitiesProvider.ExecutionNodesForBlockID(
		ctx,
		blockID,
	)
	if err != nil {
		if optimistic_sync.IsExecutionResultNotReadyError(err) {
			return "", status.Error(codes.NotFound, err.Error())
		}
		return "", rpc.ConvertError(err, "failed to select execution nodes", codes.Internal)
	}
	req := &execproto.GetTransactionErrorMessageByIndexRequest{
		BlockId: convert.IdentifierToMessage(blockID),
		Index:   index,
	}

	resp, err := e.ErrorMessageByIndexFromAnyEN(ctx, execNodes, req)
	if err != nil {
		// If no execution nodes return a valid response,
		// return a static message "failed"
		txResult, err := e.txResultsIndex.ByBlockIDTransactionIndex(blockID, height, index)
		if err != nil {
			return "", rpc.ConvertStorageError(err)
		}

		if txResult.Failed {
			return DefaultFailedErrorMessage, nil
		}

		// in case tx result is not failed
		return "", nil
	}

	return resp.ErrorMessage, nil
}

// ErrorMessagesByBlockID returns all error messages for failed transactions by blockID.
// If transaction error messages are stored locally, they will be checked first in local storage.
// If error messages are not stored locally, an RPC call will be made to the EN to fetch messages.
//
// Expected errors during normal operation:
//   - InsufficientExecutionReceipts - found insufficient receipts for the given block ID.
//   - status.Error - remote GRPC call to EN has failed.
func (e *ProviderImpl) ErrorMessagesByBlockID(
	ctx context.Context,
	blockID flow.Identifier,
	height uint64,
) (map[flow.Identifier]string, error) {
	result := make(map[flow.Identifier]string)

	if e.txResultErrorMessages != nil {
		res, err := e.txResultErrorMessages.ByBlockID(blockID)
		if err == nil {
			for _, value := range res {
				result[value.TransactionID] = value.ErrorMessage
			}

			return result, nil
		}
	}

	execNodes, err := e.execNodeIdentitiesProvider.ExecutionNodesForBlockID(
		ctx,
		blockID,
	)
	if err != nil {
		if optimistic_sync.IsExecutionResultNotReadyError(err) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, rpc.ConvertError(err, "failed to select execution nodes", codes.Internal)
	}
	req := &execproto.GetTransactionErrorMessagesByBlockIDRequest{
		BlockId: convert.IdentifierToMessage(blockID),
	}

	resp, _, err := e.ErrorMessageByBlockIDFromAnyEN(ctx, execNodes, req)
	if err != nil {
		// If no execution nodes return a valid response,
		// return a static message "failed"
		txResults, err := e.txResultsIndex.ByBlockID(blockID, height)
		if err != nil {
			return nil, rpc.ConvertStorageError(err)
		}

		for _, txResult := range txResults {
			if txResult.Failed {
				result[txResult.TransactionID] = DefaultFailedErrorMessage
			}
		}

		return result, nil
	}

	for _, value := range resp {
		result[convert.MessageToIdentifier(value.TransactionId)] = value.ErrorMessage
	}

	return result, nil
}

// ErrorMessageFromAnyEN performs an RPC call using available nodes passed as argument.
// List of nodes must be non-empty otherwise an error will be returned.
// Expected errors during normal operation:
//   - status.Error - GRPC call failed, some of possible codes are:
//   - codes.NotFound - request cannot be served by EN because of absence of data.
//   - codes.Unavailable - remote node is not unavailable.
func (e *ProviderImpl) ErrorMessageFromAnyEN(
	ctx context.Context,
	execNodes flow.IdentitySkeletonList,
	req *execproto.GetTransactionErrorMessageRequest,
) (*execproto.GetTransactionErrorMessageResponse, error) {
	// if we were passed 0 execution nodes add a specific error
	if len(execNodes) == 0 {
		return nil, errors.New("zero execution nodes")
	}

	var resp *execproto.GetTransactionErrorMessageResponse
	errToReturn := e.nodeCommunicator.CallAvailableNode(
		execNodes,
		func(node *flow.IdentitySkeleton) error {
			var err error
			resp, err = e.tryGetTransactionErrorMessageFromEN(ctx, node, req)
			if err == nil {
				e.log.Debug().
					Str("execution_node", node.String()).
					Hex("block_id", req.GetBlockId()).
					Hex("transaction_id", req.GetTransactionId()).
					Msg("Successfully got transaction error message from any node")
				return nil
			}
			return err
		},
		nil,
	)

	// log the errors
	if errToReturn != nil {
		e.log.Err(errToReturn).Msg("failed to get transaction error message from execution nodes")
		return nil, errToReturn
	}

	return resp, nil
}

// ErrorMessageByIndexFromAnyEN performs an RPC call using available nodes passed as argument.
// List of nodes must be non-empty otherwise an error will be returned.
// Expected errors during normal operation:
//   - status.Error - GRPC call failed, some of possible codes are:
//   - codes.NotFound - request cannot be served by EN because of absence of data.
//   - codes.Unavailable - remote node is not unavailable.
func (e *ProviderImpl) ErrorMessageByIndexFromAnyEN(
	ctx context.Context,
	execNodes flow.IdentitySkeletonList,
	req *execproto.GetTransactionErrorMessageByIndexRequest,
) (*execproto.GetTransactionErrorMessageResponse, error) {
	// if we were passed 0 execution nodes add a specific error
	if len(execNodes) == 0 {
		return nil, errors.New("zero execution nodes")
	}

	var resp *execproto.GetTransactionErrorMessageResponse
	errToReturn := e.nodeCommunicator.CallAvailableNode(
		execNodes,
		func(node *flow.IdentitySkeleton) error {
			var err error
			resp, err = e.tryGetTransactionErrorMessageByIndexFromEN(ctx, node, req)
			if err == nil {
				e.log.Debug().
					Str("execution_node", node.String()).
					Hex("block_id", req.GetBlockId()).
					Uint32("index", req.GetIndex()).
					Msg("Successfully got transaction error message by index from any node")
				return nil
			}
			return err
		},
		nil,
	)
	if errToReturn != nil {
		e.log.Err(errToReturn).Msg("failed to get transaction error message by index from execution nodes")
		return nil, errToReturn
	}

	return resp, nil
}

// ErrorMessageByBlockIDFromAnyEN performs an RPC call using available nodes passed as argument.
// List of nodes must be non-empty otherwise an error will be returned.
// Expected errors during normal operation:
//   - status.Error - GRPC call failed, some of possible codes are:
//   - codes.NotFound - request cannot be served by EN because of absence of data.
//   - codes.Unavailable - remote node is not unavailable.
func (e *ProviderImpl) ErrorMessageByBlockIDFromAnyEN(
	ctx context.Context,
	execNodes flow.IdentitySkeletonList,
	req *execproto.GetTransactionErrorMessagesByBlockIDRequest,
) ([]*execproto.GetTransactionErrorMessagesResponse_Result, *flow.IdentitySkeleton, error) {
	// if we were passed 0 execution nodes add a specific error
	if len(execNodes) == 0 {
		return nil, nil, errors.New("zero execution nodes")
	}

	var resp *execproto.GetTransactionErrorMessagesResponse
	var execNode *flow.IdentitySkeleton

	errToReturn := e.nodeCommunicator.CallAvailableNode(
		execNodes,
		func(node *flow.IdentitySkeleton) error {
			var err error
			execNode = node
			resp, err = e.tryGetTransactionErrorMessagesByBlockIDFromEN(ctx, node, req)
			if err == nil {
				e.log.Debug().
					Str("execution_node", node.String()).
					Hex("block_id", req.GetBlockId()).
					Msg("Successfully got transaction error messages from any node")
				return nil
			}
			return err
		},
		nil,
	)

	// log the errors
	if errToReturn != nil {
		e.log.Err(errToReturn).Msg("failed to get transaction error messages from execution nodes")
		return nil, nil, errToReturn
	}

	return resp.GetResults(), execNode, nil
}

// tryGetTransactionErrorMessageFromEN performs a grpc call to the specified execution node and returns response.
//
// Expected errors during normal operation:
//   - status.Error - GRPC call failed, some of possible codes are:
//   - codes.NotFound - request cannot be served by EN because of absence of data.
//   - codes.Unavailable - remote node is not unavailable.
func (e *ProviderImpl) tryGetTransactionErrorMessageFromEN(
	ctx context.Context,
	execNode *flow.IdentitySkeleton,
	req *execproto.GetTransactionErrorMessageRequest,
) (*execproto.GetTransactionErrorMessageResponse, error) {
	execRPCClient, closer, err := e.connFactory.GetExecutionAPIClient(execNode.Address)
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	return execRPCClient.GetTransactionErrorMessage(ctx, req)
}

// tryGetTransactionErrorMessageByIndexFromEN performs a grpc call to the specified execution node and returns response.
// Expected errors during normal operation:
//   - status.Error - GRPC call failed, some of possible codes are:
//   - codes.NotFound - request cannot be served by EN because of absence of data.
//   - codes.Unavailable - remote node is not unavailable.
func (e *ProviderImpl) tryGetTransactionErrorMessageByIndexFromEN(
	ctx context.Context,
	execNode *flow.IdentitySkeleton,
	req *execproto.GetTransactionErrorMessageByIndexRequest,
) (*execproto.GetTransactionErrorMessageResponse, error) {
	execRPCClient, closer, err := e.connFactory.GetExecutionAPIClient(execNode.Address)
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	return execRPCClient.GetTransactionErrorMessageByIndex(ctx, req)
}

// tryGetTransactionErrorMessagesByBlockIDFromEN performs a grpc call to the specified execution node and returns response.
// Expected errors during normal operation:
//   - status.Error - GRPC call failed, some of possible codes are:
//   - codes.NotFound - request cannot be served by EN because of absence of data.
//   - codes.Unavailable - remote node is not unavailable.
func (e *ProviderImpl) tryGetTransactionErrorMessagesByBlockIDFromEN(
	ctx context.Context,
	execNode *flow.IdentitySkeleton,
	req *execproto.GetTransactionErrorMessagesByBlockIDRequest,
) (*execproto.GetTransactionErrorMessagesResponse, error) {
	execRPCClient, closer, err := e.connFactory.GetExecutionAPIClient(execNode.Address)
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	return execRPCClient.GetTransactionErrorMessagesByBlockID(ctx, req)
}
