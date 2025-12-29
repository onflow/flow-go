package convert_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/onflow/flow/protobuf/go/flow/entities"
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
	signerIDs := unittest.IdentifierListFixture(5)

	msg, err := convert.BlockToMessage(block, signerIDs)
	require.NoError(t, err)

	converted, err := convert.MessageToBlock(msg)
	require.NoError(t, err)

	assert.Equal(t, block.ID(), converted.ID())
}

// TestConvertBlockLight tests that converting a block to its light form results in only the correct
// fields being set
func TestConvertBlockLight(t *testing.T) {
	t.Parallel()

	block := unittest.FullBlockFixture()
	msg := convert.BlockToMessageLight(block)

	// required fields are set
	blockID := block.ID()
	assert.Equal(t, 0, bytes.Compare(blockID[:], msg.Id))
	assert.Equal(t, block.Height, msg.Height)
	assert.Equal(t, 0, bytes.Compare(block.ParentID[:], msg.ParentId))
	assert.Equal(t, block.Timestamp, uint64(msg.Timestamp.AsTime().UnixMilli()))
	assert.Equal(t, 0, bytes.Compare(block.ParentVoterSigData, msg.Signatures[0]))

	guarantees := []*flow.CollectionGuarantee{}
	for _, g := range msg.CollectionGuarantees {
		guarantee, err := convert.MessageToCollectionGuarantee(g)
		require.NoError(t, err)
		guarantees = append(guarantees, guarantee)
	}

	assert.Equal(t, block.Payload.Guarantees, guarantees)

	// all other fields are not
	assert.Nil(t, msg.BlockHeader)
	assert.Len(t, msg.BlockSeals, 0)
	assert.Len(t, msg.ExecutionReceiptMetaList, 0)
	assert.Len(t, msg.ExecutionResultList, 0)
}

// TestConvertRootBlock tests that converting a root block to and from a protobuf message results in
// the same block
func TestConvertRootBlock(t *testing.T) {
	t.Parallel()

	block := unittest.Block.Genesis(flow.Emulator)

	msg, err := convert.BlockToMessage(block, flow.IdentifierList{})
	require.NoError(t, err)

	converted, err := convert.MessageToBlock(msg)
	require.NoError(t, err)

	assert.Equal(t, block.ID(), converted.ID())
}

// TestConvertBlockStatus tests converting protobuf BlockStatus messages to flow.BlockStatus.
func TestConvertBlockStatus(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		pbStatus entities.BlockStatus
		expected flow.BlockStatus
	}{
		{"Unknown", entities.BlockStatus_BLOCK_UNKNOWN, flow.BlockStatusUnknown},
		{"Finalized", entities.BlockStatus_BLOCK_FINALIZED, flow.BlockStatusFinalized},
		{"Sealed", entities.BlockStatus_BLOCK_SEALED, flow.BlockStatusSealed},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			converted := convert.MessageToBlockStatus(tc.pbStatus)
			assert.Equal(t, tc.expected, converted)
		})
	}
}

// TestConvertBlockSeal tests converting a flow.Seal to and from a protobuf BlockSeal message.
func TestConvertBlockSeal(t *testing.T) {
	t.Parallel()

	seal := unittest.Seal.Fixture()

	msg := convert.BlockSealToMessage(seal)
	converted, err := convert.MessageToBlockSeal(msg)
	require.NoError(t, err)

	assert.Equal(t, seal.ID(), converted.ID())
}

// TestConvertBlockSeals tests converting multiple flow.Seal to and from protobuf BlockSeal messages.
func TestConvertBlockSeals(t *testing.T) {
	t.Parallel()

	seals := unittest.Seal.Fixtures(3)

	msgs := convert.BlockSealsToMessages(seals)
	require.Len(t, msgs, len(seals))

	converted, err := convert.MessagesToBlockSeals(msgs)
	require.NoError(t, err)
	require.Len(t, converted, len(seals))

	for i, seal := range seals {
		assert.Equal(t, seal.ID(), converted[i].ID())
	}
}

// TestConvertPayloadFromMessage tests converting a protobuf Block message to a flow.Payload.
func TestConvertPayloadFromMessage(t *testing.T) {
	t.Parallel()

	block := unittest.FullBlockFixture()
	signerIDs := unittest.IdentifierListFixture(5)

	msg, err := convert.BlockToMessage(block, signerIDs)
	require.NoError(t, err)

	payload, err := convert.PayloadFromMessage(msg)
	require.NoError(t, err)

	assert.Equal(t, block.Payload.Hash(), payload.Hash())
}

// TestConvertBlockTimestamp2ProtobufTime tests converting block timestamps to protobuf Timestamp format.
func TestConvertBlockTimestamp2ProtobufTime(t *testing.T) {
	t.Parallel()

	t.Run("convert current timestamp", func(t *testing.T) {
		t.Parallel()

		// Use current time in unix milliseconds
		now := time.Now()
		timestampMillis := uint64(now.UnixMilli())

		pbTime := convert.BlockTimestamp2ProtobufTime(timestampMillis)
		require.NotNil(t, pbTime)

		// Convert back and verify
		convertedTime := pbTime.AsTime()
		assert.Equal(t, timestampMillis, uint64(convertedTime.UnixMilli()))
	})

	t.Run("convert zero timestamp", func(t *testing.T) {
		t.Parallel()

		pbTime := convert.BlockTimestamp2ProtobufTime(0)
		require.NotNil(t, pbTime)

		convertedTime := pbTime.AsTime()
		assert.Equal(t, uint64(0), uint64(convertedTime.UnixMilli()))
	})
}
