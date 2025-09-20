package convert_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/flow"
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

// TestConvertRootBlockHeader tests that converting a root block header to and from a protobuf message
// results in the same block
func TestConvertRootBlockHeader(t *testing.T) {
	t.Parallel()

	header := unittest.Block.Genesis(flow.Emulator).ToHeader()

	msg, err := convert.BlockHeaderToMessage(header, flow.IdentifierList{})
	require.NoError(t, err)

	converted, err := convert.MessageToBlockHeader(msg)
	require.NoError(t, err)

	assert.Equal(t, header.ID(), converted.ID())
}
