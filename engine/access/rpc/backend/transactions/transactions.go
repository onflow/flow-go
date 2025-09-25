package transactions

import (
	"context"
	"errors"
	"fmt"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
	accessproto "github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/onflow/flow/protobuf/go/flow/entities"
	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/access/validator"
	"github.com/onflow/flow-go/engine/access/rpc/backend/common"
	"github.com/onflow/flow-go/engine/access/rpc/backend/node_communicator"
	"github.com/onflow/flow-go/engine/access/rpc/backend/transactions/provider"
	"github.com/onflow/flow-go/engine/access/rpc/backend/transactions/retrier"
	txstatus "github.com/onflow/flow-go/engine/access/rpc/backend/transactions/status"
	"github.com/onflow/flow-go/engine/access/rpc/connection"
	"github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// ErrTransactionNotInBlock indicates that the transaction is not found in the block.
var ErrTransactionNotInBlock = errors.New("transaction not in block")

// ErrTransactionNotInCollection indicates that the transaction is not found in the collection provided in the request.
var ErrTransactionNotInCollection = errors.New("transaction not found in collection")

type Transactions struct {
	log     zerolog.Logger
	metrics module.TransactionMetrics

	state      protocol.State
	chainID    flow.ChainID
	systemTxID flow.Identifier

	// RPC Clients & Network
	collectionRPCClient         accessproto.AccessAPIClient // RPC client tied to a fixed collection node
	historicalAccessNodeClients []accessproto.AccessAPIClient
	nodeCommunicator            node_communicator.Communicator
	connectionFactory           connection.ConnectionFactory
	retrier                     retrier.Retrier

	// Storages
	blocks       storage.Blocks
	collections  storage.Collections
	transactions storage.Transactions
	events       storage.Events

	txResultCache *lru.Cache[flow.Identifier, *accessmodel.TransactionResult]

	txValidator     *validator.TransactionValidator
	txProvider      provider.TransactionProvider
	txStatusDeriver *txstatus.TxStatusDeriver

	executionStateCache       optimistic_sync.ExecutionStateCache
	execResultProvider        optimistic_sync.ExecutionResultInfoProvider
	operatorCriteria          optimistic_sync.Criteria
	scheduledCallbacksEnabled bool
}

var _ access.TransactionsAPI = (*Transactions)(nil)

type Params struct {
	Log                         zerolog.Logger
	Metrics                     module.TransactionMetrics
	State                       protocol.State
	ChainID                     flow.ChainID
	SystemTxID                  flow.Identifier
	StaticCollectionRPCClient   accessproto.AccessAPIClient
	HistoricalAccessNodeClients []accessproto.AccessAPIClient
	NodeCommunicator            node_communicator.Communicator
	ConnFactory                 connection.ConnectionFactory
	EnableRetries               bool
	NodeProvider                *rpc.ExecutionNodeIdentitiesProvider
	Blocks                      storage.Blocks
	Collections                 storage.Collections
	Transactions                storage.Transactions
	Events                      storage.Events
	TxResultCache               *lru.Cache[flow.Identifier, *accessmodel.TransactionResult]
	TxProvider                  provider.TransactionProvider
	TxValidator                 *validator.TransactionValidator
	TxStatusDeriver             *txstatus.TxStatusDeriver
	ExecutionStateCache         optimistic_sync.ExecutionStateCache
	ExecResultProvider          optimistic_sync.ExecutionResultInfoProvider
	OperatorCriteria            optimistic_sync.Criteria
	ScheduledCallbacksEnabled   bool
}

func NewTransactionsBackend(params Params) (*Transactions, error) {
	txs := &Transactions{
		log:                         params.Log,
		metrics:                     params.Metrics,
		state:                       params.State,
		chainID:                     params.ChainID,
		systemTxID:                  params.SystemTxID,
		collectionRPCClient:         params.StaticCollectionRPCClient,
		historicalAccessNodeClients: params.HistoricalAccessNodeClients,
		nodeCommunicator:            params.NodeCommunicator,
		connectionFactory:           params.ConnFactory,
		blocks:                      params.Blocks,
		collections:                 params.Collections,
		transactions:                params.Transactions,
		events:                      params.Events,
		txResultCache:               params.TxResultCache,
		txValidator:                 params.TxValidator,
		txProvider:                  params.TxProvider,
		txStatusDeriver:             params.TxStatusDeriver,
		executionStateCache:         params.ExecutionStateCache,
		execResultProvider:          params.ExecResultProvider,
		operatorCriteria:            params.OperatorCriteria,
		scheduledCallbacksEnabled:   params.ScheduledCallbacksEnabled,
		retrier:                     retrier.NewNoopRetrier(),
	}

	if params.EnableRetries {
		txs.retrier = retrier.NewRetrier(
			params.Log,
			params.Blocks,
			params.Collections,
			txs,
			params.TxStatusDeriver,
		)
	}

	return txs, nil
}

// SendTransaction forwards the transaction to the collection node
//
// CAUTION: this layer SIMPLIFIES the ERROR HANDLING convention
//   - All errors returned are guaranteed to be benign. The node can continue normal operations after such errors.
//   - To prevent delivering incorrect results to clients in case of an error, all other return values should be discarded.
//
// Expected sentinel errors providing details to clients about failed requests:
//   - [access.InvalidRequestError] - if the transaction is invalid
//   - [access.ServiceUnavailable] - if none of the collection nodes tried could be reached
//   - [access.RequestCanceledError] - if the request was canceled
//   - [access.RequestTimedOutError] - if the request timed out
//   - [access.InternalError] - for all other errors returned by collection nodes
func (t *Transactions) SendTransaction(ctx context.Context, tx *flow.TransactionBody) error {
	now := time.Now().UTC()

	// TODO: separate benign errors from internal errors
	err := t.txValidator.Validate(ctx, tx)
	if err != nil {
		return access.NewInvalidRequestError(fmt.Errorf("transaction is invalid: %w", err))
	}

	err = t.trySendTransaction(ctx, tx)
	if err != nil {
		t.metrics.TransactionSubmissionFailed()
		return access.RequireAccessError(ctx, err)
	}

	t.metrics.TransactionReceived(tx.ID(), now)

	err = t.transactions.Store(tx)
	if err != nil {
		return access.RequireNoError(ctx, fmt.Errorf("failed to store transaction: %w", err))
	}

	go t.registerTransactionForRetry(tx)

	return nil
}

// SendRawTransaction sends a raw transaction to the collection node
// This method is not part of the Access API
//
// Expected error returns during normal operations:
//   - [access.InvalidRequestError] - if the transaction is invalid
//   - [access.ServiceUnavailable] - if none of the collection nodes tried could be reached
//   - [access.RequestCanceledError] - if the request was canceled
//   - [access.RequestTimedOutError] - if the request timed out
//   - [access.InternalError] - for all other errors returned by collection nodes
func (t *Transactions) SendRawTransaction(ctx context.Context, tx *flow.TransactionBody) error {
	return t.trySendTransaction(ctx, tx)
}

// trySendTransaction sends the provided transaction to a collection node.
//
// Expected error returns during normal operations:
//   - [access.InvalidRequestError] - if the transaction is invalid
//   - [access.ServiceUnavailable] - if none of the collection nodes tried could be reached
//   - [access.RequestCanceledError] - if the request was canceled
//   - [access.RequestTimedOutError] - if the request timed out
//   - [access.InternalError] - for all other errors returned by collection nodes
func (t *Transactions) trySendTransaction(ctx context.Context, tx *flow.TransactionBody) error {
	parseGrpcError := func(err error, nodeAddress string) error {
		wrappedErr := fmt.Errorf("failed to send transaction to collection node at %s: %w", nodeAddress, err)
		switch status.Code(err) {
		case codes.InvalidArgument,
			codes.Unavailable,
			codes.Internal,
			codes.Canceled,
			codes.DeadlineExceeded:
			return access.ConvertGrpcError("send transaction", wrappedErr)
		default:
			return access.NewInternalError(wrappedErr)
		}
	}

	// if a collection node rpc client was provided at startup, just use that
	if t.collectionRPCClient != nil {
		err := t.grpcTxSend(ctx, t.collectionRPCClient, tx)
		if err != nil {
			return parseGrpcError(err, "static collection node")
		}
	}

	collNodes, err := t.chooseCollectionNodes(tx.ID())
	if err != nil {
		return fmt.Errorf("failed to determine collection node for tx %s: %w", tx.ID(), err)
	}

	var executor *flow.IdentitySkeleton
	executor, sendError := t.nodeCommunicator.CallAvailableNode(
		collNodes,
		func(node *flow.IdentitySkeleton) error {
			return t.sendTransactionToCollector(ctx, tx, node.Address)
		},
		nil,
	)

	if sendError != nil {
		t.log.Info().Err(err).Msg("failed to send transactions to collector nodes")
	}
	return parseGrpcError(err, executor.Address)
}

// chooseCollectionNodes finds a random subset of size sampleSize of collection node addresses from the
// collection node cluster responsible for the given tx
//
// No errors are expected during normal operations.
func (t *Transactions) chooseCollectionNodes(txID flow.Identifier) (flow.IdentitySkeletonList, error) {
	// retrieve the set of collector clusters
	currentEpoch, err := t.state.Final().Epochs().Current()
	if err != nil {
		return nil, fmt.Errorf("could not get current epoch: %w", err)
	}
	clusters, err := currentEpoch.Clustering()
	if err != nil {
		return nil, fmt.Errorf("could not cluster collection nodes: %w", err)
	}

	// get the cluster responsible for the transaction
	targetNodes, ok := clusters.ByTxID(txID)
	if !ok {
		return nil, fmt.Errorf("could not get local cluster by txID: %s", txID)
	}

	return targetNodes, nil
}

// sendTransactionToCollection sends the transaction to the given collection node via grpc
//
// Expected error returns during normal operations:
// - [access.ServiceUnavailable] - if the collection node could not be reached
// - [status.Error] - any error returned by the grpc call
func (t *Transactions) sendTransactionToCollector(
	ctx context.Context,
	tx *flow.TransactionBody,
	collectionNodeAddr string,
) error {
	collectionRPC, closer, err := t.connectionFactory.GetCollectionAPIClient(collectionNodeAddr, nil)
	if err != nil {
		// all errors getting the initial connection are benign and indicate an issue creating
		// the connection
		return access.NewServiceUnavailable(fmt.Errorf("failed to connect to collection node at %s: %w", collectionNodeAddr, err))
	}
	defer closer.Close()

	return t.grpcTxSend(ctx, collectionRPC, tx)
}

// grpcTxSend sends the transaction to the given collection node via grpc.
// Returns any error returned by the grpc call.
//
// All errors returned by this method are benign and indicate issues connecting with external nodes.
func (t *Transactions) grpcTxSend(
	ctx context.Context,
	client accessproto.AccessAPIClient,
	tx *flow.TransactionBody,
) error {
	colReq := &accessproto.SendTransactionRequest{
		Transaction: convert.TransactionToMessage(*tx),
	}

	clientDeadline := time.Now().Add(time.Duration(2) * time.Second)
	ctx, cancel := context.WithDeadline(ctx, clientDeadline)
	defer cancel()

	_, err := client.SendTransaction(ctx, colReq)
	return err
}

// GetTransaction returns the transaction for the provided transaction ID.
//
// CAUTION: this layer SIMPLIFIES the ERROR HANDLING convention
//   - All errors returned are guaranteed to be benign. The node can continue normal operations after such errors.
//   - To prevent delivering incorrect results to clients in case of an error, all other return values should be discarded.
//
// Expected sentinel errors providing details to clients about failed requests:
//   - [access.DataNotFoundError] - if the transaction is not found
//   - [access.InternalError] - if the transaction from the historical node cannot be converted
//     to a TransactionBody
//   - [access.RequestCanceledError] - if the request was canceled
//   - [access.RequestTimedOutError] - if the request timed out
func (t *Transactions) GetTransaction(ctx context.Context, txID flow.Identifier) (*flow.TransactionBody, error) {
	tx, err := t.transactions.ByID(txID)
	if err != nil {
		if !errors.Is(err, storage.ErrNotFound) {
			return nil, access.RequireNoError(ctx, fmt.Errorf("failed lookup transaction in storage: %w", err))
		}

		tx, err = t.getHistoricalTransaction(ctx, txID)
		if err != nil {
			return nil, access.RequireAccessError(ctx, err)
		}
		return tx, nil
	}

	return tx, nil
}

// GetTransactionsByBlockID returns the transactions for the provided block ID.
//
// CAUTION: this layer SIMPLIFIES the ERROR HANDLING convention
//   - All errors returned are guaranteed to be benign. The node can continue normal operations after such errors.
//   - To prevent delivering incorrect results to clients in case of an error, all other return values should be discarded.
//
// Expected sentinel errors providing details to clients about failed requests:
//   - [access.DataNotFoundError] - if the block or any of its collections are not found, or if there
//     are insufficient execution receipts to lookup the system transactions.
func (t *Transactions) GetTransactionsByBlockID(
	ctx context.Context,
	blockID flow.Identifier,
) ([]*flow.TransactionBody, error) {
	// TODO: consider using storage.Index.ByBlockID, the index contains collection id and seals ID
	block, err := t.blocks.ByID(blockID)
	if err != nil {
		err = access.RequireErrorIs(ctx, err, storage.ErrNotFound)
		return nil, access.NewDataNotFoundError("block", fmt.Errorf("could not find block: %w", err))
	}

	// TODO: this method needs to accept userCriteria
	criteria := t.operatorCriteria
	execResultInfo, err := t.execResultProvider.ExecutionResultInfo(blockID, criteria)
	if err != nil {
		if common.IsInsufficientExecutionReceipts(err) {
			return nil, access.NewDataNotFoundError("transactions", err)
		}
		return nil, access.RequireNoError(ctx, fmt.Errorf("failed to get execution result for last block: %w", err))
	}

	// TODO: return executor metadata
	transactions, _, err := t.txProvider.TransactionsByBlockID(ctx, block, execResultInfo)
	return transactions, err
}

// GetTransactionResult returns the transaction result for the provided transaction ID.
// This method also optionally accepts a block ID and collection ID to narrow down the search.
// When provided, the transaction must match the critera. Otherwise, the
//
// CAUTION: this layer SIMPLIFIES the ERROR HANDLING convention
//   - All errors returned are guaranteed to be benign. The node can continue normal operations after such errors.
//   - To prevent delivering incorrect results to clients in case of an error, all other return values should be discarded.
//
// Expected sentinel errors providing details to clients about failed requests:
//   - [access.DataNotFoundError] - if data required to process the request was not found
//   - [access.InternalError] - if the transaction results cannot be retrieved from the execution
//     node or were invalid.
func (t *Transactions) GetTransactionResult(
	ctx context.Context,
	txID flow.Identifier,
	blockID flow.Identifier,
	collectionID flow.Identifier,
	encodingVersion entities.EventEncodingVersion,
	userCriteria optimistic_sync.Criteria,
) (*accessmodel.TransactionResult, *accessmodel.ExecutorMetadata, error) {
	start := time.Now()

	// 1. lookup the the collection that contains the transaction. if it is not found, then the
	// collection is not yet indexed and the transaction is either unknown or pending.
	//
	// BFT corner case: Only the first finalized collection to contain the transaction is indexed.
	// If the transaction is included in multiple collections in the same or different blocks, the
	// first collection to be indexed by the node is returned. This is not guaranteed to be the
	// first collection in execution order!
	lightCollection, err := t.collections.LightByTransactionID(txID)
	if err != nil {
		if !errors.Is(err, storage.ErrNotFound) {
			return nil, nil, access.RequireNoError(ctx, fmt.Errorf("failed to find collection for transaction: %w", err))
		}

		result, err := t.getUnknownTransactionResult(ctx, txID, blockID, collectionID)
		if err != nil {
			if errors.Is(err, ErrTransactionNotInCollection) || errors.Is(err, txstatus.ErrReferenceBlockNotFound) {
				return nil, nil, access.NewDataNotFoundError("transaction result", err)
			}
			return nil, nil, access.RequireNoError(ctx, fmt.Errorf("failed to get unknown transaction result: %w", err))
		}

		// no metadata because transaction is not executed yet
		return result, nil, nil
	}
	if collectionID != flow.ZeroID && collectionID != lightCollection.ID() {
		err := fmt.Errorf("transaction found in collection %s, but %s was provided", lightCollection.ID(), collectionID)
		return nil, nil, access.NewDataNotFoundError("transaction result", err)
	}

	// 2. lookup the block containing the collection.
	block, err := t.blocks.ByCollectionID(collectionID)
	if err != nil {
		// this is an exception. the block/collection index must exist if the collection/tx is indexed,
		// otherwise the stored state is inconsistent.
		err = fmt.Errorf("failed to find block for collection %v: %w", collectionID, err)
		return nil, nil, access.RequireNoError(ctx, err)
	}
	if blockID != flow.ZeroID && blockID != block.ID() {
		err := fmt.Errorf("transaction found in block %s, but %s was provided", block.ID(), blockID)
		return nil, nil, access.NewDataNotFoundError("transaction result", err)
	}

	// 3. lookup the actual transaction result. at this point, we know the tx exists in the db
	// and we know the block and collection.

	criteria := t.operatorCriteria.OverrideWith(userCriteria)
	execResultInfo, err := t.execResultProvider.ExecutionResultInfo(blockID, criteria)
	if err != nil {
		if common.IsInsufficientExecutionReceipts(err) {
			return nil, nil, access.NewDataNotFoundError("transaction result", err)
		}
		return nil, nil, access.RequireNoError(ctx, fmt.Errorf("failed to get execution result for block: %w", err))
	}

	txResult, executorMetadata, err := t.txProvider.TransactionResult(ctx, block.ToHeader(), txID, collectionID, encodingVersion, execResultInfo)
	if err != nil {
		switch {
		case errors.Is(err, optimistic_sync.ErrSnapshotNotFound):
			err = fmt.Errorf("could not find snapshot for execution result: %w", err)
			return nil, nil, access.NewDataNotFoundError("transaction result", err)
		case common.IsInvalidDataFromExternalNodeError(err):
			err = fmt.Errorf("invalid data from execution node: %w", err)
			return nil, nil, access.NewInternalError(err)
		case common.IsFailedToQueryExternalNodeError(err):
			err = fmt.Errorf("failed to query execution node: %w", err)
			return nil, nil, access.NewInternalError(err)
		default:
			err = fmt.Errorf("failed to get transaction result by index: %w", err)
			return nil, nil, access.RequireNoError(ctx, err)
		}
	}

	// If there is still no transaction result, provide placeholder based on available information.
	if txResult == nil {
		txStatus, err := t.txStatusDeriver.DeriveFinalizedTransactionStatus(block.Height, false)
		if err != nil {
			return nil, executorMetadata, access.RequireNoError(ctx, fmt.Errorf("failed to derive finalized transaction status: %w", err))
		}

		txResult = &accessmodel.TransactionResult{
			BlockID:       blockID,
			BlockHeight:   block.Height,
			TransactionID: txID,
			Status:        txStatus,
			CollectionID:  collectionID,
		}
	}

	tx, err := t.transactions.ByID(txID)
	if err != nil {
		// since the previous lookups all succeeded, if this fails, the node's state is inconsistent,
		// which is irrecoverable.
		return nil, executorMetadata, access.RequireNoError(ctx, fmt.Errorf("failed to get transaction from storage: %w", err))
	}

	t.metrics.TransactionResultFetched(time.Since(start), len(tx.Script))

	return txResult, executorMetadata, nil
}

// GetTransactionResultByIndex returns the transaction results for a transaction identified by
// blockID and index.
//
// CAUTION: this layer SIMPLIFIES the ERROR HANDLING convention
//   - All errors returned are guaranteed to be benign. The node can continue normal operations after such errors.
//   - To prevent delivering incorrect results to clients in case of an error, all other return values should be discarded.
//
// Expected sentinel errors providing details to clients about failed requests:
//   - [access.DataNotFoundError] - if data required to process the request was not found
//   - [access.InternalError] - if the transaction results cannot be retrieved from the execution
//     node or were invalid.
func (t *Transactions) GetTransactionResultByIndex(
	ctx context.Context,
	blockID flow.Identifier,
	index uint32,
	encodingVersion entities.EventEncodingVersion,
	userCriteria optimistic_sync.Criteria,
) (*accessmodel.TransactionResult, *accessmodel.ExecutorMetadata, error) {
	block, err := t.blocks.ByID(blockID)
	if err != nil {
		err = access.RequireErrorIs(ctx, fmt.Errorf("could not find block: %w", err), storage.ErrNotFound)
		return nil, nil, access.NewDataNotFoundError("transaction result", err)
	}

	criteria := t.operatorCriteria.OverrideWith(userCriteria)
	execResultInfo, err := t.execResultProvider.ExecutionResultInfo(blockID, criteria)
	if err != nil {
		if common.IsInsufficientExecutionReceipts(err) {
			return nil, nil, access.NewDataNotFoundError("transaction result", err)
		}
		return nil, nil, access.RequireNoError(ctx, fmt.Errorf("failed to get execution result for block: %w", err))
	}

	collectionID, err := t.lookupCollectionIDByBlockAndTxIndex(block, index)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			err = fmt.Errorf("could not find collection for transaction result: %w", err)
			return nil, nil, access.NewDataNotFoundError("collection", err)
		}
		return nil, nil, access.RequireNoError(ctx, fmt.Errorf("failed to lookup collection ID in block by index: %w", err))
	}

	txResult, executorMetadata, err := t.txProvider.TransactionResultByIndex(ctx, block.ToHeader(), index, collectionID, encodingVersion, execResultInfo)
	if err != nil {
		switch {
		case errors.Is(err, optimistic_sync.ErrSnapshotNotFound):
			err = fmt.Errorf("could not find snapshot for execution result: %w", err)
			return nil, nil, access.NewDataNotFoundError("transaction result", err)
		case common.IsInvalidDataFromExternalNodeError(err):
			err = fmt.Errorf("invalid data from execution node: %w", err)
			return nil, nil, access.NewInternalError(err)
		case common.IsFailedToQueryExternalNodeError(err):
			err = fmt.Errorf("failed to query execution node: %w", err)
			return nil, nil, access.NewInternalError(err)
		default:
			err = fmt.Errorf("failed to get transaction result by index: %w", err)
			return nil, nil, access.RequireNoError(ctx, err)
		}
	}

	return txResult, executorMetadata, nil
}

// GetTransactionResultsByBlockID returns all transaction results for the provided block ID.
//
// CAUTION: this layer SIMPLIFIES the ERROR HANDLING convention
//   - All errors returned are guaranteed to be benign. The node can continue normal operations after such errors.
//   - To prevent delivering incorrect results to clients in case of an error, all other return values should be discarded.
//
// Expected sentinel errors providing details to clients about failed requests:
//   - [access.DataNotFoundError] - if data required to process the request was not found
//   - [access.InternalError] - if the transaction results cannot be retrieved from the execution
//     node or were invalid.
func (t *Transactions) GetTransactionResultsByBlockID(
	ctx context.Context,
	blockID flow.Identifier,
	encodingVersion entities.EventEncodingVersion,
	userCriteria optimistic_sync.Criteria,
) ([]*accessmodel.TransactionResult, *accessmodel.ExecutorMetadata, error) {
	// TODO: consider using storage.Index.ByBlockID, the index contains collection id and seals ID
	block, err := t.blocks.ByID(blockID)
	if err != nil {
		err = access.RequireErrorIs(ctx, fmt.Errorf("could not find block: %w", err), storage.ErrNotFound)
		return nil, nil, access.NewDataNotFoundError("transaction result", err)
	}

	criteria := t.operatorCriteria.OverrideWith(userCriteria)
	execResultInfo, err := t.execResultProvider.ExecutionResultInfo(blockID, criteria)
	if err != nil {
		if common.IsInsufficientExecutionReceipts(err) {
			return nil, nil, access.NewDataNotFoundError("transaction result", err)
		}
		return nil, nil, access.RequireNoError(ctx, fmt.Errorf("failed to get execution result for block: %w", err))
	}

	results, executorMetadata, err := t.txProvider.TransactionResultsByBlockID(ctx, block, encodingVersion, execResultInfo)
	if err != nil {
		switch {
		case errors.Is(err, optimistic_sync.ErrSnapshotNotFound):
			err = fmt.Errorf("could not find snapshot for execution result: %w", err)
			return nil, nil, access.NewDataNotFoundError("transaction result", err)
		case errors.Is(err, storage.ErrNotFound):
			err = fmt.Errorf("could not find collection: %w", err)
			return nil, nil, access.NewDataNotFoundError("collection", err)
		case common.IsInvalidDataFromExternalNodeError(err):
			err = fmt.Errorf("invalid data from execution node: %w", err)
			return nil, nil, access.NewInternalError(err)
		case common.IsFailedToQueryExternalNodeError(err):
			err = fmt.Errorf("failed to query execution node: %w", err)
			return nil, nil, access.NewInternalError(err)
		default:
			err = fmt.Errorf("failed to get transaction result by index: %w", err)
			return nil, nil, access.RequireNoError(ctx, err)
		}
	}

	return results, executorMetadata, nil
}

// GetSystemTransaction returns the system transaction for the provided block.
//
// CAUTION: this layer SIMPLIFIES the ERROR HANDLING convention
//   - All errors returned are guaranteed to be benign. The node can continue normal operations after such errors.
//   - To prevent delivering incorrect results to clients in case of an error, all other return values should be discarded.
//
// Expected sentinel errors providing details to clients about failed requests:
//   - [access.DataNotFoundError] - if data required to process the request was not found
//   - [access.InternalError] - if the transaction cannot be retrieved from the execution node or was invalid.
func (t *Transactions) GetSystemTransaction(
	ctx context.Context,
	txID flow.Identifier,
	blockID flow.Identifier,
	userCriteria optimistic_sync.Criteria,
) (*flow.TransactionBody, *accessmodel.ExecutorMetadata, error) {
	block, err := t.blocks.ByID(blockID)
	if err != nil {
		err = access.RequireErrorIs(ctx, err, storage.ErrNotFound)
		return nil, nil, access.NewDataNotFoundError("transaction result", fmt.Errorf("could not find block: %w", err))
	}

	if txID == flow.ZeroID {
		txID = t.systemTxID
	}

	criteria := t.operatorCriteria.OverrideWith(userCriteria)
	execResultInfo, err := t.execResultProvider.ExecutionResultInfo(blockID, criteria)
	if err != nil {
		if common.IsInsufficientExecutionReceipts(err) {
			return nil, nil, access.NewDataNotFoundError("system transaction", err)
		}
		return nil, nil, access.RequireNoError(ctx, fmt.Errorf("failed to get execution result for block: %w", err))
	}

	tx, executorMetadata, err := t.txProvider.SystemTransaction(ctx, block, txID, execResultInfo)
	if err != nil {
		switch {
		case errors.Is(err, provider.ErrNotASystemTransaction):
			err = fmt.Errorf("transaction %s is not a system transaction: %w", txID, err)
			return nil, nil, access.NewDataNotFoundError("system transaction", err)
		case errors.Is(err, optimistic_sync.ErrSnapshotNotFound):
			err = fmt.Errorf("could not find snapshot for execution result: %w", err)
			return nil, nil, access.NewDataNotFoundError("transaction result", err)
		case common.IsInvalidDataFromExternalNodeError(err):
			err = fmt.Errorf("invalid data from execution node: %w", err)
			return nil, nil, access.NewInternalError(err)
		case common.IsFailedToQueryExternalNodeError(err):
			err = fmt.Errorf("failed to query execution node: %w", err)
			return nil, nil, access.NewInternalError(err)
		default:
			err = fmt.Errorf("failed to get system transaction: %w", err)
			return nil, nil, access.RequireNoError(ctx, err)
		}
	}
	return tx, executorMetadata, nil
}

// GetSystemTransactionResult returns the system transaction result for the provided block.
//
// CAUTION: this layer SIMPLIFIES the ERROR HANDLING convention
//   - All errors returned are guaranteed to be benign. The node can continue normal operations after such errors.
//   - To prevent delivering incorrect results to clients in case of an error, all other return values should be discarded.
//
// Expected sentinel errors providing details to clients about failed requests:
//   - [access.DataNotFoundError] - if data required to process the request was not found
//   - [access.InternalError] - if the transaction result cannot be retrieved from the execution node or was invalid.
func (t *Transactions) GetSystemTransactionResult(
	ctx context.Context,
	txID flow.Identifier,
	blockID flow.Identifier,
	encodingVersion entities.EventEncodingVersion,
	userCriteria optimistic_sync.Criteria,
) (*accessmodel.TransactionResult, *accessmodel.ExecutorMetadata, error) {
	block, err := t.blocks.ByID(blockID)
	if err != nil {
		err = access.RequireErrorIs(ctx, err, storage.ErrNotFound)
		return nil, nil, access.NewDataNotFoundError("transaction result", fmt.Errorf("could not find block: %w", err))
	}

	criteria := t.operatorCriteria.OverrideWith(userCriteria)
	execResultInfo, err := t.execResultProvider.ExecutionResultInfo(blockID, criteria)
	if err != nil {
		if common.IsInsufficientExecutionReceipts(err) {
			return nil, nil, access.NewDataNotFoundError("system transaction result", err)
		}
		return nil, nil, access.RequireNoError(ctx, fmt.Errorf("failed to get execution result for block: %w", err))
	}

	result, executorMetadata, err := t.txProvider.TransactionResult(ctx, block.ToHeader(), t.systemTxID, flow.ZeroID, encodingVersion, execResultInfo)
	if err != nil {
		switch {
		case errors.Is(err, optimistic_sync.ErrSnapshotNotFound):
			err = fmt.Errorf("could not find snapshot for execution result: %w", err)
			return nil, nil, access.NewDataNotFoundError("transaction result", err)
		case common.IsInvalidDataFromExternalNodeError(err):
			err = fmt.Errorf("invalid data from execution node: %w", err)
			return nil, nil, access.NewInternalError(err)
		case common.IsFailedToQueryExternalNodeError(err):
			err = fmt.Errorf("failed to query execution node: %w", err)
			return nil, nil, access.NewInternalError(err)
		default:
			err = fmt.Errorf("failed to get transaction result by index: %w", err)
			return nil, nil, access.RequireNoError(ctx, err)
		}
	}

	return result, executorMetadata, nil
}

// getUnknownTransactionResult returns the transaction result for a transaction that is not yet
// indexed in a finalized block.
//
// Expected errors during normal operations:
//   - [ErrTransactionNotInCollection] - if transaction is not found in the collection provided in the request.
//   - [status.ErrReferenceBlockNotFound] - if the transaction is found, but its reference block is not.
func (t *Transactions) getUnknownTransactionResult(
	ctx context.Context,
	txID flow.Identifier,
	blockID flow.Identifier,
	collectionID flow.Identifier,
) (*accessmodel.TransactionResult, error) {
	tx, err := t.transactions.ByID(txID)
	if err == nil {
		txStatus, err := t.txStatusDeriver.DeriveUnknownTransactionStatus(tx.ReferenceBlockID)
		if err != nil {
			return nil, fmt.Errorf("failed to derive transaction status: %w", err)
		}

		return &accessmodel.TransactionResult{
			TransactionID: txID,
			Status:        txStatus,
		}, nil
	}

	if !errors.Is(err, storage.ErrNotFound) {
		return nil, fmt.Errorf("failed to get transaction from storage: %w", err)
	}

	// the transaction does not exist locally, so check if the block or collection help identify its
	// status.
	if blockID != flow.ZeroID {
		_, err := t.blocks.ByID(blockID)
		if err == nil {
			// the user's specified block exists locally, so assume the tx is not yet indexed
			// but will be eventually
			return &accessmodel.TransactionResult{
				TransactionID: txID,
				Status:        flow.TransactionStatusUnknown,
			}, nil
		}
		if !errors.Is(err, storage.ErrNotFound) {
			return nil, fmt.Errorf("failed to get block from storage: %w", err)
		}
		// search historical access nodes
	}

	if collectionID != flow.ZeroID {
		_, err := t.collections.LightByID(collectionID)
		if err == nil {
			// the user's specified collection exists locally. since the tx is not indexed, this
			// means the provided collection does not contain the tx
			return nil, ErrTransactionNotInCollection
		}
		if !errors.Is(err, storage.ErrNotFound) {
			return nil, fmt.Errorf("failed to get collection from storage: %w", err)
		}
		// search historical access nodes
	}

	historicalTxResult := t.searchHistoricalAccessNodes(ctx, txID)
	return historicalTxResult, nil
}

// getHistoricalTransaction retrieves a transaction from the historical access nodes
//
// Expected error returns during normal operations:
//   - [access.DataNotFoundError] - if the transaction is not found on any of the historical nodes.
//     Note: requests to historical nodes may fail for various reasons, so this does not guarantee
//     that the transaction does not exist on a historical node.
//   - [common.InvalidDataFromExternalNodeError] - if the transaction from the historical node cannot
//     be converted to a TransactionBody
//   - [common.FailedToQueryExternalNodeError] - if the request to the historical node failed
//
// All returned errors are benign and indicate issues with external historical access nodes, not
// with the local node.
func (t *Transactions) getHistoricalTransaction(
	ctx context.Context,
	txID flow.Identifier,
) (*flow.TransactionBody, error) {
	for i, historicalNode := range t.historicalAccessNodeClients {
		txResp, err := historicalNode.GetTransaction(ctx, &accessproto.GetTransactionRequest{Id: txID[:]})
		if err != nil {
			// TODO: provide better errors so we can differentiate between all networks searched and it
			// wasn't found, from only some networks successfully searched.
			switch status.Code(err) {
			case codes.Canceled, codes.DeadlineExceeded:
				wrappedErr := fmt.Errorf("could not get transaction result from historical node (%d): %w", i, err)
				return nil, common.NewFailedToQueryExternalNodeError(wrappedErr)
			default:
				// continue searching on any other error
				continue
			}
		}

		tx, err := convert.MessageToTransaction(txResp.Transaction, t.chainID.Chain())
		if err != nil {
			err = fmt.Errorf("could not convert transaction from historical node (%d): %w", i, err)
			return nil, common.NewInvalidDataFromExternalNodeError("transaction", flow.ZeroID, err)
		}
		return &tx, nil
	}
	return nil, access.NewDataNotFoundError("transaction", fmt.Errorf("transaction with ID %s not found on historical nodes", txID))
}

// searchHistoricalAccessNodes searches the historical access nodes for the transaction result
// and caches the result if enabled.
func (t *Transactions) searchHistoricalAccessNodes(
	ctx context.Context,
	txID flow.Identifier,
) (historicalTxResult *accessmodel.TransactionResult) {
	// if the tx is not known locally, search the historical access nodes
	if t.txResultCache != nil {
		if result, ok := t.txResultCache.Get(txID); ok {
			return result
		}
		// always cache the result even if it's an error to avoid unnecessary load on the nodes.
		// the cache is limited so retries will happen eventually. users can also query the nodes
		// directly for more precise results.
		defer func() {
			t.txResultCache.Add(txID, historicalTxResult)
		}()
	}

	historicalTxResult, err := t.getHistoricalTransactionResult(ctx, txID)
	if err != nil {
		// if tx not found on historic access nodes either, then assume that the tx was
		// submitted to a different AN and return status as unknown
		historicalTxResult = &accessmodel.TransactionResult{
			TransactionID: txID,
			Status:        flow.TransactionStatusUnknown,
		}
	}

	return historicalTxResult
}

// getHistoricalTransactionResult retrieves a transaction result from the historical access nodes
//
// Expected error returns during normal operations:
//   - [access.DataNotFoundError] - if the transaction result is not found on any of the historical
//     nodes. Note: requests to historical nodes may fail for various reasons, so this does not guarantee
//     that the transaction result does not exist on a historical node.
//   - [common.InvalidDataFromExternalNodeError] - if the transaction result from the historical node
//     cannot be converted to a TransactionResult
//     to a TransactionResult
//   - [common.FailedToQueryExternalNodeError] - if the request to the historical node failed
//
// All returned errors are benign and indicate issues with external historical access nodes, not
// with the local node.
func (t *Transactions) getHistoricalTransactionResult(
	ctx context.Context,
	txID flow.Identifier,
) (*accessmodel.TransactionResult, error) {
	for i, historicalNode := range t.historicalAccessNodeClients {
		result, err := historicalNode.GetTransactionResult(ctx, &accessproto.GetTransactionRequest{Id: txID[:]})
		if err != nil {
			// TODO: provide better errors so we can differentiate between all networks searched and
			// it wasn't found, from only some networks successfully searched.
			switch status.Code(err) {
			case codes.Canceled, codes.DeadlineExceeded:
				wrappedErr := fmt.Errorf("could not get transaction result from historical node (%d): %w", i, err)
				return nil, common.NewFailedToQueryExternalNodeError(wrappedErr)
			default:
				// continue searching on any other error
				continue
			}
		}

		// Found on a historical node. Report
		if result.GetStatus() == entities.TransactionStatus_UNKNOWN {
			// We've moved to returning Status UNKNOWN instead of an error with the NotFound status,
			// Therefore we should continue and look at the next access node for answers.
			continue
		}

		if result.GetStatus() == entities.TransactionStatus_PENDING {
			// This is on a historical node. No transactions from it will ever be
			// executed, therefore we should consider this expired
			result.Status = entities.TransactionStatus_EXPIRED
		}

		txResult, err := convert.MessageToTransactionResult(result)
		if err != nil {
			err = fmt.Errorf("could not convert transaction result from historical node (%d): %w", i, err)
			return nil, common.NewInvalidDataFromExternalNodeError("transaction result", flow.ZeroID, err)
		}

		return txResult, nil
	}

	return nil, access.NewDataNotFoundError("transaction result", fmt.Errorf("transaction with ID %s not found on historical nodes", txID))
}

// lookupCollectionIDByBlockAndTxIndex returns the collection ID that contains the transasction with
// the provided transaction index.
//
// If the index is larger that the number of user transactions, flow.ZeroID is returned, indicating
// that the transaction is a system transaction. The caller should verify that the index does in fact
// correspond to a system transaction.
//
// Expected errors during normal operations:
//   - [storage.ErrNotFound] - if any of the collections in the block cannot be found.
func (t *Transactions) lookupCollectionIDByBlockAndTxIndex(block *flow.Block, index uint32) (flow.Identifier, error) {
	txIndex := uint32(0)
	for _, guarantee := range block.Payload.Guarantees {
		collection, err := t.collections.LightByID(guarantee.CollectionID)
		if err != nil {
			return flow.ZeroID, fmt.Errorf("could not find collection %s: %w", guarantee.CollectionID, err)
		}

		for range collection.Transactions {
			if txIndex == index {
				return guarantee.CollectionID, nil
			}
			txIndex++
		}
	}

	// otherwise, assume it's a system transaction and return the ZeroID
	return flow.ZeroID, nil
}

// registerTransactionForRetry registers the transaction for retry if it is not yet executed.
func (t *Transactions) registerTransactionForRetry(tx *flow.TransactionBody) {
	referenceBlock, err := t.state.AtBlockID(tx.ReferenceBlockID).Head()
	if err != nil {
		return
	}

	t.retrier.RegisterTransaction(referenceBlock.Height, tx)
}

// ProcessFinalizedBlockHeight is called to notify the backend that a new block has been finalized.
//
// ATTENTION: might be a source of problems in future. We run this code on finalization gorotuine,
// potentially lagging finalization events if operations take long time.
// We might need to move this logic on dedicated goroutine and provide a way to skip finalization events if they are delivered
// too often for this engine. An example of similar approach - https://github.com/onflow/flow-go/blob/10b0fcbf7e2031674c00f3cdd280f27bd1b16c47/engine/common/follower/compliance_engine.go#L201..
// No errors expected during normal operations.
func (t *Transactions) ProcessFinalizedBlockHeight(height uint64) error {
	return t.retrier.Retry(height)
}
