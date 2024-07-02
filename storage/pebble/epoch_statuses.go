package pebble

import (
	"github.com/cockroachdb/pebble"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/transaction"
	"github.com/onflow/flow-go/storage/pebble/operation"
)

type EpochStatuses struct {
	db    *pebble.DB
	cache *Cache[flow.Identifier, *flow.EpochStatus]
}

var _ storage.EpochStatuses = (*EpochStatuses)(nil)

// NewEpochStatuses ...
func NewEpochStatuses(collector module.CacheMetrics, db *pebble.DB) *EpochStatuses {

	store := func(blockID flow.Identifier, status *flow.EpochStatus) func(pebble.Writer) error {
		return operation.InsertEpochStatus(blockID, status)
	}

	retrieve := func(blockID flow.Identifier) func(pebble.Reader) (*flow.EpochStatus, error) {
		return func(tx pebble.Reader) (*flow.EpochStatus, error) {
			var status flow.EpochStatus
			err := operation.RetrieveEpochStatus(blockID, &status)(tx)
			return &status, err
		}
	}

	es := &EpochStatuses{
		db: db,
		cache: newCache[flow.Identifier, *flow.EpochStatus](collector, metrics.ResourceEpochStatus,
			withLimit[flow.Identifier, *flow.EpochStatus](4*flow.DefaultTransactionExpiry),
			withStore(store),
			withRetrieve(retrieve)),
	}

	return es
}

func (es *EpochStatuses) StoreTx(blockID flow.Identifier, status *flow.EpochStatus) func(tx *transaction.Tx) error {
	return nil
}

func (es *EpochStatuses) StorePebble(blockID flow.Identifier, status *flow.EpochStatus) func(storage.PebbleReaderBatchWriter) error {
	return func(rw storage.PebbleReaderBatchWriter) error {
		_, tx := rw.ReaderWriter()
		return es.cache.PutTx(blockID, status)(tx)
	}
}

func (es *EpochStatuses) retrieveTx(blockID flow.Identifier) func(tx pebble.Reader) (*flow.EpochStatus, error) {
	return func(tx pebble.Reader) (*flow.EpochStatus, error) {
		val, err := es.cache.Get(blockID)(tx)
		if err != nil {
			return nil, err
		}
		return val, nil
	}
}

// ByBlockID will return the epoch status for the given block
// Error returns:
// * storage.ErrNotFound if EpochStatus for the block does not exist
func (es *EpochStatuses) ByBlockID(blockID flow.Identifier) (*flow.EpochStatus, error) {
	return es.retrieveTx(blockID)(es.db)
}
