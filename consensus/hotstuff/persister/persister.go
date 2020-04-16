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

// CurrentView persists the current view of hotstuff.
func (p *Persister) CurrentView(view uint64) error {
	return p.views.StoreLatest(view)
}
