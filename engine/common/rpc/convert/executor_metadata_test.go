package convert_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestExecutorMetadata verifies round-trip conversion between the access-layer
// ExecutorMetadata struct and its corresponding protobuf message representation
func TestExecutorMetadata(t *testing.T) {
	t.Parallel()

	t.Run("convert executor metadata", func(t *testing.T) {
		t.Parallel()

		metadata := &access.ExecutorMetadata{
			ExecutionResultID: unittest.IdentifierFixture(),
			ExecutorIDs:       unittest.IdentifierListFixture(4),
		}

		msg := convert.ExecutorMetadataToMessage(metadata)
		converted := convert.MessageToExecutorMetadata(msg)

		require.Equal(t, metadata, converted)
	})

	t.Run("convert nil executor metadata", func(t *testing.T) {
		t.Parallel()

		msg := convert.ExecutorMetadataToMessage(nil)
		converted := convert.MessageToExecutorMetadata(msg)

		require.Nil(t, converted)
	})

	t.Run("convert empty executor metadata", func(t *testing.T) {
		t.Parallel()

		metadata := &access.ExecutorMetadata{}
		msg := convert.ExecutorMetadataToMessage(metadata)
		converted := convert.MessageToExecutorMetadata(msg)

		// cannot do a direct comparison to metadata because the MessageToIdentifiers converter always
		// returns a slice, even if the input is nil
		require.NotNil(t, converted)
	})
}
