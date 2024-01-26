package backend

import (
	"context"
	"errors"
	"fmt"

	"google.golang.org/grpc/status"

	"github.com/onflow/flow/protobuf/go/flow/entities"
	"google.golang.org/grpc/codes"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/state"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

type TransactionErrorMessageGetter func(ctx context.Context, blockID flow.Identifier, transactionID flow.Identifier) (string, error)
type TransactionErrorMessageByIndexGetter func(ctx context.Context, blockID flow.Identifier, index uint32) (string, error)
type TransactionErrorMessagesByBlockIDGetter func(ctx context.Context, blockID flow.Identifier) (map[flow.Identifier]string, error)

type TransactionsLocalDataProvider struct {
	state       protocol.State
	results     storage.LightTransactionResults
	events      storage.Events
	collections storage.Collections
	blocks      storage.Blocks
}

func (t *TransactionsLocalDataProvider) GetTransactionResultFromStorage(
	ctx context.Context,
	block *flow.Block,
	transactionID flow.Identifier,
	requiredEventEncodingVersion entities.EventEncodingVersion,
	errorMessageGetter TransactionErrorMessageGetter,
) (*access.TransactionResult, error) {
	blockID := block.ID()
	txResult, err := t.results.ByBlockIDTransactionID(blockID, transactionID)
	if err != nil {
		return nil, rpc.ConvertStorageError(err)
	}

	var txErrorMessage string
	var txStatusCode uint = 0
	if txResult.Failed {
		if errorMessageGetter != nil {
			txErrorMessage, err = errorMessageGetter(ctx, blockID, transactionID)
			if err != nil {
				return nil, err
			}
		}
		txStatusCode = 1 // statusCode of 1 indicates an error and 0 indicates no error, the same as on EN
	}

	// tx body is irrelevant to status if it's in an executed block
	txStatus, err := t.deriveTransactionStatus(nil, true, block)
	if err != nil {
		if !errors.Is(err, state.ErrUnknownSnapshotReference) {
			irrecoverable.Throw(ctx, err)
		}
		return nil, rpc.ConvertStorageError(err)
	}

	events, err := t.events.ByBlockIDTransactionID(blockID, transactionID)
	if err != nil {
		return nil, rpc.ConvertStorageError(err)
	}

	for _, e := range events {
		// events are encoded in CCF format in storage. convert to JSON-CDC if requested
		if requiredEventEncodingVersion == entities.EventEncodingVersion_JSON_CDC_V0 {
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

func (t *TransactionsLocalDataProvider) GetTransactionResultsByBlockIDFromStorage(
	ctx context.Context,
	block *flow.Block,
	requiredEventEncodingVersion entities.EventEncodingVersion,
	errorMessageByBlockIDGetter TransactionErrorMessagesByBlockIDGetter,
) ([]*access.TransactionResult, error) {
	blockID := block.ID()
	txResults, err := t.results.ByBlockID(blockID)
	if err != nil {
		return nil, rpc.ConvertStorageError(err)
	}

	txErrors := make(map[flow.Identifier]string)
	if errorMessageByBlockIDGetter != nil {
		txErrors, err = errorMessageByBlockIDGetter(ctx, blockID)
		if err != nil {
			return nil, err
		}
	}
	numberOfTxResults := len(txResults)
	results := make([]*access.TransactionResult, 0, numberOfTxResults)

	for _, txResult := range txResults {
		txID := txResult.TransactionID

		var txErrorMessage string
		var txStatusCode uint = 0
		if txResult.Failed {
			txErrorMessage = txErrors[txResult.TransactionID]
			txStatusCode = 1
		}

		// tx body is irrelevant to status if it's in an executed block
		txStatus, err := t.deriveTransactionStatus(nil, true, block)
		if err != nil {
			if !errors.Is(err, state.ErrUnknownSnapshotReference) {
				irrecoverable.Throw(ctx, err)
			}
			return nil, rpc.ConvertStorageError(err)
		}

		events, err := t.events.ByBlockIDTransactionID(blockID, txResult.TransactionID)
		if err != nil {
			return nil, rpc.ConvertStorageError(err)
		}

		for _, e := range events {
			// events are encoded in CCF format in storage. convert to JSON-CDC if requested
			if requiredEventEncodingVersion == entities.EventEncodingVersion_JSON_CDC_V0 {
				payload, err := convert.CcfPayloadToJsonPayload(e.Payload)
				if err != nil {
					err = fmt.Errorf("failed to convert event payload for block %s: %w", blockID, err)
					return nil, rpc.ConvertError(err, "failed to convert event payload", codes.Internal)
				}
				e.Payload = payload
			}
		}

		collectionID, err := t.lookupCollectionIDInBlock(block, txID)
		if err != nil {
			return nil, err
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

func (t *TransactionsLocalDataProvider) GetTransactionResultByIndexFromStorage(
	ctx context.Context,
	block *flow.Block,
	index uint32,
	requiredEventEncodingVersion entities.EventEncodingVersion,
	errorMessageByIndexGetter TransactionErrorMessageByIndexGetter,
) (*access.TransactionResult, error) {
	blockID := block.ID()
	txResult, err := t.results.ByBlockIDTransactionIndex(blockID, index)
	if err != nil {
		return nil, rpc.ConvertStorageError(err)
	}

	var txErrorMessage string
	var txStatusCode uint = 0
	if txResult.Failed {
		if errorMessageByIndexGetter != nil {
			txErrorMessage, err = errorMessageByIndexGetter(ctx, blockID, index)
			if err != nil {
				return nil, err
			}
		}

		txStatusCode = 1 // statusCode of 1 indicates an error and 0 indicates no error, the same as on EN
	}

	// tx body is irrelevant to status if it's in an executed block
	txStatus, err := t.deriveTransactionStatus(nil, true, block)
	if err != nil {
		if !errors.Is(err, state.ErrUnknownSnapshotReference) {
			irrecoverable.Throw(ctx, err)
		}
		return nil, rpc.ConvertStorageError(err)
	}

	events, err := t.events.ByBlockIDTransactionIndex(blockID, index)
	if err != nil {
		return nil, rpc.ConvertStorageError(err)
	}

	for _, e := range events {
		// events are encoded in CCF format in storage. convert to JSON-CDC if requested
		if requiredEventEncodingVersion == entities.EventEncodingVersion_JSON_CDC_V0 {
			payload, err := convert.CcfPayloadToJsonPayload(e.Payload)
			if err != nil {
				err = fmt.Errorf("failed to convert event payload for block %s: %w", blockID, err)
				return nil, rpc.ConvertError(err, "failed to convert event payload", codes.Internal)
			}
			e.Payload = payload
		}
	}

	collectionID, err := t.lookupCollectionIDInBlock(block, txResult.TransactionID)
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

// deriveTransactionStatus derives the transaction status based on current protocol state
// Error returns:
//   - state.ErrUnknownSnapshotReference - block referenced by transaction has not been found.
//   - all other errors are unexpected and potentially symptoms of internal implementation bugs or state corruption (fatal).
func (t *TransactionsLocalDataProvider) deriveTransactionStatus(
	tx *flow.TransactionBody,
	executed bool,
	block *flow.Block,
) (flow.TransactionStatus, error) {
	if block == nil {
		// Not in a block, let's see if it's expired
		referenceBlock, err := t.state.AtBlockID(tx.ReferenceBlockID).Head()
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

	if block.Header.Height > sealed.Height {
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

// lookupCollectionIDInBlock returns the collection ID based on the transaction ID. The lookup is performed in block
// collections.
func (t *TransactionsLocalDataProvider) lookupCollectionIDInBlock(
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
				return collection.ID(), nil
			}
		}
	}
	return flow.ZeroID, status.Error(codes.NotFound, "transaction not found in block")
}
