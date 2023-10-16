package chunks_test

import (
	"fmt"
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestIsChunkFaultError tests the IsChunkFaultError function returns true for known chunk fault errors
// and false for any other error.
func TestIsChunkFaultError(t *testing.T) {
	t.Run("CFMissingRegisterTouch", func(t *testing.T) {
		cf := chunks.NewCFMissingRegisterTouch(nil, 0, unittest.IdentifierFixture(), unittest.IdentifierFixture())
		assert.Error(t, cf)
		assert.True(t, chunks.IsChunkFaultError(cf))

		var errType *chunks.CFMissingRegisterTouch
		assert.ErrorAs(t, cf, &errType)
	})

	t.Run("CFNonMatchingFinalState", func(t *testing.T) {
		cf := chunks.NewCFNonMatchingFinalState(unittest.StateCommitmentFixture(), unittest.StateCommitmentFixture(), 0, unittest.IdentifierFixture())
		assert.Error(t, cf)
		assert.True(t, chunks.IsChunkFaultError(cf))

		var errType *chunks.CFNonMatchingFinalState
		assert.ErrorAs(t, cf, &errType)
	})

	t.Run("CFInvalidEventsCollection", func(t *testing.T) {
		cf := chunks.NewCFInvalidEventsCollection(unittest.IdentifierFixture(), unittest.IdentifierFixture(), 0, unittest.IdentifierFixture(), nil)
		assert.Error(t, cf)
		assert.True(t, chunks.IsChunkFaultError(cf))

		var errType *chunks.CFInvalidEventsCollection
		assert.ErrorAs(t, cf, &errType)
	})

	t.Run("CFInvalidVerifiableChunk", func(t *testing.T) {
		cf := chunks.NewCFInvalidVerifiableChunk("", nil, 0, unittest.IdentifierFixture())
		assert.Error(t, cf)
		assert.True(t, chunks.IsChunkFaultError(cf))

		var errType *chunks.CFInvalidVerifiableChunk
		assert.ErrorAs(t, cf, &errType)
	})

	t.Run("CFSystemChunkIncludedCollection", func(t *testing.T) {
		cf := chunks.NewCFSystemChunkIncludedCollection(0, unittest.IdentifierFixture())
		assert.Error(t, cf)
		assert.True(t, chunks.IsChunkFaultError(cf))

		var errType *chunks.CFSystemChunkIncludedCollection
		assert.ErrorAs(t, cf, &errType)
	})

	t.Run("CFExecutionDataBlockIDMismatch", func(t *testing.T) {
		cf := chunks.NewCFExecutionDataBlockIDMismatch(unittest.IdentifierFixture(), unittest.IdentifierFixture(), 0, unittest.IdentifierFixture())
		assert.Error(t, cf)
		assert.True(t, chunks.IsChunkFaultError(cf))

		var errType *chunks.CFExecutionDataBlockIDMismatch
		assert.ErrorAs(t, cf, &errType)
	})

	t.Run("CFExecutionDataChunksLengthMismatch", func(t *testing.T) {
		cf := chunks.NewCFExecutionDataChunksLengthMismatch(0, 0, 0, unittest.IdentifierFixture())
		assert.Error(t, cf)
		assert.True(t, chunks.IsChunkFaultError(cf))

		var errType *chunks.CFExecutionDataChunksLengthMismatch
		assert.ErrorAs(t, cf, &errType)
	})

	t.Run("CFExecutionDataInvalidChunkCID", func(t *testing.T) {
		cf := chunks.NewCFExecutionDataInvalidChunkCID(cid.Cid{}, cid.Cid{}, 0, unittest.IdentifierFixture())
		assert.Error(t, cf)
		assert.True(t, chunks.IsChunkFaultError(cf))

		var errType *chunks.CFExecutionDataInvalidChunkCID
		assert.ErrorAs(t, cf, &errType)
	})

	t.Run("CFInvalidExecutionDataID", func(t *testing.T) {
		cf := chunks.NewCFInvalidExecutionDataID(unittest.IdentifierFixture(), unittest.IdentifierFixture(), 0, unittest.IdentifierFixture())
		assert.Error(t, cf)
		assert.True(t, chunks.IsChunkFaultError(cf))

		var errType *chunks.CFInvalidExecutionDataID
		assert.ErrorAs(t, cf, &errType)
	})

	t.Run("Non ChunkFaultError", func(t *testing.T) {
		err := fmt.Errorf("some error")
		assert.False(t, chunks.IsChunkFaultError(err))
	})
}
