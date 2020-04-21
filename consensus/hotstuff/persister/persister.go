package persister

import (
	"github.com/dapperlabs/flow-go/storage"
)

// Persister can persist relevant information for hotstuff.
type Persister struct {
	views storage.Views
}

// New creates a nev persister using the injected stores to persist
// relevant hotstuff data.
func New(views storage.Views) *Persister {
	p := &Persister{
		views: views,
	}
	return p
}

// StartedView persists the view when we start it in hotstuff.
func (p *Persister) StartedView(view uint64) error {
	return p.views.Store(ActionStarted, view)
}

// VotedView persist the view when we voted in hotstuff.
func (p *Persister) VotedView(view uint64) error {
	return p.views.Store(ActionVoted, view)
}
