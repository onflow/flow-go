package queue

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

// QueueCapacityReachedError is returned when finalized block queue has reached its capacity
type QueueCapacityReachedError struct {
	Capacity int
}

func (e QueueCapacityReachedError) Error() string {
	return fmt.Sprintf("finalized block queue has reached its capacity: %d", e.Capacity)
}

// NonCompliantHeaderError is returned when a new header is pushed to the queue
// that is not compliant with the previously added header because of height or parent id mismatch
type NonCompliantHeaderError struct {
	ExpectedBlockHeight, ReceivedBlockHeight     uint64
	ExpectedParentBlockID, ReceivedParentBlockID flow.Identifier
}

func (e NonCompliantHeaderError) Error() string {
	return fmt.Sprintf(
		`non-compliant header received:
		   expected height: %d, received height: %d",
		   expected parent ID: %x, received parent ID: %x`,
		e.ExpectedBlockHeight,
		e.ReceivedBlockHeight,
		e.ExpectedParentBlockID,
		e.ReceivedParentBlockID,
	)
}
