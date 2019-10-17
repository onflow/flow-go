package types

import (
	"fmt"

	"github.com/dapperlabs/flow-go/pkg/crypto"
)

// Chunk is a container of transactions with total compute limit less than chunkTotalGasLimit
type Chunk struct {
	Transactions          *[]Transaction
	TotalComputationLimit uint64
}

func (c *Chunk) String() string {
	if c.Transactions == nil {
		return fmt.Sprintf("An empty chunk")
	}
	if len(*c.Transactions) < 2 {
		return fmt.Sprintf("Chunk %v includes a transaction (totalComputationLimit: %v):\n %v",
			c.Hash(), c.TotalComputationLimit, (*c.Transactions)[0].Hash())

	}
	return fmt.Sprintf("Chunk %v includes %v transactions (totalComputationLimit: %v):\n %v\n ...\n %v  ", c.Hash(),
		len(*c.Transactions), c.TotalComputationLimit, (*c.Transactions)[0].Hash(),
		(*c.Transactions)[len(*c.Transactions)-1].Hash())
}

// Hash returns sha3 hash for the given chunk
func (c *Chunk) Hash() crypto.Hash {
	hasher, _ := crypto.NewHasher(crypto.SHA3_256)
	var txData = []byte("Chunk")
	for _, tx := range *c.Transactions {
		txData = append(txData, tx.CanonicalEncoding()...)
	}
	return hasher.ComputeHash(txData)
}

// Chunker knows how to group a collection of transactions into chunks
type Chunker interface {
	CollectionToChunks(collection *Collection, maxComputationLimit uint64) (*[]Chunk, error)
}
