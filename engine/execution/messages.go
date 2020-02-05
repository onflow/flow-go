package execution

import (
	"github.com/dapperlabs/flow-go/engine/execution/execution/state"
	"github.com/dapperlabs/flow-go/model/flow"
)

type CompleteCollection struct {
	Guarantee    *flow.CollectionGuarantee
	Transactions []*flow.TransactionBody
}

// TODO If the executor will be a separate process/machine we would need to rework
// sending view as local data, but that would be much greater refactor of storage anyway

type ExecutionOrder struct {
	Block        *CompleteBlock
	View *state.View
}

type CompleteBlock struct {
	Block               *flow.Block
	CompleteCollections map[flow.Identifier]*CompleteCollection
}

func (b *CompleteBlock) Collections() []*CompleteCollection {
	collections := make([]*CompleteCollection, len(b.Block.Guarantees))

	for i, cg := range b.Block.Guarantees {
		collections[i] = b.CompleteCollections[cg.ID()]
	}

	return collections
}
