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

// NonCompliantHeaderHeightError is returned when a new header is pushed to the queue
// that is not compliant with the previously added header because of height mismatch
type NonCompliantHeaderHeightError struct {
	ExpectedBlockHeight, ReceivedBlockHeight uint64
}

func (e NonCompliantHeaderHeightError) Error() string {
	return fmt.Sprintf(
		"non complient header received: expected height: %d, received height: %d",
		e.ExpectedBlockHeight,
		e.ReceivedBlockHeight,
	)
}

// NonCompliantHeaderParentIDError is returned when a new header is pushed to the queue
// that is not compliant with the previously added header because of parent header ID mismatch
type NonCompliantHeaderParentIDError struct {
	ExpectedParentBlockID, ReceivedParentBlockID flow.Identifier
}

func (e NonCompliantHeaderParentIDError) Error() string {
	return fmt.Sprintf(
		"non complient header received: expected parent ID: %x, received parent ID: %x",
		e.ExpectedParentBlockID,
		e.ReceivedParentBlockID,
	)
}

// NonCompliantHeaderAlreadyProcessedError is returned when a new header is pushed to the queue
// that is already processed in the past
type NonCompliantHeaderAlreadyProcessedError struct {
	receivedBlockHeight uint64
}

func (e NonCompliantHeaderAlreadyProcessedError) Error() string {
	return fmt.Sprintf(
		"non complient header received: block height is in the past and most likeley has already been processed: %d",
		e.receivedBlockHeight,
	)
}
