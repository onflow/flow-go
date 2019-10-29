// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package flow

import (
	"fmt"

	"github.com/dapperlabs/flow-go/crypto"
)

// Chunk is a container of transactions with total compute limit less than chunkTotalGasLimit
type Chunk struct {
	Hash                  crypto.Hash
	Transactions          []*Transaction
	TotalComputationLimit uint64
}

func (c *Chunk) String() string {
	switch len(c.Transactions) {
	case 0:
		return fmt.Sprintf("An empty chunk")
	case 1:
		return fmt.Sprintf("Chunk %v includes a transaction (totalComputationLimit: %v):\n %v",
			c.Hash, c.TotalComputationLimit, c.Transactions[0].Hash)
	default:
		return fmt.Sprintf("Chunk %v includes %v transactions (totalComputationLimit: %v):\n %v\n ...\n %v  ", c.Hash,
			len(c.Transactions), c.TotalComputationLimit, c.Transactions[0].Hash,
			c.Transactions[len(c.Transactions)-1].Hash)
	}
}

// NewChunk creates a new chunk
func NewChunk(transactions []*Transaction, totalComputationLimit uint64) *Chunk {
	return &Chunk{
		Transactions:          transactions,
		TotalComputationLimit: totalComputationLimit,
	}
}

// Chunker knows how to group a collection of transactions into chunks
type Chunker interface {
	CollectionToChunks(collection *Collection, maxComputationLimit uint64) (*[]Chunk, error)
}
