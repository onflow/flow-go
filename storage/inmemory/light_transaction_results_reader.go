package inmemory

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

type LightTransactionResultsReader struct {
	blockID   flow.Identifier
	results   []flow.LightTransactionResult
	byTxID    map[flow.Identifier]*flow.LightTransactionResult
	byTxIndex map[uint32]*flow.LightTransactionResult
}

var _ storage.LightTransactionResultsReader = (*LightTransactionResultsReader)(nil)

func NewLightTransactionResults(blockID flow.Identifier, results []flow.LightTransactionResult) *LightTransactionResultsReader {
	byTxID := make(map[flow.Identifier]*flow.LightTransactionResult)
	byTxIndex := make(map[uint32]*flow.LightTransactionResult)

	for i, result := range results {
		byTxID[result.TransactionID] = &result
		byTxIndex[uint32(i)] = &result
	}

	return &LightTransactionResultsReader{
		blockID:   blockID,
		results:   results,
		byTxID:    byTxID,
		byTxIndex: byTxIndex,
	}
}

// ByBlockIDTransactionID returns the transaction result for the given block ID and transaction
//
// Expected error returns during normal operation:
//   - [storage.ErrNotFound] if light transaction result at given blockID wasn't found.
func (l *LightTransactionResultsReader) ByBlockIDTransactionID(blockID flow.Identifier, transactionID flow.Identifier) (*flow.LightTransactionResult, error) {
	if l.blockID != blockID {
		return nil, storage.ErrNotFound
	}

	val, ok := l.byTxID[transactionID]
	if !ok {
		return nil, storage.ErrNotFound
	}

	return val, nil
}

// ByBlockIDTransactionIndex returns the transaction result for the given blockID and transaction index
//
// Expected error returns during normal operation:
//   - [storage.ErrNotFound] if light transaction result at given blockID and txIndex wasn't found.
func (l *LightTransactionResultsReader) ByBlockIDTransactionIndex(blockID flow.Identifier, txIndex uint32) (*flow.LightTransactionResult, error) {
	if l.blockID != blockID {
		return nil, storage.ErrNotFound
	}

	val, ok := l.byTxIndex[txIndex]
	if !ok {
		return nil, storage.ErrNotFound
	}

	return val, nil
}

// ByBlockID gets all transaction results for a block, ordered by transaction index
//
// Expected error returns during normal operation:
//   - [storage.ErrNotFound] if light transaction results at given blockID weren't found.
func (l *LightTransactionResultsReader) ByBlockID(id flow.Identifier) ([]flow.LightTransactionResult, error) {
	if l.blockID != id {
		return nil, storage.ErrNotFound
	}

	return l.results, nil
}
