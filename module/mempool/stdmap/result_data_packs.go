package stdmap

import (
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/verification"
)

// ResultDataPacks implements the ResultDataPacks mempool interface.
// ResultDataPacks has an LRU ejector, i.e., it evicts the oldest entity if
// it gets full.
type ResultDataPacks struct {
	*Backend
	ejector *LRUEjector
}

// NewResultDataPacks creates a new mempool for execution results.
// The mempool has a FIFO ejector.
func NewResultDataPacks(limit uint) *ResultDataPacks {
	qe := NewLRUEjector()
	return &ResultDataPacks{
		ejector: qe,
		Backend: NewBackend(WithLimit(limit), WithEject(qe.Eject)),
	}
}

// Add will add the given ResultDataPacks to the mempool. It will return
// false if it was already in the mempool.
func (r *ResultDataPacks) Add(rdp *verification.ResultDataPack) bool {
	ok := r.Backend.Add(rdp)
	if ok {
		// adds successfully stored result data pack id to the ejector
		// The ejector externally keeps track of ids added to the mempool.
		r.ejector.Track(rdp.ID())
	}
	return ok
}

// Rem removes a ResultDataPacks by identifier.
func (r *ResultDataPacks) Rem(rdpID flow.Identifier) bool {
	ok := r.Backend.Rem(rdpID)
	if ok {
		// untracks the successfully removed entity from the ejector
		r.ejector.Untrack(rdpID)
	}

	return ok
}

// Has returns true if a ResultDataPacks with the specified identifier exists.
func (r *ResultDataPacks) Has(rdpID flow.Identifier) bool {
	return r.Backend.Has(rdpID)
}

// Get returns the ResultDataPacks and true, if the ResultDataPacks is in the
// mempool. Otherwise, it returns nil and false.
func (r *ResultDataPacks) Get(rdpID flow.Identifier) (*verification.ResultDataPack, bool) {
	entity, ok := r.Backend.ByID(rdpID)
	if !ok {
		return nil, false
	}

	rdp, ok := entity.(*verification.ResultDataPack)
	if !ok {
		return nil, false
	}

	return rdp, true
}

// Size returns total number ResultDataPacks in mempool
func (r *ResultDataPacks) Size() uint {
	return r.Backend.Size()
}
