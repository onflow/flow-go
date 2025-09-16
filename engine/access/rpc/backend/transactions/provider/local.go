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
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// ErrTransactionNotInBlock represents an error indicating that the transaction is not found in the block.
var ErrTransactionNotInBlock = errors.New("transaction not in block")

// LocalTransactionProvider provides functionality for retrieving transaction results and error messages from local storages
type LocalTransactionProvider struct {
	state                     protocol.State
	blocks                    storage.Blocks
	systemTxID                flow.Identifier
	txStatusDeriver           *txstatus.TxStatusDeriver
	executionStateCache       optimistic_sync.ExecutionStateCache
	scheduledCallbacksEnabled bool
	chainID                   flow.ChainID
}

var _ TransactionProvider = (*LocalTransactionProvider)(nil)

func NewLocalTransactionProvider(
	state protocol.State,
	blocks storage.Blocks,
	systemTxID flow.Identifier,
	txStatusDeriver *txstatus.TxStatusDeriver,
	executionStateCache optimistic_sync.ExecutionStateCache,
	chainID flow.ChainID,
	scheduledCallbacksEnabled bool,
) *LocalTransactionProvider {
	return &LocalTransactionProvider{
		state:                     state,
		blocks:                    blocks,
		systemTxID:                systemTxID,
		txStatusDeriver:           txStatusDeriver,
		executionStateCache:       executionStateCache,
		scheduledCallbacksEnabled: scheduledCallbacksEnabled,
		chainID:                   chainID,
	}
}

// TransactionResult retrieves a transaction result from storage by block ID and transaction ID.
//
// Expected error returns during normal operation:
//   - [storage.ErrNotFound] when no snapshot is found for the execution result
func (t *LocalTransactionProvider) TransactionResult(
	_ context.Context,
	block *flow.Header,
	transactionID flow.Identifier,
	encodingVersion entities.EventEncodingVersion,
	executionResultInfo *optimistic_sync.ExecutionResultInfo,
) (*accessmodel.TransactionResult, accessmodel.ExecutorMetadata, error) {
	blockID := block.ID()

	metadata := accessmodel.ExecutorMetadata{
		ExecutionResultID: executionResultInfo.ExecutionResult.ID(),
		ExecutorIDs:       executionResultInfo.ExecutionNodes.NodeIDs(),
	}

	snapshot, err := t.executionStateCache.Snapshot(metadata.ExecutionResultID)
	if err != nil {
		return nil, metadata, fmt.Errorf("failed to get snapshot for execution result %s: %w", metadata.ExecutionResultID, err)
	}

	txResult, err := snapshot.LightTransactionResults().ByBlockIDTransactionID(blockID, transactionID)
	if err != nil {
		// this method does not return an error for empty or missing data, so any error here is an exception
		return nil, metadata, fmt.Errorf("failed to get transaction results: %w", err)
	}

	var txErrorMessageStr string
	var txStatusCode uint = 0
	if txResult.Failed {
		txErrorMessage, err := snapshot.TransactionResultErrorMessages().ByBlockIDTransactionID(blockID, transactionID)
		if err != nil {
			if !errors.Is(err, storage.ErrNotFound) {
				return nil, metadata, fmt.Errorf("failed to get transaction error message: %w", err)
			}
			// it's possible that the error lookup request failed during the indexing process.
			// use a placeholder error message in this case to ensure graceful degradation.
			txErrorMessageStr = error_messages.DefaultFailedErrorMessage
		} else {
			txErrorMessageStr = txErrorMessage.ErrorMessage
		}

		if len(txErrorMessageStr) == 0 {
			// this means that the error message stored in the db is inconsistent with the tx result.
			return nil, metadata, fmt.Errorf("transaction failed but error message is empty")
		}

		txStatusCode = 1 // statusCode of 1 indicates an error and 0 indicates no error, the same as on EN
	}

	events, err := snapshot.Events().ByBlockIDTransactionID(blockID, transactionID)
	if err != nil {
		// this method will return an empty slice when no data is found, so any error is irrecoverable.
		return nil, metadata, fmt.Errorf("could not find events: %w", err)
	}

	// events are encoded in CCF format in storage. convert to JSON-CDC if requested
	if encodingVersion == entities.EventEncodingVersion_JSON_CDC_V0 {
		events, err = convert.CcfEventsToJsonEvents(events)
		if err != nil {
			// if conversion fails, one of these cases must be true:
			// 1. the data added to the block is invalid
			// 2. the data in storage is corrupted
			// 3. there is a software bug or configuration mismatch
			// all cases point to either inconsistent state or a software bug, and continuing is not safe.
			return nil, metadata, fmt.Errorf("failed to convert event payloads: %w", err)
		}
	}

	txStatus := t.txStatusDeriver.DeriveExecutedTransactionStatus(snapshot.BlockStatus())

	return &accessmodel.TransactionResult{
		TransactionID: txResult.TransactionID,
		Status:        txStatus,
		StatusCode:    txStatusCode,
		Events:        events,
		ErrorMessage:  txErrorMessageStr,
		BlockID:       blockID,
		BlockHeight:   block.Height,
	}, metadata, nil
}

// TransactionResultByIndex retrieves a transaction result by index from storage.
//
// Expected error returns during normal operation:
//   - [storage.ErrNotFound] when no snapshot is found for the execution result
func (t *LocalTransactionProvider) TransactionResultByIndex(
	_ context.Context,
	block *flow.Block,
	index uint32,
	eventEncoding entities.EventEncodingVersion,
	executionResultInfo *optimistic_sync.ExecutionResultInfo,
) (*accessmodel.TransactionResult, accessmodel.ExecutorMetadata, error) {
	blockID := block.ID()

	metadata := accessmodel.ExecutorMetadata{
		ExecutionResultID: executionResultInfo.ExecutionResult.ID(),
		ExecutorIDs:       executionResultInfo.ExecutionNodes.NodeIDs(),
	}

	snapshot, err := t.executionStateCache.Snapshot(metadata.ExecutionResultID)
	if err != nil {
		return nil, metadata, fmt.Errorf("failed to get snapshot for execution result %s: %w", metadata.ExecutionResultID, err)
	}

	txResult, err := snapshot.LightTransactionResults().ByBlockIDTransactionIndex(blockID, index)
	if err != nil {
		// this method does not return an error for empty or missing data, so any error here is an exception
		return nil, metadata, fmt.Errorf("failed to get transaction results: %w", err)
	}

	var txErrorMessageStr string
	var txStatusCode uint = 0
	if txResult.Failed {
		txErrorMessage, err := snapshot.TransactionResultErrorMessages().ByBlockIDTransactionIndex(blockID, index)
		if err != nil {
			if !errors.Is(err, storage.ErrNotFound) {
				return nil, metadata, fmt.Errorf("failed to get transaction error message: %w", err)
			}
			// it's possible that the error lookup request failed during the indexing process.
			// use a placeholder error message in this case to ensure graceful degradation.
			txErrorMessageStr = error_messages.DefaultFailedErrorMessage
		} else {
			txErrorMessageStr = txErrorMessage.ErrorMessage
		}

		if len(txErrorMessage.ErrorMessage) == 0 {
			// this means that the error message stored in the db is inconsistent with the tx result.
			return nil, metadata, fmt.Errorf("transaction failed but error message is empty")
		}

		txStatusCode = 1 // statusCode of 1 indicates an error and 0 indicates no error, the same as on EN
	}

	events, err := snapshot.Events().ByBlockIDTransactionIndex(blockID, index)
	if err != nil {
		return nil, metadata, rpc.ConvertIndexError(err, block.Height, "failed to get events")
	}

	// events are encoded in CCF format in storage. convert to JSON-CDC if requested
	if eventEncoding == entities.EventEncodingVersion_JSON_CDC_V0 {
		events, err = convert.CcfEventsToJsonEvents(events)
		if err != nil {
			// if conversion fails, one of these cases must be true:
			// 1. the data added to the block is invalid
			// 2. the data in storage is corrupted
			// 3. there is a software bug or configuration mismatch
			// all cases point to either inconsistent state or a software bug, and continuing is not safe.
			return nil, metadata, fmt.Errorf("failed to convert event payloads: %w", err)
		}
	}

	collectionID, err := t.lookupCollectionIDInBlock(block, txResult.TransactionID, snapshot.Collections())
	if err != nil {
		// since tx results and collections are indexed together within the snapshot, this call
		// must succeed, otherwise there is an inconsistency in the state.
		return nil, metadata, err
	}

	txStatus := t.txStatusDeriver.DeriveExecutedTransactionStatus(snapshot.BlockStatus())

	return &accessmodel.TransactionResult{
		TransactionID: txResult.TransactionID,
		Status:        txStatus,
		StatusCode:    txStatusCode,
		Events:        events,
		ErrorMessage:  txErrorMessageStr,
		BlockID:       blockID,
		BlockHeight:   block.Height,
		CollectionID:  collectionID,
	}, metadata, nil
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
) ([]*flow.TransactionBody, accessmodel.ExecutorMetadata, error) {
	var transactions []*flow.TransactionBody
	blockID := block.ID()

	metadata := accessmodel.ExecutorMetadata{
		ExecutionResultID: executionResultInfo.ExecutionResult.ID(),
		ExecutorIDs:       executionResultInfo.ExecutionNodes.NodeIDs(),
	}

	snapshot, err := t.executionStateCache.Snapshot(metadata.ExecutionResultID)
	if err != nil {
		return nil, metadata, fmt.Errorf("failed to get snapshot for execution result %s: %w", metadata.ExecutionResultID, err)
	}

	collectionsReader := snapshot.Collections()

	for _, guarantee := range block.Payload.Guarantees {
		collection, err := collectionsReader.ByID(guarantee.CollectionID)
		if err != nil {
			return nil, metadata, rpc.ConvertStorageError(err)
		}

		transactions = append(transactions, collection.Transactions...)
	}

	if !t.scheduledCallbacksEnabled {
		systemTx, err := blueprints.SystemChunkTransaction(t.chainID.Chain())
		if err != nil {
			return nil, metadata, fmt.Errorf("failed to construct system chunk transaction: %w", err)
		}

		return append(transactions, systemTx), metadata, nil
	}

	events, err := snapshot.Events().ByBlockID(blockID)
	if err != nil {
		return nil, metadata, rpc.ConvertIndexError(err, block.Height, "failed to get events")
	}

	sysCollection, err := blueprints.SystemCollection(t.chainID.Chain(), events)
	if err != nil {
		return nil, metadata, status.Errorf(codes.Internal, "could not construct system collection: %v", err)
	}

	return append(transactions, sysCollection.Transactions...), metadata, nil
}

// TransactionResultsByBlockID retrieves transaction results by block ID from storage
//
// Expected error returns during normal operation:
//   - [storage.ErrNotFound] when no snapshot is found for the execution result
func (t *LocalTransactionProvider) TransactionResultsByBlockID(
	_ context.Context,
	block *flow.Block,
	eventEncoding entities.EventEncodingVersion,
	executionResultInfo *optimistic_sync.ExecutionResultInfo,
) ([]*accessmodel.TransactionResult, accessmodel.ExecutorMetadata, error) {
	blockID := block.ID()

	metadata := accessmodel.ExecutorMetadata{
		ExecutionResultID: executionResultInfo.ExecutionResult.ID(),
		ExecutorIDs:       executionResultInfo.ExecutionNodes.NodeIDs(),
	}

	snapshot, err := t.executionStateCache.Snapshot(metadata.ExecutionResultID)
	if err != nil {
		return nil, metadata, fmt.Errorf("failed to get snapshot for execution result %s: %w", metadata.ExecutionResultID, err)
	}

	txResults, err := snapshot.LightTransactionResults().ByBlockID(blockID)
	if err != nil {
		// this method does not return an error for empty or missing data, so any error here is an exception
		return nil, metadata, fmt.Errorf("failed to get transaction results: %w", err)
	}

	txErrorMessageList, err := snapshot.TransactionResultErrorMessages().ByBlockID(blockID)
	if err != nil {
		if !errors.Is(err, storage.ErrNotFound) {
			return nil, metadata, fmt.Errorf("failed to get transaction error messages: %w", err)
		}
		// if no data was found, continue. we will add a placeholder error message.
	}

	txErrorMessages := make(map[flow.Identifier]string, len(txErrorMessageList))
	for _, txErrorMessage := range txErrorMessageList {
		txErrorMessages[txErrorMessage.TransactionID] = txErrorMessage.ErrorMessage
	}

	numberOfTxResults := len(txResults)
	results := make([]*accessmodel.TransactionResult, 0, numberOfTxResults)

	// cache the tx to collectionID mapping to avoid repeated lookups
	txToCollectionID, err := t.buildTxIDToCollectionIDMapping(block, snapshot.Collections())
	if err != nil {
		// this indicates that one or more of the collections for the block are not indexed. Since
		// lookups are gated on the indexer signaling it has finished processing all data for the
		// block, all data must be available in storage, otherwise there is an inconsistency in the
		// state.
		return nil, metadata, fmt.Errorf("unexpected missing collection: %w", err)
	}

	txStatus := t.txStatusDeriver.DeriveExecutedTransactionStatus(snapshot.BlockStatus())

	eventsReader := snapshot.Events()

	for _, txResult := range txResults {
		txID := txResult.TransactionID

		var txErrorMessageStr string
		var txStatusCode uint = 0
		if txResult.Failed {
			var ok bool
			txErrorMessageStr, ok = txErrorMessages[txID]
			if !ok {
				txErrorMessageStr = error_messages.DefaultFailedErrorMessage
			}

			if len(txErrorMessageStr) == 0 {
				// this means that the error message stored in the db is inconsistent with the tx result.
				return nil, metadata, fmt.Errorf("transaction failed but error message is empty (txID: %s)", txID)
			}

			txStatusCode = 1 // statusCode of 1 indicates an error and 0 indicates no error, the same as on EN
		}

		events, err := eventsReader.ByBlockIDTransactionID(blockID, txResult.TransactionID)
		if err != nil {
			// this method does not return an error for empty or missing data, so any error here is an exception
			return nil, metadata, fmt.Errorf("failed to get events (txID: %s): %w", txID, err)
		}

		// events are encoded in CCF format in storage. convert to JSON-CDC if requested
		if eventEncoding == entities.EventEncodingVersion_JSON_CDC_V0 {
			events, err = convert.CcfEventsToJsonEvents(events)
			if err != nil {
				// if conversion fails, one of these cases must be true:
				// 1. the data added to the block is invalid
				// 2. the data in storage is corrupted
				// 3. there is a software bug or configuration mismatch
				// all cases point to either inconsistent state or a software bug, and continuing is not safe.
				return nil, metadata, fmt.Errorf("failed to convert event payloads (txID: %s): %w", txID, err)
			}
		}

		collectionID, ok := txToCollectionID[txID]
		if !ok {
			// for all the transactions that are not in the block's user collections we assign the
			// ZeroID indicating system collection.
			collectionID = flow.ZeroID
		}

		results = append(results, &accessmodel.TransactionResult{
			Status:        txStatus,
			StatusCode:    txStatusCode,
			Events:        events,
			ErrorMessage:  txErrorMessageStr,
			BlockID:       blockID,
			TransactionID: txID,
			CollectionID:  collectionID,
			BlockHeight:   block.Height,
		})
	}

	return results, metadata, nil
}

// SystemTransaction rebuilds the system transaction from storage
func (t *LocalTransactionProvider) SystemTransaction(
	ctx context.Context,
	block *flow.Block,
	txID flow.Identifier,
	executionResultInfo *optimistic_sync.ExecutionResultInfo,
) (*flow.TransactionBody, accessmodel.ExecutorMetadata, error) {
	blockID := block.ID()

	if txID == t.systemTxID || !t.scheduledCallbacksEnabled {
		systemTx, err := blueprints.SystemChunkTransaction(t.chainID.Chain())
		if err != nil {
			return nil, accessmodel.ExecutorMetadata{}, status.Errorf(codes.Internal, "failed to construct system chunk transaction: %v", err)
		}

		if txID == systemTx.ID() {
			return systemTx, accessmodel.ExecutorMetadata{}, nil
		}
		return nil, accessmodel.ExecutorMetadata{}, fmt.Errorf("transaction %s not found in block %s", txID, blockID)
	}

	metadata := accessmodel.ExecutorMetadata{
		ExecutionResultID: executionResultInfo.ExecutionResult.ID(),
		ExecutorIDs:       executionResultInfo.ExecutionNodes.NodeIDs(),
	}

	snapshot, err := t.executionStateCache.Snapshot(metadata.ExecutionResultID)
	if err != nil {
		return nil, metadata, fmt.Errorf("failed to get snapshot for execution result %s: %w", metadata.ExecutionResultID, err)
	}

	events, err := snapshot.Events().ByBlockID(blockID)
	if err != nil {
		return nil, metadata, rpc.ConvertIndexError(err, block.Height, "failed to get events")
	}

	sysCollection, err := blueprints.SystemCollection(t.chainID.Chain(), events)
	if err != nil {
		return nil, metadata, status.Errorf(codes.Internal, "could not construct system collection: %v", err)
	}

	for _, tx := range sysCollection.Transactions {
		if tx.ID() == txID {
			return tx, metadata, nil
		}
	}

	return nil, metadata, status.Errorf(codes.NotFound, "system transaction not found")
}

func (t *LocalTransactionProvider) SystemTransactionResult(
	ctx context.Context,
	block *flow.Block,
	txID flow.Identifier,
	eventEncoding entities.EventEncodingVersion,
	executionResultInfo *optimistic_sync.ExecutionResultInfo,
) (*accessmodel.TransactionResult, accessmodel.ExecutorMetadata, error) {
	// make sure the request is for a system transaction
	if txID != t.systemTxID {
		if _, metadata, err := t.SystemTransaction(ctx, block, txID, executionResultInfo); err != nil {
			return nil, metadata, status.Errorf(codes.NotFound, "system transaction not found")
		}
	}
	return t.TransactionResult(ctx, block.ToHeader(), txID, eventEncoding, executionResultInfo)
}

// lookupCollectionIDInBlock returns the collection ID based on the transaction ID.
// The lookup is performed in block collections.
//
// Expected errors during normal operations:
//   - [storage.ErrNotFound] if a collection within the block is not found.
//   - [ErrTransactionNotInBlock] if the transaction is not found in the block.
func (t *LocalTransactionProvider) lookupCollectionIDInBlock(
	block *flow.Block,
	txID flow.Identifier,
	collectionsReader storage.CollectionsReader,
) (flow.Identifier, error) {
	for _, guarantee := range block.Payload.Guarantees {
		collection, err := collectionsReader.LightByID(guarantee.CollectionID)
		if err != nil {
			return flow.ZeroID, fmt.Errorf("could not find collection %s in indexed block: %w", guarantee.CollectionID, err)
		}

		for _, collectionTxID := range collection.Transactions {
			if collectionTxID == txID {
				return guarantee.CollectionID, nil
			}
		}
	}
	return flow.ZeroID, ErrTransactionNotInBlock
}

// buildTxIDToCollectionIDMapping returns a map of transaction ID to collection ID based on the provided block.
//
// Expected errors during normal operations:
//   - [storage.ErrNotFound] if a collection within the block is not found.
func (t *LocalTransactionProvider) buildTxIDToCollectionIDMapping(
	block *flow.Block,
	collectionsReader storage.CollectionsReader,
) (map[flow.Identifier]flow.Identifier, error) {
	txToCollectionID := make(map[flow.Identifier]flow.Identifier)
	for _, guarantee := range block.Payload.Guarantees {
		collection, err := collectionsReader.LightByID(guarantee.CollectionID)
		if err != nil {
			return nil, fmt.Errorf("could not find collection %s in indexed block: %w", guarantee.CollectionID, err)
		}
		for _, txID := range collection.Transactions {
			txToCollectionID[txID] = guarantee.CollectionID
		}
	}

	return txToCollectionID, nil
}
