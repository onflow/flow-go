package backend

import (
	"context"
	"errors"
	"fmt"

	"google.golang.org/grpc/status"

	"github.com/onflow/flow/protobuf/go/flow/entities"
	"google.golang.org/grpc/codes"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/index"
	"github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/state"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// TransactionErrorMessage declares the lookup transaction error methods by different input parameters.
type TransactionErrorMessage interface {
	// LookupErrorMessageByTransactionID is a function type for getting transaction error message by block ID and transaction ID.
	// Expected errors during normal operation:
	//   - InsufficientExecutionReceipts - found insufficient receipts for given block ID.
	//   - status.Error - remote GRPC call to EN has failed.
	LookupErrorMessageByTransactionID(ctx context.Context, blockID flow.Identifier, transactionID flow.Identifier) (string, error)

	// LookupErrorMessageByIndex is a function type for getting transaction error message by index.
	// Expected errors during normal operation:
	//   - status.Error[codes.NotFound] - transaction result for given block ID and tx index is not available.
	//   - InsufficientExecutionReceipts - found insufficient receipts for given block ID.
	//   - status.Error - remote GRPC call to EN has failed.
	LookupErrorMessageByIndex(ctx context.Context, blockID flow.Identifier, height uint64, index uint32) (string, error)

	// LookupErrorMessagesByBlockID is a function type for getting transaction error messages by block ID.
	// Expected errors during normal operation:
	//   - status.Error[codes.NotFound] - transaction results for given block ID are not available.
	//   - InsufficientExecutionReceipts - found insufficient receipts for given block ID.
	//   - status.Error - remote GRPC call to EN has failed.
	LookupErrorMessagesByBlockID(ctx context.Context, blockID flow.Identifier, height uint64) (map[flow.Identifier]string, error)
}

// TransactionsLocalDataProvider provides functionality for retrieving transaction results and error messages from local storages
type TransactionsLocalDataProvider struct {
	state           protocol.State
	collections     storage.Collections
	blocks          storage.Blocks
	eventsIndex     *index.EventsIndex
	txResultsIndex  *index.TransactionResultsIndex
	txErrorMessages TransactionErrorMessage
	systemTxID      flow.Identifier
}

// GetTransactionResultFromStorage retrieves a transaction result from storage by block ID and transaction ID.
// Expected errors during normal operation:
//   - codes.NotFound when result cannot be provided by storage due to the absence of data.
//   - codes.Internal if event payload conversion failed.
//   - indexer.ErrIndexNotInitialized when txResultsIndex not initialized
//   - storage.ErrHeightNotIndexed when data is unavailable
//
// All other errors are considered as state corruption (fatal) or internal errors in the transaction error message
// getter or when deriving transaction status.
func (t *TransactionsLocalDataProvider) GetTransactionResultFromStorage(
	ctx context.Context,
	block *flow.Block,
	transactionID flow.Identifier,
	requiredEventEncodingVersion entities.EventEncodingVersion,
) (*access.TransactionResult, error) {
	blockID := block.ID()
	txResult, err := t.txResultsIndex.ByBlockIDTransactionID(blockID, block.Header.Height, transactionID)
	if err != nil {
		return nil, rpc.ConvertIndexError(err, block.Header.Height, "failed to get transaction result")
	}

	var txErrorMessage string
	var txStatusCode uint = 0
	if txResult.Failed {
		txErrorMessage, err = t.txErrorMessages.LookupErrorMessageByTransactionID(ctx, blockID, transactionID)
		if err != nil {
			return nil, err
		}

		if len(txErrorMessage) == 0 {
			return nil, status.Errorf(codes.Internal, "transaction failed but error message is empty for tx ID: %s block ID: %s", txResult.TransactionID, blockID)
		}

		txStatusCode = 1 // statusCode of 1 indicates an error and 0 indicates no error, the same as on EN
	}

	txStatus, err := t.DeriveTransactionStatus(blockID, block.Header.Height, true)
	if err != nil {
		if !errors.Is(err, state.ErrUnknownSnapshotReference) {
			irrecoverable.Throw(ctx, err)
		}
		return nil, rpc.ConvertStorageError(err)
	}

	events, err := t.eventsIndex.ByBlockIDTransactionID(blockID, block.Header.Height, transactionID)
	if err != nil {
		return nil, rpc.ConvertIndexError(err, block.Header.Height, "failed to get events")
	}

	// events are encoded in CCF format in storage. convert to JSON-CDC if requested
	if requiredEventEncodingVersion == entities.EventEncodingVersion_JSON_CDC_V0 {
		for _, e := range events {
			payload, err := convert.CcfPayloadToJsonPayload(e.Payload)
			if err != nil {
				err = fmt.Errorf("failed to convert event payload for block %s: %w", blockID, err)
				return nil, rpc.ConvertError(err, "failed to convert event payload", codes.Internal)
			}
			e.Payload = payload
		}
	}

	return &access.TransactionResult{
		TransactionID: txResult.TransactionID,
		Status:        txStatus,
		StatusCode:    txStatusCode,
		Events:        events,
		ErrorMessage:  txErrorMessage,
		BlockID:       blockID,
		BlockHeight:   block.Header.Height,
	}, nil
}

// GetTransactionResultsByBlockIDFromStorage retrieves transaction results by block ID from storage
// Expected errors during normal operation:
//   - codes.NotFound if result cannot be provided by storage due to the absence of data.
//   - codes.Internal when event payload conversion failed.
//   - indexer.ErrIndexNotInitialized when txResultsIndex not initialized
//   - storage.ErrHeightNotIndexed when data is unavailable
//
// All other errors are considered as state corruption (fatal) or internal errors in the transaction error message
// getter or when deriving transaction status.
func (t *TransactionsLocalDataProvider) GetTransactionResultsByBlockIDFromStorage(
	ctx context.Context,
	block *flow.Block,
	requiredEventEncodingVersion entities.EventEncodingVersion,
) ([]*access.TransactionResult, error) {
	blockID := block.ID()
	txResults, err := t.txResultsIndex.ByBlockID(blockID, block.Header.Height)
	if err != nil {
		return nil, rpc.ConvertIndexError(err, block.Header.Height, "failed to get transaction result")
	}

	txErrors, err := t.txErrorMessages.LookupErrorMessagesByBlockID(ctx, blockID, block.Header.Height)
	if err != nil {
		return nil, err
	}

	numberOfTxResults := len(txResults)
	results := make([]*access.TransactionResult, 0, numberOfTxResults)

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

		txStatus, err := t.DeriveTransactionStatus(blockID, block.Header.Height, true)
		if err != nil {
			if !errors.Is(err, state.ErrUnknownSnapshotReference) {
				irrecoverable.Throw(ctx, err)
			}
			return nil, rpc.ConvertStorageError(err)
		}

		events, err := t.eventsIndex.ByBlockIDTransactionID(blockID, block.Header.Height, txResult.TransactionID)
		if err != nil {
			return nil, rpc.ConvertIndexError(err, block.Header.Height, "failed to get events")
		}

		// events are encoded in CCF format in storage. convert to JSON-CDC if requested
		if requiredEventEncodingVersion == entities.EventEncodingVersion_JSON_CDC_V0 {
			for _, e := range events {
				payload, err := convert.CcfPayloadToJsonPayload(e.Payload)
				if err != nil {
					err = fmt.Errorf("failed to convert event payload for block %s: %w", blockID, err)
					return nil, rpc.ConvertError(err, "failed to convert event payload", codes.Internal)
				}
				e.Payload = payload
			}
		}

		collectionID, ok := txToCollectionID[txID]
		if !ok {
			return nil, status.Errorf(codes.Internal, "transaction %s not found in block %s", txID, blockID)
		}

		results = append(results, &access.TransactionResult{
			Status:        txStatus,
			StatusCode:    txStatusCode,
			Events:        events,
			ErrorMessage:  txErrorMessage,
			BlockID:       blockID,
			TransactionID: txID,
			CollectionID:  collectionID,
			BlockHeight:   block.Header.Height,
		})
	}

	return results, nil
}

// GetTransactionResultByIndexFromStorage retrieves a transaction result by index from storage.
// Expected errors during normal operation:
//   - codes.NotFound if result cannot be provided by storage due to the absence of data.
//   - codes.Internal when event payload conversion failed.
//   - indexer.ErrIndexNotInitialized when txResultsIndex not initialized
//   - storage.ErrHeightNotIndexed when data is unavailable
//
// All other errors are considered as state corruption (fatal) or internal errors in the transaction error message
// getter or when deriving transaction status.
func (t *TransactionsLocalDataProvider) GetTransactionResultByIndexFromStorage(
	ctx context.Context,
	block *flow.Block,
	index uint32,
	requiredEventEncodingVersion entities.EventEncodingVersion,
) (*access.TransactionResult, error) {
	blockID := block.ID()
	txResult, err := t.txResultsIndex.ByBlockIDTransactionIndex(blockID, block.Header.Height, index)
	if err != nil {
		return nil, rpc.ConvertIndexError(err, block.Header.Height, "failed to get transaction result")
	}

	var txErrorMessage string
	var txStatusCode uint = 0
	if txResult.Failed {
		txErrorMessage, err = t.txErrorMessages.LookupErrorMessageByIndex(ctx, blockID, block.Header.Height, index)
		if err != nil {
			return nil, err
		}

		if len(txErrorMessage) == 0 {
			return nil, status.Errorf(codes.Internal, "transaction failed but error message is empty for tx ID: %s block ID: %s", txResult.TransactionID, blockID)
		}

		txStatusCode = 1 // statusCode of 1 indicates an error and 0 indicates no error, the same as on EN
	}

	txStatus, err := t.DeriveTransactionStatus(blockID, block.Header.Height, true)
	if err != nil {
		if !errors.Is(err, state.ErrUnknownSnapshotReference) {
			irrecoverable.Throw(ctx, err)
		}
		return nil, rpc.ConvertStorageError(err)
	}

	events, err := t.eventsIndex.ByBlockIDTransactionIndex(blockID, block.Header.Height, index)
	if err != nil {
		return nil, rpc.ConvertIndexError(err, block.Header.Height, "failed to get events")
	}

	// events are encoded in CCF format in storage. convert to JSON-CDC if requested
	if requiredEventEncodingVersion == entities.EventEncodingVersion_JSON_CDC_V0 {
		for _, e := range events {
			payload, err := convert.CcfPayloadToJsonPayload(e.Payload)
			if err != nil {
				err = fmt.Errorf("failed to convert event payload for block %s: %w", blockID, err)
				return nil, rpc.ConvertError(err, "failed to convert event payload", codes.Internal)
			}
			e.Payload = payload
		}
	}

	collectionID, err := t.LookupCollectionIDInBlock(block, txResult.TransactionID)
	if err != nil {
		return nil, err
	}

	return &access.TransactionResult{
		TransactionID: txResult.TransactionID,
		Status:        txStatus,
		StatusCode:    txStatusCode,
		Events:        events,
		ErrorMessage:  txErrorMessage,
		BlockID:       blockID,
		BlockHeight:   block.Header.Height,
		CollectionID:  collectionID,
	}, nil
}

// DeriveUnknownTransactionStatus is used to determine the status of transaction
// that are not in a block yet based on the provided reference block ID.
func (t *TransactionsLocalDataProvider) DeriveUnknownTransactionStatus(refBlockID flow.Identifier) (flow.TransactionStatus, error) {
	referenceBlock, err := t.state.AtBlockID(refBlockID).Head()
	if err != nil {
		return flow.TransactionStatusUnknown, err
	}
	refHeight := referenceBlock.Height
	// get the latest finalized block from the state
	finalized, err := t.state.Final().Head()
	if err != nil {
		return flow.TransactionStatusUnknown, irrecoverable.NewExceptionf("failed to lookup final header: %w", err)
	}
	finalizedHeight := finalized.Height

	// if we haven't seen the expiry block for this transaction, it's not expired
	if !isExpired(refHeight, finalizedHeight) {
		return flow.TransactionStatusPending, nil
	}

	// At this point, we have seen the expiry block for the transaction.
	// This means that, if no collections  prior to the expiry block contain
	// the transaction, it can never be included and is expired.
	//
	// To ensure this, we need to have received all collections  up to the
	// expiry block to ensure the transaction did not appear in any.

	// the last full height is the height where we have received all
	// collections  for all blocks with a lower height
	fullHeight, err := t.blocks.GetLastFullBlockHeight()
	if err != nil {
		return flow.TransactionStatusUnknown, err
	}

	// if we have received collections  for all blocks up to the expiry block, the transaction is expired
	if isExpired(refHeight, fullHeight) {
		return flow.TransactionStatusExpired, nil
	}

	// tx found in transaction storage and collection storage but not in block storage
	// However, this will not happen as of now since the ingestion engine doesn't subscribe
	// for collections
	return flow.TransactionStatusPending, nil
}

// DeriveTransactionStatus is used to determine the status of a transaction based on the provided block ID, block height, and execution status.
// No errors expected during normal operations.
func (t *TransactionsLocalDataProvider) DeriveTransactionStatus(blockID flow.Identifier, blockHeight uint64, executed bool) (flow.TransactionStatus, error) {
	if !executed {
		// If we've gotten here, but the block has not yet been executed, report it as only been finalized
		return flow.TransactionStatusFinalized, nil
	}

	// From this point on, we know for sure this transaction has at least been executed

	// get the latest sealed block from the State
	sealed, err := t.state.Sealed().Head()
	if err != nil {
		return flow.TransactionStatusUnknown, irrecoverable.NewExceptionf("failed to lookup sealed header: %w", err)
	}

	if blockHeight > sealed.Height {
		// The block is not yet sealed, so we'll report it as only executed
		return flow.TransactionStatusExecuted, nil
	}

	// otherwise, this block has been executed, and sealed, so report as sealed
	return flow.TransactionStatusSealed, nil
}

// isExpired checks whether a transaction is expired given the height of the
// transaction's reference block and the height to compare against.
func isExpired(refHeight, compareToHeight uint64) bool {
	if compareToHeight <= refHeight {
		return false
	}
	return compareToHeight-refHeight > flow.DefaultTransactionExpiry
}

// LookupCollectionIDInBlock returns the collection ID based on the transaction ID. The lookup is performed in block
// collections.
func (t *TransactionsLocalDataProvider) LookupCollectionIDInBlock(
	block *flow.Block,
	txID flow.Identifier,
) (flow.Identifier, error) {
	for _, guarantee := range block.Payload.Guarantees {
		collection, err := t.collections.LightByID(guarantee.ID())
		if err != nil {
			return flow.ZeroID, err
		}

		for _, collectionTxID := range collection.Transactions {
			if collectionTxID == txID {
				return guarantee.ID(), nil
			}
		}
	}
	return flow.ZeroID, status.Error(codes.NotFound, "transaction not found in block")
}

// buildTxIDToCollectionIDMapping returns a map of transaction ID to collection ID based on the provided block.
// No errors expected during normal operations.
func (t *TransactionsLocalDataProvider) buildTxIDToCollectionIDMapping(block *flow.Block) (map[flow.Identifier]flow.Identifier, error) {
	txToCollectionID := make(map[flow.Identifier]flow.Identifier)
	for _, guarantee := range block.Payload.Guarantees {
		collection, err := t.collections.LightByID(guarantee.ID())
		if err != nil {
			// if the tx result is in storage, the collection must be too.
			return nil, fmt.Errorf("failed to get collection %s in indexed block: %w", guarantee.ID(), err)
		}
		for _, txID := range collection.Transactions {
			txToCollectionID[txID] = guarantee.ID()
		}
	}

	txToCollectionID[t.systemTxID] = flow.ZeroID

	return txToCollectionID, nil
}
