package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/onflow/flow/protobuf/go/flow/entities"

	"github.com/onflow/flow-go/engine/access/rpc/backend/transactions/error_messages"
	txstatus "github.com/onflow/flow-go/engine/access/rpc/backend/transactions/status"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
	"github.com/onflow/flow-go/storage"
)

// ErrTransactionNotInBlock represents an error indicating that the transaction is not found in the block.
var ErrTransactionNotInBlock = errors.New("transaction not in block")

// LocalTransactionProvider provides functionality for retrieving transaction results and error messages from local storages
type LocalTransactionProvider struct {
	txStatusDeriver     *txstatus.TxStatusDeriver
	executionStateCache optimistic_sync.ExecutionStateCache
}

var _ TransactionProvider = (*LocalTransactionProvider)(nil)

func NewLocalTransactionProvider(
	txStatusDeriver *txstatus.TxStatusDeriver,
	executionStateCache optimistic_sync.ExecutionStateCache,
) *LocalTransactionProvider {
	return &LocalTransactionProvider{
		txStatusDeriver:     txStatusDeriver,
		executionStateCache: executionStateCache,
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
) (*accessmodel.TransactionResult, accessmodel.ExecutorMetadata, error) {
	blockID := header.ID()

	metadata := accessmodel.ExecutorMetadata{
		ExecutionResultID: executionResultInfo.ExecutionResult.ID(),
		ExecutorIDs:       executionResultInfo.ExecutionNodes.NodeIDs(),
	}

	snapshot, err := t.executionStateCache.Snapshot(metadata.ExecutionResultID)
	if err != nil {
		if errors.Is(err, optimistic_sync.ErrSnapshotNotFound) {
			return nil, metadata, fmt.Errorf("could not find snapshot for execution result %s: %w", metadata.ExecutionResultID, err)
		}
		return nil, metadata, fmt.Errorf("failed to get snapshot for execution result %s: %w", metadata.ExecutionResultID, err)
	}

	txResult, err := snapshot.LightTransactionResults().ByBlockIDTransactionID(blockID, transactionID)
	if err != nil {
		// this method does not return an error for empty or missing data, so any error here is an exception
		return nil, metadata, fmt.Errorf("failed to get transaction results: %w", err)
	}

	var txErrorMessage *flow.TransactionResultErrorMessage
	if txResult.Failed {
		txErrorMessage, err = snapshot.TransactionResultErrorMessages().ByBlockIDTransactionID(blockID, transactionID)
		if err != nil && !errors.Is(err, storage.ErrNotFound) {
			return nil, metadata, fmt.Errorf("failed to get transaction error message: %w", err)
		}
	}

	events, err := snapshot.Events().ByBlockIDTransactionID(blockID, transactionID)
	if err != nil {
		// this method will return an empty slice when no data is found, so any error is irrecoverable.
		return nil, metadata, fmt.Errorf("could not find events: %w", err)
	}

	txStatus := t.txStatusDeriver.DeriveExecutedTransactionStatus(snapshot.BlockStatus())

	result, err := t.buildTransactionResult(blockID, header.Height, txResult, txErrorMessage, events, txStatus, collectionID, encodingVersion)
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
) (*accessmodel.TransactionResult, accessmodel.ExecutorMetadata, error) {
	blockID := header.ID()

	metadata := accessmodel.ExecutorMetadata{
		ExecutionResultID: executionResultInfo.ExecutionResult.ID(),
		ExecutorIDs:       executionResultInfo.ExecutionNodes.NodeIDs(),
	}

	snapshot, err := t.executionStateCache.Snapshot(metadata.ExecutionResultID)
	if err != nil {
		if errors.Is(err, optimistic_sync.ErrSnapshotNotFound) {
			return nil, metadata, fmt.Errorf("could not find snapshot for execution result %s: %w", metadata.ExecutionResultID, err)
		}
		return nil, metadata, fmt.Errorf("failed to get snapshot for execution result %s: %w", metadata.ExecutionResultID, err)
	}

	txResult, err := snapshot.LightTransactionResults().ByBlockIDTransactionIndex(blockID, index)
	if err != nil {
		// this method does not return an error for empty or missing data, so any error here is an exception
		return nil, metadata, fmt.Errorf("failed to get transaction results: %w", err)
	}

	var txErrorMessage *flow.TransactionResultErrorMessage
	if txResult.Failed {
		txErrorMessage, err = snapshot.TransactionResultErrorMessages().ByBlockIDTransactionIndex(blockID, index)
		if err != nil && !errors.Is(err, storage.ErrNotFound) {
			return nil, metadata, fmt.Errorf("failed to get transaction error message: %w", err)
		}
	}

	events, err := snapshot.Events().ByBlockIDTransactionIndex(blockID, index)
	if err != nil {
		// this method will return an empty slice when no data is found, so any error is irrecoverable.
		return nil, metadata, fmt.Errorf("could not find events: %w", err)
	}

	txStatus := t.txStatusDeriver.DeriveExecutedTransactionStatus(snapshot.BlockStatus())

	result, err := t.buildTransactionResult(blockID, header.Height, txResult, txErrorMessage, events, txStatus, collectionID, encodingVersion)
	return result, metadata, err
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
) ([]*accessmodel.TransactionResult, accessmodel.ExecutorMetadata, error) {
	blockID := block.ID()

	metadata := accessmodel.ExecutorMetadata{
		ExecutionResultID: executionResultInfo.ExecutionResult.ID(),
		ExecutorIDs:       executionResultInfo.ExecutionNodes.NodeIDs(),
	}

	snapshot, err := t.executionStateCache.Snapshot(metadata.ExecutionResultID)
	if err != nil {
		if errors.Is(err, optimistic_sync.ErrSnapshotNotFound) {
			return nil, metadata, fmt.Errorf("could not find snapshot for execution result %s: %w", metadata.ExecutionResultID, err)
		}
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

	txErrorMessages := make(map[flow.Identifier]*flow.TransactionResultErrorMessage, len(txErrorMessageList))
	for _, txErrorMessage := range txErrorMessageList {
		txErrorMessages[txErrorMessage.TransactionID] = &txErrorMessage
	}

	// cache the tx to collectionID mapping to avoid repeated lookups
	// when looking up the collectionID, if the transaction is missing from the list, the flow.ZeroID
	// is returned. this is the desired behavior for system transactions which have may dynamic ID
	// when scheduled transactions are included.
	txToCollectionID, err := buildTxIDToCollectionIDMapping(block, snapshot.Collections())
	if err != nil {
		// this indicates that one or more of the collections for the block are not indexed. Since
		// lookups are gated on the indexer signaling it has finished processing all data for the
		// block, all data must be available in storage, otherwise there is an inconsistency in the
		// state.
		return nil, metadata, fmt.Errorf("unexpected missing collection: %w", err)
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
			return nil, metadata, fmt.Errorf("failed to get events for tx %s: %w", txID, err)
		}

		result, err := t.buildTransactionResult(blockID, header.Height, &txResult, txErrorMessages[txID], events, txStatus, txToCollectionID[txID], encodingVersion)
		if err != nil {
			return nil, metadata, fmt.Errorf("failed to build transaction result for tx %s: %w", txID, err)
		}

		results = append(results, result)
	}

	return results, metadata, nil
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
func buildTxIDToCollectionIDMapping(
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
