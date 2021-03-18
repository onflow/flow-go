package entity

import (
	"github.com/onflow/flow-go/model/flow"
)

// A complete collection contains the guarantee and the transactions.
// the guarantee is the hash of all the transactions. The execution node
// receives the guarantee from the block, and queries the transactions by
// the guarantee from the collection node.
// when receiving a collection from collection node, the execution node will
// update the Transactions field of a CompleteCollection and make it complete.
type CompleteCollection struct {
	Guarantee    *flow.CollectionGuarantee
	Transactions []*flow.TransactionBody
}

// ExecutableBlock represents a block that can be executed by the VM
//
// It assumes that the Block attached is immutable, so take care in not modifying or changing the inner
// *flow.Block, otherwise the struct will be in an inconsistent state. It requires the Block is immutable
// because the it lazy lodas the Block.ID() into the private id field, on the first call to ExecutableBlock.ID()
// All future calls to ID will not call Block.ID(), therefore it Block changes, the id will not match the Block.
type ExecutableBlock struct {
	id                  flow.Identifier
	Block               *flow.Block
	CompleteCollections map[flow.Identifier]*CompleteCollection // key is the collection ID.
	StartState          flow.StateCommitment
}

// BlocksByCollection represents a collection that the execution node
// has not received its transactions yet.
// it also holds references to the blocks that contains this collection
// and are waiting to be executed.
type BlocksByCollection struct {
	CollectionID flow.Identifier
	// a reversed map to look up which block contains this collection. key is the collection id
	ExecutableBlocks map[flow.Identifier]*ExecutableBlock
}

func (c CompleteCollection) Collection() flow.Collection {
	return flow.Collection{Transactions: c.Transactions}
}

func (c CompleteCollection) IsCompleted() bool {
	return len(c.Transactions) > 0
}

func (b *BlocksByCollection) ID() flow.Identifier {
	return b.CollectionID
}

func (b *BlocksByCollection) Checksum() flow.Identifier {
	return b.CollectionID
}

// ID lazy loads the Block.ID() into the private id field on the first call, and returns
// the id field in all future calls
func (b *ExecutableBlock) ID() flow.Identifier {
	if b.id == flow.ZeroID {
		b.id = b.Block.ID()
	}
	return b.id
}

func (b *ExecutableBlock) Checksum() flow.Identifier {
	return b.Block.Checksum()
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
		collections[i] = b.CompleteCollections[cg.ID()]
	}

	return collections
}

// HasAllTransactions returns whether all the transactions for all collections
// in the block have been received.
func (b *ExecutableBlock) HasAllTransactions() bool {
	for _, collection := range b.Block.Payload.Guarantees {

		completeCollection, ok := b.CompleteCollections[collection.ID()]
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
	return b.StartState != flow.EmptyStateCommitment
}

// IsComplete returns whether all the data needed to executed the block are
// ready.
func (b *ExecutableBlock) IsComplete() bool {
	return b.HasAllTransactions() && b.HasStartState()
}
