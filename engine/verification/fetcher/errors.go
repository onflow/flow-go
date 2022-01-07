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
	resultID        flow.Identifier
	chunkIndex      uint64
	err             error
}

func NewChunkDataPackValidationError(originID flow.Identifier,
	resultID flow.Identifier,
	chunkIndex uint64,
	chunkDataPackID flow.Identifier,
	chunkID flow.Identifier,
	collectionID flow.Identifier,
	err error) error {

	return ChunkDataPackValidationError{
		originID:        originID,
		chunkDataPackID: chunkDataPackID,
		chunkID:         chunkID,
		collectionID:    collectionID,
		resultID:        resultID,
		chunkIndex:      chunkIndex,
		err:             err,
	}
}

func (c ChunkDataPackValidationError) Error() string {
	return fmt.Sprintf(
		"chunk data pack validation failed, originID: %x, resultID: %x, chunkIndex: %d, chunkDataPackID: %x, chunkID: %x, collectionID: %x, error: %v",
		c.originID,
		c.resultID,
		c.chunkIndex,
		c.chunkDataPackID,
		c.chunkID,
		c.collectionID,
		c.err)
}

func IsChunkDataPackValidationError(err error) bool {
	return errors.As(err, &ChunkDataPackValidationError{})
}
