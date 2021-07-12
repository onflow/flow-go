package execution

import (
	"github.com/onflow/flow-go/model/flow"
)

// storedChunkDataPack is an internal representation of chunk data pack specific to the execution node to be stored on persistent storage.
type storedChunkDataPack struct {
	ChunkID      flow.Identifier
	StartState   flow.StateCommitment
	Proof        flow.StorageProof
	CollectionID flow.Identifier
}
