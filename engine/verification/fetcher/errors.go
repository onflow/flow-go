package fetcher

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

type ChunkDataPackValidationError struct {
	originID        flow.Identifier
	chunkDataPackID flow.Identifier
	chunkID         flow.Identifier
	collectionID    flow.Identifier
	err             error
}

func NewChunkDataPackValidationError(originID flow.Identifier,
	chunkDataPackID flow.Identifier,
	chunkID flow.Identifier,
	collectionID flow.Identifier,
	err error) error {

	return ChunkDataPackValidationError{
		originID:        originID,
		chunkDataPackID: chunkDataPackID,
		chunkID:         chunkID,
		collectionID:    collectionID,
		err:             err,
	}
}

func (c ChunkDataPackValidationError) Error() string {
	return fmt.Sprintf(
		"chunk data pack validation failed, originID: %x, chunkDataPackID: %x, chunkID: %x, collectionID: %x, error: %v",
		c.originID, c.chunkDataPackID, c.chunkID, c.collectionID, c.err)
}

func IsChunkDataPackValidationError(err error) bool {
	return errors.As(err, &ChunkDataPackValidationError{})
}
