package entity

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

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
	CompleteCollections map[flow.Identifier]*CompleteCollection
	StartState          flow.StateCommitment
}

type BlocksByCollection struct {
	CollectionID     flow.Identifier
	ExecutableBlocks map[flow.Identifier]*ExecutableBlock
}

func (c *CompleteCollection) Collection() flow.Collection {
	return flow.Collection{Transactions: c.Transactions}
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

func (b *ExecutableBlock) HasAllTransactions() bool {
	for _, collection := range b.Block.Payload.Guarantees {

		completeCollection, ok := b.CompleteCollections[collection.ID()]
		if ok && completeCollection.Transactions != nil {
			continue
		}
		return false
	}
	return true
}

func (b *ExecutableBlock) IsComplete() bool {
	return b.HasAllTransactions() && len(b.StartState) > 0
}
