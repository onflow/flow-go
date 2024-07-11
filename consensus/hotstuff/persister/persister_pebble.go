package persister

import (
	"github.com/cockroachdb/pebble"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage/pebble/operation"
)

// PersisterPebble can persist relevant information for hotstuff.
// PersisterPebble depends on protocol.State root snapshot bootstrapping to set initial values for
// SafetyData and LivenessData. These values must be initialized before first use of Persister.
type PersisterPebble struct {
	db      *pebble.DB
	chainID flow.ChainID
}

var _ hotstuff.Persister = (*PersisterPebble)(nil)

// New creates a new Persister using the injected data base to persist
// relevant hotstuff data.
func NewPersisterPebble(db *pebble.DB, chainID flow.ChainID) *PersisterPebble {
	p := &PersisterPebble{
		db:      db,
		chainID: chainID,
	}
	return p
}

// GetSafetyData will retrieve last persisted safety data.
// During normal operations, no errors are expected.
func (p *PersisterPebble) GetSafetyData() (*hotstuff.SafetyData, error) {
	var safetyData hotstuff.SafetyData
	err := operation.RetrieveSafetyData(p.chainID, &safetyData)(p.db)
	return &safetyData, err
}

// GetLivenessData will retrieve last persisted liveness data.
// During normal operations, no errors are expected.
func (p *PersisterPebble) GetLivenessData() (*hotstuff.LivenessData, error) {
	var livenessData hotstuff.LivenessData
	err := operation.RetrieveLivenessData(p.chainID, &livenessData)(p.db)
	return &livenessData, err
}

// PutSafetyData persists the last safety data.
// During normal operations, no errors are expected.
func (p *PersisterPebble) PutSafetyData(safetyData *hotstuff.SafetyData) error {
	return operation.UpdateSafetyData(p.chainID, safetyData)(p.db)
}

// PutLivenessData persists the last liveness data.
// During normal operations, no errors are expected.
func (p *PersisterPebble) PutLivenessData(livenessData *hotstuff.LivenessData) error {
	return operation.UpdateLivenessData(p.chainID, livenessData)(p.db)
}
