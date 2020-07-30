package persister

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
)

// Persister can persist relevant information for hotstuff.
type Persister struct {
	db      *badger.DB
	chainID flow.ChainID
}

// New creates a nev persister using the injected stores to persist
// relevant hotstuff data.
func New(db *badger.DB, chainID flow.ChainID) *Persister {
	p := &Persister{
		db:      db,
		chainID: chainID,
	}
	return p
}

// GetStarted returns the last persisted started view.
func (p *Persister) GetStarted() (uint64, error) {
	var view uint64
	err := p.db.View(operation.RetrieveStartedView(p.chainID.String(), &view))
	return view, err
}

// GetVoted returns the last persisted started view.
func (p *Persister) GetVoted() (uint64, error) {
	var view uint64
	err := p.db.View(operation.RetrieveVotedView(p.chainID.String(), &view))
	return view, err
}

// PutStarted persists the view when we start it in hotstuff.
func (p *Persister) PutStarted(view uint64) error {
	return operation.RetryOnConflict(p.db.Update, operation.UpdateStartedView(p.chainID.String(), view))
}

// PutVoted persist the view when we voted in hotstuff.
func (p *Persister) PutVoted(view uint64) error {
	return operation.RetryOnConflict(p.db.Update, operation.UpdateVotedView(p.chainID.String(), view))
}
