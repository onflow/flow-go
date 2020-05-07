// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package stdmap

import (
	"sync"

	"github.com/dapperlabs/flow-go/model/flow"
)

// Receipts implements the execution receipts memory pool of the consensus node,
// used to store execution receipts and to generate block seals.
type Receipts struct {
	sync.Mutex
	*Backend
	byBlock map[flow.Identifier](map[flow.Identifier]struct{})
}

// NewReceipts creates a new memory pool for execution receipts.
func NewReceipts(limit uint) (*Receipts, error) {

	// create the receipts memory pool with the lookup maps
	r := &Receipts{
		byBlock: make(map[flow.Identifier](map[flow.Identifier]struct{})),
	}

	// create the hook to clean up lookups upon removal of items from backend
	eject := func(entities map[flow.Identifier]flow.Entity) (flow.Identifier, flow.Entity) {
		entityID, entity := EjectTrueRandom(entities)
		receipt := entity.(*flow.ExecutionReceipt)
		r.cleanup(entityID, receipt)
		return entityID, entity
	}

	// initialize the backend with the given hook
	r.Backend = NewBackend(
		WithLimit(limit),
		WithEject(eject),
	)

	return r, nil
}

// Add adds an execution receipt to the mempool.
func (r *Receipts) Add(receipt *flow.ExecutionReceipt) error {
	r.register(receipt)
	err := r.Backend.Add(receipt)
	if err != nil {
		return err
	}
	return nil
}

// Rem will remove a receipt by ID.
func (r *Receipts) Rem(receiptID flow.Identifier) bool {
	entity, err := r.Backend.ByID(receiptID)
	if err != nil {
		return false
	}
	_ = r.Backend.Rem(receiptID)
	receipt := entity.(*flow.ExecutionReceipt)
	r.cleanup(receiptID, receipt)
	return true
}

// ByBlockID returns an execution receipt by approval ID.
func (r *Receipts) ByBlockID(blockID flow.Identifier) []*flow.ExecutionReceipt {
	byBlock, ok := r.byBlock[blockID]
	if !ok {
		return nil
	}
	receipts := make([]*flow.ExecutionReceipt, 0, len(byBlock))
	for receiptID := range byBlock {
		entity, err := r.Backend.ByID(receiptID)
		if err != nil || entity == nil {
			continue
		}
		receipts = append(receipts, entity.(*flow.ExecutionReceipt))
	}
	return receipts
}

// DropForBlock drops all execution receipts for the given block.
func (r *Receipts) DropForBlock(blockID flow.Identifier) {
	byBlock, ok := r.byBlock[blockID]
	if !ok {
		return
	}
	for receiptID := range byBlock {
		_ = r.Backend.Rem(receiptID)
	}
	delete(r.byBlock, blockID)
}

// All will return all execution receipts in the memory pool.
func (r *Receipts) All() []*flow.ExecutionReceipt {
	entities := r.Backend.All()
	receipts := make([]*flow.ExecutionReceipt, 0, len(entities))
	for _, entity := range entities {
		receipts = append(receipts, entity.(*flow.ExecutionReceipt))
	}
	return receipts
}

// register will add the given receipt to the lookup maps.
func (r *Receipts) register(receipt *flow.ExecutionReceipt) {
	r.Lock()
	defer r.Unlock()
	blockID := receipt.ExecutionResult.BlockID
	forBlock, ok := r.byBlock[blockID]
	if !ok {
		forBlock = make(map[flow.Identifier]struct{})
		r.byBlock[blockID] = forBlock
	}
	forBlock[receipt.ID()] = struct{}{}
}

// cleanup will remove the given receipt from the lookup tables.
func (r *Receipts) cleanup(receiptID flow.Identifier, receipt *flow.ExecutionReceipt) {
	r.Lock()
	defer r.Unlock()
	blockID := receipt.ExecutionResult.BlockID
	forBlock := r.byBlock[blockID]
	delete(forBlock, receiptID)
	if len(forBlock) > 0 {
		return
	}
	delete(r.byBlock, blockID)
}
