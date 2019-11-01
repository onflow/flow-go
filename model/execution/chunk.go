package execution

import (
	"fmt"

	"github.com/dapperlabs/flow-go/pkg/crypto"
	"github.com/dapperlabs/flow-go/pkg/types"
	"github.com/ethereum/go-ethereum/rlp"
)

// Chunk is a collection of transactions, we assume Tx content correctness and orders in block has been verfied by Chunk builder
type Chunk struct {
	Transactions          []types.Transaction
	TotalGasSpent         uint64
	StartState            StateCommitment
	FirstTxInTheNextChunk *types.Transaction
}

func (c *Chunk) String() string {
	switch len(c.Transactions) {
	case 0:
		return fmt.Sprintf("An empty chunk")
	case 1:
		return fmt.Sprintf("Chunk %v includes a transaction (TotalGasSpent: %v):\n %v",
			c.Hash(), c.TotalGasSpent, c.Transactions[0].Hash())
	default:
		return fmt.Sprintf("Chunk %v includes %v transactions (TotalGasSpent: %v):\n %v\n ...\n %v  ", c.Hash(),
			len(c.Transactions), c.TotalGasSpent, c.Transactions[0].Hash(),
			c.Transactions[len(c.Transactions)-1].Hash())
	}
}

// CanonicalEncoding returns the encoded canonical chunk as bytes.
func (c *Chunk) CanonicalEncoding() []byte {
	var items []interface{}
	items = append(items, c.TotalGasSpent)
	items = append(items, c.StartState)
	if c.FirstTxInTheNextChunk != nil {
		items = append(items, c.FirstTxInTheNextChunk.Hash())
	}
	for _, tx := range c.Transactions {
		items = append(items, tx.Hash())
	}
	b, _ := rlp.EncodeToBytes(items)
	return b
}

// Hash generates hash for the given chunk
func (c *Chunk) Hash() crypto.Hash {
	hasher, _ := crypto.NewHasher(crypto.SHA3_256)
	return hasher.ComputeHash(c.CanonicalEncoding())
}
