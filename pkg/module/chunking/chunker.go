package chunking

import (
	"errors"
	"fmt"

	exec "github.com/dapperlabs/flow-go/pkg/model/execution"
	"github.com/dapperlabs/flow-go/pkg/types"
)

// Chunker converts executed transactions into chunks
type Chunker interface {
	GetChunks(Txs []exec.ExecutedTransaction, maxGasSpentPerChunk uint64) ([]exec.Chunk, error)
}

// GetChunks returns an array of chunks given an slice of ExecutedTransaction (greedy chunker)
func GetChunks(Txs []exec.ExecutedTransaction, maxGasSpentPerChunk uint64) ([]exec.Chunk, error) {
	var totalGasSpent uint64
	var activeTxs []types.Transaction
	var chunks []exec.Chunk
	for _, tx := range Txs {
		if tx.GasSpent > maxGasSpentPerChunk {
			message := fmt.Sprintf("maxGasSpentInAChunk is too small. A transaction found with higher GasSpent (%d) than maxGasSpentInAChunk (%d)", tx.GasSpent, maxGasSpentPerChunk)
			return nil, errors.New(message)
		}
		// if adding tx would overflow the chunk
		if totalGasSpent+tx.GasSpent > maxGasSpentPerChunk {
			chunks = append(chunks, exec.Chunk{Transactions: activeTxs, TotalGasSpent: totalGasSpent, FirstTxInTheNextChunk: tx.Tx})
			activeTxs = make([]types.Transaction, 0)
			totalGasSpent = 0
		}
		activeTxs = append(activeTxs, *tx.Tx)
		totalGasSpent += tx.GasSpent
	}
	// complete last chunk
	if len(activeTxs) > 0 {
		chunks = append(chunks, exec.Chunk{Transactions: activeTxs, TotalGasSpent: totalGasSpent})
	}
	return chunks, nil
}
