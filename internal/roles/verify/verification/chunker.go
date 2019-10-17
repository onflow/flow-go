package verification

import (
	"errors"
	"fmt"

	"github.com/dapperlabs/flow-go/pkg/types"
)

const maxComputationLimit = uint64(400)

// CollectionToChunks returns an array of chunks given a collection of transactions
func CollectionToChunks(collection *types.Collection, maxComputationLimit uint64) (*[]types.Chunk, error) {
	totalComputationLimit := uint64(0)
	activeTxs := []types.Transaction{}
	chunks := []types.Chunk{}
	if collection == nil {
		return nil, errors.New("Collection is nil. Can't be broken into chunks")
	}
	for _, tx := range *collection.Transactions {
		if tx.ComputeLimit > maxComputationLimit {
			message := fmt.Sprintf("maxComputationLimit is too small. A transaction found with higher computeLimit (%d) than maxComputationLimit (%d)", tx.ComputeLimit, maxComputationLimit)
			return nil, errors.New(message)
		}
		// if adding tx would overflow the chunk
		if totalComputationLimit+tx.ComputeLimit > maxComputationLimit {
			chunkTxs := make([]types.Transaction, len(activeTxs))
			copy(chunkTxs, activeTxs)
			chunks = append(chunks, types.Chunk{Transactions: &chunkTxs, TotalComputationLimit: totalComputationLimit})
			activeTxs = []types.Transaction{}
			totalComputationLimit = 0
		}
		activeTxs = append(activeTxs, tx)
		totalComputationLimit += tx.ComputeLimit
	}
	// complete last chunk
	if activeTxs != nil && len(activeTxs) >= 0 {
		chunkTxs := make([]types.Transaction, len(activeTxs))
		copy(chunkTxs, activeTxs)
		chunks = append(chunks, types.Chunk{Transactions: &chunkTxs, TotalComputationLimit: totalComputationLimit})
	}
	return &chunks, nil
}
