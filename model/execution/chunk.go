package execution

import (
	"fmt"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/encoding"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/hash"
	"github.com/dapperlabs/flow-go/storage"
)

// Chunk is a collection of transactions, we assume Tx content correctness and
// orders in block has been verified by Chunk builder.
type Chunk struct {
	Transactions                  []flow.Transaction
	TotalGasSpent                 uint64
	StartState                    storage.StateCommitment
	FinalState                    storage.StateCommitment
	FirstTxInTheNextChunk         *flow.Transaction
	FirstTxGasSpent               uint64 // T0
	FirstTxInTheNextChunkGasSpent uint64 // T'0
}

func (c *Chunk) String() string {
	switch len(c.Transactions) {
	case 0:
		return "An empty chunk"
	case 1:
		return fmt.Sprintf("Chunk %v includes a transaction (TotalGasSpent: %v):\n %v",
			c.Hash(), c.TotalGasSpent, c.Transactions[0].Hash())
	default:
		firstTx := c.Transactions[0]
		lastTx := c.Transactions[len(c.Transactions)-1]
		return fmt.Sprintf(
			"Chunk %v includes %v transactions (TotalGasSpent: %v):\n %v\n ...\n %v",
			c.Hash(),
			len(c.Transactions),
			c.TotalGasSpent,
			firstTx.Hash(),
			lastTx.Hash(),
		)
	}
}

// Hash returns the canonical hash of this chunk.
func (c *Chunk) Hash() crypto.Hash {
	return hash.DefaultHasher.ComputeHash(c.Encode())
}

// Encode returns the canonical encoding of this chunk.
func (c *Chunk) Encode() []byte {
	w := wrapChunk(*c)
	return encoding.DefaultEncoder.MustEncode(&w)
}

type chunkWrapper struct {
	Transactions    []flow.TransactionWrapper
	TotalGasSpent   uint64
	StartState      storage.StateCommitment
	FirstTxGasSpent uint64 // T0
}

func wrapChunk(c Chunk) chunkWrapper {
	transactions := make([]flow.TransactionWrapper, len(c.Transactions))
	for i, tx := range c.Transactions {
		transactions[i] = flow.WrapTransaction(tx)
	}
	return chunkWrapper{
		Transactions:    transactions,
		TotalGasSpent:   c.TotalGasSpent,
		StartState:      c.StartState,
		FirstTxGasSpent: c.FirstTxGasSpent}
}
