package internal

import (
	"fmt"
	"time"

	"github.com/onflow/flow-go/model/flow"
)

// DuplicateMessagesCounterEntity cache record that keeps track of the amount of duplicate messages received from a peer.
type DuplicateMessagesCounterEntity struct {
	// ID the entity ID.
	Id flow.Identifier
	// Gauge the number of duplicate messages.
	Gauge       float64
	lastUpdated time.Time
}

func NewDuplicateMessagesCounter(id flow.Identifier) DuplicateMessagesCounterEntity {
	return DuplicateMessagesCounterEntity{
		Id:          id,
		Gauge:       0.0,
		lastUpdated: time.Now(),
	}
}

var _ flow.Entity = (*DuplicateMessagesCounterEntity)(nil)

func (d DuplicateMessagesCounterEntity) ID() flow.Identifier {
	return d.Id
}

func (d DuplicateMessagesCounterEntity) Checksum() flow.Identifier {
	return d.Id
}

// duplicateMessagesCounter infers the DuplicateMessagesCounterEntity type from the flow entity provided, panics if the wrong type is encountered.
func duplicateMessagesCounter(entity flow.Entity) DuplicateMessagesCounterEntity {
	record, ok := entity.(DuplicateMessagesCounterEntity)
	if !ok {
		// sanity check
		// This should never happen, because the cache only contains DuplicateMessagesCounterEntity entities.
		panic(fmt.Sprintf("invalid entity type, expected DuplicateMessagesCounterEntity type, got: %T", entity))
	}
	return record
}
