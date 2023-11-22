package convert_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestConvertBlockHeader tests that converting a header to and from a protobuf message results in the same
// header
func TestConvertBlockHeader(t *testing.T) {
	t.Parallel()

	header := unittest.BlockHeaderFixture()

	signerIDs := unittest.IdentifierListFixture(5)

	msg, err := convert.BlockHeaderToMessage(header, signerIDs)
	require.NoError(t, err)

	converted, err := convert.MessageToBlockHeader(msg)
	require.NoError(t, err)

	assert.Equal(t, header, converted)
}
