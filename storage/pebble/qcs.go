package pebble

import (
	"github.com/cockroachdb/pebble"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/pebble/operation"
)

// QuorumCertificates implements persistent storage for quorum certificates.
type QuorumCertificates struct {
	db    *pebble.DB
	cache *Cache[flow.Identifier, *flow.QuorumCertificate]
}

var _ storage.QuorumCertificates = (*QuorumCertificates)(nil)

// NewQuorumCertificates Creates QuorumCertificates instance which is a database of quorum certificates
// which supports storing, caching and retrieving by block ID.
func NewQuorumCertificates(collector module.CacheMetrics, db *pebble.DB, cacheSize uint) *QuorumCertificates {
	store := func(_ flow.Identifier, qc *flow.QuorumCertificate) func(pebble.Writer) error {
		return operation.InsertQuorumCertificate(qc)
	}

	retrieve := func(blockID flow.Identifier) func(tx pebble.Reader) (*flow.QuorumCertificate, error) {
		return func(tx pebble.Reader) (*flow.QuorumCertificate, error) {
			var qc flow.QuorumCertificate
			err := operation.RetrieveQuorumCertificate(blockID, &qc)(tx)
			return &qc, err
		}
	}

	return &QuorumCertificates{
		db: db,
		cache: newCache[flow.Identifier, *flow.QuorumCertificate](collector, metrics.ResourceQC,
			withLimit[flow.Identifier, *flow.QuorumCertificate](cacheSize),
			withStore(store),
			withRetrieve(retrieve)),
	}
}

func (q *QuorumCertificates) StoreTx(qc *flow.QuorumCertificate) func(interface{}) error {
	return q.cache.PutTxInterface(qc.BlockID, qc)
}

func (q *QuorumCertificates) StorePebble(qc *flow.QuorumCertificate) func(storage.PebbleReaderBatchWriter) error {
	return func(rw storage.PebbleReaderBatchWriter) error {
		return q.cache.PutTxInterface(qc.BlockID, qc)(rw)
	}
}

func (q *QuorumCertificates) ByBlockID(blockID flow.Identifier) (*flow.QuorumCertificate, error) {
	return q.retrieveTx(blockID)(q.db)
}

func (q *QuorumCertificates) retrieveTx(blockID flow.Identifier) func(pebble.Reader) (*flow.QuorumCertificate, error) {
	return func(tx pebble.Reader) (*flow.QuorumCertificate, error) {
		val, err := q.cache.Get(blockID)(tx)
		if err != nil {
			return nil, err
		}
		return val, nil
	}
}
