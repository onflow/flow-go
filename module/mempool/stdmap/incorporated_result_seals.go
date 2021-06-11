package stdmap

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/mempool"
)

type sealSet map[flow.Identifier]*flow.IncorporatedResultSeal

// IncorporatedResultSeals implements the incorporated result seals memory pool
// of the consensus nodes, used to store seals that need to be added to blocks.
type IncorporatedResultSeals struct {
	*Backend
	// index the seals by the height of the executed block
	byHeight     map[uint64]sealSet
	lowestHeight uint64
}

func indexByHeight(seal *flow.IncorporatedResultSeal) uint64 {
	return seal.Header.Height
}

// NewIncorporatedResultSeals creates a mempool for the incorporated result seals
func NewIncorporatedResultSeals(limit uint) *IncorporatedResultSeals {
	byHeight := make(map[uint64]sealSet)

	// assuming all the entities are for unsealed blocks, then we will remove a seal
	// with the largest height.
	ejector := func(entities map[flow.Identifier]flow.Entity) (flow.Identifier, flow.Entity) {
		maxHeight := uint64(0)
		var sealsAtMaxHeight sealSet
		for height, seals := range byHeight {
			if height > maxHeight {
				maxHeight = height
				sealsAtMaxHeight = seals
			}
		}
		if len(sealsAtMaxHeight) == 0 {
			// this can only happen if mempool is empty or if the secondary index was inconsistently updated
			panic("cannot eject element from empty mempool")
		}

		for sealID, seal := range sealsAtMaxHeight {
			return sealID, seal
		}
		panic("cannot eject element from empty mempool")
	}

	r := &IncorporatedResultSeals{
		Backend:  NewBackend(WithLimit(limit), WithEject(ejector)),
		byHeight: byHeight,
	}

	// when eject a entity, also update the secondary indx
	r.RegisterEjectionCallbacks(func(entity flow.Entity) {
		seal := entity.(*flow.IncorporatedResultSeal)
		sealID := seal.ID()
		r.removeFromIndex(sealID, seal.Header.Height)
	})

	return r
}

func (ir *IncorporatedResultSeals) removeFromIndex(id flow.Identifier, height uint64) {
	sealsAtHeight := ir.byHeight[height]
	delete(sealsAtHeight, id)
	if len(sealsAtHeight) == 0 {
		delete(ir.byHeight, height)
	}
}

func (ir *IncorporatedResultSeals) removeByHeight(height uint64) {
	for sealID := range ir.byHeight[height] {
		ir.Backdata.Rem(sealID)
	}
	delete(ir.byHeight, height)
}

// Add adds an IncorporatedResultSeal to the mempool
func (ir *IncorporatedResultSeals) Add(seal *flow.IncorporatedResultSeal) (bool, error) {
	added := false
	sealID := seal.ID()
	err := ir.Backend.Run(func(entities map[flow.Identifier]flow.Entity) error {
		// skip elements below the pruned
		if seal.Header.Height < ir.lowestHeight {
			return nil
		}

		added = ir.Backdata.Add(seal)
		if !added {
			return nil
		}

		height := indexByHeight(seal)
		sameHeight, ok := ir.byHeight[height]
		if !ok {
			sameHeight = make(sealSet)
			ir.byHeight[height] = sameHeight
		}
		sameHeight[sealID] = seal
		return nil
	})

	return added, err
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

// ByID gets an IncorporatedResultSeal by IncorporatedResult ID
func (ir *IncorporatedResultSeals) ByID(id flow.Identifier) (*flow.IncorporatedResultSeal, bool) {
	entity, ok := ir.Backend.ByID(id)
	if !ok {
		return nil, false
	}
	// uncaught type assertion; should never panic as the mempool only stores IncorporatedResultSeal:
	return entity.(*flow.IncorporatedResultSeal), true
}

// Rem removes an IncorporatedResultSeal from the mempool
func (ir *IncorporatedResultSeals) Rem(id flow.Identifier) bool {
	removed := false
	err := ir.Backend.Run(func(entities map[flow.Identifier]flow.Entity) error {
		var entity flow.Entity
		entity, removed = ir.Backdata.Rem(id)
		if !removed {
			return nil
		}
		seal := entity.(*flow.IncorporatedResultSeal)
		ir.removeFromIndex(id, seal.Header.Height)
		return nil
	})
	if err != nil {
		panic(err)
	}
	return removed
}

func (ir *IncorporatedResultSeals) Clear() {
	err := ir.Backend.Run(func(_ map[flow.Identifier]flow.Entity) error {
		ir.Backdata.Clear()
		ir.byHeight = make(map[uint64]sealSet)
		return nil
	})
	if err != nil {
		panic(err)
	}
}

// RegisterEjectionCallbacks adds the provided OnEjection callbacks
func (ir *IncorporatedResultSeals) RegisterEjectionCallbacks(callbacks ...mempool.OnEjection) {
	ir.Backend.RegisterEjectionCallbacks(callbacks...)
}

// PruneUpToHeight remove all seals for blocks whose height is strictly
// smaller that height. Note: seals for blocks at height are retained.
func (ir *IncorporatedResultSeals) PruneUpToHeight(height uint64) error {
	return ir.Backend.Run(func(entities map[flow.Identifier]flow.Entity) error {
		if height < ir.lowestHeight {
			return fmt.Errorf("new pruning height %v cannot be smaller than previous pruned height:%v", height, ir.lowestHeight)
		}

		if len(entities) == 0 {
			ir.lowestHeight = height
			return nil
		}
		// Optimization: if there are less height in the index than the height range to prune,
		// range to prune, then just go through each seal.
		// Otherwise, go through each height to prune.
		if uint64(len(ir.byHeight)) < height-ir.lowestHeight {
			for h := range ir.byHeight {
				if h < height {
					ir.removeByHeight(h)
				}
			}
		} else {
			for h := ir.lowestHeight; h < height; h++ {
				ir.removeByHeight(h)
			}
		}
		ir.lowestHeight = height
		return nil
	})
}
