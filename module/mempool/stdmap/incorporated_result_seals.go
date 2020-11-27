package stdmap

import "github.com/onflow/flow-go/model/flow"

// IncorporatedResultSeals implements the incorporated result seals memory pool
// of the consensus nodes, used to store seals that need to be added to blocks.
type IncorporatedResultSeals struct {
	*Backend
}

// NewIncorporatedResults creates a mempool for the incorporated result seals
func NewIncorporatedResultSeals(limit uint, opts ...OptionFunc) *IncorporatedResultSeals {
	return &IncorporatedResultSeals{
		Backend: NewBackend(append(opts, WithLimit(limit))...),
	}
}

// Add adds an IncorporatedResultSeal to the mempool
func (ir *IncorporatedResultSeals) Add(seal *flow.IncorporatedResultSeal) bool {
	return ir.Backend.Add(seal)
}

// All returns all the items in the mempool
func (ir *IncorporatedResultSeals) All() []*flow.IncorporatedResultSeal {
	entities := ir.Backend.All()
	res := make([]*flow.IncorporatedResultSeal, 0, len(ir.entities))
	for _, entity := range entities {
		// uncaught type assertion; should never panic as the mempool only stores IncorporatedResultSeal:
		res = append(res, entity.(*flow.IncorporatedResultSeal))
	}
	return res
}

// ByID gets an IncorporatedResultSeal by ID
func (ir *IncorporatedResultSeals) ByID(id flow.Identifier) (*flow.IncorporatedResultSeal, bool) {
	entity, ok := ir.Backend.ByID(id)
	if !ok {
		return nil, false
	}
	// uncaught type assertion; should never panic as the mempool only stores IncorporatedResultSeal:
	return entity.(*flow.IncorporatedResultSeal), true
}

// Rem removes an IncorporatedResultSeal from the mempool
func (ir *IncorporatedResultSeals) Rem(incorporatedResultID flow.Identifier) bool {
	return ir.Backend.Rem(incorporatedResultID)
}
