package stdmap

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/mempool"
)

type sealSet map[flow.Identifier]*flow.IncorporatedResultSeal

// IncorporatedResultSeals implements the incorporated result seals memory pool
// of the consensus nodes, used to store seals that need to be added to blocks.
// Incorporated result seals are keyed by the ID of the incorporated result.
// ATTENTION: This data structure should NEVER eject seals because it can break liveness.
// Modules that are using this structure expect that it NEVER ejects a seal.
type IncorporatedResultSeals struct {
	*Backend[flow.Identifier, *flow.IncorporatedResultSeal]

	// CAUTION: byHeight and lowestHeight are protected by the Backend lock and must be modified within `Backend.Run`
	// byHeight indexes seals by the height of the executed block
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
	r := &IncorporatedResultSeals{
		Backend: NewBackend(
			WithLimit[flow.Identifier, *flow.IncorporatedResultSeal](limit),
			WithEject(EjectPanic[flow.Identifier, *flow.IncorporatedResultSeal]),
		),
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
		ir.mutableBackData.Remove(sealID)
	}
	delete(ir.byHeight, height)
}

// Add adds an IncorporatedResultSeal to the mempool
func (ir *IncorporatedResultSeals) Add(seal *flow.IncorporatedResultSeal) (bool, error) {
	added := false
	resultID := seal.IncorporatedResult.ID()
	err := ir.Backend.Run(func(backData mempool.BackData[flow.Identifier, *flow.IncorporatedResultSeal]) error {
		// skip elements below the pruned
		if seal.Header.Height < ir.lowestHeight {
			return nil
		}

		added = backData.Add(resultID, seal)
		if !added {
			return nil
		}

		height := indexByHeight(seal)
		sameHeight, ok := ir.byHeight[height]
		if !ok {
			sameHeight = make(sealSet)
			ir.byHeight[height] = sameHeight
		}
		sameHeight[resultID] = seal
		return nil
	})

	return added, err
}

// All returns all the items in the mempool
func (ir *IncorporatedResultSeals) All() []*flow.IncorporatedResultSeal {
	all := ir.Backend.All()
	results := make([]*flow.IncorporatedResultSeal, 0, len(all))
	for _, result := range all {
		results = append(results, result)
	}
	return results
}

// Remove removes an IncorporatedResultSeal from the mempool
func (ir *IncorporatedResultSeals) Remove(id flow.Identifier) bool {
	removed := false
	err := ir.Backend.Run(func(backData mempool.BackData[flow.Identifier, *flow.IncorporatedResultSeal]) error {
		if seal, ok := backData.Remove(id); ok {
			ir.removeFromIndex(id, seal.Header.Height)
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
	err := ir.Backend.Run(func(backData mempool.BackData[flow.Identifier, *flow.IncorporatedResultSeal]) error {
		backData.Clear()
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
// and the sentinel mempool.BelowPrunedThresholdError is returned.
func (ir *IncorporatedResultSeals) PruneUpToHeight(height uint64) error {
	return ir.Backend.Run(func(backData mempool.BackData[flow.Identifier, *flow.IncorporatedResultSeal]) error {
		if height < ir.lowestHeight {
			return mempool.NewBelowPrunedThresholdErrorf(
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
