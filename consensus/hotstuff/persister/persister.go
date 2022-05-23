package persister

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage/badger/operation"
)

// Persister can persist relevant information for hotstuff.
type Persister struct {
	db      *badger.DB
	chainID flow.ChainID
}

var _ hotstuff.Persister = (*Persister)(nil)

// New creates a nev persister using the injected stores to persist
// relevant hotstuff data.
func New(db *badger.DB, chainID flow.ChainID) *Persister {
	p := &Persister{
		db:      db,
		chainID: chainID,
	}
	return p
}

// GetSafetyData will retrieve last persisted safety data.
func (p *Persister) GetSafetyData() (*hotstuff.SafetyData, error) {
	var safetyData hotstuff.SafetyData
	err := p.db.View(operation.RetrieveSafetyData(p.chainID, &safetyData))
	return &safetyData, err
}

// GetLivenessData will retrieve last persisted liveness data.
func (p *Persister) GetLivenessData() (*hotstuff.LivenessData, error) {
	var livenessData hotstuff.LivenessData
	err := p.db.View(operation.RetrieveLivenessData(p.chainID, &livenessData))
	return &livenessData, err
}

// PutSafetyData persists the last safety data.
func (p *Persister) PutSafetyData(safetyData *hotstuff.SafetyData) error {
	return operation.RetryOnConflict(p.db.Update, operation.UpdateSafetyData(p.chainID, safetyData))
}

// PutLivenessData persists the last liveness data.
func (p *Persister) PutLivenessData(livenessData *hotstuff.LivenessData) error {
	return operation.RetryOnConflict(p.db.Update, operation.UpdateLivenessData(p.chainID, livenessData))
}
