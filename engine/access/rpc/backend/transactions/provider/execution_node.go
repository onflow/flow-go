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
	"github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/fvm/blueprints"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/access/systemcollection"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

type ENTransactionProvider struct {
	log     zerolog.Logger
	state   protocol.State
	chainID flow.ChainID

	collections storage.Collections

	connFactory      connection.ConnectionFactory
	nodeCommunicator node_communicator.Communicator
	nodeProvider     *rpc.ExecutionNodeIdentitiesProvider

	txStatusDeriver *txstatus.TxStatusDeriver

	systemCollections                    *systemcollection.Versioned
	processScheduledTransactionEventType flow.EventType
}

var _ TransactionProvider = (*ENTransactionProvider)(nil)

func NewENTransactionProvider(
	log zerolog.Logger,
	state protocol.State,
	collections storage.Collections,
	connFactory connection.ConnectionFactory,
	nodeCommunicator node_communicator.Communicator,
	execNodeIdentitiesProvider *rpc.ExecutionNodeIdentitiesProvider,
	txStatusDeriver *txstatus.TxStatusDeriver,
	systemCollections *systemcollection.Versioned,
	chainID flow.ChainID,
) *ENTransactionProvider {
	env := systemcontracts.SystemContractsForChain(chainID).AsTemplateEnv()
	return &ENTransactionProvider{
		log:                                  log.With().Str("transaction_provider", "execution_node").Logger(),
		state:                                state,
		collections:                          collections,
		connFactory:                          connFactory,
		nodeCommunicator:                     nodeCommunicator,
		nodeProvider:                         execNodeIdentitiesProvider,
		txStatusDeriver:                      txStatusDeriver,
		systemCollections:                    systemCollections,
		chainID:                              chainID,
		processScheduledTransactionEventType: blueprints.PendingExecutionEventType(env),
	}
}

func (e *ENTransactionProvider) TransactionResult(
	ctx context.Context,
	block *flow.Header,
	transactionID flow.Identifier,
	collectionID flow.Identifier,
	requiredEventEncodingVersion entities.EventEncodingVersion,
) (*accessmodel.TransactionResult, error) {
	blockID := block.ID()
	// create an execution API request for events at blockID and transactionID
	req := &execproto.GetTransactionResultRequest{
		BlockId:       blockID[:],
		TransactionId: transactionID[:],
	}

	execNodes, err := e.nodeProvider.ExecutionNodesForBlockID(
		ctx,
		blockID,
	)
	if err != nil {
		// if no execution receipt were found, return a NotFound GRPC error
		if common.IsInsufficientExecutionReceipts(err) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, err
	}

	resp, err := e.getTransactionResultFromAnyExeNode(ctx, execNodes, req)
	if err != nil {
		return nil, err
	}

	// tx body is irrelevant to status if it's in an executed block
	txStatus, err := e.txStatusDeriver.DeriveTransactionStatus(block.Height, true)
	if err != nil {
		// this is an executed transaction. If we can't derive transaction status something is very wrong.
		irrecoverable.Throw(ctx, fmt.Errorf("failed to derive transaction status: %w", err))
		return nil, err
	}

	events, err := convert.MessagesToEventsWithEncodingConversion(resp.GetEvents(), resp.GetEventEncodingVersion(), requiredEventEncodingVersion)
	if err != nil {
		return nil, rpc.ConvertError(err, "failed to convert events to message", codes.Internal)
	}

	return &accessmodel.TransactionResult{
		TransactionID:   transactionID,
		Status:          txStatus,
		StatusCode:      uint(resp.GetStatusCode()),
		Events:          events,
		ErrorMessage:    resp.GetErrorMessage(),
		BlockID:         blockID,
		BlockHeight:     block.Height,
		CollectionID:    collectionID,
		ComputationUsed: resp.GetComputationUsage(),
	}, nil
}

// TransactionsByBlockID returns the transaction for the given block ID.
//
// Expected error returns during normal operation:
//   - [codes.NotFound]: If the requested data is not found.
//   - [codes.Unavailable]: If no nodes are available or a connection to an execution node cannot be established.
//   - [codes.Internal]: If the system collection cannot be constructed.
func (e *ENTransactionProvider) TransactionsByBlockID(
	ctx context.Context,
	block *flow.Block,
) ([]*flow.TransactionBody, error) {
	var transactions []*flow.TransactionBody
	blockID := block.ID()

	// user transactions
	for _, guarantee := range block.Payload.Guarantees {
		collection, err := e.collections.ByID(guarantee.CollectionID)
		if err != nil {
			return nil, rpc.ConvertStorageError(err)
		}

		transactions = append(transactions, collection.Transactions...)
	}

	eventProvider := func() (flow.EventsList, error) {
		return e.getBlockEvents(ctx, blockID, e.processScheduledTransactionEventType)
	}

	systemCollection, err := e.systemCollections.
		ByHeight(block.Height).
		SystemCollection(e.chainID.Chain(), eventProvider)
	if err != nil {
		return nil, rpc.ConvertError(err, "could not construct system collection", codes.Internal)
	}

	return append(transactions, systemCollection.Transactions...), nil
}

func (e *ENTransactionProvider) TransactionResultByIndex(
	ctx context.Context,
	block *flow.Block,
	index uint32,
	collectionID flow.Identifier,
	encodingVersion entities.EventEncodingVersion,
) (*accessmodel.TransactionResult, error) {
	blockID := block.ID()
	// create request and forward to EN
	req := &execproto.GetTransactionByIndexRequest{
		BlockId: blockID[:],
		Index:   index,
	}

	// first, try to find the transaction ID within the locally available data.
	// if it's not found, then it's unlikely the EN will have the data either.
	txID, err := e.getTransactionIDByIndex(ctx, block, index)
	if err != nil {
		return nil, err
	}

	execNodes, err := e.nodeProvider.ExecutionNodesForBlockID(
		ctx,
		blockID,
	)
	if err != nil {
		if common.IsInsufficientExecutionReceipts(err) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, rpc.ConvertError(err, "failed to retrieve result from any execution node", codes.Internal)
	}

	resp, err := e.getTransactionResultByIndexFromAnyExeNode(ctx, execNodes, req)
	if err != nil {
		return nil, rpc.ConvertError(err, "failed to retrieve result from execution node", codes.Internal)
	}

	// tx body is irrelevant to status if it's in an executed block
	txStatus, err := e.txStatusDeriver.DeriveTransactionStatus(block.Height, true)
	if err != nil {
		irrecoverable.Throw(ctx, fmt.Errorf("failed to derive transaction status: %w", err))
		return nil, err
	}

	events, err := convert.MessagesToEventsWithEncodingConversion(resp.GetEvents(), resp.GetEventEncodingVersion(), encodingVersion)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to convert events in blockID %x: %v", blockID, err)
	}

	// convert to response, cache and return
	return &accessmodel.TransactionResult{
		Status:          txStatus,
		StatusCode:      uint(resp.GetStatusCode()),
		Events:          events,
		ErrorMessage:    resp.GetErrorMessage(),
		BlockID:         blockID,
		BlockHeight:     block.Height,
		TransactionID:   txID,
		CollectionID:    collectionID,
		ComputationUsed: resp.GetComputationUsage(),
	}, nil
}

// TransactionResultsByBlockID get the transaction results by block ID.
//
// Expected error returns during normal operation:
//   - [codes.NotFound]: If the requested data is not found.
//   - [codes.Unavailable]: If no nodes are available or a connection to an execution node cannot be established.
//   - [codes.Internal]: For internal execution node failures.
func (e *ENTransactionProvider) TransactionResultsByBlockID(
	ctx context.Context,
	block *flow.Block,
	requiredEventEncodingVersion entities.EventEncodingVersion,
) ([]*accessmodel.TransactionResult, error) {
	blockID := block.ID()
	req := &execproto.GetTransactionsByBlockIDRequest{
		BlockId: blockID[:],
	}

	execNodes, err := e.nodeProvider.ExecutionNodesForBlockID(
		ctx,
		blockID,
	)
	if err != nil {
		if common.IsInsufficientExecutionReceipts(err) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, rpc.ConvertError(err, "failed to retrieve result from any execution node", codes.Internal)
	}

	executionResponse, err := e.getTransactionResultsByBlockIDFromAnyExeNode(ctx, execNodes, req)
	if err != nil {
		return nil, rpc.ConvertError(err, "failed to retrieve result from execution node", codes.Internal)
	}

	txStatus, err := e.txStatusDeriver.DeriveTransactionStatus(block.Height, true)
	if err != nil {
		irrecoverable.Throw(ctx, fmt.Errorf("failed to derive transaction status: %w", err))
		return nil, err
	}

	userTxResults, err := e.userTransactionResults(
		executionResponse,
		block,
		blockID,
		txStatus,
		requiredEventEncodingVersion,
	)
	if err != nil {
		return nil, rpc.ConvertError(err, "failed to construct user transaction results", codes.Internal)
	}

	// root block has no system transaction result
	if block.Height == e.state.Params().SporkRootBlockHeight() {
		return userTxResults, nil
	}

	// there must be at least one system transaction result
	if len(userTxResults) >= len(executionResponse.TransactionResults) {
		return nil, status.Errorf(codes.Internal, "no system transaction results")
	}

	remainingTxResults := executionResponse.TransactionResults[len(userTxResults):]

	systemTxResults, err := e.systemTransactionResults(
		remainingTxResults,
		block,
		blockID,
		txStatus,
		executionResponse,
		requiredEventEncodingVersion,
	)
	if err != nil {
		return nil, rpc.ConvertError(err, "failed to construct system transaction results", codes.Internal)
	}

	return append(userTxResults, systemTxResults...), nil
}

// ScheduledTransactionsByBlockID constructs the scheduled transaction bodies using events from the
// execution node response.
//
// Expected error returns during normal operation:
//   - [codes.Internal]: If the scheduled transactions cannot be constructed.
//   - [codes.Unavailable]: If no nodes are available or a connection to an execution node cannot be established.
//   - [status.Error]: For any error returned by the execution node.
func (e *ENTransactionProvider) ScheduledTransactionsByBlockID(
	ctx context.Context,
	header *flow.Header,
) ([]*flow.TransactionBody, error) {
	events, err := e.getBlockEvents(ctx, header.ID(), e.processScheduledTransactionEventType)
	if err != nil {
		return nil, rpc.ConvertError(err, "failed to retrieve events from any execution node", codes.Internal)
	}

	txs, err := e.systemCollections.
		ByHeight(header.Height).
		ExecuteCallbacksTransactions(e.chainID.Chain(), events)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "could not construct scheduled transactions: %v", err)
	}

	return txs, nil
}

// userTransactionResults constructs the user transaction results from the execution node response.
//
// It does so by iterating through all user collections (without system collection) in the block
// and constructing the transaction results.
func (e *ENTransactionProvider) userTransactionResults(
	resp *execproto.GetTransactionResultsResponse,
	block *flow.Block,
	blockID flow.Identifier,
	txStatus flow.TransactionStatus,
	requiredEventEncodingVersion entities.EventEncodingVersion,
) ([]*accessmodel.TransactionResult, error) {

	results := make([]*accessmodel.TransactionResult, 0, len(resp.TransactionResults))
	errInsufficientResults := status.Errorf(
		codes.Internal,
		"number of transaction results returned by execution node is less than the number of transactions in the block",
	)

	i := 0
	for _, guarantee := range block.Payload.Guarantees {
		collection, err := e.collections.LightByID(guarantee.CollectionID)
		if err != nil {
			return nil, rpc.ConvertStorageError(err)
		}

		for _, txID := range collection.Transactions {
			// bounds check. this means the EN returned fewer transaction results than the transactions  in the block
			if i >= len(resp.TransactionResults) {
				return nil, errInsufficientResults
			}
			txResult := resp.TransactionResults[i]

			events, err := convert.MessagesToEventsWithEncodingConversion(txResult.GetEvents(), resp.GetEventEncodingVersion(), requiredEventEncodingVersion)
			if err != nil {
				return nil, status.Errorf(codes.Internal,
					"failed to convert events to message in txID %x: %v", txID, err)
			}

			results = append(results, &accessmodel.TransactionResult{
				Status:          txStatus,
				StatusCode:      uint(txResult.GetStatusCode()),
				Events:          events,
				ErrorMessage:    txResult.GetErrorMessage(),
				BlockID:         blockID,
				TransactionID:   txID,
				CollectionID:    guarantee.CollectionID,
				BlockHeight:     block.Height,
				ComputationUsed: txResult.GetComputationUsage(),
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
func (e *ENTransactionProvider) systemTransactionResults(
	systemTxResults []*execproto.GetTransactionResultResponse,
	block *flow.Block,
	blockID flow.Identifier,
	txStatus flow.TransactionStatus,
	resp *execproto.GetTransactionResultsResponse,
	requiredEventEncodingVersion entities.EventEncodingVersion,
) ([]*accessmodel.TransactionResult, error) {
	systemTxIDs, err := e.systemTransactionIDs(block.Height, systemTxResults, resp.GetEventEncodingVersion())
	if err != nil {
		return nil, rpc.ConvertError(err, "failed to determine system transaction IDs", codes.Internal)
	}

	// systemTransactionIDs automatically detects if scheduled callbacks was enabled for the block
	// based on the number of system transactions in the response. The resulting list should always
	// have the same length as the number of system transactions in the response.
	if len(systemTxIDs) != len(systemTxResults) {
		return nil, status.Errorf(codes.Internal, "system transaction count mismatch: expected %d, got %d", len(systemTxResults), len(systemTxIDs))
	}

	results := make([]*accessmodel.TransactionResult, 0, len(systemTxResults))
	for i, systemTxResult := range systemTxResults {
		events, err := convert.MessagesToEventsWithEncodingConversion(systemTxResult.GetEvents(), resp.GetEventEncodingVersion(), requiredEventEncodingVersion)
		if err != nil {
			return nil, rpc.ConvertError(err, "failed to convert events from system tx result", codes.Internal)
		}

		results = append(results, &accessmodel.TransactionResult{
			Status:          txStatus,
			StatusCode:      uint(systemTxResult.GetStatusCode()),
			Events:          events,
			ErrorMessage:    systemTxResult.GetErrorMessage(),
			BlockID:         blockID,
			TransactionID:   systemTxIDs[i],
			CollectionID:    flow.ZeroID,
			BlockHeight:     block.Height,
			ComputationUsed: systemTxResult.GetComputationUsage(),
		})
	}

	return results, nil
}

// systemTransactionIDs determines the system transaction IDs upfront
func (e *ENTransactionProvider) systemTransactionIDs(
	blockHeight uint64,
	systemTxResults []*execproto.GetTransactionResultResponse,
	actualEventEncodingVersion entities.EventEncodingVersion,
) ([]flow.Identifier, error) {
	eventProvider := func() (flow.EventsList, error) {
		// if scheduled callbacks are enabled, the first transaction will always be the "process" transaction
		// get its events to reconstruct the system collection
		processResult := systemTxResults[0]

		// SystemCollection builder requires events are CCF encoded
		return convert.MessagesToEventsWithEncodingConversion(
			processResult.GetEvents(),
			actualEventEncodingVersion,
			entities.EventEncodingVersion_CCF_V0,
		)
	}

	sysCollection, err := e.systemCollections.
		ByHeight(blockHeight).
		SystemCollection(e.chainID.Chain(), eventProvider)
	if err != nil {
		return nil, rpc.ConvertError(err, "could not construct system collection", codes.Internal)
	}

	var systemTxIDs []flow.Identifier
	for _, tx := range sysCollection.Transactions {
		systemTxIDs = append(systemTxIDs, tx.ID())
	}

	return systemTxIDs, nil
}

// getBlockEvents returns all events by the given block ID.
//
// Expected error returns during normal operation:
//   - [codes.NotFound]: If the requested data is not found.
//   - [codes.Unavailable]: If no nodes are available or a connection to an execution node cannot be established.
//   - [codes.Internal]: For internal execution node failures.
func (e *ENTransactionProvider) getBlockEvents(
	ctx context.Context,
	blockID flow.Identifier,
	eventType flow.EventType,
) (flow.EventsList, error) {
	execNodes, err := e.nodeProvider.ExecutionNodesForBlockID(
		ctx,
		blockID,
	)
	if err != nil {
		if common.IsInsufficientExecutionReceipts(err) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, rpc.ConvertError(err, "failed to retrieve result from any execution node", codes.Internal)
	}

	request := &execproto.GetEventsForBlockIDsRequest{
		BlockIds: [][]byte{blockID[:]},
		Type:     string(eventType),
	}

	resp, err := e.getBlockEventsByBlockIDsFromAnyExeNode(ctx, execNodes, request)
	if err != nil {
		return nil, rpc.ConvertError(err, "failed to retrieve result from any execution node", codes.Internal)
	}

	var events flow.EventsList
	for _, result := range resp.GetResults() {
		resultEvents, err := convert.MessagesToEventsWithEncodingConversion(
			result.GetEvents(),
			resp.GetEventEncodingVersion(),
			entities.EventEncodingVersion_CCF_V0,
		)
		if err != nil {
			return nil, rpc.ConvertError(err, "failed to convert events", codes.Internal)
		}
		events = append(events, resultEvents...)
	}

	return events, nil
}

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

// getBlockEventsByBlockIDsFromAnyExeNode get events by block ID from the execution node.
//
// Expected error returns during normal operation:
//   - [codes.Unavailable]: If no nodes are available or a connection to an execution node cannot be established.
//   - [status.Error]: If the execution node returns a gRPC error.
func (e *ENTransactionProvider) getBlockEventsByBlockIDsFromAnyExeNode(
	ctx context.Context,
	execNodes flow.IdentitySkeletonList,
	req *execproto.GetEventsForBlockIDsRequest,
) (*execproto.GetEventsForBlockIDsResponse, error) {
	var errToReturn error
	defer func() {
		if errToReturn != nil {
			e.log.Info().Err(errToReturn).Msg("failed to get block events from execution nodes")
		}
	}()

	if len(execNodes) == 0 {
		return nil, errors.New("zero execution nodes provided")
	}

	var resp *execproto.GetEventsForBlockIDsResponse
	errToReturn = e.nodeCommunicator.CallAvailableNode(
		execNodes,
		func(node *flow.IdentitySkeleton) error {
			var err error
			resp, err = e.tryGetBlockEventsByBlockIDs(ctx, node, req)
			return err
		},
		nil,
	)

	return resp, errToReturn
}

func (e *ENTransactionProvider) tryGetTransactionResult(
	ctx context.Context,
	execNode *flow.IdentitySkeleton,
	req *execproto.GetTransactionResultRequest,
) (*execproto.GetTransactionResultResponse, error) {
	execRPCClient, closer, err := e.connFactory.GetExecutionAPIClient(execNode.Address)
	if err != nil {
		return nil, err
	}
	defer closer.Close()

	resp, err := execRPCClient.GetTransactionResult(ctx, req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (e *ENTransactionProvider) tryGetTransactionResultsByBlockID(
	ctx context.Context,
	execNode *flow.IdentitySkeleton,
	req *execproto.GetTransactionsByBlockIDRequest,
) (*execproto.GetTransactionResultsResponse, error) {
	execRPCClient, closer, err := e.connFactory.GetExecutionAPIClient(execNode.Address)
	if err != nil {
		return nil, err
	}
	defer closer.Close()

	resp, err := execRPCClient.GetTransactionResultsByBlockID(ctx, req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (e *ENTransactionProvider) tryGetTransactionResultByIndex(
	ctx context.Context,
	execNode *flow.IdentitySkeleton,
	req *execproto.GetTransactionByIndexRequest,
) (*execproto.GetTransactionResultResponse, error) {
	execRPCClient, closer, err := e.connFactory.GetExecutionAPIClient(execNode.Address)
	if err != nil {
		return nil, err
	}
	defer closer.Close()

	resp, err := execRPCClient.GetTransactionResultByIndex(ctx, req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

// tryGetBlockEventsByBlockIDs attempts to get events by block ID from the given execution node.
//
// Expected error returns during normal operation:
//   - [codes.Unavailable]: If a connection to an execution node cannot be established.
//   - [status.Error]: If the execution node returns a gRPC error.
func (e *ENTransactionProvider) tryGetBlockEventsByBlockIDs(
	ctx context.Context,
	execNode *flow.IdentitySkeleton,
	req *execproto.GetEventsForBlockIDsRequest,
) (*execproto.GetEventsForBlockIDsResponse, error) {
	execRPCClient, closer, err := e.connFactory.GetExecutionAPIClient(execNode.Address)
	if err != nil {
		return nil, err
	}
	defer closer.Close()

	resp, err := execRPCClient.GetEventsForBlockIDs(ctx, req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

// getTransactionIDByIndex returns the transaction ID for the given transaction index in the block.
// This is found by searching guarantees and then the system collection until the transaction is found.
//
// Expected error returns during normal operation:
//   - [codes.NotFound]: if the transaction is not found in the block.
//   - [codes.Unavailable]: If no nodes are available or a connection to an execution node cannot be established.
//   - [codes.Internal]: if the system collection cannot be constructed.
func (e *ENTransactionProvider) getTransactionIDByIndex(ctx context.Context, block *flow.Block, index uint32) (flow.Identifier, error) {
	i := uint32(0)

	// first search the user transactions within the guarantees
	for _, guarantee := range block.Payload.Guarantees {
		collection, err := e.collections.LightByID(guarantee.CollectionID)
		if err != nil {
			return flow.ZeroID, rpc.ConvertStorageError(err)
		}

		for _, txID := range collection.Transactions {
			if i == index {
				return txID, nil
			}
			i++
		}
	}

	eventProvider := func() (flow.EventsList, error) {
		return e.getBlockEvents(ctx, block.ID(), e.processScheduledTransactionEventType)
	}

	systemCollection, err := e.systemCollections.
		ByHeight(block.Height).
		SystemCollection(e.chainID.Chain(), eventProvider)
	if err != nil {
		return flow.ZeroID, rpc.ConvertError(err, "could not construct system collection", codes.Internal)
	}

	for _, tx := range systemCollection.Transactions {
		if i == index {
			return tx.ID(), nil
		}
		i++
	}

	return flow.ZeroID, status.Errorf(codes.NotFound, "transaction with index %d not found in block", index)
}
