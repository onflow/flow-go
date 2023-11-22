package convert_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestConvertBlock tests that converting a block to and from a protobuf message results in the same
// block
func TestConvertBlock(t *testing.T) {
	t.Parallel()

	block := unittest.FullBlockFixture()
	block.SetPayload(unittest.PayloadFixture(unittest.WithAllTheFixins))

	signerIDs := unittest.IdentifierListFixture(5)

	msg, err := convert.BlockToMessage(&block, signerIDs)
	require.NoError(t, err)

	converted, err := convert.MessageToBlock(msg)
	require.NoError(t, err)

	assert.Equal(t, block, *converted)
}

// TestConvertBlockLight tests that converting a block to its light form results in only the correct
// fields being set
func TestConvertBlockLight(t *testing.T) {
	t.Parallel()

	block := unittest.FullBlockFixture()
	block.SetPayload(unittest.PayloadFixture(unittest.WithAllTheFixins))

	msg := convert.BlockToMessageLight(&block)

	// required fields are set
	blockID := block.ID()
	assert.Equal(t, 0, bytes.Compare(blockID[:], msg.Id))
	assert.Equal(t, block.Header.Height, msg.Height)
	assert.Equal(t, 0, bytes.Compare(block.Header.ParentID[:], msg.ParentId))
	assert.Equal(t, block.Header.Timestamp, msg.Timestamp.AsTime())
	assert.Equal(t, 0, bytes.Compare(block.Header.ParentVoterSigData, msg.Signatures[0]))

	guarantees := []*flow.CollectionGuarantee{}
	for _, g := range msg.CollectionGuarantees {
		guarantee := convert.MessageToCollectionGuarantee(g)
		guarantees = append(guarantees, guarantee)
	}

	assert.Equal(t, block.Payload.Guarantees, guarantees)

	// all other fields are not
	assert.Nil(t, msg.BlockHeader)
	assert.Len(t, msg.BlockSeals, 0)
	assert.Len(t, msg.ExecutionReceiptMetaList, 0)
	assert.Len(t, msg.ExecutionResultList, 0)
}
