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
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

type IncorrectResultCountError struct {
	expected int
	actual   int
}

func NewIncorrectResultCountError(expected, actual int) *IncorrectResultCountError {
	return &IncorrectResultCountError{
		expected: expected,
		actual:   actual,
	}
}

func (e *IncorrectResultCountError) Error() string {
	return fmt.Sprintf("execution node returned an incorrect number of transaction results. expected %d, got %d",
		e.expected, e.actual)
}

func IsIncorrectResultCountError(err error) bool {
	var target *IncorrectResultCountError
	return errors.As(err, &target)
}

type InvalidDataFromExternalNodeError struct {
	dataType string
	err      error
	nodeID   flow.Identifier
}

func NewInvalidDataFromExternalNodeError(dataType string, nodeID flow.Identifier, err error) *InvalidDataFromExternalNodeError {
	return &InvalidDataFromExternalNodeError{
		dataType: dataType,
		nodeID:   nodeID,
		err:      err,
	}
}

func (e *InvalidDataFromExternalNodeError) Error() string {
	return fmt.Sprintf("received invalid %s data from external node %s: %v", e.dataType, e.nodeID, e.err)
}

func (e *InvalidDataFromExternalNodeError) Unwrap() error {
	return e.err
}

func IsInvalidDataFromExternalNodeError(err error) bool {
	var target *InvalidDataFromExternalNodeError
	return errors.As(err, &target)
}

type FailedToQueryExternalNodeError struct {
	err error
}

func NewFailedToQueryExternalNodeError(err error) *FailedToQueryExternalNodeError {
	return &FailedToQueryExternalNodeError{
		err: err,
	}
}

func (e *FailedToQueryExternalNodeError) Error() string {
	return fmt.Sprintf("failed to query external node: %v", e.err)
}

func (e *FailedToQueryExternalNodeError) Unwrap() error {
	return e.err
}

func IsFailedToQueryExternalNodeError(err error) bool {
	var target *FailedToQueryExternalNodeError
	return errors.As(err, &target)
}

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
//   - [InvalidDataFromExternalNodeError] when data returned by execution node is invalid
//   - [FailedToQueryExternalNodeError] when the request to execution node failed
func (e *ENTransactionProvider) TransactionResult(
	ctx context.Context,
	block *flow.Header,
	transactionID flow.Identifier,
	collectionID flow.Identifier,
	encodingVersion entities.EventEncodingVersion,
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

	resp, executorID, err := e.getTransactionResultFromAnyExeNode(ctx, executionResultInfo.ExecutionNodes, req)
	if err != nil {
		return nil, metadata, NewFailedToQueryExternalNodeError(err)
	}

	txStatus, err := e.txStatusDeriver.DeriveFinalizedTransactionStatus(block.Height, true)
	if err != nil {
		return nil, metadata, fmt.Errorf("failed to derive transaction status: %w", err)
	}

	events, err := convert.MessagesToEventsWithEncodingConversion(resp.GetEvents(), resp.GetEventEncodingVersion(), encodingVersion)
	if err != nil {
		err = fmt.Errorf("failed to convert event payloads: %w", err)
		return nil, metadata, NewInvalidDataFromExternalNodeError("events", executorID, err)
	}

	return &accessmodel.TransactionResult{
		TransactionID: transactionID,
		Status:        txStatus,
		StatusCode:    uint(resp.GetStatusCode()),
		Events:        events,
		ErrorMessage:  resp.GetErrorMessage(),
		BlockID:       blockID,
		BlockHeight:   block.Height,
		CollectionID:  collectionID,
	}, metadata, nil
}

// TransactionResultByIndex retrieves a transaction result from execution nodes by block ID and index.
//
// Expected error returns during normal operation:
//   - [InvalidDataFromExternalNodeError] when data returned by execution node is invalid
//   - [FailedToQueryExternalNodeError] when the request to execution node failed
func (e *ENTransactionProvider) TransactionResultByIndex(
	ctx context.Context,
	header *flow.Header,
	index uint32,
	collectionID flow.Identifier,
	encodingVersion entities.EventEncodingVersion,
	executionResultInfo *optimistic_sync.ExecutionResultInfo,
) (*accessmodel.TransactionResult, accessmodel.ExecutorMetadata, error) {
	blockID := header.ID()
	req := &execproto.GetTransactionByIndexRequest{
		BlockId: blockID[:],
		Index:   index,
	}

	metadata := accessmodel.ExecutorMetadata{
		ExecutionResultID: executionResultInfo.ExecutionResult.ID(),
		ExecutorIDs:       executionResultInfo.ExecutionNodes.NodeIDs(),
	}

	resp, executorID, err := e.getTransactionResultByIndexFromAnyExeNode(ctx, executionResultInfo.ExecutionNodes, req)
	if err != nil {
		return nil, metadata, NewFailedToQueryExternalNodeError(err)
	}

	txStatus, err := e.txStatusDeriver.DeriveFinalizedTransactionStatus(header.Height, true)
	if err != nil {
		return nil, metadata, fmt.Errorf("failed to derive transaction status: %w", err)
	}

	events, err := convert.MessagesToEventsWithEncodingConversion(resp.GetEvents(), resp.GetEventEncodingVersion(), encodingVersion)
	if err != nil {
		err = fmt.Errorf("failed to convert event payloads: %w", err)
		return nil, metadata, NewInvalidDataFromExternalNodeError("events", executorID, err)
	}

	return &accessmodel.TransactionResult{
		Status:       txStatus,
		StatusCode:   uint(resp.GetStatusCode()),
		Events:       events,
		ErrorMessage: resp.GetErrorMessage(),
		BlockID:      blockID,
		BlockHeight:  header.Height,
		CollectionID: collectionID,
	}, metadata, nil
}

// TransactionResultsByBlockID retrieves a transaction result from execution nodes by block ID.
//
// Expected error returns during normal operation:
//   - [storage.ErrNotFound] when collection from block is not found
//   - [InvalidDataFromExternalNodeError] when data returned by execution node is invalid
//   - [FailedToQueryExternalNodeError] when the request to execution node failed
func (e *ENTransactionProvider) TransactionResultsByBlockID(
	ctx context.Context,
	block *flow.Block,
	encodingVersion entities.EventEncodingVersion,
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

	resp, executorID, err := e.getTransactionResultsByBlockIDFromAnyExeNode(ctx, executionResultInfo.ExecutionNodes, req)
	if err != nil {
		return nil, metadata, NewFailedToQueryExternalNodeError(err)
	}

	txStatus, err := e.txStatusDeriver.DeriveFinalizedTransactionStatus(block.Height, true)
	if err != nil {
		return nil, metadata, fmt.Errorf("failed to derive transaction status: %w", err)
	}

	// track the number of user transactions in the block
	txCount := 0

	results := make([]*accessmodel.TransactionResult, 0, len(resp.TransactionResults))
	for _, guarantee := range block.Payload.Guarantees {
		collection, err := e.collections.LightByID(guarantee.CollectionID)
		if err != nil {
			return nil, metadata, fmt.Errorf("could not find collection %s: %w", guarantee.CollectionID, err)
		}

		for _, txID := range collection.Transactions {
			// bounds check. this means the EN returned fewer transaction results than the transactions in the block
			if txCount >= len(resp.TransactionResults) {
				err := NewIncorrectResultCountError(len(resp.TransactionResults)-1, txCount)
				return nil, metadata, NewInvalidDataFromExternalNodeError("transaction results", executorID, err)
			}
			txResult := resp.TransactionResults[txCount]

			events, err := convert.MessagesToEventsWithEncodingConversion(txResult.GetEvents(), resp.GetEventEncodingVersion(), encodingVersion)
			if err != nil {
				err = fmt.Errorf("failed to convert event payloads for tx %s: %w", txID, err)
				return nil, metadata, NewInvalidDataFromExternalNodeError("events", executorID, err)
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

			txCount++
		}
	}

	// after iterating through all transactions  in each collection, i equals the total number of
	// user transactions  in the block
	sporkRootBlockHeight := e.state.Params().SporkRootBlockHeight()

	// root block has no system transaction result
	if block.Height > sporkRootBlockHeight {
		// system chunk transaction

		// response should include the system tx result, so there should be exactly one more result
		// than txCount
		if txCount != len(resp.TransactionResults)-1 {
			err := NewIncorrectResultCountError(len(resp.TransactionResults)-1, txCount)
			return nil, metadata, NewInvalidDataFromExternalNodeError("transaction results", executorID, err)
		}

		systemTxResult := resp.TransactionResults[len(resp.TransactionResults)-1]

		events, err := convert.MessagesToEventsWithEncodingConversion(systemTxResult.GetEvents(), resp.GetEventEncodingVersion(), encodingVersion)
		if err != nil {
			err = fmt.Errorf("failed to convert event payloads for system tx: %w", err)
			return nil, metadata, NewInvalidDataFromExternalNodeError("events", executorID, err)
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
) (*execproto.GetTransactionResultResponse, flow.Identifier, error) {
	var resp *execproto.GetTransactionResultResponse
	executionNode, errToReturn := e.nodeCommunicator.CallAvailableNode(
		execNodes,
		func(node *flow.IdentitySkeleton) error {
			var err error
			resp, err = e.tryGetTransactionResult(ctx, node, req)
			return err
		},
		nil,
	)

	return resp, executionNode.NodeID, errToReturn
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
) (*execproto.GetTransactionResultsResponse, flow.Identifier, error) {
	var resp *execproto.GetTransactionResultsResponse
	executionNode, errToReturn := e.nodeCommunicator.CallAvailableNode(
		execNodes,
		func(node *flow.IdentitySkeleton) error {
			var err error
			resp, err = e.tryGetTransactionResultsByBlockID(ctx, node, req)
			return err
		},
		nil,
	)

	return resp, executionNode.NodeID, errToReturn
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
) (*execproto.GetTransactionResultResponse, flow.Identifier, error) {
	var resp *execproto.GetTransactionResultResponse
	executionNode, errToReturn := e.nodeCommunicator.CallAvailableNode(
		execNodes,
		func(node *flow.IdentitySkeleton) error {
			var err error
			resp, err = e.tryGetTransactionResultByIndex(ctx, node, req)
			return err
		},
		nil,
	)

	return resp, executionNode.NodeID, errToReturn
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
