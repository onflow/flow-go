package ejectors

import (
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage"
)

// LatestSeal is a mempool ejector for block seals that ejects newest-first.
// NOTE: should be initialized with its own headers instance with cache size
// equal to the mempool size.
type LatestSeal struct {
	headers storage.Headers
}

func NewLatestSeal(headers storage.Headers) *LatestSeal {
	ejector := &LatestSeal{
		headers: headers,
	}
	return ejector
}

func (ls *LatestSeal) Eject(entities map[flow.Identifier]flow.Entity) (flow.Identifier, flow.Entity) {
	maxHeight := uint64(0)
	maxID := flow.ZeroID

	for id := range entities {
		block, err := ls.headers.ByBlockID(id)
		if err != nil {
			continue
		}
		if block.Height > maxHeight {
			maxHeight = block.Height
			maxID = id
		}
	}

	return maxID, entities[maxID]
}
