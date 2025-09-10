package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/onflow/flow/protobuf/go/flow/entities"
	execproto "github.com/onflow/flow/protobuf/go/flow/execution"
	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/engine/access/rpc/backend/node_communicator"
	txstatus "github.com/onflow/flow-go/engine/access/rpc/backend/transactions/status"
	"github.com/onflow/flow-go/engine/access/rpc/connection"
	"github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

var (
	// errTooManyResults is returned when the execution node included more results in its response than there are tx in the block
	errTooManyResults = errors.New("number of transaction results returned by execution node is more than the number of transactions in the block")

	// errTooFewResults is returned when the execution node included fewer results in its response than there are tx in the block
	errTooFewResults = errors.New("number of transaction results returned by execution node is less than the number of transactions in the block")
)

type ENTransactionProvider struct {
	log   zerolog.Logger
	state protocol.State

	collections storage.Collections

	connFactory             connection.ConnectionFactory
	nodeCommunicator        node_communicator.Communicator
	executionResultProvider optimistic_sync.ExecutionResultProvider

	txStatusDeriver *txstatus.TxStatusDeriver

	systemTxID flow.Identifier
	systemTx   *flow.TransactionBody
}

var _ TransactionProvider = (*ENTransactionProvider)(nil)

func NewENTransactionProvider(
	log zerolog.Logger,
	state protocol.State,
	collections storage.Collections,
	connFactory connection.ConnectionFactory,
	nodeCommunicator node_communicator.Communicator,
	executionResultProvider optimistic_sync.ExecutionResultProvider,
	txStatusDeriver *txstatus.TxStatusDeriver,
	systemTxID flow.Identifier,
	systemTx *flow.TransactionBody,
) *ENTransactionProvider {
	return &ENTransactionProvider{
		log:                     log.With().Str("transaction_provider", "execution_node").Logger(),
		state:                   state,
		collections:             collections,
		connFactory:             connFactory,
		nodeCommunicator:        nodeCommunicator,
		executionResultProvider: executionResultProvider,
		txStatusDeriver:         txStatusDeriver,
		systemTxID:              systemTxID,
		systemTx:                systemTx,
	}
}

// TransactionResult retrieves a transaction result from execution nodes by block ID and transaction ID.
//
// Expected error returns during normal operation:
//   - [codes.NotFound] when transaction result is not found
//   - [codes.Internal] when data returned by execution node is invalid or inconsistent
//   - [status.Error] when the request to execution node failed
func (e *ENTransactionProvider) TransactionResult(
	ctx context.Context,
	block *flow.Header,
	transactionID flow.Identifier,
	requiredEventEncodingVersion entities.EventEncodingVersion,
	executionResultInfo *optimistic_sync.ExecutionResultInfo,
) (*accessmodel.TransactionResult, accessmodel.ExecutorMetadata, error) {
	blockID := block.ID()
	req := &execproto.GetTransactionResultRequest{
		BlockId:       blockID[:],
		TransactionId: transactionID[:],
	}

	metadata := accessmodel.ExecutorMetadata{
		ExecutionResultID: executionResultInfo.ExecutionResult.ID(),
		ExecutorIDs:       executionResultInfo.ExecutionNodes.NodeIDs(),
	}

	resp, err := e.getTransactionResultFromAnyExeNode(ctx, executionResultInfo.ExecutionNodes, req)
	if err != nil {
		return nil, metadata, err
	}

	txStatus, err := e.txStatusDeriver.DeriveFinalizedTransactionStatus(block.Height, true)
	if err != nil {
		return nil, metadata, fmt.Errorf("failed to derive transaction status: %w", err)
	}

	events, err := convert.MessagesToEventsWithEncodingConversion(resp.GetEvents(), resp.GetEventEncodingVersion(), requiredEventEncodingVersion)
	if err != nil {
		return nil, metadata, status.Errorf(codes.Internal, "failed to convert events to message: %v", err)
	}

	return &accessmodel.TransactionResult{
		TransactionID: transactionID,
		Status:        txStatus,
		StatusCode:    uint(resp.GetStatusCode()),
		Events:        events,
		ErrorMessage:  resp.GetErrorMessage(),
		BlockID:       blockID,
		BlockHeight:   block.Height,
	}, metadata, nil
}

// TransactionResultByIndex retrieves a transaction result from execution nodes by block ID and index.
//
// Expected error returns during normal operation:
//   - [codes.NotFound] when transaction result is not found
//   - [codes.Internal] when data returned by execution node is invalid or inconsistent
//   - [status.Error] when the request to execution node failed
func (e *ENTransactionProvider) TransactionResultByIndex(
	ctx context.Context,
	block *flow.Block,
	index uint32,
	encodingVersion entities.EventEncodingVersion,
	executionResultInfo *optimistic_sync.ExecutionResultInfo,
) (*accessmodel.TransactionResult, accessmodel.ExecutorMetadata, error) {
	blockID := block.ID()
	req := &execproto.GetTransactionByIndexRequest{
		BlockId: blockID[:],
		Index:   index,
	}

	metadata := accessmodel.ExecutorMetadata{
		ExecutionResultID: executionResultInfo.ExecutionResult.ID(),
		ExecutorIDs:       executionResultInfo.ExecutionNodes.NodeIDs(),
	}

	resp, err := e.getTransactionResultByIndexFromAnyExeNode(ctx, executionResultInfo.ExecutionNodes, req)
	if err != nil {
		return nil, metadata, status.Errorf(codes.Internal, "failed to retrieve result from execution node: %v", err)
	}

	txStatus, err := e.txStatusDeriver.DeriveFinalizedTransactionStatus(block.Height, true)
	if err != nil {
		return nil, metadata, fmt.Errorf("failed to derive transaction status: %w", err)
	}

	events, err := convert.MessagesToEventsWithEncodingConversion(resp.GetEvents(), resp.GetEventEncodingVersion(), encodingVersion)
	if err != nil {
		return nil, metadata, status.Errorf(codes.Internal, "failed to convert events in blockID %x: %v",
			blockID, err)
	}

	return &accessmodel.TransactionResult{
		Status:       txStatus,
		StatusCode:   uint(resp.GetStatusCode()),
		Events:       events,
		ErrorMessage: resp.GetErrorMessage(),
		BlockID:      blockID,
		BlockHeight:  block.Height,
	}, metadata, nil
}

// TransactionResultsByBlockID retrieves a transaction result from execution nodes by block ID.
//
// Expected error returns during normal operation:
//   - [codes.NotFound] when transaction results or collection from block are not found
//   - [codes.Internal] when data returned by execution node is invalid or inconsistent
//   - [status.Error] when the request to execution node failed
func (e *ENTransactionProvider) TransactionResultsByBlockID(
	ctx context.Context,
	block *flow.Block,
	requiredEventEncodingVersion entities.EventEncodingVersion,
	executionResultInfo *optimistic_sync.ExecutionResultInfo,
) ([]*accessmodel.TransactionResult, accessmodel.ExecutorMetadata, error) {
	blockID := block.ID()
	req := &execproto.GetTransactionsByBlockIDRequest{
		BlockId: blockID[:],
	}

	metadata := accessmodel.ExecutorMetadata{
		ExecutionResultID: executionResultInfo.ExecutionResult.ID(),
		ExecutorIDs:       executionResultInfo.ExecutionNodes.NodeIDs(),
	}

	resp, err := e.getTransactionResultsByBlockIDFromAnyExeNode(ctx, executionResultInfo.ExecutionNodes, req)
	if err != nil {
		return nil, metadata, rpc.ConvertError(err, "failed to retrieve result from execution node", codes.Internal)
	}

	txStatus, err := e.txStatusDeriver.DeriveFinalizedTransactionStatus(block.Height, true)
	if err != nil {
		return nil, metadata, fmt.Errorf("failed to derive transaction status: %w", err)
	}

	results := make([]*accessmodel.TransactionResult, 0, len(resp.TransactionResults))
	i := 0
	for _, guarantee := range block.Payload.Guarantees {
		collection, err := e.collections.LightByID(guarantee.CollectionID)
		if err != nil {
			return nil, metadata, rpc.ConvertStorageError(err)
		}

		for _, txID := range collection.Transactions {
			// bounds check. this means the EN returned fewer transaction results than the transactions  in the block
			if i >= len(resp.TransactionResults) {
				return nil, metadata, status.Errorf(codes.Internal, errTooFewResults.Error())
			}
			txResult := resp.TransactionResults[i]

			events, err := convert.MessagesToEventsWithEncodingConversion(txResult.GetEvents(), resp.GetEventEncodingVersion(), requiredEventEncodingVersion)
			if err != nil {
				return nil, metadata, status.Errorf(codes.Internal, "failed to convert events to message in txID %x: %v", txID, err)
			}

			results = append(results, &accessmodel.TransactionResult{
				Status:        txStatus,
				StatusCode:    uint(txResult.GetStatusCode()),
				Events:        events,
				ErrorMessage:  txResult.GetErrorMessage(),
				BlockID:       blockID,
				TransactionID: txID,
				CollectionID:  guarantee.CollectionID,
				BlockHeight:   block.Height,
			})

			i++
		}
	}

	// after iterating through all transactions  in each collection, i equals the total number of
	// user transactions  in the block
	txCount := i
	sporkRootBlockHeight := e.state.Params().SporkRootBlockHeight()

	// root block has no system transaction result
	if block.Height > sporkRootBlockHeight {
		// system chunk transaction

		// resp.TransactionResultsByBlockID includes the system tx result, so there should be exactly one
		// more result than txCount
		if txCount != len(resp.TransactionResults)-1 {
			if txCount >= len(resp.TransactionResults) {
				return nil, metadata, status.Errorf(codes.Internal, errTooFewResults.Error())
			}
			// otherwise there are extra results
			// TODO(bft): slashable offense
			return nil, metadata, status.Errorf(codes.Internal, errTooManyResults.Error())
		}

		systemTxResult := resp.TransactionResults[len(resp.TransactionResults)-1]
<<<<<<< HEAD
=======
		systemTxStatus, err := e.txStatusDeriver.DeriveFinalizedTransactionStatus(block.Height, true)
		if err != nil {
			irrecoverable.Throw(ctx, fmt.Errorf("failed to derive transaction status: %w", err))
			return nil, err
		}
>>>>>>> illia-malachyn/7652-fork-aware-events-endpoint

		events, err := convert.MessagesToEventsWithEncodingConversion(systemTxResult.GetEvents(), resp.GetEventEncodingVersion(), requiredEventEncodingVersion)
		if err != nil {
			return nil, metadata, status.Errorf(codes.Internal, "failed to convert events from system tx result: %v", err)
		}

		results = append(results, &accessmodel.TransactionResult{
			Status:        txStatus,
			StatusCode:    uint(systemTxResult.GetStatusCode()),
			Events:        events,
			ErrorMessage:  systemTxResult.GetErrorMessage(),
			BlockID:       blockID,
			TransactionID: e.systemTxID,
			BlockHeight:   block.Height,
		})
	}
	return results, metadata, nil
}

// tryGetTransactionResultFromAnyExeNode retrieves a transaction result from execution nodes by block ID and transaction ID.
//
// Expected errors during normal operation:
//   - [codes.Unavailable] when the connection to the execution node fails.
//   - [status.Error] when the GRPC call failed.
func (e *ENTransactionProvider) getTransactionResultFromAnyExeNode(
	ctx context.Context,
	execNodes flow.IdentitySkeletonList,
	req *execproto.GetTransactionResultRequest,
) (*execproto.GetTransactionResultResponse, error) {
	var errToReturn error

	defer func() {
		if errToReturn != nil {
			e.log.Info().Err(errToReturn).Msg("failed to get transaction result from execution nodes")
		}
	}()

	var resp *execproto.GetTransactionResultResponse
	errToReturn = e.nodeCommunicator.CallAvailableNode(
		execNodes,
		func(node *flow.IdentitySkeleton) error {
			var err error
			resp, err = e.tryGetTransactionResult(ctx, node, req)
			if err == nil {
				e.log.Debug().
					Str("execution_node", node.String()).
					Hex("block_id", req.GetBlockId()).
					Hex("transaction_id", req.GetTransactionId()).
					Msg("Successfully got transaction results from any node")
				return nil
			}
			return err
		},
		nil,
	)

	return resp, errToReturn
}

// tryGetTransactionResultsByBlockID retrieves a transaction result from execution nodes by block ID.
//
// Expected errors during normal operation:
//   - [codes.Unavailable] when the connection to the execution node fails.
//   - [status.Error] when the GRPC call failed.
func (e *ENTransactionProvider) getTransactionResultsByBlockIDFromAnyExeNode(
	ctx context.Context,
	execNodes flow.IdentitySkeletonList,
	req *execproto.GetTransactionsByBlockIDRequest,
) (*execproto.GetTransactionResultsResponse, error) {
	var errToReturn error

	defer func() {
		// log the errors
		if errToReturn != nil {
			e.log.Err(errToReturn).Msg("failed to get transaction results from execution nodes")
		}
	}()

	// if we were passed 0 execution nodes add a specific error
	if len(execNodes) == 0 {
		return nil, errors.New("zero execution nodes")
	}

	var resp *execproto.GetTransactionResultsResponse
	errToReturn = e.nodeCommunicator.CallAvailableNode(
		execNodes,
		func(node *flow.IdentitySkeleton) error {
			var err error
			resp, err = e.tryGetTransactionResultsByBlockID(ctx, node, req)
			if err == nil {
				e.log.Debug().
					Str("execution_node", node.String()).
					Hex("block_id", req.GetBlockId()).
					Msg("Successfully got transaction results from any node")
				return nil
			}
			return err
		},
		nil,
	)

	return resp, errToReturn
}

// tryGetTransactionResultByIndex retrieves a transaction result from execution nodes by block ID and index.
//
// Expected errors during normal operation:
//   - [codes.Unavailable] when the connection to the execution node fails.
//   - [status.Error] when the GRPC call failed.
func (e *ENTransactionProvider) getTransactionResultByIndexFromAnyExeNode(
	ctx context.Context,
	execNodes flow.IdentitySkeletonList,
	req *execproto.GetTransactionByIndexRequest,
) (*execproto.GetTransactionResultResponse, error) {
	var errToReturn error
	defer func() {
		if errToReturn != nil {
			e.log.Info().Err(errToReturn).Msg("failed to get transaction result from execution nodes")
		}
	}()

	if len(execNodes) == 0 {
		return nil, errors.New("zero execution nodes provided")
	}

	var resp *execproto.GetTransactionResultResponse
	errToReturn = e.nodeCommunicator.CallAvailableNode(
		execNodes,
		func(node *flow.IdentitySkeleton) error {
			var err error
			resp, err = e.tryGetTransactionResultByIndex(ctx, node, req)
			if err == nil {
				e.log.Debug().
					Str("execution_node", node.String()).
					Hex("block_id", req.GetBlockId()).
					Uint32("index", req.GetIndex()).
					Msg("Successfully got transaction results from any node")
				return nil
			}
			return err
		},
		nil,
	)

	return resp, errToReturn
}

// tryGetTransactionResult retrieves a transaction result from execution nodes by block ID and transaction ID.
//
// Expected errors during normal operation:
//   - [codes.Unavailable] when the connection to the execution node fails.
//   - [status.Error] when the GRPC call failed.
func (e *ENTransactionProvider) tryGetTransactionResult(
	ctx context.Context,
	execNode *flow.IdentitySkeleton,
	req *execproto.GetTransactionResultRequest,
) (*execproto.GetTransactionResultResponse, error) {
	execRPCClient, closer, err := e.connFactory.GetExecutionAPIClient(execNode.Address)
	if err != nil {
		return nil, status.Errorf(codes.Unavailable, "failed to connect to execution node: %v", err)
	}
	defer closer.Close()

	resp, err := execRPCClient.GetTransactionResult(ctx, req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

// tryGetTransactionResultsByBlockID retrieves a transaction result from execution nodes by block ID.
//
// Expected errors during normal operation:
//   - [codes.Unavailable] when the connection to the execution node fails.
//   - [status.Error] when the GRPC call failed.
func (e *ENTransactionProvider) tryGetTransactionResultsByBlockID(
	ctx context.Context,
	execNode *flow.IdentitySkeleton,
	req *execproto.GetTransactionsByBlockIDRequest,
) (*execproto.GetTransactionResultsResponse, error) {
	execRPCClient, closer, err := e.connFactory.GetExecutionAPIClient(execNode.Address)
	if err != nil {
		return nil, status.Errorf(codes.Unavailable, "failed to connect to execution node: %v", err)
	}
	defer closer.Close()

	resp, err := execRPCClient.GetTransactionResultsByBlockID(ctx, req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

// tryGetTransactionResultByIndex retrieves a transaction result from execution nodes by block ID and index.
//
// Expected errors during normal operation:
//   - [codes.Unavailable] when the connection to the execution node fails.
//   - [status.Error] when the GRPC call failed.
func (e *ENTransactionProvider) tryGetTransactionResultByIndex(
	ctx context.Context,
	execNode *flow.IdentitySkeleton,
	req *execproto.GetTransactionByIndexRequest,
) (*execproto.GetTransactionResultResponse, error) {
	execRPCClient, closer, err := e.connFactory.GetExecutionAPIClient(execNode.Address)
	if err != nil {
		return nil, status.Errorf(codes.Unavailable, "failed to connect to execution node: %v", err)
	}
	defer closer.Close()

	resp, err := execRPCClient.GetTransactionResultByIndex(ctx, req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}
