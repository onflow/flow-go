package extended

import (
	"errors"

	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/storage"
)

var (
	ErrAlreadyIndexed = errors.New("data already indexed for height")
	ErrFutureHeight   = errors.New("cannot index future height")
)

type BlockData struct {
	Header       *flow.Header
	Transactions []*flow.TransactionBody
	Events       []flow.Event
}

type Indexer interface {
	// Name returns the name of the indexer.
	Name() string

	// IndexBlockData indexes the block data for the given height.
	// If the header in `data` does not match the expected height, an error is returned.
	//
	// CAUTION: Not safe for concurrent use.
	//
	// Expected error returns during normal operations:
	//   - [ErrAlreadyIndexed]: if the data is already indexed for the height.
	//   - [ErrFutureHeight]: if the data is for a future height.
	IndexBlockData(lctx lockctx.Proof, data BlockData, batch storage.ReaderBatchWriter) error

	// NextHeight returns the next height that the indexer will index.
	//
	// No error returns are expected during normal operation.
	NextHeight() (uint64, error)
}

// IndexerManager orchestrates indexing for all extended indexers. It handles both indexing from the
// latest block submitted via the Index methods, and backfilling from storage.
type IndexerManager interface {
	// IndexBlockExecutionData indexes the block data for the given height.
	// If the header in `data` does not match the expected height, an error is returned.
	//
	// Not safe for concurrent use.
	//
	// No error returns are expected during normal operation.
	IndexBlockExecutionData(data *execution_data.BlockExecutionDataEntity) error

	// IndexBlockData indexes the block data for the given height.
	// If the header in `data` does not match the expected height, an error is returned.
	//
	// Not safe for concurrent use.
	//
	// No error returns are expected during normal operation.
	IndexBlockData(header *flow.Header, transactions []*flow.TransactionBody, events []flow.Event) error
}
