package stdmap

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/mempool"
)

// IncorporatedResultSeals implements the incorporated result seals memory pool
// of the consensus nodes, used to store seals that need to be added to blocks.
type IncorporatedResultSeals struct {
	*Backend
	// index the seals by the height of the executed block
	byHeight     map[uint64][]flow.Identifier
	lowestHeight uint64
}

func indexByHeight(seal *flow.IncorporatedResultSeal) uint64 {
	return seal.Header.Height
}

// NewIncorporatedResults creates a mempool for the incorporated result seals
func NewIncorporatedResultSeals(opts ...OptionFunc) *IncorporatedResultSeals {
	r := &IncorporatedResultSeals{
		Backend:  NewBackend(opts...),
		byHeight: make(map[uint64][]flow.Identifier),
	}

	// assuming all the entities are for unsealed blocks, then we will remove a seal
	// with the highest height.
	r.eject = func(entities map[flow.Identifier]flow.Entity) (flow.Identifier, flow.Entity) {
		maxHeight := uint64(0)
		for height := range r.byHeight {
			if height > maxHeight {
				maxHeight = height
			}
		}
		seals, ok := r.byHeight[maxHeight]
		if !ok {
			panic(fmt.Sprintf("can't find seals by height: %v", maxHeight))
		}

		if len(seals) == 0 {
			panic(fmt.Sprintf("empty seal at height: %v", maxHeight))
		}

		sealID := seals[0]
		seal, ok := entities[sealID]
		if !ok {
			panic(fmt.Sprintf("inconsistent index, can not find seal by id: %v", sealID))
		}
		return sealID, seal
	}

	// when eject a entity, also update the secondary indx
	r.RegisterEjectionCallbacks(func(entity flow.Entity) {
		seal := entity.(*flow.IncorporatedResultSeal)
		removeSeal(seal, r.entities, r.byHeight)
	})

	return r
}

func removeSeal(seal *flow.IncorporatedResultSeal,
	entities map[flow.Identifier]flow.Entity,
	byHeight map[uint64][]flow.Identifier) {
	sealID := seal.ID()
	delete(entities, sealID)
	index := indexByHeight(seal)
	siblings := byHeight[index]
	newsiblings := make([]flow.Identifier, 0, len(siblings))
	for _, sibling := range siblings {
		if sibling != sealID {
			newsiblings = append(newsiblings, sibling)
		}
	}

	if len(newsiblings) == 0 {
		delete(byHeight, index)
	} else {
		byHeight[index] = newsiblings
	}
}

func removeByHeight(height uint64, entities map[flow.Identifier]flow.Entity,
	byHeight map[uint64][]flow.Identifier) error {
	sameHeight, ok := byHeight[height]
	if !ok {
		return fmt.Errorf("cannot find seals by height: %v", height)
	}
	for _, s := range sameHeight {
		entity, ok := entities[s]
		if !ok {
			return fmt.Errorf("inconsistent index, could not find seal at height %v by id %v", height, s)
		}
		seal := entity.(*flow.IncorporatedResultSeal)
		removeSeal(seal, entities, byHeight)
	}
	return nil
}

// Add adds an IncorporatedResultSeal to the mempool and update the secondary index
func (ir *IncorporatedResultSeals) Add(seal *flow.IncorporatedResultSeal) (bool, error) {
	added := false
	err := ir.Backend.Run(func(entities map[flow.Identifier]flow.Entity) error {
		// skip sealed height
		if seal.Header.Height <= ir.lowestHeight {
			return nil
		}

		sealID := seal.ID()
		_, exists := entities[sealID]
		if exists {
			return nil
		}

		entities[sealID] = seal

		height := indexByHeight(seal)
		sameHeight, ok := ir.byHeight[height]
		if !ok {
			ir.byHeight[height] = []flow.Identifier{sealID}
		} else {
			ir.byHeight[height] = append(sameHeight, sealID)
		}
		added = true
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
		entity, ok := entities[id]
		if ok {
			seal := entity.(*flow.IncorporatedResultSeal)
			removeSeal(seal, entities, ir.byHeight)
			removed = true
		}
		return nil
	})
	if err != nil {
		panic(err)
	}
	return removed
}

func (ir *IncorporatedResultSeals) Clear() {
	err := ir.Backend.Run(func(entities map[flow.Identifier]flow.Entity) error {
		for k := range entities {
			delete(entities, k)
		}
		ir.byHeight = make(map[uint64][]flow.Identifier)
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
		// optimization, if there are less height than the height range to prune,
		// then just go through each seal.
		// otherwise, go through each height to prune
		if uint64(len(ir.byHeight)) < height-ir.lowestHeight {
			for h := range ir.byHeight {
				if h <= height {
					err := removeByHeight(h, entities, ir.byHeight)
					if err != nil {
						return err
					}
				}
			}
		} else {
			for h := ir.lowestHeight + 1; h <= height; h++ {
				err := removeByHeight(h, entities, ir.byHeight)
				if err != nil {
					return err
				}
			}
		}
		ir.lowestHeight = height
		return nil
	})
}
