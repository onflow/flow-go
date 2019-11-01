package chunking

// import (
// 	"errors"

// 	exec "github.com/dapperlabs/flow-go/pkg/model/execution"
// 	"github.com/dapperlabs/flow-go/pkg/types"
// )

// // ChunkVerifier verifies each individual chunk
// type ChunkVerifier interface {
// 	ChunkVerify(chunk exec.Chunk, vm exec.VirtualMachine) (verified bool, err error)
// }

// // ChunkVerify implements a chunk verifier
// func ChunkVerify(chunk exec.Chunk, vm exec.VirtualMachine, maxGasSpentPerChunk uint64) (verified bool, err error) {
// 	var totalGasSpent uint64
// 	var finalState types.StateCommitment
// 	for i, tx := range chunk.Transactions {
// 		// TODO we need to fetch Txs
// 		etx, err := vm.ExecuteTransaction(&tx, chunk.StartState)
// 		if err != nil {
// 			return false, errors.New("failed to execute a transaction")
// 		}
// 		// ensure first transaction gas spend is accurate
// 		if i == 0 {
// 			if chunk.T0 != etx.GasSpent {
// 				return false, nil
// 			}
// 		}
// 		totalGasSpent += etx.GasSpent
// 		// last step
// 		if i == len(chunk.Transactions)-1 {
// 			finalState = etx.EndState
// 		}
// 	}
// 	// assert computation consumption for entire chunk is correct
// 	if totalGasSpent != chunk.TotalGasSpent {
// 		return false, nil
// 	}
// 	// assert computation consumption does not exceed limit
// 	if totalGasSpent > maxGasSpentPerChunk {
// 		return false, nil
// 	}
// 	// assert chunk is full: no more translations can be appended to chunk
// 	// TODO handle case for chunk.T1, one option is to set it to maxGasLimit
// 	if totalGasSpent+chunk.FirstTxInTheNextChunk <= maxGasSpentPerChunk {
// 		return false, nil
// 	}
// 	// assert final state
// 	if finalState != chunk.FinalState {
// 		return false, nil
// 	}

// 	return true, nil
// }
