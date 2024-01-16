package internal

import (
	"fmt"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/onflow/flow-go/model/flow"
)

// duplicateMessagesCounterEntity cache record that keeps track of the amount of duplicate messages received from a peer.
type duplicateMessagesCounterEntity struct {
	// ID the entity ID.
	Id flow.Identifier
	// Value the number of duplicate messages.
	Value       float64
	lastUpdated time.Time
}

func NewDuplicateMessagesCounter(id flow.Identifier) duplicateMessagesCounterEntity {
	return duplicateMessagesCounterEntity{
		Id:          id,
		Value:       0.0,
		lastUpdated: time.Now(),
	}
}

var _ flow.Entity = (*duplicateMessagesCounterEntity)(nil)

func (d duplicateMessagesCounterEntity) ID() flow.Identifier {
	return d.Id
}

func (d duplicateMessagesCounterEntity) Checksum() flow.Identifier {
	return d.Id
}

// mustBeDuplicateMessagesCounterEntity is a helper function for type assertion of the flow.Entity to duplicateMessagesCounterEntity.
// It panics if the type assertion fails.
// Args:
// - entity: the flow.Entity to be type asserted.
// Returns:
// - the duplicateMessagesCounterEntity entity.
func mustBeDuplicateMessagesCounterEntity(entity flow.Entity) duplicateMessagesCounterEntity {
	c, ok := entity.(duplicateMessagesCounterEntity)
	if !ok {
		// sanity check
		// This should never happen, because the cache only contains duplicateMessagesCounterEntity entities.
		panic(fmt.Sprintf("invalid entity type, expected duplicateMessagesCounterEntity type, got: %T", entity))
	}
	return c
}

// makeId is a helper function for creating the id field of the duplicateMessagesCounterEntity by hashing the peerID.
// Returns:
// - the hash of the peerID as a flow.Identifier.
func makeId(peerID peer.ID) flow.Identifier {
	return flow.MakeID([]byte(peerID))
}
