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
	execNodeIdentitiesProvider *rpc.ExecutionNodeIdentitiesProvider,
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
		nodeProvider:                      execNodeIdentitiesProvider,
		txStatusDeriver:                   txStatusDeriver,
		systemTxID:                        systemTxID,
		chainID:                           chainID,
		scheduledCallbacksEnabled:         scheduledCallbacksEnabled,
		processScheduledCallbackEventType: blueprints.PendingExecutionEventType(env),
	}
}

func (e *ENTransactionProvider) TransactionResult(
	ctx context.Context,
	block *flow.Header,
	transactionID flow.Identifier,
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
		TransactionID: transactionID,
		Status:        txStatus,
		StatusCode:    uint(resp.GetStatusCode()),
		Events:        events,
		ErrorMessage:  resp.GetErrorMessage(),
		BlockID:       blockID,
		BlockHeight:   block.Height,
	}, nil
}

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

	// system transactions
	// TODO: implement system that allows this endpoint to dynamically determine if scheduled
	// transactions were enabled for this block. See https://github.com/onflow/flow-go/issues/7873
	if !e.scheduledCallbacksEnabled {
		systemTx, err := blueprints.SystemChunkTransaction(e.chainID.Chain())
		if err != nil {
			return nil, fmt.Errorf("failed to construct system chunk transaction: %w", err)
		}

		return append(transactions, systemTx), nil
	}

	events, err := e.getBlockEvents(ctx, blockID, e.processScheduledCallbackEventType)
	if err != nil {
		return nil, rpc.ConvertError(err, "failed to retrieve events from any execution node", codes.Internal)
	}

	sysCollection, err := blueprints.SystemCollection(e.chainID.Chain(), events)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "could not construct system collection: %v", err)
	}

	return append(transactions, sysCollection.Transactions...), nil
}

func (e *ENTransactionProvider) TransactionResultByIndex(
	ctx context.Context,
	block *flow.Block,
	index uint32,
	encodingVersion entities.EventEncodingVersion,
) (*accessmodel.TransactionResult, error) {
	blockID := block.ID()
	// create request and forward to EN
	req := &execproto.GetTransactionByIndexRequest{
		BlockId: blockID[:],
		Index:   index,
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
		Status:       txStatus,
		StatusCode:   uint(resp.GetStatusCode()),
		Events:       events,
		ErrorMessage: resp.GetErrorMessage(),
		BlockID:      blockID,
		BlockHeight:  block.Height,
	}, nil
}

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
		ctx,
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

func (e *ENTransactionProvider) SystemTransaction(
	ctx context.Context,
	block *flow.Block,
	txID flow.Identifier,
) (*flow.TransactionBody, error) {
	blockID := block.ID()

	if txID == e.systemTxID || !e.scheduledCallbacksEnabled {
		systemTx, err := blueprints.SystemChunkTransaction(e.chainID.Chain())
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to construct system chunk transaction: %v", err)
		}

		if txID == systemTx.ID() {
			return systemTx, nil
		}
		return nil, fmt.Errorf("transaction %s not found in block %s", txID, blockID)
	}

	events, err := e.getBlockEvents(ctx, blockID, e.processScheduledCallbackEventType)
	if err != nil {
		return nil, rpc.ConvertError(err, "failed to retrieve events from any execution node", codes.Internal)
	}

	sysCollection, err := blueprints.SystemCollection(e.chainID.Chain(), events)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "could not construct system collection: %v", err)
	}

	for _, tx := range sysCollection.Transactions {
		if tx.ID() == txID {
			return tx, nil
		}
	}

	return nil, status.Errorf(codes.NotFound, "system transaction not found")
}

func (e *ENTransactionProvider) SystemTransactionResult(
	ctx context.Context,
	block *flow.Block,
	txID flow.Identifier,
	requiredEventEncodingVersion entities.EventEncodingVersion,
) (*accessmodel.TransactionResult, error) {
	// make sure the request is for a system transaction
	if txID != e.systemTxID {
		if _, err := e.SystemTransaction(ctx, block, txID); err != nil {
			return nil, status.Errorf(codes.NotFound, "system transaction not found")
		}
	}
	return e.TransactionResult(ctx, block.ToHeader(), txID, requiredEventEncodingVersion)

}

// userTransactionResults constructs the user transaction results from the execution node response.
//
// It does so by iterating through all user collections (without system collection) in the block
// and constructing the transaction results.
func (e *ENTransactionProvider) userTransactionResults(
	ctx context.Context,
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
func (e *ENTransactionProvider) systemTransactionResults(
	systemTxResults []*execproto.GetTransactionResultResponse,
	block *flow.Block,
	blockID flow.Identifier,
	txStatus flow.TransactionStatus,
	resp *execproto.GetTransactionResultsResponse,
	requiredEventEncodingVersion entities.EventEncodingVersion,
) ([]*accessmodel.TransactionResult, error) {
	systemTxIDs, err := e.systemTransactionIDs(systemTxResults, resp.GetEventEncodingVersion())
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

// systemTransactionIDs determines the system transaction IDs upfront
func (e *ENTransactionProvider) systemTransactionIDs(
	systemTxResults []*execproto.GetTransactionResultResponse,
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
		return nil, rpc.ConvertError(err, "failed to convert events", codes.Internal)
	}

	sysCollection, err := blueprints.SystemCollection(e.chainID.Chain(), events)
	if err != nil {
		return nil, rpc.ConvertError(err, "failed to construct system collection", codes.Internal)
	}

	var systemTxIDs []flow.Identifier
	for _, tx := range sysCollection.Transactions {
		systemTxIDs = append(systemTxIDs, tx.ID())
	}

	return systemTxIDs, nil
}

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
