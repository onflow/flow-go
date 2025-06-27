package verification_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/verification"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestNewChunkDataPackResponse tests the NewChunkDataPackResponse constructor with valid and invalid inputs.
//
// Valid Case:
//
// 1. Valid input with non-empty locator and non-nil ChunkDataPack:
//   - Should successfully construct a ChunkDataPackResponse.
//
// Invalid Cases:
//
// 2. Invalid input with empty locator:
//   - Should return an error indicating the locator is empty.
//
// 3. Invalid input with nil ChunkDataPack:
//   - Should return an error indicating the chunk data pack must not be nil.
func TestNewChunkDataPackResponse(t *testing.T) {
	t.Run("valid input with locator and chunk data pack", func(t *testing.T) {
		response, err := verification.NewChunkDataPackResponse(
			verification.UntrustedChunkDataPackResponse{
				Locator: *unittest.ChunkLocatorFixture(unittest.IdentifierFixture(), 0),
				Cdp:     unittest.ChunkDataPackFixture(unittest.IdentifierFixture()),
			})

		require.NoError(t, err)
		require.NotNil(t, response)
	})

	t.Run("invalid input with empty locator", func(t *testing.T) {
		_, err := verification.NewChunkDataPackResponse(
			verification.UntrustedChunkDataPackResponse{
				Locator: chunks.Locator{}, // empty locator
				Cdp:     unittest.ChunkDataPackFixture(unittest.IdentifierFixture()),
			},
		)

		require.Error(t, err)
		require.Contains(t, err.Error(), "locator is empty")
	})

	t.Run("invalid input with nil chunk data pack", func(t *testing.T) {
		_, err := verification.NewChunkDataPackResponse(
			verification.UntrustedChunkDataPackResponse{
				Locator: *unittest.ChunkLocatorFixture(unittest.IdentifierFixture(), 0),
				Cdp:     nil,
			},
		)

		require.Error(t, err)
		require.Contains(t, err.Error(), "chunk data pack must not be nil")
	})
}
