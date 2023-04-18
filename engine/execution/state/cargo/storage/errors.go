package storage

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

// HeightNotAvailableError is returned when request for the given height is not available
type HeightNotAvailableError struct {
	requestedHeight,
	minHeightAvailable,
	maxHeightAvailable uint64
}

func (e HeightNotAvailableError) Error() string {
	return fmt.Sprintf(
		"requested height is not available: requested: %d available range (%d, %d)",
		e.requestedHeight,
		e.minHeightAvailable,
		e.maxHeightAvailable,
	)
}

// NonCompliantHeaderError is returned when a new commit is made but its header is
// not compliant with the previously commit's header because of height or parent id mismatch
type NonCompliantHeaderError struct {
	ExpectedBlockHeight, ReceivedBlockHeight     uint64
	ExpectedParentBlockID, ReceivedParentBlockID flow.Identifier
}

func (e NonCompliantHeaderError) Error() string {
	return fmt.Sprintf(
		`commit with a non-compliant header received:
		   expected height: %d, received height: %d",
		   expected parent ID: %x, received parent ID: %x`,
		e.ExpectedBlockHeight,
		e.ReceivedBlockHeight,
		e.ExpectedParentBlockID,
		e.ReceivedParentBlockID,
	)
}
