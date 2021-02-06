package stdmap

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

// PendingReceipts stores pending receipts indexed by the id.
// It also maintains a secondary index on the previous result id.
// in order to allow to find receipts by the previous result id.
type PendingReceipts struct {
	*Backend
	// secondary index by parent result id, since multiple receipts could
	// have the same parent result, (even if they have different result)
	byPreviousResultID map[flow.Identifier][]flow.Identifier
}

func indexByPreviousResultID(receipt *flow.ExecutionReceipt) flow.Identifier {
	return receipt.ExecutionResult.PreviousResultID
}

// NewPendingReceipts creates a new memory pool for execution receipts.
func NewPendingReceipts(limit uint) *PendingReceipts {
	// create the receipts memory pool with the lookup maps
	r := &PendingReceipts{
		Backend:            NewBackend(WithLimit(limit)),
		byPreviousResultID: make(map[flow.Identifier][]flow.Identifier),
	}
	r.RegisterEjectionCallbacks(func(entity flow.Entity) {
		receipt := entity.(*flow.ExecutionReceipt)
		removeReceipt(receipt, r.entities, r.byPreviousResultID)
	})
	return r
}

func removeReceipt(
	receipt *flow.ExecutionReceipt,
	entities map[flow.Identifier]flow.Entity,
	byPreviousResultID map[flow.Identifier][]flow.Identifier) {

	receiptID := receipt.ID()
	delete(entities, receiptID)

	index := indexByPreviousResultID(receipt)
	siblings := byPreviousResultID[index]
	newsiblings := make([]flow.Identifier, 0, len(siblings))
	for _, sibling := range siblings {
		if sibling != receiptID {
			newsiblings = append(newsiblings, sibling)
		}
	}
	if len(newsiblings) == 0 {
		delete(byPreviousResultID, index)
	} else {
		byPreviousResultID[index] = newsiblings
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

		entities[receiptID] = receipt

		// update index AND the backdata in one "transaction"
		previousResultID := indexByPreviousResultID(receipt)
		siblings, ok := r.byPreviousResultID[previousResultID]
		if !ok {
			r.byPreviousResultID[previousResultID] = []flow.Identifier{receiptID}
		} else {
			r.byPreviousResultID[previousResultID] = append(siblings, receiptID)
		}
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
			removeReceipt(receipt, r.entities, r.byPreviousResultID)
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
func (r *PendingReceipts) ByPreviousResultID(previousReusltID flow.Identifier) []*flow.ExecutionReceipt {
	var receipts []*flow.ExecutionReceipt
	err := r.Backend.Run(func(entities map[flow.Identifier]flow.Entity) error {
		siblings, foundIndex := r.byPreviousResultID[previousReusltID]
		if foundIndex {
			for _, receiptID := range siblings {
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
