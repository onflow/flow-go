package storage

import (
	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
)

// ClusterPayloads handles storing and retrieving payloads for collection
// node cluster consensus.
type ClusterPayloads interface {

	// Store stores and indexes the given cluster payload.
	Store(blockID flow.Identifier, payload *cluster.Payload) error

	// ByBlockID returns the cluster payload for the given block ID.
	ByBlockID(blockID flow.Identifier) (*cluster.Payload, error)
}
