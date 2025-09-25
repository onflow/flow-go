package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/onflow/flow/protobuf/go/flow/entities"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/engine/access/rpc/backend/transactions/error_messages"
	txstatus "github.com/onflow/flow-go/engine/access/rpc/backend/transactions/status"
	"github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/fvm/blueprints"
	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
	"github.com/onflow/flow-go/storage"
)

// ErrTransactionNotInBlock represents an error indicating that the transaction is not found in the block.
var ErrTransactionNotInBlock = errors.New("transaction not in block")

// LocalTransactionProvider provides functionality for retrieving transaction results and error messages from local storages
type LocalTransactionProvider struct {
	collections               storage.CollectionsReader
	systemTxID                flow.Identifier
	txStatusDeriver           *txstatus.TxStatusDeriver
	executionStateCache       optimistic_sync.ExecutionStateCache
	chainID                   flow.ChainID
	scheduledCallbacksEnabled bool
}

var _ TransactionProvider = (*LocalTransactionProvider)(nil)

func NewLocalTransactionProvider(
	collections storage.CollectionsReader,
	systemTxID flow.Identifier,
	txStatusDeriver *txstatus.TxStatusDeriver,
	executionStateCache optimistic_sync.ExecutionStateCache,
	chainID flow.ChainID,
	scheduledCallbacksEnabled bool,
) *LocalTransactionProvider {
	return &LocalTransactionProvider{
		collections:               collections,
		systemTxID:                systemTxID,
		txStatusDeriver:           txStatusDeriver,
		chainID:                   chainID,
		scheduledCallbacksEnabled: scheduledCallbacksEnabled,
		executionStateCache:       executionStateCache,
	}
}

// TransactionResult retrieves a transaction result from storage by block ID and transaction ID.
//
// Expected error returns during normal operation:
//   - [optimistic_sync.ErrSnapshotNotFound] when no snapshot is found for the execution result
func (t *LocalTransactionProvider) TransactionResult(
	_ context.Context,
	header *flow.Header,
	transactionID flow.Identifier,
	collectionID flow.Identifier,
	encodingVersion entities.EventEncodingVersion,
	executionResultInfo *optimistic_sync.ExecutionResultInfo,
) (*accessmodel.TransactionResult, *accessmodel.ExecutorMetadata, error) {
	blockID := header.ID()

	snapshot, err := t.executionStateCache.Snapshot(executionResultInfo.ExecutionResultID)
	if err != nil {
		if errors.Is(err, optimistic_sync.ErrSnapshotNotFound) {
			return nil, nil, fmt.Errorf("could not find snapshot for execution result %s: %w", executionResultInfo.ExecutionResultID, err)
		}
		return nil, nil, fmt.Errorf("failed to get snapshot for execution result %s: %w", executionResultInfo.ExecutionResultID, err)
	}

	txResult, err := snapshot.LightTransactionResults().ByBlockIDTransactionID(blockID, transactionID)
	if err != nil {
		// this method does not return an error for empty or missing data, so any error here is an exception
		return nil, nil, fmt.Errorf("failed to get transaction results: %w", err)
	}

	var txErrorMessage *flow.TransactionResultErrorMessage
	if txResult.Failed {
		txErrorMessage, err = snapshot.TransactionResultErrorMessages().ByBlockIDTransactionID(blockID, transactionID)
		if err != nil && !errors.Is(err, storage.ErrNotFound) {
			return nil, nil, fmt.Errorf("failed to get transaction error message: %w", err)
		}
	}

	events, err := snapshot.Events().ByBlockIDTransactionID(blockID, transactionID)
	if err != nil {
		// this method will return an empty slice when no data is found, so any error is irrecoverable.
		return nil, nil, fmt.Errorf("could not find events: %w", err)
	}

	txStatus := t.txStatusDeriver.DeriveExecutedTransactionStatus(snapshot.BlockStatus())

	result, err := t.buildTransactionResult(blockID, header.Height, txResult, txErrorMessage, events, txStatus, collectionID, encodingVersion)

	metadata := &accessmodel.ExecutorMetadata{
		ExecutionResultID: executionResultInfo.ExecutionResultID,
		ExecutorIDs:       executionResultInfo.ExecutionNodes.NodeIDs(),
	}

	return result, metadata, err
}

// TransactionResultByIndex retrieves a transaction result by index from storage.
//
// Expected error returns during normal operation:
//   - [optimistic_sync.ErrSnapshotNotFound] when no snapshot is found for the execution result
func (t *LocalTransactionProvider) TransactionResultByIndex(
	_ context.Context,
	header *flow.Header,
	index uint32,
	collectionID flow.Identifier,
	encodingVersion entities.EventEncodingVersion,
	executionResultInfo *optimistic_sync.ExecutionResultInfo,
) (*accessmodel.TransactionResult, *accessmodel.ExecutorMetadata, error) {
	blockID := header.ID()

	snapshot, err := t.executionStateCache.Snapshot(executionResultInfo.ExecutionResultID)
	if err != nil {
		if errors.Is(err, optimistic_sync.ErrSnapshotNotFound) {
			return nil, nil, fmt.Errorf("could not find snapshot for execution result %s: %w", executionResultInfo.ExecutionResultID, err)
		}
		return nil, nil, fmt.Errorf("failed to get snapshot for execution result %s: %w", executionResultInfo.ExecutionResultID, err)
	}

	txResult, err := snapshot.LightTransactionResults().ByBlockIDTransactionIndex(blockID, index)
	if err != nil {
		// this method does not return an error for empty or missing data, so any error here is an exception
		return nil, nil, fmt.Errorf("failed to get transaction results: %w", err)
	}

	var txErrorMessage *flow.TransactionResultErrorMessage
	if txResult.Failed {
		txErrorMessage, err = snapshot.TransactionResultErrorMessages().ByBlockIDTransactionIndex(blockID, index)
		if err != nil && !errors.Is(err, storage.ErrNotFound) {
			return nil, nil, fmt.Errorf("failed to get transaction error message: %w", err)
		}
	}

	events, err := snapshot.Events().ByBlockIDTransactionIndex(blockID, index)
	if err != nil {
		// this method will return an empty slice when no data is found, so any error is irrecoverable.
		return nil, nil, fmt.Errorf("could not find events: %w", err)
	}

	txStatus := t.txStatusDeriver.DeriveExecutedTransactionStatus(snapshot.BlockStatus())

	result, err := t.buildTransactionResult(blockID, header.Height, txResult, txErrorMessage, events, txStatus, collectionID, encodingVersion)

	metadata := &accessmodel.ExecutorMetadata{
		ExecutionResultID: executionResultInfo.ExecutionResultID,
		ExecutorIDs:       executionResultInfo.ExecutionNodes.NodeIDs(),
	}

	return result, metadata, err
}

// TransactionsByBlockID retrieves transactions by block ID from storage
// Expected errors during normal operation:
//   - codes.NotFound if result cannot be provided by storage due to the absence of data.
//   - codes.Internal when event payload conversion failed.
//   - indexer.ErrIndexNotInitialized when txResultsIndex not initialized
//   - storage.ErrHeightNotIndexed when data is unavailable
//
// All other errors are considered as state corruption (fatal) or internal errors in the transaction error message
// getter or when deriving transaction status.
func (t *LocalTransactionProvider) TransactionsByBlockID(
	ctx context.Context,
	block *flow.Block,
	executionResultInfo *optimistic_sync.ExecutionResultInfo,
) ([]*flow.TransactionBody, *accessmodel.ExecutorMetadata, error) {
	var transactions []*flow.TransactionBody
	blockID := block.ID()

	snapshot, err := t.executionStateCache.Snapshot(executionResultInfo.ExecutionResultID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get snapshot for execution result %s: %w",
			executionResultInfo.ExecutionResultID, err)
	}

	for _, guarantee := range block.Payload.Guarantees {
		collection, err := t.collections.ByID(guarantee.CollectionID)
		if err != nil {
			return nil, nil, rpc.ConvertStorageError(err)
		}

		transactions = append(transactions, collection.Transactions...)
	}

	if !t.scheduledCallbacksEnabled {
		systemTx, err := blueprints.SystemChunkTransaction(t.chainID.Chain())
		if err != nil {
			return nil, nil, fmt.Errorf("failed to construct system chunk transaction: %w", err)
		}

		// no executor metadata returned because no execution data was used to construct the response
		return append(transactions, systemTx), nil, nil
	}

	events, err := snapshot.Events().ByBlockID(blockID)
	if err != nil {
		return nil, nil, rpc.ConvertIndexError(err, block.Height, "failed to get events")
	}

	sysCollection, err := blueprints.SystemCollection(t.chainID.Chain(), events)
	if err != nil {
		return nil, nil,
			status.Errorf(codes.Internal, "could not construct system collection: %v", err)
	}

	metadata := &accessmodel.ExecutorMetadata{
		ExecutionResultID: executionResultInfo.ExecutionResultID,
		ExecutorIDs:       executionResultInfo.ExecutionNodes.NodeIDs(),
	}

	return append(transactions, sysCollection.Transactions...), metadata, nil
}

// TransactionResultsByBlockID retrieves transaction results by block ID from storage
//
// Expected error returns during normal operation:
//   - [optimistic_sync.ErrSnapshotNotFound] when no snapshot is found for the execution result
func (t *LocalTransactionProvider) TransactionResultsByBlockID(
	_ context.Context,
	block *flow.Block,
	encodingVersion entities.EventEncodingVersion,
	executionResultInfo *optimistic_sync.ExecutionResultInfo,
) ([]*accessmodel.TransactionResult, *accessmodel.ExecutorMetadata, error) {
	blockID := block.ID()

	snapshot, err := t.executionStateCache.Snapshot(executionResultInfo.ExecutionResultID)
	if err != nil {
		if errors.Is(err, optimistic_sync.ErrSnapshotNotFound) {
			return nil, nil, fmt.Errorf("could not find snapshot for execution result %s: %w", executionResultInfo.ExecutionResultID, err)
		}
		return nil, nil, fmt.Errorf("failed to get snapshot for execution result %s: %w", executionResultInfo.ExecutionResultID, err)
	}

	txResults, err := snapshot.LightTransactionResults().ByBlockID(blockID)
	if err != nil {
		// this method does not return an error for empty or missing data, so any error here is an exception
		return nil, nil, fmt.Errorf("failed to get transaction results: %w", err)
	}

	txErrorMessageList, err := snapshot.TransactionResultErrorMessages().ByBlockID(blockID)
	if err != nil {
		if !errors.Is(err, storage.ErrNotFound) {
			return nil, nil, fmt.Errorf("failed to get transaction error messages: %w", err)
		}
		// if no data was found, continue. we will add a placeholder error message.
	}

	txErrorMessages := make(map[flow.Identifier]*flow.TransactionResultErrorMessage, len(txErrorMessageList))
	for _, txErrorMessage := range txErrorMessageList {
		txErrorMessages[txErrorMessage.TransactionID] = &txErrorMessage
	}

	// cache the tx to collectionID mapping to avoid repeated lookups
	// when looking up the collectionID, if the transaction is missing from the list, the flow.ZeroID
	// is returned. this is the desired behavior for system transactions which have may dynamic ID
	// when scheduled transactions are included.
	txToCollectionID, err := t.buildTxIDToCollectionIDMapping(block)
	if err != nil {
		// this indicates that one or more of the collections for the block are not indexed. Since
		// lookups are gated on the indexer signaling it has finished processing all data for the
		// block, all data must be available in storage, otherwise there is an inconsistency in the
		// state.
		return nil, nil, fmt.Errorf("unexpected missing collection: %w", err)
	}

	txStatus := t.txStatusDeriver.DeriveExecutedTransactionStatus(snapshot.BlockStatus())

	eventsReader := snapshot.Events()
	header := block.ToHeader()

	results := make([]*accessmodel.TransactionResult, 0, len(txResults))
	for _, txResult := range txResults {
		txID := txResult.TransactionID

		events, err := eventsReader.ByBlockIDTransactionID(blockID, txResult.TransactionID)
		if err != nil {
			// this method does not return an error for empty or missing data, so any error here is an exception
			return nil, nil, fmt.Errorf("failed to get events for tx %s: %w", txID, err)
		}

		result, err := t.buildTransactionResult(blockID, header.Height, &txResult, txErrorMessages[txID], events, txStatus, txToCollectionID[txID], encodingVersion)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to build transaction result for tx %s: %w", txID, err)
		}

		results = append(results, result)
	}

	metadata := &accessmodel.ExecutorMetadata{
		ExecutionResultID: executionResultInfo.ExecutionResultID,
		ExecutorIDs:       executionResultInfo.ExecutionNodes.NodeIDs(),
	}

	return results, metadata, nil
}

// SystemTransaction rebuilds the system transaction from storage
func (t *LocalTransactionProvider) SystemTransaction(
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

	if txID == t.systemTxID || !t.scheduledCallbacksEnabled {
		systemTx, err := blueprints.SystemChunkTransaction(t.chainID.Chain())
		if err != nil {
			return nil, nil, status.Errorf(codes.Internal, "failed to construct system chunk transaction: %v", err)
		}

		if txID == systemTx.ID() {
			return systemTx, metadata, nil
		}
		return nil, nil, fmt.Errorf("transaction %s not found in block %s", txID, blockID)
	}

	snapshot, err := t.executionStateCache.Snapshot(executionResultInfo.ExecutionResultID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get snapshot for execution result %s: %w",
			executionResultInfo.ExecutionResultID, err)
	}

	events, err := snapshot.Events().ByBlockID(blockID)
	if err != nil {
		return nil, nil, rpc.ConvertIndexError(err, block.Height, "failed to get events")
	}

	sysCollection, err := blueprints.SystemCollection(t.chainID.Chain(), events)
	if err != nil {
		return nil, nil, status.Errorf(codes.Internal, "could not construct system collection: %v", err)
	}

	for _, tx := range sysCollection.Transactions {
		if tx.ID() == txID {
			return tx, metadata, nil
		}
	}

	return nil, nil, status.Errorf(codes.NotFound, "system transaction not found")
}

func (t *LocalTransactionProvider) SystemTransactionResult(
	ctx context.Context,
	block *flow.Block,
	txID flow.Identifier,
	eventEncoding entities.EventEncodingVersion,
	executionResultInfo *optimistic_sync.ExecutionResultInfo,
) (*accessmodel.TransactionResult, *accessmodel.ExecutorMetadata, error) {
	// make sure the request is for a system transaction
	if txID != t.systemTxID {
		if _, metadata, err := t.SystemTransaction(ctx, block, txID, executionResultInfo); err != nil {
			return nil, metadata, status.Errorf(codes.NotFound, "system transaction not found")
		}
	}
	return t.TransactionResult(ctx, block.ToHeader(), txID, flow.ZeroID, eventEncoding, executionResultInfo)
}

// buildTransactionResult formats the provided data into a TransactionResult.
//
// No errors are expected during normal operations.
func (t *LocalTransactionProvider) buildTransactionResult(
	blockID flow.Identifier,
	blockHeight uint64,
	txResult *flow.LightTransactionResult,
	txErrorMessage *flow.TransactionResultErrorMessage,
	events []flow.Event,
	txStatus flow.TransactionStatus,
	collectionID flow.Identifier,
	encodingVersion entities.EventEncodingVersion,
) (*accessmodel.TransactionResult, error) {
	txStatusCode := accessmodel.TransactionStatusCodeSuccess
	var txErrorMessageStr string
	if txResult.Failed {
		if txErrorMessage == nil {
			// it's possible that the error lookup request failed during the indexing process.
			// use a placeholder error message in this case to ensure graceful degradation.
			txErrorMessageStr = error_messages.DefaultFailedErrorMessage
		} else {
			txErrorMessageStr = txErrorMessage.ErrorMessage
		}

		if len(txErrorMessageStr) == 0 {
			// this means that the error message stored in the db is inconsistent with the tx result,
			// which is an irrecoverable exception indicating an inconsistent state.
			return nil, fmt.Errorf("transaction failed but error message is empty")
		}

		txStatusCode = accessmodel.TransactionStatusCodeFailed
	}

	// events are encoded in CCF format in storage. convert to JSON-CDC if requested
	if encodingVersion == entities.EventEncodingVersion_JSON_CDC_V0 {
		var err error
		events, err = convert.CcfEventsToJsonEvents(events)
		if err != nil {
			// if conversion fails, one of these cases must be true:
			// 1. the data added to the block is invalid
			// 2. the data in storage is corrupted
			// 3. there is a software bug or configuration mismatch
			// all cases point to either inconsistent state or a software bug, and continuing is not safe.
			return nil, fmt.Errorf("failed to convert event payloads: %w", err)
		}
	}

	return &accessmodel.TransactionResult{
		TransactionID: txResult.TransactionID,
		Status:        txStatus,
		StatusCode:    txStatusCode,
		Events:        events,
		ErrorMessage:  txErrorMessageStr,
		BlockID:       blockID,
		BlockHeight:   blockHeight,
		CollectionID:  collectionID,
	}, nil
}

// buildTxIDToCollectionIDMapping returns a map of transaction ID to collection ID based on the provided block.
//
// Expected errors during normal operations:
//   - [storage.ErrNotFound] if a collection within the block is not found.
func (t *LocalTransactionProvider) buildTxIDToCollectionIDMapping(block *flow.Block) (map[flow.Identifier]flow.Identifier, error) {
	txToCollectionID := make(map[flow.Identifier]flow.Identifier)
	for _, guarantee := range block.Payload.Guarantees {
		collection, err := t.collections.LightByID(guarantee.CollectionID)
		if err != nil {
			return nil, fmt.Errorf("could not find collection %s in indexed block: %w", guarantee.CollectionID, err)
		}
		for _, txID := range collection.Transactions {
			txToCollectionID[txID] = guarantee.CollectionID
		}
	}

	return txToCollectionID, nil
}
