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

	"github.com/onflow/flow-go/engine/access/rpc/backend/common"
	"github.com/onflow/flow-go/engine/access/rpc/backend/node_communicator"
	txstatus "github.com/onflow/flow-go/engine/access/rpc/backend/transactions/status"
	"github.com/onflow/flow-go/engine/access/rpc/connection"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/fvm/blueprints"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// IncorrectResultCountError is returned when the number of transaction results returned by the
// execution node does not match the expected number of transaction results in the block.
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

type ENTransactionProvider struct {
	log     zerolog.Logger
	state   protocol.State
	chainID flow.ChainID

	collections storage.Collections

	connFactory      connection.ConnectionFactory
	nodeCommunicator node_communicator.Communicator
	txStatusDeriver  *txstatus.TxStatusDeriver

	systemTxID                        flow.Identifier
	scheduledCallbacksEnabled         bool
	processScheduledCallbackEventType flow.EventType
}

var _ TransactionProvider = (*ENTransactionProvider)(nil)

func NewENTransactionProvider(
	log zerolog.Logger,
	state protocol.State,
	collections storage.Collections,
	connFactory connection.ConnectionFactory,
	nodeCommunicator node_communicator.Communicator,
	txStatusDeriver *txstatus.TxStatusDeriver,
	systemTxID flow.Identifier,
	chainID flow.ChainID,
	scheduledCallbacksEnabled bool,
) *ENTransactionProvider {
	env := systemcontracts.SystemContractsForChain(chainID).AsTemplateEnv()
	return &ENTransactionProvider{
		log:                               log.With().Str("transaction_provider", "execution_node").Logger(),
		state:                             state,
		collections:                       collections,
		connFactory:                       connFactory,
		nodeCommunicator:                  nodeCommunicator,
		txStatusDeriver:                   txStatusDeriver,
		systemTxID:                        systemTxID,
		chainID:                           chainID,
		scheduledCallbacksEnabled:         scheduledCallbacksEnabled,
		processScheduledCallbackEventType: blueprints.PendingExecutionEventType(env),
	}
}

// TransactionResult retrieves a transaction result from execution nodes by block ID and transaction ID.
//
// Expected error returns during normal operation:
//   - [common.InvalidDataFromExternalNodeError] - when transaction result data returned by execution node is invalid
//   - [common.FailedToQueryExternalNodeError] - when the request to execution node failed
func (e *ENTransactionProvider) TransactionResult(
	ctx context.Context,
	block *flow.Header,
	transactionID flow.Identifier,
	collectionID flow.Identifier,
	encodingVersion entities.EventEncodingVersion,
	executionResultInfo *optimistic_sync.ExecutionResultInfo,
) (*accessmodel.TransactionResult, *accessmodel.ExecutorMetadata, error) {
	blockID := block.ID()
	req := &execproto.GetTransactionResultRequest{
		BlockId:       blockID[:],
		TransactionId: transactionID[:],
	}

	resp, executorID, err := e.getTransactionResultFromAnyExeNode(ctx, executionResultInfo.ExecutionNodes, req)
	if err != nil {
		return nil, nil, common.NewFailedToQueryExternalNodeError(err)
	}

	txStatus, err := e.txStatusDeriver.DeriveFinalizedTransactionStatus(block.Height, true)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to derive transaction status: %w", err)
	}

	events, err := convert.MessagesToEventsWithEncodingConversion(resp.GetEvents(), resp.GetEventEncodingVersion(), encodingVersion)
	if err != nil {
		err = fmt.Errorf("failed to convert event payloads: %w", err)
		return nil, nil, common.NewInvalidDataFromExternalNodeError("events", executorID, err)
	}

	metadata := &accessmodel.ExecutorMetadata{
		ExecutionResultID: executionResultInfo.ExecutionResultID,
		ExecutorIDs:       executionResultInfo.ExecutionNodes.NodeIDs(),
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
//   - [common.InvalidDataFromExternalNodeError] - when data returned by execution node is invalid
//   - [common.FailedToQueryExternalNodeError] - when the request to execution node failed
func (e *ENTransactionProvider) TransactionResultByIndex(
	ctx context.Context,
	header *flow.Header,
	index uint32,
	collectionID flow.Identifier,
	encodingVersion entities.EventEncodingVersion,
	executionResultInfo *optimistic_sync.ExecutionResultInfo,
) (*accessmodel.TransactionResult, *accessmodel.ExecutorMetadata, error) {
	blockID := header.ID()
	req := &execproto.GetTransactionByIndexRequest{
		BlockId: blockID[:],
		Index:   index,
	}

	resp, executorID, err := e.getTransactionResultByIndexFromAnyExeNode(ctx, executionResultInfo.ExecutionNodes, req)
	if err != nil {
		return nil, nil, common.NewFailedToQueryExternalNodeError(err)
	}

	txStatus, err := e.txStatusDeriver.DeriveFinalizedTransactionStatus(header.Height, true)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to derive transaction status: %w", err)
	}

	events, err := convert.MessagesToEventsWithEncodingConversion(resp.GetEvents(), resp.GetEventEncodingVersion(), encodingVersion)
	if err != nil {
		err = fmt.Errorf("failed to convert event payloads: %w", err)
		return nil, nil, common.NewInvalidDataFromExternalNodeError("events", executorID, err)
	}

	metadata := &accessmodel.ExecutorMetadata{
		ExecutionResultID: executionResultInfo.ExecutionResultID,
		ExecutorIDs:       executionResultInfo.ExecutionNodes.NodeIDs(),
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
//   - [ErrBlockCollectionNotFound] - when a collection from the block is not found
//   - [NewIncorrectResultCountError] - when the number of transaction results returned by the execution
//     node is less than the number of transactions in the block
//   - [common.InvalidDataFromExternalNodeError] - when data returned by execution node is invalid
//   - [common.FailedToQueryExternalNodeError] - when the request to execution node failed
func (e *ENTransactionProvider) TransactionResultsByBlockID(
	ctx context.Context,
	block *flow.Block,
	encodingVersion entities.EventEncodingVersion,
	executionResultInfo *optimistic_sync.ExecutionResultInfo,
) ([]*accessmodel.TransactionResult, *accessmodel.ExecutorMetadata, error) {
	blockID := block.ID()
	req := &execproto.GetTransactionsByBlockIDRequest{
		BlockId: blockID[:],
	}

	executionResponse, executorID, err := e.getTransactionResultsByBlockIDFromAnyExeNode(ctx, executionResultInfo.ExecutionNodes, req)
	if err != nil {
		return nil, nil, common.NewFailedToQueryExternalNodeError(err)
	}

	txStatus, err := e.txStatusDeriver.DeriveFinalizedTransactionStatus(block.Height, true)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to derive transaction status: %w", err)
	}

	userTxResults, err := e.userTransactionResults(
		executionResponse,
		executorID,
		block,
		blockID,
		txStatus,
		encodingVersion,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to process user transaction results: %w", err)
	}

	metadata := &accessmodel.ExecutorMetadata{
		ExecutionResultID: executionResultInfo.ExecutionResultID,
		ExecutorIDs:       common.OrderedExecutors(executorID, executionResultInfo.ExecutionNodes.NodeIDs()),
	}

	// root block has no system transaction result
	if block.View == e.state.Params().SporkRootBlockView() {
		return userTxResults, metadata, nil
	}

	// there must be at least one system transaction result
	if len(userTxResults) >= len(executionResponse.TransactionResults) {
		return nil, nil, NewIncorrectResultCountError(len(userTxResults)+1, len(executionResponse.TransactionResults))
	}

	remainingTxResults := executionResponse.TransactionResults[len(userTxResults):]

	systemTxResults, err := e.systemTransactionResults(
		remainingTxResults,
		executorID,
		block,
		blockID,
		txStatus,
		executionResponse,
		encodingVersion,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to process system transaction results: %w", err)
	}

	return append(userTxResults, systemTxResults...), metadata, nil
}

// TransactionsByBlockID retrieves transactions by block ID.
//
// Expected error returns during normal operation:
//   - [ErrBlockCollectionNotFound] - when a collection from the block is not found
//   - [common.InvalidDataFromExternalNodeError] - when data returned by execution node is invalid
//   - [common.FailedToQueryExternalNodeError] - when the request to execution node failed
func (e *ENTransactionProvider) TransactionsByBlockID(
	ctx context.Context,
	block *flow.Block,
	executionResultInfo *optimistic_sync.ExecutionResultInfo,
) ([]*flow.TransactionBody, *accessmodel.ExecutorMetadata, error) {
	// user transactions
	var transactions []*flow.TransactionBody
	for _, guarantee := range block.Payload.Guarantees {
		collection, err := e.collections.ByID(guarantee.CollectionID)
		if err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				return nil, nil, ErrBlockCollectionNotFound
			}
			return nil, nil, fmt.Errorf("failed to get collection: %w", err)
		}

		transactions = append(transactions, collection.Transactions...)
	}

	metadata := &accessmodel.ExecutorMetadata{
		ExecutionResultID: executionResultInfo.ExecutionResultID,
		ExecutorIDs:       executionResultInfo.ExecutionNodes.NodeIDs(),
	}

	// system transactions
	// TODO: implement system that allows this endpoint to dynamically determine if scheduled
	// transactions were enabled for this block. See https://github.com/onflow/flow-go/issues/7873
	if !e.scheduledCallbacksEnabled {
		systemTx, err := blueprints.SystemChunkTransaction(e.chainID.Chain())
		if err != nil {
			return nil, nil, fmt.Errorf("failed to construct system chunk transaction: %w", err)
		}

		return append(transactions, systemTx), metadata, nil
	}

	events, executorID, err := e.getBlockEvents(ctx, block.ID(), e.processScheduledCallbackEventType, executionResultInfo)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get events for scheduled transaction parsing: %w", err)
	}

	sysCollection, err := blueprints.SystemCollection(e.chainID.Chain(), events)
	if err != nil {
		// this means either the execution node provided invalid event data, or there is a bug. Since
		// we cannot determine easily from the error, assume that it was an issue with the execution
		// response. If it was an issue with the software building the system collection, error details
		// will be included in the response to the user, and can be surfaced to the operator.
		return nil, nil, common.NewInvalidDataFromExternalNodeError("process scheduled transaction events", executorID, err)
	}

	return append(transactions, sysCollection.Transactions...), metadata, nil
}

// SystemTransaction retrieves a system transaction from execution nodes.
//
// Expected error returns during normal operation:
//   - [ErrNotASystemTransaction] - when the requested transaction is not a system transaction
//   - [common.InvalidDataFromExternalNodeError] - when events data returned by execution node is invalid
//   - [common.FailedToQueryExternalNodeError] - when the request to execution node failed
func (e *ENTransactionProvider) SystemTransaction(
	ctx context.Context,
	block *flow.Block,
	txID flow.Identifier,
	executionResultInfo *optimistic_sync.ExecutionResultInfo,
) (*flow.TransactionBody, *accessmodel.ExecutorMetadata, error) {
	blockID := block.ID()

	metadata := &accessmodel.ExecutorMetadata{
		ExecutionResultID: executionResultInfo.ExecutionResultID,
		ExecutorIDs:       executionResultInfo.ExecutionNodes.NodeIDs(),
	}

	if txID == e.systemTxID || !e.scheduledCallbacksEnabled {
		systemTx, err := blueprints.SystemChunkTransaction(e.chainID.Chain())
		if err != nil {
			return nil, nil, fmt.Errorf("failed to construct system chunk transaction: %w", err)
		}

		if txID == systemTx.ID() {
			return systemTx, metadata, nil
		}
		return nil, nil, fmt.Errorf("transaction %s not found in block %s", txID, blockID)
	}

	events, executorID, err := e.getBlockEvents(ctx, blockID, e.processScheduledCallbackEventType, executionResultInfo)
	if err != nil {
		return nil, nil, common.NewFailedToQueryExternalNodeError(err)
	}

	sysCollection, err := blueprints.SystemCollection(e.chainID.Chain(), events)
	if err != nil {
		// this means either the execution node provided invalid event data, or there is a bug. Since
		// we cannot determine easily from the error, assume that it was an issue with the execution
		// response. If it was an issue with the software building the system collection, error details
		// will be included in the response to the user, and can be surfaced to the operator.
		return nil, nil, common.NewInvalidDataFromExternalNodeError("process scheduled transaction events", executorID, err)
	}

	for _, tx := range sysCollection.Transactions {
		if tx.ID() == txID {
			return tx, metadata, nil
		}
	}

	return nil, nil, ErrNotASystemTransaction
}

// SystemTransactionResult retrieves a system transaction result from execution nodes.
//
// Expected error returns during normal operation:
//   - [ErrNotASystemTransaction] - when the requested transaction is not a system transaction
//   - [common.InvalidDataFromExternalNodeError] - when transaction result data returned by execution node is invalid
//   - [common.FailedToQueryExternalNodeError] - when the request to execution node failed
func (e *ENTransactionProvider) SystemTransactionResult(
	ctx context.Context,
	block *flow.Block,
	txID flow.Identifier,
	encodingVersion entities.EventEncodingVersion,
	executionResultInfo *optimistic_sync.ExecutionResultInfo,
) (*accessmodel.TransactionResult, *accessmodel.ExecutorMetadata, error) {
	if txID != e.systemTxID {
		if _, metadata, err := e.SystemTransaction(ctx, block, txID, executionResultInfo); err != nil {
			return nil, metadata, err
		}
	}
	return e.TransactionResult(ctx, block.ToHeader(), txID, flow.ZeroID, encodingVersion, executionResultInfo)
}

// userTransactionResults constructs the user transaction results from the execution node response.
//
// Expected error returns during normal operation:
//   - [ErrBlockCollectionNotFound] - when a collection from the block is not found
//   - [NewIncorrectResultCountError] - when the number of transaction results returned by the execution
//     node is less than the number of transactions in the block
//   - [common.InvalidDataFromExternalNodeError] - when data returned by execution node is invalid
//   - [common.FailedToQueryExternalNodeError] - when the request to execution node failed
func (e *ENTransactionProvider) userTransactionResults(
	resp *execproto.GetTransactionResultsResponse,
	executorID flow.Identifier,
	block *flow.Block,
	blockID flow.Identifier,
	txStatus flow.TransactionStatus,
	encodingVersion entities.EventEncodingVersion,
) ([]*accessmodel.TransactionResult, error) {
	results := make([]*accessmodel.TransactionResult, 0, len(resp.TransactionResults))
	i := 0
	for _, guarantee := range block.Payload.Guarantees {
		collection, err := e.collections.LightByID(guarantee.CollectionID)
		if err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				return nil, ErrBlockCollectionNotFound
			}
			return nil, fmt.Errorf("failed to get collection: %w", err)
		}

		for _, txID := range collection.Transactions {
			// bounds check. this means the EN returned fewer transaction results than the transactions  in the block
			if i >= len(resp.TransactionResults) {
				return nil, NewIncorrectResultCountError(i, len(resp.TransactionResults))
			}
			txResult := resp.TransactionResults[i]

			events, err := convert.MessagesToEventsWithEncodingConversion(txResult.GetEvents(), resp.GetEventEncodingVersion(), encodingVersion)
			if err != nil {
				return nil, common.NewInvalidDataFromExternalNodeError("events", executorID, fmt.Errorf("failed to convert event payloads for txID %s: %v", txID, err))
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

	return results, nil
}

// systemTransactionResults constructs the system transaction results from the execution node response.
//
// It does so by iterating through all system transactions in the block and constructing the transaction results.
// System transactions are transactions that follow the user transactions from the execution node response.
// We should always return transaction result for system chunk transaction, but if scheduled callbacks are enabled
// we also return results for the process and execute callbacks transactions.
//
// Expected error returns during normal operation:
//   - [NewIncorrectResultCountError] - when the number of system transactions returned by the execution
//     node does not match the number of expected number of system transactions in the block
//   - [common.InvalidDataFromExternalNodeError] - when the events data returned by execution node is invalid
//   - [common.FailedToQueryExternalNodeError] - when the events request to execution node failed
func (e *ENTransactionProvider) systemTransactionResults(
	systemTxResults []*execproto.GetTransactionResultResponse,
	executorID flow.Identifier,
	block *flow.Block,
	blockID flow.Identifier,
	txStatus flow.TransactionStatus,
	resp *execproto.GetTransactionResultsResponse,
	encodingVersion entities.EventEncodingVersion,
) ([]*accessmodel.TransactionResult, error) {
	systemTxIDs, err := e.systemTransactionIDs(systemTxResults, executorID, resp.GetEventEncodingVersion())
	if err != nil {
		return nil, fmt.Errorf("failed to determine system transaction IDs: %w", err)
	}

	// systemTransactionIDs automatically detects if scheduled callbacks was enabled for the block
	// based on the number of system transactions in the response. The resulting list should always
	// have the same length as the number of system transactions in the response.
	if len(systemTxIDs) != len(systemTxResults) {
		return nil, fmt.Errorf("system transaction count mismatch: %w", NewIncorrectResultCountError(len(systemTxResults), len(systemTxIDs)))
	}

	results := make([]*accessmodel.TransactionResult, 0, len(systemTxResults))
	for i, systemTxResult := range systemTxResults {
		events, err := convert.MessagesToEventsWithEncodingConversion(systemTxResult.GetEvents(), resp.GetEventEncodingVersion(), encodingVersion)
		if err != nil {
			return nil, common.NewInvalidDataFromExternalNodeError("events", executorID, err)
		}

		results = append(results, &accessmodel.TransactionResult{
			Status:        txStatus,
			StatusCode:    uint(systemTxResult.GetStatusCode()),
			Events:        events,
			ErrorMessage:  systemTxResult.GetErrorMessage(),
			BlockID:       blockID,
			TransactionID: systemTxIDs[i],
			CollectionID:  flow.ZeroID,
			BlockHeight:   block.Height,
		})
	}

	return results, nil
}

// systemTransactionIDs returns the list of system transaction IDs for the block.
//
// Expected error returns during normal operation:
//   - [common.InvalidDataFromExternalNodeError] - when the events data returned by execution node is invalid
//   - [common.FailedToQueryExternalNodeError] - when the events request to execution node failed
func (e *ENTransactionProvider) systemTransactionIDs(
	systemTxResults []*execproto.GetTransactionResultResponse,
	executorID flow.Identifier,
	actualEventEncodingVersion entities.EventEncodingVersion,
) ([]flow.Identifier, error) {
	// TODO: implement system that allows this endpoint to dynamically determine if scheduled
	// transactions were enabled for this block. See https://github.com/onflow/flow-go/issues/7873
	if len(systemTxResults) == 1 {
		return []flow.Identifier{e.systemTxID}, nil
	}

	// if scheduled callbacks are enabled, the first transaction will always be the "process" transaction
	// get its events to reconstruct the system collection
	processResult := systemTxResults[0]

	// blueprints.SystemCollection requires events are CCF encoded
	events, err := convert.MessagesToEventsWithEncodingConversion(
		processResult.GetEvents(),
		actualEventEncodingVersion,
		entities.EventEncodingVersion_CCF_V0,
	)
	if err != nil {
		return nil, common.NewInvalidDataFromExternalNodeError("events", executorID, err)
	}

	sysCollection, err := blueprints.SystemCollection(e.chainID.Chain(), events)
	if err != nil {
		return nil, common.NewInvalidDataFromExternalNodeError("process scheduled transaction events", executorID, err)
	}

	var systemTxIDs []flow.Identifier
	for _, tx := range sysCollection.Transactions {
		systemTxIDs = append(systemTxIDs, tx.ID())
	}

	return systemTxIDs, nil
}

// getBlockEvents retrieves the events for the given block ID and event type from execution nodes
// included in the execution result info.
//
// Expected error returns during normal operation:
//   - [common.InvalidDataFromExternalNodeError] - when data returned by execution node is invalid
//   - [common.FailedToQueryExternalNodeError] - when the request to execution node failed
func (e *ENTransactionProvider) getBlockEvents(
	ctx context.Context,
	blockID flow.Identifier,
	eventType flow.EventType,
	executionResultInfo *optimistic_sync.ExecutionResultInfo,
) (flow.EventsList, flow.Identifier, error) {
	request := &execproto.GetEventsForBlockIDsRequest{
		BlockIds: [][]byte{blockID[:]},
		Type:     string(eventType),
	}

	resp, executorID, err := e.getBlockEventsByBlockIDsFromAnyExeNode(ctx, executionResultInfo.ExecutionNodes, request)
	if err != nil {
		return nil, flow.ZeroID, common.NewFailedToQueryExternalNodeError(err)
	}

	var events flow.EventsList
	for _, result := range resp.GetResults() {
		resultEvents, err := convert.MessagesToEventsWithEncodingConversion(
			result.GetEvents(),
			resp.GetEventEncodingVersion(),
			entities.EventEncodingVersion_CCF_V0,
		)
		if err != nil {
			return nil, flow.ZeroID, common.NewInvalidDataFromExternalNodeError("events", executorID, err)
		}
		events = append(events, resultEvents...)
	}

	return events, executorID, nil
}

// tryGetTransactionResultFromAnyExeNode retrieves a transaction result from execution nodes by block ID and transaction ID.
//
// Expected error returns during normal operation:
//   - [codes.Unavailable] - when the connection to the execution node fails.
//   - [status.Error] - when the GRPC call failed.
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
// Expected error returns during normal operation:
//   - [codes.Unavailable] - when the connection to the execution node fails.
//   - [status.Error] - when the GRPC call failed.
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
// Expected error returns during normal operation:
//   - [codes.Unavailable] - when the connection to the execution node fails.
//   - [status.Error] - when the GRPC call failed.
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

// getBlockEventsByBlockIDsFromAnyExeNode retrieves events from execution nodes by block IDs.
//
// Expected error returns during normal operation:
//   - [codes.Unavailable] - when the connection to the execution node fails.
//   - [status.Error] - when the GRPC call failed.
func (e *ENTransactionProvider) getBlockEventsByBlockIDsFromAnyExeNode(
	ctx context.Context,
	execNodes flow.IdentitySkeletonList,
	req *execproto.GetEventsForBlockIDsRequest,
) (*execproto.GetEventsForBlockIDsResponse, flow.Identifier, error) {
	var resp *execproto.GetEventsForBlockIDsResponse
	executionNode, errToReturn := e.nodeCommunicator.CallAvailableNode(
		execNodes,
		func(node *flow.IdentitySkeleton) error {
			var err error
			resp, err = e.tryGetBlockEventsByBlockIDs(ctx, node, req)
			return err
		},
		nil,
	)
	return resp, executionNode.NodeID, errToReturn
}

// tryGetTransactionResult retrieves a transaction result from execution nodes by block ID and transaction ID.
//
// Expected error returns during normal operation:
//   - [codes.Unavailable] - when the connection to the execution node fails.
//   - [status.Error] - when the GRPC call failed.
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
// Expected error returns during normal operation:
//   - [codes.Unavailable] - when the connection to the execution node fails.
//   - [status.Error] - when the GRPC call failed.
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
// Expected error returns during normal operation:
//   - [codes.Unavailable] - when the connection to the execution node fails.
//   - [status.Error] - when the GRPC call failed.
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

// tryGetBlockEventsByBlockIDs retrieves events from execution nodes by block IDs.
//
// Expected error returns during normal operation:
//   - [codes.Unavailable] - when the connection to the execution node fails.
//   - [status.Error] - when the GRPC call failed.
func (e *ENTransactionProvider) tryGetBlockEventsByBlockIDs(
	ctx context.Context,
	execNode *flow.IdentitySkeleton,
	req *execproto.GetEventsForBlockIDsRequest,
) (*execproto.GetEventsForBlockIDsResponse, error) {
	execRPCClient, closer, err := e.connFactory.GetExecutionAPIClient(execNode.Address)
	if err != nil {
		return nil, status.Errorf(codes.Unavailable, "failed to connect to execution node: %v", err)
	}
	defer closer.Close()

	resp, err := execRPCClient.GetEventsForBlockIDs(ctx, req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}
