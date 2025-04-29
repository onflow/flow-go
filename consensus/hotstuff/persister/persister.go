package persister

import (
	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
)

// Persister is responsible for persisting minimal critical safety and liveness data for HotStuff:
// specifically [hotstuff.LivenessData] and [hotstuff.SafetyData].
//
// Persister depends on protocol.State and cluster.State bootstrapping to set initial values for
// SafetyData and LivenessData, for each distinct chain ID. This bootstrapping must be complete
// before constructing a Persister instance with New (otherwise it will return an error).
type Persister struct {
	db      storage.DB
	chainID flow.ChainID
}

var _ hotstuff.Persister = (*Persister)(nil)
var _ hotstuff.PersisterReader = (*Persister)(nil)

// New creates a new Persister.
//
// Persister depends on protocol.State and cluster.State bootstrapping to set initial values for
// SafetyData and LivenessData, for each distinct chain ID. This bootstrapping must be completed
// before first using a Persister instance.
func New(db storage.DB, chainID flow.ChainID) (*Persister, error) {
	p := &Persister{
		db:      db,
		chainID: chainID,
	}
	return p, nil
}

// NewReader returns a new Persister as a PersisterReader type (only read methods accessible).
func NewReader(db storage.DB, chainID flow.ChainID) (hotstuff.PersisterReader, error) {
	return New(db, chainID)
}

// GetSafetyData will retrieve last persisted safety data.
// During normal operations, no errors are expected.
func (p *Persister) GetSafetyData() (*hotstuff.SafetyData, error) {
	var safetyData hotstuff.SafetyData
	err := operation.RetrieveSafetyData(p.db.Reader(), p.chainID, &safetyData)
	return &safetyData, err
}

// GetLivenessData will retrieve last persisted liveness data.
// During normal operations, no errors are expected.
func (p *Persister) GetLivenessData() (*hotstuff.LivenessData, error) {
	var livenessData hotstuff.LivenessData
	err := operation.RetrieveLivenessData(p.db.Reader(), p.chainID, &livenessData)
	return &livenessData, err
}

// PutSafetyData persists the last safety data.
// During normal operations, no errors are expected.
func (p *Persister) PutSafetyData(safetyData *hotstuff.SafetyData) error {
	return p.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		return operation.UpsertSafetyData(rw.Writer(), p.chainID, safetyData)
	})
}

// PutLivenessData persists the last liveness data.
// During normal operations, no errors are expected.
func (p *Persister) PutLivenessData(livenessData *hotstuff.LivenessData) error {
	return p.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		return operation.UpsertLivenessData(rw.Writer(), p.chainID, livenessData)
	})
}
