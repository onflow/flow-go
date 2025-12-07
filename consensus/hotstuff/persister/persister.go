package persister

import (
	"fmt"

	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
)

// Persister is responsible for persisting minimal critical safety and liveness data for HotStuff:
// specifically [hotstuff.LivenessData] and [hotstuff.SafetyData].
// Persister depends on protocol.State and cluster.State bootstrapping to set initial values for
// SafetyData and LivenessData, for each distinct chain ID. This bootstrapping must be complete
// before constructing a Persister instance with New (otherwise it will return an error).
type Persister struct {
	db          storage.DB
	chainID     flow.ChainID
	lockManager lockctx.Manager
}

var _ hotstuff.Persister = (*Persister)(nil)
var _ hotstuff.PersisterReader = (*Persister)(nil)

// New creates a new Persister.
// Persister depends on protocol.State and cluster.State bootstrapping to set initial values for
// SafetyData and LivenessData, for each distinct chain ID. This bootstrapping must be completed
// before first using a Persister instance.
func New(db storage.DB, chainID flow.ChainID, lockManager lockctx.Manager) (*Persister, error) {
	err := ensureSafetyDataAndLivenessDataAreBootstrapped(db, chainID)
	if err != nil {
		return nil, fmt.Errorf("fail to check persister was properly bootstrapped: %w", err)
	}

	p := &Persister{
		db:          db,
		chainID:     chainID,
		lockManager: lockManager,
	}
	return p, nil
}

// ensureSafetyDataAndLivenessDataAreBootstrapped checks if the safety and liveness data are
// bootstrapped for the given chain ID. If not, it returns an error.
// The Flow Protocol mandates that SafetyData and LivenessData is provided as part of the bootstrapping
// data. For a node, the SafetyData and LivenessData is among the most safety-critical data. We require
// the protocol.State's or cluster.State's bootstrapping logic to properly initialize these values in the
// database.
func ensureSafetyDataAndLivenessDataAreBootstrapped(db storage.DB, chainID flow.ChainID) error {
	var safetyData hotstuff.SafetyData
	err := operation.RetrieveSafetyData(db.Reader(), chainID, &safetyData)
	if err != nil {
		return fmt.Errorf("fail to retrieve safety data: %w", err)
	}

	var livenessData hotstuff.LivenessData
	err = operation.RetrieveLivenessData(db.Reader(), chainID, &livenessData)
	if err != nil {
		return fmt.Errorf("fail to retrieve liveness data: %w", err)
	}

	return nil
}

// NewReader returns a new Persister as a PersisterReader type (only read methods accessible).
func NewReader(db storage.DB, chainID flow.ChainID, lockManager lockctx.Manager) (hotstuff.PersisterReader, error) {
	return New(db, chainID, lockManager)
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
	return storage.WithLock(p.lockManager, storage.LockInsertSafetyData, func(lctx lockctx.Context) error {
		return p.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return operation.UpsertSafetyData(lctx, rw, p.chainID, safetyData)
		})
	})
}

// PutLivenessData persists the last liveness data.
// During normal operations, no errors are expected.
func (p *Persister) PutLivenessData(livenessData *hotstuff.LivenessData) error {
	return storage.WithLock(p.lockManager, storage.LockInsertLivenessData, func(lctx lockctx.Context) error {
		return p.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return operation.UpsertLivenessData(lctx, rw, p.chainID, livenessData)
		})
	})
}
