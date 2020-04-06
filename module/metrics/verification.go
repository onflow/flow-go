package metrics

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// VerificationMetrics implements an interface of event handlers that are called upon certain events to capture
// metrics for sake of monitoring
type VerificationMetrics interface {
	// OnChunkVerificationStated is called whenever the verification of a chunk is started
	// it starts the timer to record the execution time
	OnChunkVerificationStated(chunkID flow.Identifier)
	// OnChunkVerificationFinished is called whenever chunkID verification gets finished
	// it finishes recording the duration of execution and increases number of checked chunks for the blockID
	OnChunkVerificationFinished(chunkID flow.Identifier, blockID flow.Identifier)

	// OnResultApproval is called whenever a result approval for block ID is emitted
	// it increases the result approval counter for this chunk
	OnResultApproval(blockID flow.Identifier)

	// OnStorageAdded is called whenever something is added to the persistent (on disk) storage
	// of verification node. It records the size of stored object.
	OnStorageAdded(size float64)

	// OnStorageAdded is called whenever something is removed from the persistent (on disk) storage
	// of verification node. It records the size of stored object.
	OnStorageRemoved(size float64)

	// OnChunkDataAdded is called whenever something is added to related to chunkID to the in-memory mempools
	// of verification node. It records the size of stored object.
	OnChunkDataAdded(chunkID flow.Identifier, size float64)

	// OnChunkDataRemoved is called whenever something is removed that is related to chunkID from the in-memory mempools
	// of verification node. It records the size of stored object.
	OnChunkDataRemoved(chunkID flow.Identifier, size float64)
}
