package provider

import (
	"context"
	"errors"
	"fmt"

	"google.golang.org/grpc/status"

	"github.com/onflow/flow/protobuf/go/flow/entities"
	"google.golang.org/grpc/codes"

	txstatus "github.com/onflow/flow-go/engine/access/rpc/backend/transactions/status"
	"github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/state"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// ErrTransactionNotInBlock represents an error indicating that the transaction is not found in the block.
var ErrTransactionNotInBlock = errors.New("transaction not in block")

// LocalTransactionProvider provides functionality for retrieving transaction results and error messages from local storages
type LocalTransactionProvider struct {
	state                   protocol.State
	collections             storage.Collections
	blocks                  storage.Blocks
	systemTxID              flow.Identifier
	txStatusDeriver         *txstatus.TxStatusDeriver
	executionResultProvider optimistic_sync.ExecutionResultProvider
	executionStateCache     optimistic_sync.ExecutionStateCache
}

var _ TransactionProvider = (*LocalTransactionProvider)(nil)

func NewLocalTransactionProvider(
	state protocol.State,
	collections storage.Collections,
	blocks storage.Blocks,
	systemTxID flow.Identifier,
	txStatusDeriver *txstatus.TxStatusDeriver,
	executionResultProvider optimistic_sync.ExecutionResultProvider,
	executionStateCache optimistic_sync.ExecutionStateCache,
) *LocalTransactionProvider {
	return &LocalTransactionProvider{
		state:                   state,
		collections:             collections,
		blocks:                  blocks,
		systemTxID:              systemTxID,
		txStatusDeriver:         txStatusDeriver,
		executionResultProvider: executionResultProvider,
		executionStateCache:     executionStateCache,
	}
}

// TransactionResult retrieves a transaction result from storage by block ID and transaction ID.
// Expected errors during normal operation:
//   - codes.NotFound when result cannot be provided by storage due to the absence of data.
//   - codes.Internal if event payload conversion failed.
//   - indexer.ErrIndexNotInitialized when txResultsIndex not initialized
//   - storage.ErrHeightNotIndexed when data is unavailable
//   - All other errors are potential indicators of bugs or corrupted internal state (continuation impossible)
func (t *LocalTransactionProvider) TransactionResult(
	ctx context.Context,
	block *flow.Header,
	transactionID flow.Identifier,
	encodingVersion entities.EventEncodingVersion,
	query entities.ExecutionStateQuery,
) (*accessmodel.TransactionResult, *optimistic_sync.ExecutionResultInfo, error) {
	blockID := block.ID()

	snapshot, executionResultInfo, err := t.getSnapshotForBlock(blockID, query)
	if err != nil {
		return nil, nil, err
	}

	txResultsReader := snapshot.LightTransactionResults()

	txResult, err := txResultsReader.ByBlockIDTransactionID(blockID, transactionID)
	if err != nil {
		return nil, executionResultInfo, rpc.ConvertIndexError(err, block.Height, "failed to get transaction result")
	}

	var txErrorMessageStr string
	var txStatusCode uint = 0
	if txResult.Failed {
		txErrorMessagesReader := snapshot.TransactionResultErrorMessages()
		txErrorMessage, err := txErrorMessagesReader.ByBlockIDTransactionID(blockID, transactionID)
		if err != nil {
			return nil, executionResultInfo, err
		}

		if len(txErrorMessage.ErrorMessage) == 0 {
			return nil, executionResultInfo, status.Errorf(
				codes.Internal,
				"transaction failed but error message is empty for tx ID: %s block ID: %s",
				txErrorMessage.TransactionID,
				blockID,
			)
		}

		txErrorMessageStr = txErrorMessage.ErrorMessage
		txStatusCode = 1 // statusCode of 1 indicates an error and 0 indicates no error, the same as on EN
	}

	txStatus, err := t.txStatusDeriver.DeriveTransactionStatus(block.Height, true)
	if err != nil {
		if !errors.Is(err, state.ErrUnknownSnapshotReference) {
			irrecoverable.Throw(ctx, err)
		}
		return nil, executionResultInfo, rpc.ConvertStorageError(err)
	}

	eventsReader := snapshot.Events()
	events, err := eventsReader.ByBlockIDTransactionID(blockID, transactionID)
	if err != nil {
		return nil, executionResultInfo, rpc.ConvertIndexError(err, block.Height, "failed to get events")
	}

	// events are encoded in CCF format in storage. convert to JSON-CDC if requested
	if encodingVersion == entities.EventEncodingVersion_JSON_CDC_V0 {
		events, err = convert.CcfEventsToJsonEvents(events)
		if err != nil {
			return nil, executionResultInfo, rpc.ConvertError(err, "failed to convert event payload", codes.Internal)
		}
	}

	return &accessmodel.TransactionResult{
		TransactionID: txResult.TransactionID,
		Status:        txStatus,
		StatusCode:    txStatusCode,
		Events:        events,
		ErrorMessage:  txErrorMessageStr,
		BlockID:       blockID,
		BlockHeight:   block.Height,
	}, executionResultInfo, nil
}

// TransactionResultByIndex retrieves a transaction result by index from storage.
// Expected errors during normal operation:
//   - codes.NotFound if result cannot be provided by storage due to the absence of data.
//   - codes.Internal when event payload conversion failed.
//   - indexer.ErrIndexNotInitialized when txResultsIndex not initialized
//   - storage.ErrHeightNotIndexed when data is unavailable
//   - All other errors are potential indicators of bugs or corrupted internal state (continuation impossible)
func (t *LocalTransactionProvider) TransactionResultByIndex(
	ctx context.Context,
	block *flow.Block,
	index uint32,
	requiredEventEncodingVersion entities.EventEncodingVersion,
	query entities.ExecutionStateQuery,
) (*accessmodel.TransactionResult, *optimistic_sync.ExecutionResultInfo, error) {
	blockID := block.ID()

	snapshot, executionResultInfo, err := t.getSnapshotForBlock(blockID, query)
	if err != nil {
		return nil, nil, err
	}

	txResultsReader := snapshot.LightTransactionResults()

	txResult, err := txResultsReader.ByBlockIDTransactionIndex(blockID, index)
	if err != nil {
		return nil, executionResultInfo, rpc.ConvertIndexError(err, block.Header.Height,
			"failed to get transaction result")
	}

	var txErrorMessageStr string
	var txStatusCode uint = 0
	if txResult.Failed {
		txErrorMessagesReader := snapshot.TransactionResultErrorMessages()
		txErrorMessage, err := txErrorMessagesReader.ByBlockIDTransactionIndex(blockID, index)
		if err != nil {
			return nil, executionResultInfo, err
		}

		if len(txErrorMessage.ErrorMessage) == 0 {
			return nil, executionResultInfo, status.Errorf(
				codes.Internal,
				"transaction failed but error message is empty for tx ID: %s block ID: %s",
				txErrorMessage.TransactionID,
				blockID,
			)
		}

		txErrorMessageStr = txErrorMessage.ErrorMessage
		txStatusCode = 1 // statusCode of 1 indicates an error and 0 indicates no error, the same as on EN
	}

	txStatus, err := t.txStatusDeriver.DeriveTransactionStatus(block.Header.Height, true)
	if err != nil {
		if !errors.Is(err, state.ErrUnknownSnapshotReference) {
			irrecoverable.Throw(ctx, err)
		}
		return nil, executionResultInfo, rpc.ConvertStorageError(err)
	}

	eventsReader := snapshot.Events()
	events, err := eventsReader.ByBlockIDTransactionIndex(blockID, index)
	if err != nil {
		return nil, executionResultInfo, rpc.ConvertIndexError(err, block.Header.Height, "failed to get events")
	}

	// events are encoded in CCF format in storage. convert to JSON-CDC if requested
	if requiredEventEncodingVersion == entities.EventEncodingVersion_JSON_CDC_V0 {
		events, err = convert.CcfEventsToJsonEvents(events)
		if err != nil {
			return nil, executionResultInfo, rpc.ConvertError(err, "failed to convert event payload", codes.Internal)
		}
	}

	// TODO: Do we need lookupCollectionID
	collectionID, err := t.lookupCollectionIDInBlock(block, txResult.TransactionID, snapshot.Collections())
	if err != nil {
		return nil, executionResultInfo, err
	}

	return &accessmodel.TransactionResult{
		TransactionID: txResult.TransactionID,
		Status:        txStatus,
		StatusCode:    txStatusCode,
		Events:        events,
		ErrorMessage:  txErrorMessageStr,
		BlockID:       blockID,
		BlockHeight:   block.Header.Height,
		CollectionID:  collectionID,
	}, executionResultInfo, nil
}

// TransactionResultsByBlockID retrieves transaction results by block ID from storage
// Expected errors during normal operation:
//   - codes.NotFound if result cannot be provided by storage due to the absence of data.
//   - codes.Internal when event payload conversion failed.
//   - indexer.ErrIndexNotInitialized when txResultsIndex not initialized
//   - storage.ErrHeightNotIndexed when data is unavailable
//   - All other errors are potential indicators of bugs or corrupted internal state (continuation impossible)
func (t *LocalTransactionProvider) TransactionResultsByBlockID(
	ctx context.Context,
	block *flow.Block,
	requiredEventEncodingVersion entities.EventEncodingVersion,
	query entities.ExecutionStateQuery,
) ([]*accessmodel.TransactionResult, *optimistic_sync.ExecutionResultInfo, error) {
	blockID := block.ID()

	snapshot, executionResultInfo, err := t.getSnapshotForBlock(blockID, query)
	if err != nil {
		return nil, nil, err
	}

	txResultsReader := snapshot.LightTransactionResults()

	txResults, err := txResultsReader.ByBlockID(blockID)
	if err != nil {
		return nil, executionResultInfo, rpc.ConvertIndexError(err, block.Header.Height,
			"failed to get transaction result")
	}

	txErrorMessagesReader := snapshot.TransactionResultErrorMessages()

	numberOfTxResults := len(txResults)
	results := make([]*accessmodel.TransactionResult, 0, numberOfTxResults)

	// cache the tx to collectionID mapping to avoid repeated lookups
	txToCollectionID, err := t.buildTxIDToCollectionIDMapping(block, snapshot.Collections())
	if err != nil {
		// this indicates that one or more of the collections for the block are not indexed. Since
		// lookups are gated on the indexer signaling it has finished processing all data for the
		// block, all data must be available in storage, otherwise there is an inconsistency in the
		// state.
		irrecoverable.Throw(ctx, fmt.Errorf("inconsistent index state: %w", err))
		return nil, executionResultInfo, status.Errorf(codes.Internal, "failed to map tx to collection ID: %v", err)
	}

	eventsReader := snapshot.Events()

	for _, txResult := range txResults {
		txID := txResult.TransactionID

		var txErrorMessageStr string
		var txStatusCode uint = 0
		if txResult.Failed {
			txErrorMessage, err := txErrorMessagesReader.ByBlockIDTransactionID(blockID, txID)
			if err != nil {
				return nil, executionResultInfo, err
			}

			if len(txErrorMessage.ErrorMessage) == 0 {
				return nil, executionResultInfo, status.Errorf(
					codes.Internal,
					"transaction failed but error message is empty for tx ID: %s block ID: %s",
					txErrorMessage.TransactionID,
					blockID,
				)
			}

			txErrorMessageStr = txErrorMessage.ErrorMessage
			txStatusCode = 1 // statusCode of 1 indicates an error and 0 indicates no error, the same as on EN
		}

		txStatus, err := t.txStatusDeriver.DeriveTransactionStatus(block.Header.Height, true)
		if err != nil {
			if !errors.Is(err, state.ErrUnknownSnapshotReference) {
				irrecoverable.Throw(ctx, err)
			}
			return nil, executionResultInfo, rpc.ConvertStorageError(err)
		}

		events, err := eventsReader.ByBlockIDTransactionID(blockID, txResult.TransactionID)
		if err != nil {
			return nil, executionResultInfo, rpc.ConvertIndexError(err, block.Header.Height, "failed to get events")
		}

		// events are encoded in CCF format in storage. convert to JSON-CDC if requested
		if requiredEventEncodingVersion == entities.EventEncodingVersion_JSON_CDC_V0 {
			events, err = convert.CcfEventsToJsonEvents(events)
			if err != nil {
				return nil, executionResultInfo, rpc.ConvertError(err, "failed to convert event payload",
					codes.Internal)
			}
		}

		collectionID, ok := txToCollectionID[txID]
		if !ok {
			return nil, executionResultInfo, status.Errorf(codes.Internal, "transaction %s not found in block %s",
				txID, blockID)
		}

		results = append(results, &accessmodel.TransactionResult{
			Status:        txStatus,
			StatusCode:    txStatusCode,
			Events:        events,
			ErrorMessage:  txErrorMessageStr,
			BlockID:       blockID,
			TransactionID: txID,
			CollectionID:  collectionID,
			BlockHeight:   block.Header.Height,
		})
	}

	return results, executionResultInfo, nil
}

// lookupCollectionIDInBlock returns the collection ID based on the transaction ID.
// The lookup is performed in block collections.
func (t *LocalTransactionProvider) lookupCollectionIDInBlock(
	block *flow.Block,
	txID flow.Identifier,
	collectionsReader storage.CollectionsReader,
) (flow.Identifier, error) {
	for _, guarantee := range block.Payload.Guarantees {
		collectionID := guarantee.ID()
		collection, err := collectionsReader.LightByID(collectionID)
		if err != nil {
			return flow.ZeroID, fmt.Errorf("failed to get collection %s in indexed block: %w", collectionID, err)
		}

		for _, collectionTxID := range collection.Transactions {
			if collectionTxID == txID {
				return collectionID, nil
			}
		}
	}
	return flow.ZeroID, ErrTransactionNotInBlock
}

// buildTxIDToCollectionIDMapping returns a map of transaction ID to collection ID based on the provided block.
// No errors expected during normal operations.
func (t *LocalTransactionProvider) buildTxIDToCollectionIDMapping(block *flow.Block, collectionsReader storage.CollectionsReader) (map[flow.Identifier]flow.Identifier, error) {
	txToCollectionID := make(map[flow.Identifier]flow.Identifier)
	for _, guarantee := range block.Payload.Guarantees {
		collectionID := guarantee.ID()
		collection, err := collectionsReader.LightByID(collectionID)
		if err != nil {
			// if the tx result is in storage, the collection must be too.
			return nil, fmt.Errorf("failed to get collection %s in indexed block: %w", collectionID, err)
		}
		for _, txID := range collection.Transactions {
			txToCollectionID[txID] = collectionID
		}
	}
	txToCollectionID[t.systemTxID] = flow.ZeroID

	return txToCollectionID, nil
}

// getSnapshotForBlock retrieves a snapshot for the given block ID and query parameters.
// It uses the executionResultProvider to get an execution result query and then
// uses the executionStateCache to get a snapshot based on the execution result ID.
func (t *LocalTransactionProvider) getSnapshotForBlock(
	blockID flow.Identifier,
	query entities.ExecutionStateQuery,
) (optimistic_sync.Snapshot, *optimistic_sync.ExecutionResultInfo, error) {
	executionResultInfo, err := t.executionResultProvider.ExecutionResult(blockID, optimistic_sync.Criteria{
		AgreeingExecutorsCount: uint(query.AgreeingExecutorsCount),
		RequiredExecutors:      convert.MessagesToIdentifiers(query.RequiredExecutorId),
	})
	if err != nil {
		return nil, nil, err
	}

	snapshot, err := t.executionStateCache.Snapshot(executionResultInfo.ExecutionResult.ID())
	if err != nil {
		return nil, nil, err
	}

	return snapshot, executionResultInfo, nil
}
