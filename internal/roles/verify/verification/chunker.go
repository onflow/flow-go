package verification

import (
	"errors"
	"fmt"

	"github.com/dapperlabs/flow-go/pkg/types"
)

// CollectionToChunks returns an array of chunks given a collection of transactions
func CollectionToChunks(collection *types.Collection, maxComputationLimit uint64) ([]types.Chunk, error) {
	var totalComputationLimit uint64
	var activeTxs []*types.Transaction
	chunks := []types.Chunk{}
	for _, tx := range collection.Transactions {
		if tx.ComputeLimit > maxComputationLimit {
			message := fmt.Sprintf("maxComputationLimit is too small. A transaction found with higher computeLimit (%d) than maxComputationLimit (%d)", tx.ComputeLimit, maxComputationLimit)
			return nil, errors.New(message)
		}
		// if adding tx would overflow the chunk
		if totalComputationLimit+tx.ComputeLimit > maxComputationLimit {
			chunks = append(chunks, types.Chunk{Transactions: activeTxs, TotalComputationLimit: totalComputationLimit})
			activeTxs = make([]*types.Transaction, 0)
			totalComputationLimit = 0
		}
		activeTxs = append(activeTxs, tx)
		totalComputationLimit += tx.ComputeLimit
	}
	// complete last chunk
	if len(activeTxs) >= 0 {
		chunks = append(chunks, types.Chunk{Transactions: activeTxs, TotalComputationLimit: totalComputationLimit})
	}
	return chunks, nil
}
