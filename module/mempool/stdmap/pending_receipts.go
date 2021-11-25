package stdmap

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/mempool"
	"github.com/onflow/flow-go/storage"
)

type receiptsSet map[flow.Identifier]struct{}

// PendingReceipts stores pending receipts indexed by the id.
// It also maintains a secondary index on the previous result id.
// in order to allow to find receipts by the previous result id.
type PendingReceipts struct {
	*Backend
	headers storage.Headers // used to query headers of executed blocks
	// secondary index by parent result id, since multiple receipts could
	// have the same parent result, (even if they have different result)
	byPreviousResultID map[flow.Identifier]receiptsSet
	// secondary index by height, we need this to prune pending receipts to some height
	// it's safe to cleanup this index only when pruning. Even if some receipts are deleted manually,
	// eventually index will be cleaned up.
	byHeight     map[uint64]receiptsSet
	lowestHeight uint64
}

func indexByPreviousResultID(receipt *flow.ExecutionReceipt) flow.Identifier {
	return receipt.ExecutionResult.PreviousResultID
}

func (r *PendingReceipts) indexByHeight(receipt *flow.ExecutionReceipt) (uint64, error) {
	header, err := r.headers.ByBlockID(receipt.ExecutionResult.BlockID)
	if err != nil {
		return 0, fmt.Errorf("could not retreieve block by ID %v: %w", receipt.ExecutionResult.BlockID, err)
	}
	return header.Height, nil
}

// NewPendingReceipts creates a new memory pool for execution receipts.
func NewPendingReceipts(headers storage.Headers, limit uint) *PendingReceipts {
	// create the receipts memory pool with the lookup maps
	r := &PendingReceipts{
		Backend:            NewBackend(WithLimit(limit)),
		headers:            headers,
		byPreviousResultID: make(map[flow.Identifier]receiptsSet),
		byHeight:           make(map[uint64]receiptsSet),
	}
	// TODO: there is smarter eject exists. For instance:
	// if the mempool fills up, we want to eject the receipts for the highest blocks
	// See https://github.com/onflow/flow-go/pull/387/files#r574228078
	r.RegisterEjectionCallbacks(func(entity flow.Entity) {
		receipt := entity.(*flow.ExecutionReceipt)
		removeReceipt(receipt, r.backData, r.byPreviousResultID)
	})
	return r
}

func removeReceipt(
	receipt *flow.ExecutionReceipt,
	entities mempool.BackData,
	byPreviousResultID map[flow.Identifier]receiptsSet) {

	receiptID := receipt.ID()
	entities.Rem(receiptID)

	index := indexByPreviousResultID(receipt)
	siblings := byPreviousResultID[index]
	delete(siblings, receiptID)
	if len(siblings) == 0 {
		delete(byPreviousResultID, index)
	}
}

// Add adds an execution receipt to the mempool.
func (r *PendingReceipts) Add(receipt *flow.ExecutionReceipt) bool {
	added := false
	err := r.Backend.Run(func(entities map[flow.Identifier]flow.Entity) error {
		receiptID := receipt.ID()
		_, exists := entities[receiptID]
		if exists {
			// duplication
			return nil
		}

		height, err := r.indexByHeight(receipt)
		if err != nil {
			return err
		}

		// skip elements below the pruned
		if height < r.lowestHeight {
			return nil
		}

		entities[receiptID] = receipt

		// update index AND the backdata in one "transaction"
		previousResultID := indexByPreviousResultID(receipt)
		siblings, ok := r.byPreviousResultID[previousResultID]
		if !ok {
			siblings = make(receiptsSet)
			r.byPreviousResultID[previousResultID] = siblings
		}
		siblings[receiptID] = struct{}{}

		sameHeight, ok := r.byHeight[height]
		if !ok {
			sameHeight = make(receiptsSet)
			r.byHeight[height] = sameHeight
		}
		sameHeight[receiptID] = struct{}{}

		added = true
		return nil
	})
	if err != nil {
		panic(err)
	}

	return added
}

// Rem will remove a receipt by ID.
func (r *PendingReceipts) Rem(receiptID flow.Identifier) bool {
	removed := false
	err := r.Backend.Run(func(entities map[flow.Identifier]flow.Entity) error {
		entity, ok := entities[receiptID]
		if ok {
			receipt := entity.(*flow.ExecutionReceipt)
			removeReceipt(receipt, r.backData, r.byPreviousResultID)
			removed = true
		}
		return nil
	})
	if err != nil {
		panic(err)
	}
	return removed
}

// ByPreviousResultID returns receipts whose previous result ID matches the given ID
func (r *PendingReceipts) ByPreviousResultID(previousResultID flow.Identifier) []*flow.ExecutionReceipt {
	var receipts []*flow.ExecutionReceipt
	err := r.Backend.Run(func(entities map[flow.Identifier]flow.Entity) error {
		siblings, foundIndex := r.byPreviousResultID[previousResultID]
		if !foundIndex {
			return nil
		}
		for receiptID := range siblings {
			entity, ok := entities[receiptID]
			if !ok {
				return fmt.Errorf("inconsistent index. can not find entity by id: %v", receiptID)
			}
			receipt, ok := entity.(*flow.ExecutionReceipt)
			if !ok {
				return fmt.Errorf("could not convert entity to receipt: %v", receiptID)
			}
			receipts = append(receipts, receipt)
		}
		return nil
	})
	if err != nil {
		panic(err)
	}

	return receipts
}

// Size will return the total number of pending receipts
func (r *PendingReceipts) Size() uint {
	return r.Backend.Size()
}

// PruneUpToHeight remove all receipts for blocks whose height is strictly
// smaller that height. Note: receipts for blocks at height are retained.
// After pruning, receipts below for blocks below the given height are dropped.
//
// Monotonicity Requirement:
// The pruned height cannot decrease, as we cannot recover already pruned elements.
// If `height` is smaller than the previous value, the previous value is kept
// and the sentinel mempool.DecreasingPruningHeightError is returned.
func (r *PendingReceipts) PruneUpToHeight(height uint64) error {
	return r.Backend.Run(func(entities map[flow.Identifier]flow.Entity) error {
		if height < r.lowestHeight {
			return mempool.NewDecreasingPruningHeightErrorf(
				"pruning height: %d, existing height: %d", height, r.lowestHeight)
		}

		if len(entities) == 0 {
			r.lowestHeight = height
			return nil
		}
		// Optimization: if there are less height in the index than the height range to prune,
		// range to prune, then just go through each seal.
		// Otherwise, go through each height to prune.
		if uint64(len(r.byHeight)) < height-r.lowestHeight {
			for h := range r.byHeight {
				if h < height {
					r.removeByHeight(h, entities)
				}
			}
		} else {
			for h := r.lowestHeight; h < height; h++ {
				r.removeByHeight(h, entities)
			}
		}
		r.lowestHeight = height
		return nil
	})
}

func (r *PendingReceipts) removeByHeight(height uint64, entities map[flow.Identifier]flow.Entity) {
	for receiptID := range r.byHeight[height] {
		entity, ok := entities[receiptID]
		if ok {
			removeReceipt(entity.(*flow.ExecutionReceipt), r.backData, r.byPreviousResultID)
		}
	}
	delete(r.byHeight, height)
}
