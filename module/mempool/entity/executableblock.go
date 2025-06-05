package entity

import (
	"github.com/onflow/flow-go/model/flow"
)

// A complete collection contains the guarantee and the transactions.
// the guarantee is the hash of all the transactions. The execution node
// receives the guarantee from the block, and queries the transactions by
// the guarantee from the collection node.
// when receiving a collection from collection node, the execution node will
// update the Collection field of a CompleteCollection and make it complete.
type CompleteCollection struct {
	Guarantee  *flow.CollectionGuarantee
	Collection *flow.Collection
}

// ExecutableBlock represents a block that can be executed by the VM
//
// It assumes that the attached Block is immutable, so take care in not modifying or changing the inner
// *flow.Block, otherwise the struct will be in an inconsistent state. It requires the Block is immutable
// because it lazy loads the Block.ID() into the private blockID field, on the first call to ExecutableBlock.BlockID()
// All future calls to BlockID will not call Block.ID(), therefore if the Block changes, the blockID will not match the Block.
type ExecutableBlock struct {
	blockID             flow.Identifier
	Block               *flow.Block
	CompleteCollections map[flow.Identifier]*CompleteCollection // key is the collection ID.
	StartState          *flow.StateCommitment
	Executing           bool // flag used to indicate if block is being executed, to avoid re-execution
}

func (c CompleteCollection) IsCompleted() bool {
	return len(c.Collection.Transactions) > 0
}

// BlockID lazy loads the Block.ID() into the private blockID field on the first call, and returns
// the id field in all future calls
func (b *ExecutableBlock) BlockID() flow.Identifier {
	if b.blockID == flow.ZeroID {
		b.blockID = b.Block.ID()
	}
	return b.blockID
}

func (b *ExecutableBlock) Height() uint64 {
	return b.Block.Header.Height
}

func (b *ExecutableBlock) ParentID() flow.Identifier {
	return b.Block.Header.ParentID
}

func (b *ExecutableBlock) Collections() []*CompleteCollection {
	collections := make([]*CompleteCollection, len(b.Block.Payload.Guarantees))

	for i, cg := range b.Block.Payload.Guarantees {
		collections[i] = b.CompleteCollections[cg.CollectionID]
	}

	return collections
}

// CompleteCollectionAt returns a complete collection at the given index,
// if index out of range, nil will be returned
func (b *ExecutableBlock) CompleteCollectionAt(index int) *CompleteCollection {
	if index < 0 || index >= len(b.Block.Payload.Guarantees) {
		return nil
	}
	return b.CompleteCollections[b.Block.Payload.Guarantees[index].CollectionID]
}

// CollectionAt returns a collection at the given index,
// if index out of range, nil will be returned
func (b *ExecutableBlock) CollectionAt(index int) *flow.Collection {
	cc := b.CompleteCollectionAt(index)
	if cc == nil {
		return nil
	}
	return cc.Collection
}

// HasAllTransactions returns whether all the transactions for all collections
// in the block have been received.
func (b *ExecutableBlock) HasAllTransactions() bool {
	for _, collection := range b.Block.Payload.Guarantees {

		completeCollection, ok := b.CompleteCollections[collection.CollectionID]
		if ok && completeCollection.IsCompleted() {
			continue
		}
		return false
	}
	return true
}

// HasStartState returns whether the block has StartState, which
// indicates whether its parent has been executed.
func (b *ExecutableBlock) HasStartState() bool {
	return b.StartState != nil
}

// IsComplete returns whether all the data needed to executed the block are
// ready.
func (b *ExecutableBlock) IsComplete() bool {
	return b.HasAllTransactions() && b.HasStartState()
}
