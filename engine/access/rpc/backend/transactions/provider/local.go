package provider

import (
	"context"
	"errors"
	"fmt"

	"google.golang.org/grpc/status"

	"github.com/onflow/flow/protobuf/go/flow/entities"
	"google.golang.org/grpc/codes"

	"github.com/onflow/flow-go/engine/access/rpc/backend/transactions/error_messages"
	txstatus "github.com/onflow/flow-go/engine/access/rpc/backend/transactions/status"
	"github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/access/systemcollection"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// ErrTransactionNotInBlock represents an error indicating that the transaction is not found in the block.
var ErrTransactionNotInBlock = errors.New("transaction not in block")

// LocalTransactionProvider provides functionality for retrieving transaction results and error messages from local storages
type LocalTransactionProvider struct {
	state                  protocol.State
	collections            storage.Collections
	blocks                 storage.Blocks
	txErrorMessages        error_messages.Provider
	systemCollections      *systemcollection.Versioned
	txStatusDeriver        *txstatus.TxStatusDeriver
	chainID                flow.ChainID
	execResultInfoProvider optimistic_sync.ExecutionResultInfoProvider
	execStateCache         optimistic_sync.ExecutionStateCache
}

var _ TransactionProvider = (*LocalTransactionProvider)(nil)

func NewLocalTransactionProvider(
	state protocol.State,
	collections storage.Collections,
	blocks storage.Blocks,
	txErrorMessages error_messages.Provider,
	systemCollections *systemcollection.Versioned,
	txStatusDeriver *txstatus.TxStatusDeriver,
	chainID flow.ChainID,
	execResultInfoProvider optimistic_sync.ExecutionResultInfoProvider,
	execStateCache optimistic_sync.ExecutionStateCache,
) *LocalTransactionProvider {
	return &LocalTransactionProvider{
		state:                  state,
		collections:            collections,
		blocks:                 blocks,
		txErrorMessages:        txErrorMessages,
		systemCollections:      systemCollections,
		txStatusDeriver:        txStatusDeriver,
		chainID:                chainID,
		execResultInfoProvider: execResultInfoProvider,
		execStateCache:         execStateCache,
	}
}

// TransactionResult retrieves a transaction result from storage by block ID and transaction ID.
// Expected errors during normal operation:
//   - codes.NotFound when result cannot be provided by storage due to the absence of data.
//   - codes.Internal if event payload conversion failed.
//   - All other errors are potential indicators of bugs or corrupted internal state (continuation impossible)
func (t *LocalTransactionProvider) TransactionResult(
	ctx context.Context,
	header *flow.Header,
	transactionID flow.Identifier,
	collectionID flow.Identifier,
	encodingVersion entities.EventEncodingVersion,
	criteria optimistic_sync.Criteria,
) (*accessmodel.TransactionResult, error) {
	blockID := header.ID()
	execResultInfo, err := t.execResultInfoProvider.ExecutionResultInfo(blockID, criteria)
	if err != nil {
		// Execution result info is not available
		// TODO: check error conversion
		return nil, err
	}

	snapshot, err := t.execStateCache.Snapshot(execResultInfo.ExecutionResultID)
	if err != nil {
		// Snapshot is not available
		return nil, err
	}

	blockHeight := header.Height

	// Get snapshot readers
	txResultsReader := snapshot.LightTransactionResults()
	errorMessagesReader := snapshot.TransactionResultErrorMessages()
	eventsReader := snapshot.Events()

	// Get light transaction result
	lightTxResult, err := txResultsReader.ByBlockIDTransactionID(blockID, transactionID)
	if err != nil {
		return nil, err
	}

	// Get error message if transaction failed
	var txErrorMessage string
	var txStatusCode uint = 0
	if lightTxResult.Failed {
		errorMsg, err := errorMessagesReader.ByBlockIDTransactionID(blockID, transactionID)

		if err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				return nil, status.Errorf(
					codes.Internal,
					"transaction failed but error message not found for tx ID: %s block ID: %s",
					transactionID,
					blockID,
				)
			}
			return nil, err
		}

		txErrorMessage = errorMsg.ErrorMessage
		if len(txErrorMessage) == 0 {
			return nil, status.Errorf(
				codes.Internal,
				"transaction failed but error message is empty for tx ID: %s block ID: %s",
				transactionID,
				blockID,
			)
		}

		txStatusCode = 1 // statusCode of 1 indicates an error and 0 indicates no error, the same as on EN
	}

	// Derive transaction status
	txStatus, err := t.txStatusDeriver.DeriveFinalizedTransactionStatus(blockHeight, true)
	if err != nil {
		irrecoverable.Throw(ctx, fmt.Errorf("failed to derive transaction status: %w", err))
		return nil, err
	}

	// Get events
	events, err := eventsReader.ByBlockIDTransactionID(blockID, transactionID)
	if err != nil {
		// TODO: check err conversion
		return nil, err
	}

	// Events are encoded in CCF format in storage. Convert to JSON-CDC if requested
	if encodingVersion == entities.EventEncodingVersion_JSON_CDC_V0 {
		events, err = convert.CcfEventsToJsonEvents(events)
		if err != nil {
			// TODO: check err conversion
			return nil, status.Errorf(codes.Internal, "failed to convert event payload: %v", err)
		}
	}

	// Build complete transaction result
	return &accessmodel.TransactionResult{
		TransactionID: lightTxResult.TransactionID,
		Status:        txStatus,
		StatusCode:    txStatusCode,
		Events:        events,
		ErrorMessage:  txErrorMessage,
		BlockID:       blockID,
		BlockHeight:   blockHeight,
	}, nil
}

// TransactionResultByIndex retrieves a transaction result by index from storage.
// Expected errors during normal operation:
//   - codes.NotFound if result cannot be provided by storage due to the absence of data.
//   - codes.Internal when event payload conversion failed.
//   - storage.ErrHeightNotIndexed when data is unavailable.
//   - All other errors are potential indicators of bugs or corrupted internal state (continuation impossible)
func (t *LocalTransactionProvider) TransactionResultByIndex(
	ctx context.Context,
	block *flow.Block,
	index uint32,
	collectionID flow.Identifier,
	eventEncoding entities.EventEncodingVersion,
) (*accessmodel.TransactionResult, error) {
	blockID := block.ID()

	execResultInfo, err := t.execResultInfoProvider.ExecutionResultInfo(blockID, optimistic_sync.Criteria{})
	if err != nil {
		// Execution result info is not available
		return nil, err
	}

	snapshot, err := t.execStateCache.Snapshot(execResultInfo.ExecutionResultID)
	if err != nil {
		// Snapshot is not available
		return nil, err
	}

	blockHeight := block.Height

	// Get snapshot readers
	txResultsReader := snapshot.LightTransactionResults()
	errorMessagesReader := snapshot.TransactionResultErrorMessages()
	eventsReader := snapshot.Events()

	// Get light transaction result by index
	lightTxResult, err := txResultsReader.ByBlockIDTransactionIndex(blockID, index)
	if err != nil {
		return nil, err
	}

	// Get error message if transaction failed
	var txErrorMessage string
	var txStatusCode uint = 0
	if lightTxResult.Failed {
		errorMsg, err := errorMessagesReader.ByBlockIDTransactionIndex(blockID, index)
		if err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				return nil, status.Errorf(
					codes.Internal,
					"transaction failed but error message not found for tx ID: %s block ID: %s",
					lightTxResult.TransactionID,
					blockID,
				)
			}
			return nil, err
		}

		txErrorMessage = errorMsg.ErrorMessage
		if len(txErrorMessage) == 0 {
			return nil, status.Errorf(
				codes.Internal,
				"transaction failed but error message is empty for tx ID: %s block ID: %s",
				lightTxResult.TransactionID,
				blockID,
			)
		}

		txStatusCode = 1 // statusCode of 1 indicates an error and 0 indicates no error, the same as on EN
	}

	// Derive transaction status
	txStatus, err := t.txStatusDeriver.DeriveFinalizedTransactionStatus(blockHeight, true)
	if err != nil {
		irrecoverable.Throw(ctx, fmt.Errorf("failed to derive transaction status: %w", err))
		return nil, err
	}

	// Get events by index
	events, err := eventsReader.ByBlockIDTransactionIndex(blockID, index)
	if err != nil {
		return nil, rpc.ConvertIndexError(err, blockHeight, "failed to get events")
	}

	// Events are encoded in CCF format in storage. Convert to JSON-CDC if requested
	if eventEncoding == entities.EventEncodingVersion_JSON_CDC_V0 {
		events, err = convert.CcfEventsToJsonEvents(events)
		if err != nil {
			return nil, rpc.ConvertError(err, "failed to convert event payload", codes.Internal)
		}
	}

	return &accessmodel.TransactionResult{
		TransactionID: lightTxResult.TransactionID,
		Status:        txStatus,
		StatusCode:    txStatusCode,
		Events:        events,
		ErrorMessage:  txErrorMessage,
		BlockID:       blockID,
		BlockHeight:   blockHeight,
		CollectionID:  collectionID,
	}, nil
}

// TransactionsByBlockID retrieves transactions by block ID from storage
// Expected errors during normal operation:
//   - codes.NotFound if result cannot be provided by storage due to the absence of data.
//   - codes.Internal when event payload conversion failed.
//   - storage.ErrHeightNotIndexed when data is unavailable
//
// All other errors are considered as state corruption (fatal) or internal errors in the transaction error message
// getter or when deriving transaction status.
func (t *LocalTransactionProvider) TransactionsByBlockID(
	ctx context.Context,
	block *flow.Block,
) ([]*flow.TransactionBody, error) {
	var transactions []*flow.TransactionBody
	blockID := block.ID()

	for _, guarantee := range block.Payload.Guarantees {
		collection, err := t.collections.ByID(guarantee.CollectionID)
		if err != nil {
			return nil, rpc.ConvertStorageError(err)
		}

		transactions = append(transactions, collection.Transactions...)
	}

	execResultInfo, err := t.execResultInfoProvider.ExecutionResultInfo(blockID, optimistic_sync.Criteria{})
	if err != nil {
		// Execution result info is not available
		return nil, err
	}

	snapshot, err := t.execStateCache.Snapshot(execResultInfo.ExecutionResultID)
	if err != nil {
		// Snapshot is not available
		return nil, err
	}

	// Get events reader
	eventsReader := snapshot.Events()

	// generate the system collection which includes scheduled transactions
	eventProvider := func() (flow.EventsList, error) {
		return eventsReader.ByBlockID(blockID)
	}

	sysCollection, err := t.systemCollections.
		ByHeight(block.Height).
		SystemCollection(t.chainID.Chain(), eventProvider)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "could not construct system collection: %v", err)
	}

	return append(transactions, sysCollection.Transactions...), nil
}

// TransactionResultsByBlockID retrieves transaction results by block ID from storage
// Expected errors during normal operation:
//   - codes.NotFound if result cannot be provided by storage due to the absence of data.
//   - codes.Internal when event payload conversion failed.
//   - storage.ErrHeightNotIndexed when data is unavailable
//   - All other errors are potential indicators of bugs or corrupted internal state (continuation impossible)
func (t *LocalTransactionProvider) TransactionResultsByBlockID(
	ctx context.Context,
	block *flow.Block,
	requiredEventEncodingVersion entities.EventEncodingVersion,
) ([]*accessmodel.TransactionResult, error) {
	blockID := block.ID()

	execResultInfo, err := t.execResultInfoProvider.ExecutionResultInfo(blockID, optimistic_sync.Criteria{})
	if err != nil {
		// Execution result info is not available
		return nil, err
	}

	snapshot, err := t.execStateCache.Snapshot(execResultInfo.ExecutionResultID)
	if err != nil {
		// Snapshot is not available
		return nil, err
	}

	blockHeight := block.Height

	// Get snapshot readers
	txResultsReader := snapshot.LightTransactionResults()
	errorMessagesReader := snapshot.TransactionResultErrorMessages()
	eventsReader := snapshot.Events()

	// Get all light transaction results for the block
	lightTxResults, err := txResultsReader.ByBlockID(blockID)
	if err != nil {
		return nil, err
	}

	// Get all error messages for the block
	txErrorMessages, err := errorMessagesReader.ByBlockID(blockID)
	if err != nil {
		return nil, err
	}

	// Build map of transaction ID to error message
	txErrors := make(map[flow.Identifier]string)
	for _, errorMsg := range txErrorMessages {
		txErrors[errorMsg.TransactionID] = errorMsg.ErrorMessage
	}

	numberOfTxResults := len(lightTxResults)
	results := make([]*accessmodel.TransactionResult, 0, numberOfTxResults)

	// Cache the tx to collectionID mapping to avoid repeated lookups
	txToCollectionID, err := t.buildTxIDToCollectionIDMapping(block)
	if err != nil {
		// this indicates that one or more of the collections for the block are not indexed. Since
		// lookups are gated on the indexer signaling it has finished processing all data for the
		// block, all data must be available in storage, otherwise there is an inconsistency in the
		// state.
		irrecoverable.Throw(ctx, fmt.Errorf("inconsistent index state: %w", err))
		return nil, status.Errorf(codes.Internal, "failed to map tx to collection ID: %v", err)
	}

	txStatus, err := t.txStatusDeriver.DeriveFinalizedTransactionStatus(blockHeight, true)
	if err != nil {
		irrecoverable.Throw(ctx, fmt.Errorf("failed to derive transaction status: %w", err))
		return nil, err
	}

	for _, lightTxResult := range lightTxResults {
		txID := lightTxResult.TransactionID

		var txErrorMessage string
		var txStatusCode uint = 0
		if lightTxResult.Failed {
			txErrorMessage = txErrors[lightTxResult.TransactionID]
			if len(txErrorMessage) == 0 {
				return nil, status.Errorf(codes.Internal, "transaction failed but error message is empty for tx ID: %s block ID: %s", txID, blockID)
			}
			txStatusCode = 1
		}

		events, err := eventsReader.ByBlockIDTransactionID(blockID, lightTxResult.TransactionID)
		if err != nil {
			return nil, err
		}

		// Events are encoded in CCF format in storage. Convert to JSON-CDC if requested
		if requiredEventEncodingVersion == entities.EventEncodingVersion_JSON_CDC_V0 {
			events, err = convert.CcfEventsToJsonEvents(events)
			if err != nil {
				return nil, status.Errorf(codes.Internal, "failed to convert event payload: %v", err)
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
			ErrorMessage:  txErrorMessage,
			BlockID:       blockID,
			TransactionID: txID,
			CollectionID:  collectionID,
			BlockHeight:   blockHeight,
		})
	}

	return results, nil
}

// ScheduledTransactionsByBlockID constructs the scheduled transaction bodies using events from the
// local storage.
//
// Expected error returns during normal operation:
//   - [codes.NotFound]: if the events are not found for the block ID.
//   - [codes.OutOfRange]: if the events are not available for the block height.
//   - [codes.FailedPrecondition]: if the events index is not initialized.
//   - [codes.Internal]: if the scheduled transactions cannot be constructed.
func (t *LocalTransactionProvider) ScheduledTransactionsByBlockID(
	ctx context.Context,
	header *flow.Header,
) ([]*flow.TransactionBody, error) {

	execResultInfo, err := t.execResultInfoProvider.ExecutionResultInfo(header.ID(), optimistic_sync.Criteria{})
	if err != nil {
		// Execution result info is not available
		return nil, err
	}

	snapshot, err := t.execStateCache.Snapshot(execResultInfo.ExecutionResultID)
	if err != nil {
		// Snapshot is not available
		return nil, err
	}

	// Get events reader
	eventsReader := snapshot.Events()

	// generate the system collection which includes scheduled transactions
	events, err := eventsReader.ByBlockID(header.ID())
	if err != nil {
		return nil, rpc.ConvertIndexError(err, header.Height, "failed to get events to reconstruct scheduled transactions")
	}

	txs, err := t.systemCollections.
		ByHeight(header.Height).
		ExecuteCallbacksTransactions(t.chainID.Chain(), events)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "could not construct scheduled transactions: %v", err)
	}

	return txs, nil
}

// buildTxIDToCollectionIDMapping returns a map of transaction ID to collection ID based on the provided block.
// No errors expected during normal operations.
func (t *LocalTransactionProvider) buildTxIDToCollectionIDMapping(block *flow.Block) (map[flow.Identifier]flow.Identifier, error) {
	txToCollectionID := make(map[flow.Identifier]flow.Identifier)
	for _, guarantee := range block.Payload.Guarantees {
		collection, err := t.collections.LightByID(guarantee.CollectionID)
		if err != nil {
			// if the tx result is in storage, the collection must be too.
			return nil, fmt.Errorf("failed to get collection %s in indexed block: %w", guarantee.CollectionID, err)
		}
		for _, txID := range collection.Transactions {
			txToCollectionID[txID] = guarantee.CollectionID
		}
	}

	return txToCollectionID, nil
}
