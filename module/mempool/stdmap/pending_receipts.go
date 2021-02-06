package stdmap

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

// PendingReceipts stores pending receipts indexed by the id.
// It also maintains a secondary index on the previous result id, which is unique,
// in order to allow to find a receipt by the previous result id.
type PendingReceipts struct {
	*Backend
	// secondary index by parent result id, has to be unqiue
	byPreviousResultID map[flow.Identifier]flow.Identifier
}

func indexByPreviousResultID(receipt *flow.ExecutionReceipt) flow.Identifier {
	return receipt.ExecutionResult.PreviousResultID
}

// NewPendingReceipts creates a new memory pool for execution receipts.
func NewPendingReceipts(limit uint) *PendingReceipts {
	// create the receipts memory pool with the lookup maps
	r := &PendingReceipts{
		Backend:            NewBackend(WithLimit(limit)),
		byPreviousResultID: make(map[flow.Identifier]flow.Identifier),
	}
	r.RegisterEjectionCallbacks(func(entity flow.Entity) {
		receipt := entity.(*flow.ExecutionReceipt)
		previousResultID := indexByPreviousResultID(receipt)
		delete(r.byPreviousResultID, previousResultID)
	})
	return r
}

// Add adds an execution receipt to the mempool.
func (r *PendingReceipts) Add(receipt *flow.ExecutionReceipt) bool {
	previousResultID := indexByPreviousResultID(receipt)
	var exists bool
	err := r.Backend.Run(func(entities map[flow.Identifier]flow.Entity) error {
		_, exists = r.byPreviousResultID[previousResultID]
		if !exists {
			id := receipt.ID()
			// update index AND the backdata in one "transaction"
			r.byPreviousResultID[previousResultID] = id
			entities[id] = receipt
			exists = true
		}
		return nil
	})
	if err != nil {
		panic(err)
	}

	return exists
}

// Rem will remove a receipt by ID.
func (r *PendingReceipts) Rem(receiptID flow.Identifier) bool {
	exists := false
	err := r.Backend.Run(func(entities map[flow.Identifier]flow.Entity) error {
		entity, ok := entities[receiptID]
		if ok {
			receipt := entity.(*flow.ExecutionReceipt)
			delete(entities, receiptID)
			delete(r.byPreviousResultID, indexByPreviousResultID(receipt))
			exists = true
		}
		return nil
	})
	if err != nil {
		panic(err)
	}
	return exists
}

// ByPreviousResultID returns a receipt whose previous result ID matches the given ID
func (r *PendingReceipts) ByPreviousResultID(previousReusltID flow.Identifier) (*flow.ExecutionReceipt, bool) {
	var receipt *flow.ExecutionReceipt
	var found bool
	err := r.Backend.Run(func(entities map[flow.Identifier]flow.Entity) error {
		receiptID, foundIndex := r.byPreviousResultID[previousReusltID]
		if foundIndex {
			entity, ok := entities[receiptID]
			if !ok {
				return fmt.Errorf("inconsistent index. can not find entity by id: %v", receiptID)
			}

			receipt = entity.(*flow.ExecutionReceipt)
			found = true
		}
		return nil
	})

	if err != nil {
		panic(err)
	}

	if found {
		return receipt, true
	}
	return nil, false
}

// Size will return the total number of pending receipts
func (r *PendingReceipts) Size() uint {
	return r.Backend.Size()
}
