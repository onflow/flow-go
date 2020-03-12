package execution

import (
	"github.com/dapperlabs/flow-go/engine/execution/state"
	"github.com/dapperlabs/flow-go/model/flow"
)

type CompleteCollection struct {
	Guarantee    *flow.CollectionGuarantee
	Transactions []*flow.TransactionBody
}

// TODO If the executor will be a separate process/machine we would need to rework
// sending view as local data, but that would be much greater refactor of storage anyway

type ComputationOrder struct {
	Block      *CompleteBlock
	View       *state.View
	StartState flow.StateCommitment
}

type CompleteBlock struct {
	Block               *flow.Block
	CompleteCollections map[flow.Identifier]*CompleteCollection
}

type ComputationResult struct {
	CompleteBlock *CompleteBlock
	StateViews    []*state.View
}

func (b *CompleteBlock) Collections() []*CompleteCollection {
	collections := make([]*CompleteCollection, len(b.Block.Guarantees))

	for i, cg := range b.Block.Guarantees {
		collections[i] = b.CompleteCollections[cg.ID()]
	}

	return collections
}

func (b *CompleteBlock) IsComplete() bool {
	for _, collection := range b.Block.Guarantees {

		completeCollection, ok := b.CompleteCollections[collection.ID()]
		if ok && completeCollection.Transactions != nil {
			continue
		}
		return false
	}

	return true
}
