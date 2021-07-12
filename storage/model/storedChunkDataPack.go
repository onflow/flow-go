package storagemodel

import (
	"github.com/onflow/flow-go/model/flow"
)

// StoredChunkDataPack is an internal representation of chunk data pack specific to the execution node to be stored on persistent storage.
type StoredChunkDataPack struct {
	ChunkID      flow.Identifier
	StartState   flow.StateCommitment
	Proof        flow.StorageProof
	CollectionID flow.Identifier
}
