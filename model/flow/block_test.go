package flow_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vmihailenco/msgpack/v4"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestGenesisEncodingJSON(t *testing.T) {
	genesis := unittest.Block.Genesis(flow.Mainnet)
	genesisID := genesis.ID()
	data, err := json.Marshal(genesis)
	require.NoError(t, err)
	var decoded flow.Block
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)
	decodedID := decoded.ID()
	assert.Equal(t, genesisID, decodedID)
	assert.Equal(t, genesis, &decoded)
}

func TestGenesisDecodingMsgpack(t *testing.T) {
	genesis := unittest.Block.Genesis(flow.Mainnet)
	genesisID := genesis.ID()
	data, err := msgpack.Marshal(genesis)
	require.NoError(t, err)
	var decoded flow.Block
	err = msgpack.Unmarshal(data, &decoded)
	require.NoError(t, err)
	decodedID := decoded.ID()
	assert.Equal(t, genesisID, decodedID)
	assert.Equal(t, genesis, &decoded)
}

func TestBlockEncodingJSON(t *testing.T) {
	block := unittest.BlockFixture()
	blockID := block.ID()
	data, err := json.Marshal(block)
	require.NoError(t, err)
	var decoded flow.Block
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)
	decodedID := decoded.ID()
	assert.Equal(t, blockID, decodedID)
	assert.Equal(t, block, &decoded)
}

func TestBlockEncodingMsgpack(t *testing.T) {
	block := unittest.BlockFixture()
	blockID := block.ID()
	data, err := msgpack.Marshal(block)
	require.NoError(t, err)
	var decoded flow.Block
	err = msgpack.Unmarshal(data, &decoded)
	require.NoError(t, err)
	decodedID := decoded.ID()
	assert.Equal(t, blockID, decodedID)
	assert.Equal(t, block, &decoded)
}

func TestNilProducesSameHashAsEmptySlice(t *testing.T) {
	nilPayload := flow.Payload{
		Guarantees: nil,
		Seals:      nil,
	}

	slicePayload := flow.Payload{
		Guarantees: make([]*flow.CollectionGuarantee, 0),
		Seals:      make([]*flow.Seal, 0),
	}

	assert.Equal(t, nilPayload.Hash(), slicePayload.Hash())
}

func TestOrderingChangesHash(t *testing.T) {
	seals := unittest.Seal.Fixtures(5)

	payload1 := flow.Payload{
		Seals: seals,
	}

	payload2 := flow.Payload{
		Seals: []*flow.Seal{seals[3], seals[2], seals[4], seals[1], seals[0]},
	}

	assert.NotEqual(t, payload1.Hash(), payload2.Hash())
}

func TestBlock_Status(t *testing.T) {
	statuses := map[flow.BlockStatus]string{
		flow.BlockStatusUnknown:   "BLOCK_UNKNOWN",
		flow.BlockStatusFinalized: "BLOCK_FINALIZED",
		flow.BlockStatusSealed:    "BLOCK_SEALED",
	}

	for status, value := range statuses {
		assert.Equal(t, status.String(), value)
	}
}

// TestBlockMalleability checks that flow.Block is not malleable: any change in its data
// should result in a different ID.
// Because our NewHeaderBody constructor enforces ParentView < View we use
// WithFieldGenerator to safely pass it.
func TestBlockMalleability(t *testing.T) {
	block := unittest.FullBlockFixture()
	unittest.RequireEntityNonMalleable(
		t,
		unittest.FullBlockFixture(),
		unittest.WithFieldGenerator("HeaderBody.ParentView", func() uint64 {
			return block.View - 1 // ParentView must stay below View, so set it to View-1
		}),
		unittest.WithFieldGenerator("HeaderBody.Timestamp", func() time.Time { return time.Now().UTC() }),
		unittest.WithFieldGenerator("Payload.Results", func() flow.ExecutionResultList {
			return flow.ExecutionResultList{unittest.ExecutionResultFixture()}
		}),
	)
}

// TestNewBlock verifies the behavior of the NewBlock constructor.
// It ensures proper handling of both valid and invalid untrusted input fields.
//
// Test Cases:
//
// 1. Valid input:
//   - Verifies that a properly populated UntrustedBlock results in a valid Block.
//
// 2. Invalid input with invalid HeaderBody:
//   - Ensures an error is returned when the HeaderBody.ParentID is flow.ZeroID.
//
// 3. Invalid input with invalid Payload:
//   - Ensures an error is returned when the Payload.ProtocolStateID is flow.ZeroID.
func TestNewBlock(t *testing.T) {
	t.Run("valid input", func(t *testing.T) {
		block := unittest.BlockFixture()

		res, err := flow.NewBlock(flow.UntrustedBlock(*block))
		require.NoError(t, err)
		require.NotNil(t, res)
	})

	t.Run("invalid input with invalid header body", func(t *testing.T) {
		block := unittest.BlockFixture()
		block.ParentID = flow.ZeroID

		res, err := flow.NewBlock(flow.UntrustedBlock(*block))
		require.Error(t, err)
		require.Nil(t, res)
		require.Contains(t, err.Error(), "invalid header body")
	})

	t.Run("invalid input with invalid payload", func(t *testing.T) {
		block := unittest.BlockFixture()
		block.Payload.ProtocolStateID = flow.ZeroID

		res, err := flow.NewBlock(flow.UntrustedBlock(*block))
		require.Error(t, err)
		require.Nil(t, res)
		require.Contains(t, err.Error(), "invalid payload")
	})
}

// TestNewRootBlock verifies the behavior of the NewRootBlock constructor.
// It ensures proper handling of both valid and invalid untrusted input fields.
//
// Test Cases:
//
// 1. Valid input:
//   - Verifies that a properly populated UntrustedBlock results in a valid root Block.
//
// 2. Invalid input with invalid HeaderBody:
//   - Ensures an error is returned when the HeaderBody.ParentView is not zero.
//
// 3. Invalid input with invalid Payload:
//   - Ensures an error is returned when the Payload.ProtocolStateID is flow.ZeroID.
func TestNewRootBlock(t *testing.T) {
	// validRootBlockFixture returns a new valid root flow.UntrustedBlock for use in tests.
	validRootBlockFixture := func() flow.UntrustedBlock {
		return flow.UntrustedBlock{
			HeaderBody: flow.HeaderBody{
				ChainID:            flow.Emulator,
				ParentID:           unittest.IdentifierFixture(),
				Height:             10,
				Timestamp:          time.Now(),
				View:               0,
				ParentView:         0,
				ParentVoterIndices: []byte{},
				ParentVoterSigData: []byte{},
				ProposerID:         flow.ZeroID,
				LastViewTC:         nil,
			},
			Payload: unittest.PayloadFixture(),
		}
	}

	t.Run("valid input", func(t *testing.T) {
		res, err := flow.NewRootBlock(validRootBlockFixture())
		require.NoError(t, err)
		require.NotNil(t, res)
	})

	t.Run("invalid input with invalid header body", func(t *testing.T) {
		block := validRootBlockFixture()
		block.ParentView = 1

		res, err := flow.NewRootBlock(block)
		require.Error(t, err)
		require.Nil(t, res)
		require.Contains(t, err.Error(), "invalid root header body")
	})

	t.Run("invalid input with invalid payload", func(t *testing.T) {
		block := validRootBlockFixture()
		block.Payload.ProtocolStateID = flow.ZeroID

		res, err := flow.NewRootBlock(block)
		require.Error(t, err)
		require.Nil(t, res)
		require.Contains(t, err.Error(), "invalid payload")
	})
}
