package stdmap

import (
	"log"

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

	// This mempool implementation supports pruning by height, meaning that as soon as sealing advances
	// seals will be gradually removed from mempool
	// ejecting a seal from mempool means that we have reached our limit and something is very bad, meaning that sealing
	// is not actually happening.
	// By setting high limit ~12 hours we ensure that we have some safety window for sealing to recover and make progress
	ejector := func(b *Backend) (flow.Identifier, flow.Entity, bool) {
		log.Fatalf("incorporated result seals reached max capacity %d", limit)
		panic("incorporated result seals reached max capacity")
	}

	r := &IncorporatedResultSeals{
		Backend:  NewBackend(WithLimit(limit), WithEject(ejector)),
		byHeight: byHeight,
	}

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
		ir.backData.Rem(sealID)
	}
	delete(ir.byHeight, height)
}

// Add adds an IncorporatedResultSeal to the mempool
func (ir *IncorporatedResultSeals) Add(seal *flow.IncorporatedResultSeal) (bool, error) {
	added := false
	sealID := seal.ID()
	err := ir.Backend.Run(func(_ mempool.BackData) error {
		// skip elements below the pruned
		if seal.Header.Height < ir.lowestHeight {
			return nil
		}

		added = ir.backData.Add(sealID, seal)
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

// Size returns the size of the underlying backing store
func (ir *IncorporatedResultSeals) Size() uint {
	return ir.Backend.Size()
}

// All returns all the items in the mempool
func (ir *IncorporatedResultSeals) All() []*flow.IncorporatedResultSeal {
	entities := ir.Backend.All()
	res := make([]*flow.IncorporatedResultSeal, 0, ir.backData.Size())
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
	err := ir.Backend.Run(func(_ mempool.BackData) error {
		var entity flow.Entity
		entity, removed = ir.backData.Rem(id)
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
	err := ir.Backend.Run(func(_ mempool.BackData) error {
		ir.backData.Clear()
		ir.byHeight = make(map[uint64]sealSet)
		return nil
	})
	if err != nil {
		panic(err)
	}
}

// PruneUpToHeight remove all seals for blocks whose height is strictly
// smaller that height. Note: seals for blocks at height are retained.
// After pruning, seals below for blocks below the given height are dropped.
//
// Monotonicity Requirement:
// The pruned height cannot decrease, as we cannot recover already pruned elements.
// If `height` is smaller than the previous value, the previous value is kept
// and the sentinel mempool.DecreasingPruningHeightError is returned.
func (ir *IncorporatedResultSeals) PruneUpToHeight(height uint64) error {
	return ir.Backend.Run(func(backData mempool.BackData) error {
		if height < ir.lowestHeight {
			return mempool.NewDecreasingPruningHeightErrorf(
				"pruning height: %d, existing height: %d", height, ir.lowestHeight)
		}

		if backData.Size() == 0 {
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
