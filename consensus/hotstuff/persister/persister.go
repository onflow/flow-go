package persister

import (
	"fmt"
	"sync"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage/badger/operation"
)

// Persister is responsible for persisting minimal critical safety and liveness data for HotStuff:
// specifically [hotstuff.LivenessData] and [hotstuff.SafetyData].
//
// Persister depends on protocol.State and cluster.State bootstrapping to set initial values for
// SafetyData and LivenessData, for each distinct chain ID. This bootstrapping must be complete
// before constructing a Persister instance with New (otherwise it will return an error).
type Persister struct {
	db           *badger.DB
	chainID      flow.ChainID
	mu           *sync.Mutex
	safetyData   *hotstuff.SafetyData
	livenessData *hotstuff.LivenessData
}

var _ hotstuff.Persister = (*Persister)(nil)

// New creates a new Persister.
// Persister depends on protocol.State and cluster.State bootstrapping to set initial values for
// SafetyData and LivenessData, for each distinct chain ID. This bootstrapping must be complete
// before constructing a Persister instance with New (otherwise it will return an error).
func New(db *badger.DB, chainID flow.ChainID) (*Persister, error) {
	var safetyData hotstuff.SafetyData
	var livenessData hotstuff.LivenessData
	err := db.View(func(txn *badger.Txn) error {
		if err := operation.RetrieveSafetyData(chainID, &safetyData)(txn); err != nil {
			return err
		}
		err := operation.RetrieveLivenessData(chainID, &livenessData)(txn)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize hotstuff persisted for chain %s: %w", chainID, err)
	}

	p := &Persister{
		db:      db,
		chainID: chainID,
		mu:      &sync.Mutex{},
	}
	return p, nil
}

// GetSafetyData will retrieve last persisted safety data.
// During normal operations, no errors are expected.
func (p *Persister) GetSafetyData() (*hotstuff.SafetyData, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.safetyData, nil
}

// GetLivenessData will retrieve last persisted liveness data.
// During normal operations, no errors are expected.
func (p *Persister) GetLivenessData() (*hotstuff.LivenessData, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.livenessData, nil
}

// PutSafetyData persists the last safety data.
// During normal operations, no errors are expected.
func (p *Persister) PutSafetyData(safetyData *hotstuff.SafetyData) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	err := operation.RetryOnConflict(p.db.Update, operation.UpdateSafetyData(p.chainID, safetyData))
	if err != nil {
		return err
	}
	p.safetyData = safetyData
	return nil
}

// PutLivenessData persists the last liveness data.
// During normal operations, no errors are expected.
func (p *Persister) PutLivenessData(livenessData *hotstuff.LivenessData) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	err := operation.RetryOnConflict(p.db.Update, operation.UpdateLivenessData(p.chainID, livenessData))
	if err != nil {
		return err
	}
	p.livenessData = livenessData
	return nil
}
