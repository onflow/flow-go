package chunking

import (
	"bytes"
	"errors"
	"fmt"

	exec "github.com/dapperlabs/flow-go/model/execution"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage"
)

// Chunker converts executed transactions into chunks
type Chunker interface {
	GetChunks(Txs []exec.ExecutedTransaction, maxGasSpentPerChunk uint64) ([]exec.Chunk, error)
}

// ChunkVerifier verifies each individual chunk
type ChunkVerifier interface {
	ChunkVerify(chunk exec.Chunk, vm exec.VirtualMachine) (verified bool, err error)
}

// GetChunks returns an array of chunks given an slice of ExecutedTransaction (greedy chunker)
func GetChunks(Txs []exec.ExecutedTransaction, maxGasSpentPerChunk uint64) ([]exec.Chunk, error) {
	var totalGasSpent uint64
	var firstTxGasSpent uint64
	var startState storage.StateCommitment
	var finalState storage.StateCommitment
	var activeTxs []flow.Transaction
	var chunks []exec.Chunk
	for _, tx := range Txs {
		if tx.GasSpent > maxGasSpentPerChunk {
			message := fmt.Sprintf("maxGasSpentInAChunk is too small. A transaction found with higher GasSpent (%d) than maxGasSpentInAChunk (%d)", tx.GasSpent, maxGasSpentPerChunk)
			return nil, errors.New(message)
		}
		// if adding tx would overflow the chunk
		if totalGasSpent+tx.GasSpent > maxGasSpentPerChunk {
			chunks = append(chunks, exec.Chunk{Transactions: activeTxs,
				TotalGasSpent:                 totalGasSpent,
				FirstTxInTheNextChunk:         tx.Tx,
				FirstTxGasSpent:               firstTxGasSpent,
				FirstTxInTheNextChunkGasSpent: tx.GasSpent,
				StartState:                    startState,
				FinalState:                    tx.StartState,
			})
			activeTxs = make([]flow.Transaction, 0)
			totalGasSpent = 0
			firstTxGasSpent = tx.GasSpent
			startState = tx.StartState
		}
		activeTxs = append(activeTxs, *tx.Tx)
		finalState = tx.EndState
		totalGasSpent += tx.GasSpent
	}
	// complete last chunk
	if len(activeTxs) > 0 {
		chunks = append(chunks, exec.Chunk{Transactions: activeTxs,
			TotalGasSpent:                 totalGasSpent,
			FirstTxGasSpent:               firstTxGasSpent,
			FirstTxInTheNextChunkGasSpent: maxGasSpentPerChunk,
			StartState:                    startState,
			FinalState:                    finalState,
		})
	}
	return chunks, nil
}

// ChunkVerify implements a chunk verifier
func ChunkVerify(chunk exec.Chunk, vm exec.VirtualMachine, maxGasSpentPerChunk uint64) (verified bool, err error) {
	var totalGasSpent uint64
	var finalState storage.StateCommitment = chunk.StartState
	for i, tx := range chunk.Transactions {
		etx, err := vm.ExecuteTransaction(&tx, finalState)
		if err != nil {
			return false, errors.New("failed to execute a transaction")
		}
		// ensure first transaction gas spend is accurate
		if i == 0 && chunk.FirstTxGasSpent != etx.GasSpent {
			return false, nil
		}
		totalGasSpent += etx.GasSpent
		// last step
		finalState = etx.EndState
	}
	// assert computation consumption for entire chunk is correct
	if totalGasSpent != chunk.TotalGasSpent {
		return false, nil
	}
	// assert computation consumption does not exceed limit
	if totalGasSpent > maxGasSpentPerChunk {
		return false, nil
	}
	// assert chunk is full: no more translations can be appended to chunk
	if totalGasSpent+chunk.FirstTxInTheNextChunkGasSpent <= maxGasSpentPerChunk {
		return false, nil
	}
	// assert final state
	if !bytes.Equal(finalState, chunk.FinalState) {
		return false, nil
	}

	return true, nil
}
