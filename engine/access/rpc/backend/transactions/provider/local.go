package provider

import (
	"context"
	"errors"
	"fmt"

	"google.golang.org/grpc/status"

	"github.com/onflow/flow/protobuf/go/flow/entities"
	"google.golang.org/grpc/codes"

	"github.com/onflow/flow-go/engine/access/index"
	"github.com/onflow/flow-go/engine/access/rpc/backend/transactions/error_messages"
	txstatus "github.com/onflow/flow-go/engine/access/rpc/backend/transactions/status"
	"github.com/onflow/flow-go/engine/access/rpc/backend/transactions/system"
	"github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/fvm/blueprints"
	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/state"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// ErrTransactionNotInBlock represents an error indicating that the transaction is not found in the block.
var ErrTransactionNotInBlock = errors.New("transaction not in block")

// LocalTransactionProvider provides functionality for retrieving transaction results and error messages from local storages
type LocalTransactionProvider struct {
	state                        protocol.State
	collections                  storage.Collections
	blocks                       storage.Blocks
	eventsIndex                  *index.EventsIndex
	txResultsIndex               *index.TransactionResultsIndex
	txErrorMessages              error_messages.Provider
	systemCollection             *system.SystemCollection
	txStatusDeriver              *txstatus.TxStatusDeriver
	scheduledTransactionsEnabled bool
	chainID                      flow.ChainID
}

var _ TransactionProvider = (*LocalTransactionProvider)(nil)

func NewLocalTransactionProvider(
	state protocol.State,
	collections storage.Collections,
	blocks storage.Blocks,
	eventsIndex *index.EventsIndex,
	txResultsIndex *index.TransactionResultsIndex,
	txErrorMessages error_messages.Provider,
	systemCollection *system.SystemCollection,
	txStatusDeriver *txstatus.TxStatusDeriver,
	chainID flow.ChainID,
	scheduledTransactionsEnabled bool,
) *LocalTransactionProvider {
	return &LocalTransactionProvider{
		state:                        state,
		collections:                  collections,
		blocks:                       blocks,
		eventsIndex:                  eventsIndex,
		txResultsIndex:               txResultsIndex,
		txErrorMessages:              txErrorMessages,
		systemCollection:             systemCollection,
		txStatusDeriver:              txStatusDeriver,
		scheduledTransactionsEnabled: scheduledTransactionsEnabled,
		chainID:                      chainID,
	}
}

// TransactionResult retrieves a transaction result from storage by block ID and transaction ID.
// Expected errors during normal operation:
//   - codes.NotFound when result cannot be provided by storage due to the absence of data.
//   - codes.Internal if event payload conversion failed.
//   - indexer.ErrIndexNotInitialized when txResultsIndex not initialized
//   - storage.ErrHeightNotIndexed when data is unavailable
//
// All other errors are considered as state corruption (fatal) or internal errors in the transaction error message
// getter or when deriving transaction status.
func (t *LocalTransactionProvider) TransactionResult(
	ctx context.Context,
	block *flow.Header,
	transactionID flow.Identifier,
	encodingVersion entities.EventEncodingVersion,
) (*accessmodel.TransactionResult, error) {
	blockID := block.ID()
	txResult, err := t.txResultsIndex.ByBlockIDTransactionID(blockID, block.Height, transactionID)
	if err != nil {
		return nil, rpc.ConvertIndexError(err, block.Height, "failed to get transaction result")
	}

	var txErrorMessage string
	var txStatusCode uint = 0
	if txResult.Failed {
		txErrorMessage, err = t.txErrorMessages.ErrorMessageByTransactionID(ctx, blockID, block.Height, transactionID)
		if err != nil {
			return nil, err
		}

		if len(txErrorMessage) == 0 {
			return nil, status.Errorf(
				codes.Internal,
				"transaction failed but error message is empty for tx ID: %s block ID: %s",
				txResult.TransactionID,
				blockID,
			)
		}

		txStatusCode = 1 // statusCode of 1 indicates an error and 0 indicates no error, the same as on EN
	}

	txStatus, err := t.txStatusDeriver.DeriveTransactionStatus(block.Height, true)
	if err != nil {
		if !errors.Is(err, state.ErrUnknownSnapshotReference) {
			irrecoverable.Throw(ctx, err)
		}
		return nil, rpc.ConvertStorageError(err)
	}

	events, err := t.eventsIndex.ByBlockIDTransactionID(blockID, block.Height, transactionID)
	if err != nil {
		return nil, rpc.ConvertIndexError(err, block.Height, "failed to get events")
	}

	// events are encoded in CCF format in storage. convert to JSON-CDC if requested
	if encodingVersion == entities.EventEncodingVersion_JSON_CDC_V0 {
		events, err = convert.CcfEventsToJsonEvents(events)
		if err != nil {
			return nil, rpc.ConvertError(err, "failed to convert event payload", codes.Internal)
		}
	}

	return &accessmodel.TransactionResult{
		TransactionID: txResult.TransactionID,
		Status:        txStatus,
		StatusCode:    txStatusCode,
		Events:        events,
		ErrorMessage:  txErrorMessage,
		BlockID:       blockID,
		BlockHeight:   block.Height,
	}, nil
}

// TransactionResultByIndex retrieves a transaction result by index from storage.
// Expected errors during normal operation:
//   - codes.NotFound if result cannot be provided by storage due to the absence of data.
//   - codes.Internal when event payload conversion failed.
//   - indexer.ErrIndexNotInitialized when txResultsIndex not initialized
//   - storage.ErrHeightNotIndexed when data is unavailable
//
// All other errors are considered as state corruption (fatal) or internal errors in the transaction error message
// getter or when deriving transaction status.
func (t *LocalTransactionProvider) TransactionResultByIndex(
	ctx context.Context,
	block *flow.Block,
	index uint32,
	eventEncoding entities.EventEncodingVersion,
) (*accessmodel.TransactionResult, error) {
	blockID := block.ID()
	txResult, err := t.txResultsIndex.ByBlockIDTransactionIndex(blockID, block.Height, index)
	if err != nil {
		return nil, rpc.ConvertIndexError(err, block.Height, "failed to get transaction result")
	}

	var txErrorMessage string
	var txStatusCode uint = 0
	if txResult.Failed {
		txErrorMessage, err = t.txErrorMessages.ErrorMessageByIndex(ctx, blockID, block.Height, index)
		if err != nil {
			return nil, err
		}

		if len(txErrorMessage) == 0 {
			return nil, status.Errorf(codes.Internal, "transaction failed but error message is empty for tx ID: %s block ID: %s", txResult.TransactionID, blockID)
		}

		txStatusCode = 1 // statusCode of 1 indicates an error and 0 indicates no error, the same as on EN
	}

	txStatus, err := t.txStatusDeriver.DeriveTransactionStatus(block.Height, true)
	if err != nil {
		if !errors.Is(err, state.ErrUnknownSnapshotReference) {
			irrecoverable.Throw(ctx, err)
		}
		return nil, rpc.ConvertStorageError(err)
	}

	events, err := t.eventsIndex.ByBlockIDTransactionIndex(blockID, block.Height, index)
	if err != nil {
		return nil, rpc.ConvertIndexError(err, block.Height, "failed to get events")
	}

	// events are encoded in CCF format in storage. convert to JSON-CDC if requested
	if eventEncoding == entities.EventEncodingVersion_JSON_CDC_V0 {
		events, err = convert.CcfEventsToJsonEvents(events)
		if err != nil {
			return nil, rpc.ConvertError(err, "failed to convert event payload", codes.Internal)
		}
	}

	collectionID, err := t.lookupCollectionIDInBlock(block, txResult.TransactionID)
	if err != nil {
		return nil, err
	}

	return &accessmodel.TransactionResult{
		TransactionID: txResult.TransactionID,
		Status:        txStatus,
		StatusCode:    txStatusCode,
		Events:        events,
		ErrorMessage:  txErrorMessage,
		BlockID:       blockID,
		BlockHeight:   block.Height,
		CollectionID:  collectionID,
	}, nil
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

	if !t.scheduledTransactionsEnabled {
		return append(transactions, t.systemCollection.SystemTx()), nil
	}

	events, err := t.eventsIndex.ByBlockID(blockID, block.Height)
	if err != nil {
		return nil, rpc.ConvertIndexError(err, block.Height, "failed to get events")
	}

	sysCollection, err := blueprints.SystemCollection(t.chainID.Chain(), events)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "could not construct system collection: %v", err)
	}

	return append(transactions, sysCollection.Transactions...), nil
}

// TransactionResultsByBlockID retrieves transaction results by block ID from storage
// Expected errors during normal operation:
//   - codes.NotFound if result cannot be provided by storage due to the absence of data.
//   - codes.Internal when event payload conversion failed.
//   - indexer.ErrIndexNotInitialized when txResultsIndex not initialized
//   - storage.ErrHeightNotIndexed when data is unavailable
//
// All other errors are considered as state corruption (fatal) or internal errors in the transaction error message
// getter or when deriving transaction status.
func (t *LocalTransactionProvider) TransactionResultsByBlockID(
	ctx context.Context,
	block *flow.Block,
	requiredEventEncodingVersion entities.EventEncodingVersion,
) ([]*accessmodel.TransactionResult, error) {
	blockID := block.ID()
	txResults, err := t.txResultsIndex.ByBlockID(blockID, block.Height)
	if err != nil {
		return nil, rpc.ConvertIndexError(err, block.Height, "failed to get transaction result")
	}

	txErrors, err := t.txErrorMessages.ErrorMessagesByBlockID(ctx, blockID, block.Height)
	if err != nil {
		return nil, err
	}

	numberOfTxResults := len(txResults)
	results := make([]*accessmodel.TransactionResult, 0, numberOfTxResults)

	// cache the tx to collectionID mapping to avoid repeated lookups
	txToCollectionID, err := t.buildTxIDToCollectionIDMapping(block)
	if err != nil {
		// this indicates that one or more of the collections for the block are not indexed. Since
		// lookups are gated on the indexer signaling it has finished processing all data for the
		// block, all data must be available in storage, otherwise there is an inconsistency in the
		// state.
		irrecoverable.Throw(ctx, fmt.Errorf("inconsistent index state: %w", err))
		return nil, status.Errorf(codes.Internal, "failed to map tx to collection ID: %v", err)
	}

	txStatus, err := t.txStatusDeriver.DeriveTransactionStatus(block.Height, true)
	if err != nil {
		if !errors.Is(err, state.ErrUnknownSnapshotReference) {
			irrecoverable.Throw(ctx, err)
		}
		return nil, rpc.ConvertStorageError(err)
	}

	for _, txResult := range txResults {
		txID := txResult.TransactionID

		var txErrorMessage string
		var txStatusCode uint = 0
		if txResult.Failed {
			txErrorMessage = txErrors[txResult.TransactionID]
			if len(txErrorMessage) == 0 {
				return nil, status.Errorf(codes.Internal, "transaction failed but error message is empty for tx ID: %s block ID: %s", txID, blockID)
			}
			txStatusCode = 1
		}

		events, err := t.eventsIndex.ByBlockIDTransactionID(blockID, block.Height, txResult.TransactionID)
		if err != nil {
			return nil, rpc.ConvertIndexError(err, block.Height, "failed to get events")
		}

		// events are encoded in CCF format in storage. convert to JSON-CDC if requested
		if requiredEventEncodingVersion == entities.EventEncodingVersion_JSON_CDC_V0 {
			events, err = convert.CcfEventsToJsonEvents(events)
			if err != nil {
				return nil, rpc.ConvertError(err, "failed to convert event payload", codes.Internal)
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
			BlockHeight:   block.Height,
		})
	}

	return results, nil
}

// SystemTransaction rebuilds the system transaction from storage
func (t *LocalTransactionProvider) SystemTransaction(
	ctx context.Context,
	block *flow.Block,
	txID flow.Identifier,
) (*flow.TransactionBody, error) {
	if !t.scheduledTransactionsEnabled {
		if txID == t.systemCollection.SystemTxID() {
			return t.systemCollection.SystemTx(), nil
		}
		return nil, status.Errorf(codes.NotFound, "system transaction not found")
	}

	// check if it's one of the static system tx
	if tx, ok := t.systemCollection.ByID(txID); ok {
		return tx, nil
	}

	// check if it's a scheduled system transaction
	events, err := t.eventsIndex.ByBlockID(block.ID(), block.Height)
	if err != nil {
		return nil, rpc.ConvertIndexError(err, block.Height, "failed to get events")
	}

	sysCollection, err := blueprints.SystemCollection(t.chainID.Chain(), events)
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

func (t *LocalTransactionProvider) SystemTransactionResult(
	ctx context.Context,
	block *flow.Block,
	txID flow.Identifier,
	requiredEventEncodingVersion entities.EventEncodingVersion,
) (*accessmodel.TransactionResult, error) {
	// make sure the request is for a system transaction
	// if it's not a static system transaction, use the SystemTransaction method to perform the lookup
	if _, ok := t.systemCollection.ByID(txID); !ok {
		if _, err := t.SystemTransaction(ctx, block, txID); err != nil {
			return nil, err
		}
	}
	return t.TransactionResult(ctx, block.ToHeader(), txID, requiredEventEncodingVersion)
}

// lookupCollectionIDInBlock returns the collection ID based on the transaction ID.
// The lookup is performed in block collections.
func (t *LocalTransactionProvider) lookupCollectionIDInBlock(
	block *flow.Block,
	txID flow.Identifier,
) (flow.Identifier, error) {
	for _, guarantee := range block.Payload.Guarantees {
		collection, err := t.collections.LightByID(guarantee.CollectionID)
		if err != nil {
			return flow.ZeroID, fmt.Errorf("failed to get collection %s in indexed block: %w", guarantee.CollectionID, err)
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
