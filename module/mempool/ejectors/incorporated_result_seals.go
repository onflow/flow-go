package ejectors

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// LatestIncorporatedResultSeal is a mempool ejector for incorporated result
// seals that ejects newest-first.
// NOTE: should be initialized with its own headers instance with cache size
// equal to the mempool size.
type LatestIncorporatedResultSeal struct {
	headers storage.Headers
}

func NewLatestIncorporatedResultSeal(headers storage.Headers) *LatestIncorporatedResultSeal {
	ejector := &LatestIncorporatedResultSeal{
		headers: headers,
	}
	return ejector
}

func (ls *LatestIncorporatedResultSeal) Eject(entities map[flow.Identifier]flow.Entity) (flow.Identifier, flow.Entity) {
	maxHeight := uint64(0)
	maxID := flow.ZeroID

	for id, entity := range entities {
		irSeal := entity.(*flow.IncorporatedResultSeal)
		block, err := ls.headers.ByBlockID(irSeal.Seal.BlockID)
		if err != nil {
			// eject seals first, whose block is not even known (which are presumably newer than any other entity)
			return id, entity
		}
		if block.Height >= maxHeight {
			maxHeight = block.Height
			maxID = id
		}
	}

	return maxID, entities[maxID]
}
