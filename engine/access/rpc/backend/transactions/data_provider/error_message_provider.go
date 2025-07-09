package transactions

import (
	"context"
	"errors"

	execproto "github.com/onflow/flow/protobuf/go/flow/execution"
	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/engine/access/index"
	"github.com/onflow/flow-go/engine/access/rpc/connection"
	"github.com/onflow/flow-go/engine/common/rpc/convert"

	"github.com/onflow/flow-go/engine/access/rpc/backend"
	"github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

const DefaultFailedErrorMessage = "failed"

// ErrorMessageProvider declares the lookup transaction error methods by different input parameters.
type ErrorMessageProvider interface {
	// LookupErrorMessageByTransactionID is a function type for getting transaction error message by block ID and transaction ID.
	// Expected errors during normal operation:
	//   - InsufficientExecutionReceipts - found insufficient receipts for given block ID.
	//   - status.Error - remote GRPC call to EN has failed.
	LookupErrorMessageByTransactionID(ctx context.Context, blockID flow.Identifier, height uint64, transactionID flow.Identifier) (string, error)

	// LookupErrorMessageByIndex is a function type for getting transaction error message by index.
	// Expected errors during normal operation:
	//   - InsufficientExecutionReceipts - found insufficient receipts for given block ID.
	//   - status.Error - remote GRPC call to EN has failed.
	LookupErrorMessageByIndex(ctx context.Context, blockID flow.Identifier, height uint64, index uint32) (string, error)

	// LookupErrorMessagesByBlockID is a function type for getting transaction error messages by block ID.
	// Expected errors during normal operation:
	//   - InsufficientExecutionReceipts - found insufficient receipts for given block ID.
	//   - status.Error - remote GRPC call to EN has failed.
	LookupErrorMessagesByBlockID(ctx context.Context, blockID flow.Identifier, height uint64) (map[flow.Identifier]string, error)
}

type ErrorMessageProviderImpl struct {
	log zerolog.Logger

	txResultErrorMessages storage.TransactionResultErrorMessages
	txResultsIndex        *index.TransactionResultsIndex

	connFactory                connection.ConnectionFactory
	nodeCommunicator           backend.Communicator
	execNodeIdentitiesProvider *rpc.ExecutionNodeIdentitiesProvider
}

var _ ErrorMessageProvider = (*ErrorMessageProviderImpl)(nil)

func NewErrorMessageProvider(
	log zerolog.Logger,
	txResultErrorMessages storage.TransactionResultErrorMessages,
	txResultsIndex *index.TransactionResultsIndex,
	connFactory connection.ConnectionFactory,
	nodeCommunicator backend.Communicator,
	execNodeIdentitiesProvider *rpc.ExecutionNodeIdentitiesProvider,
) *ErrorMessageProviderImpl {
	return &ErrorMessageProviderImpl{
		log:                        log,
		txResultErrorMessages:      txResultErrorMessages,
		txResultsIndex:             txResultsIndex,
		connFactory:                connFactory,
		nodeCommunicator:           nodeCommunicator,
		execNodeIdentitiesProvider: execNodeIdentitiesProvider,
	}
}

func (e *ErrorMessageProviderImpl) LookupErrorMessageByTransactionID(
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
		if backend.IsInsufficientExecutionReceipts(err) {
			return "", status.Error(codes.NotFound, err.Error())
		}
		return "", rpc.ConvertError(err, "failed to select execution nodes", codes.Internal)
	}
	req := &execproto.GetTransactionErrorMessageRequest{
		BlockId:       convert.IdentifierToMessage(blockID),
		TransactionId: convert.IdentifierToMessage(transactionID),
	}

	resp, err := e.getTransactionErrorMessageFromAnyEN(ctx, execNodes, req)
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

func (e *ErrorMessageProviderImpl) LookupErrorMessageByIndex(
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
		if backend.IsInsufficientExecutionReceipts(err) {
			return "", status.Error(codes.NotFound, err.Error())
		}
		return "", rpc.ConvertError(err, "failed to select execution nodes", codes.Internal)
	}
	req := &execproto.GetTransactionErrorMessageByIndexRequest{
		BlockId: convert.IdentifierToMessage(blockID),
		Index:   index,
	}

	resp, err := e.getTransactionErrorMessageByIndexFromAnyEN(ctx, execNodes, req)
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

func (e *ErrorMessageProviderImpl) LookupErrorMessagesByBlockID(
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
		if backend.IsInsufficientExecutionReceipts(err) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, rpc.ConvertError(err, "failed to select execution nodes", codes.Internal)
	}
	req := &execproto.GetTransactionErrorMessagesByBlockIDRequest{
		BlockId: convert.IdentifierToMessage(blockID),
	}

	resp, _, err := e.getTransactionErrorMessagesFromAnyEN(ctx, execNodes, req)
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

// getTransactionErrorMessagesFromAnyEN performs an RPC call using available nodes passed as argument. List of nodes must be non-empty otherwise an error will be returned.
// Expected errors during normal operation:
//   - status.Error - GRPC call failed, some of possible codes are:
//   - codes.NotFound - request cannot be served by EN because of absence of data.
//   - codes.Unavailable - remote node is not unavailable.
func (e *ErrorMessageProviderImpl) getTransactionErrorMessagesFromAnyEN(
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

// getTransactionErrorMessageFromAnyEN performs an RPC call using available nodes passed as argument. List of nodes must be non-empty otherwise an error will be returned.
// Expected errors during normal operation:
//   - status.Error - GRPC call failed, some of possible codes are:
//   - codes.NotFound - request cannot be served by EN because of absence of data.
//   - codes.Unavailable - remote node is not unavailable.
func (e *ErrorMessageProviderImpl) getTransactionErrorMessageFromAnyEN(
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

// getTransactionErrorMessageByIndexFromAnyEN performs an RPC call using available nodes passed as argument. List of nodes must be non-empty otherwise an error will be returned.
// Expected errors during normal operation:
//   - status.Error - GRPC call failed, some of possible codes are:
//   - codes.NotFound - request cannot be served by EN because of absence of data.
//   - codes.Unavailable - remote node is not unavailable.
func (e *ErrorMessageProviderImpl) getTransactionErrorMessageByIndexFromAnyEN(
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

// Expected errors during normal operation:
//   - status.Error - GRPC call failed, some of possible codes are:
//   - codes.NotFound - request cannot be served by EN because of absence of data.
//   - codes.Unavailable - remote node is not unavailable.
//
// tryGetTransactionErrorMessageFromEN performs a grpc call to the specified execution node and returns response.
func (e *ErrorMessageProviderImpl) tryGetTransactionErrorMessageFromEN(
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
func (e *ErrorMessageProviderImpl) tryGetTransactionErrorMessageByIndexFromEN(
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
func (e *ErrorMessageProviderImpl) tryGetTransactionErrorMessagesByBlockIDFromEN(
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
