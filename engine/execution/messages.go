package execution

import "github.com/dapperlabs/flow-go/model/flow"

type CompleteCollection struct {
	Guarantee    *flow.CollectionGuarantee
	Transactions []*flow.TransactionBody
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

// StateRequest represents a request for the portion of execution state
// used by all the transactions in the chunk specified by the chunk ID.
type StateRequest struct {
	ChunkID flow.Identifier
}

// StateResponse is the response to a state request. It includes all the
// registers as a map required for the requested chunk.
type StateResponse struct {
	ChunkID   flow.Identifier
	Registers map[string][]byte
}
