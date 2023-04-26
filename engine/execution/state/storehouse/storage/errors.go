package storage

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

// HeightNotAvailableError is returned when request for the given height is not available
type HeightNotAvailableError struct {
	RequestedHeight,
	MinHeightAvailable,
	MaxHeightAvailable uint64
}

func (e HeightNotAvailableError) Error() string {
	return fmt.Sprintf(
		"requested height is not available: requested: %d available range (%d, %d)",
		e.RequestedHeight,
		e.MinHeightAvailable,
		e.MaxHeightAvailable,
	)
}

// InvalidBlockIDError is returned when the requested blockID isn't store in the storage
type InvalidBlockIDError struct {
	BlockID flow.Identifier
}

func (e InvalidBlockIDError) Error() string {
	return fmt.Sprintf(
		"requested blockID (%x) is not found",
		e.BlockID,
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

// ParentBlockNotFoundError is returned when the results for the parent block is not
// received before this block
type ParentBlockNotFoundError struct {
	ParentBlockID flow.Identifier
}

func (e ParentBlockNotFoundError) Error() string {
	return fmt.Sprintf(
		"parent block (%x) result is not found",
		e.ParentBlockID,
	)
}
