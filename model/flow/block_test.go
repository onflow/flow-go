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
	genesis := flow.Genesis(flow.Mainnet)
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
	genesis := flow.Genesis(flow.Mainnet)
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
func TestBlockMalleability(t *testing.T) {
	unittest.RequireEntityNonMalleable(
		t,
		unittest.FullBlockFixture(),
		unittest.WithFieldGenerator("Header.Timestamp", func() time.Time { return time.Now().UTC() }),
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
// 2. Invalid input with zero ParentID:
//   - Ensures an error is returned when the Header.ParentID is flow.ZeroID.
//
// 3. Invalid input with nil ParentVoterIndices:
//   - Ensures an error is returned when the Header.ParentVoterIndices is nil.
//
// 4. Invalid input with empty ParentVoterIndices:
//   - Ensures an error is returned when the Header.ParentVoterIndices is an empty slice.
//
// 5. Invalid input with nil ParentVoterSigData:
//   - Ensures an error is returned when the Header.ParentVoterSigData is nil.
//
// 6. Invalid input with empty ParentVoterSigData:
//   - Ensures an error is returned when the Header.ParentVoterSigData is an empty slice.
//
// 7. Invalid input with zero ProposerID:
//   - Ensures an error is returned when the Header.ProposerID is flow.ZeroID.
//
// 8. Invalid input where ParentView is greater than or equal to View:
//   - Ensures an error is returned when the Header.ParentView is not less than Header.View.
func TestNewBlock(t *testing.T) {
	t.Run("valid input", func(t *testing.T) {
		block := unittest.BlockFixture()

		res, err := flow.NewBlock(flow.UntrustedBlock(*block))
		require.NoError(t, err)
		require.NotNil(t, res)
	})

	t.Run("invalid input with zero ParentID", func(t *testing.T) {
		block := unittest.BlockFixture()
		block.Header.ParentID = flow.ZeroID

		res, err := flow.NewBlock(flow.UntrustedBlock(*block))
		require.Error(t, err)
		require.Nil(t, res)
		require.Contains(t, err.Error(), "parent ID must not be zero")
	})

	t.Run("invalid input with nil ParentVoterIndices", func(t *testing.T) {
		block := unittest.BlockFixture()
		block.Header.ParentVoterIndices = nil

		res, err := flow.NewBlock(flow.UntrustedBlock(*block))
		require.Error(t, err)
		require.Nil(t, res)
		require.Contains(t, err.Error(), "parent voter indices must not be empty")
	})

	t.Run("invalid input with empty ParentVoterIndices", func(t *testing.T) {
		block := unittest.BlockFixture()
		block.Header.ParentVoterIndices = []byte{}

		res, err := flow.NewBlock(flow.UntrustedBlock(*block))
		require.Error(t, err)
		require.Nil(t, res)
		require.Contains(t, err.Error(), "parent voter indices must not be empty")
	})

	t.Run("invalid input with nil ParentVoterSigData", func(t *testing.T) {
		block := unittest.BlockFixture()
		block.Header.ParentVoterSigData = nil

		res, err := flow.NewBlock(flow.UntrustedBlock(*block))
		require.Error(t, err)
		require.Nil(t, res)
		require.Contains(t, err.Error(), "parent voter signature must not be empty")
	})

	t.Run("invalid input with empty ParentVoterSigData", func(t *testing.T) {
		block := unittest.BlockFixture()
		block.Header.ParentVoterSigData = []byte{}

		res, err := flow.NewBlock(flow.UntrustedBlock(*block))
		require.Error(t, err)
		require.Nil(t, res)
		require.Contains(t, err.Error(), "parent voter signature must not be empty")
	})

	t.Run("invalid input with zero ProposerID", func(t *testing.T) {
		block := unittest.BlockFixture()
		block.Header.ProposerID = flow.ZeroID

		res, err := flow.NewBlock(flow.UntrustedBlock(*block))
		require.Error(t, err)
		require.Nil(t, res)
		require.Contains(t, err.Error(), "proposer ID must not be zero")
	})

	t.Run("invalid input with ParentView is greater than or equal to View", func(t *testing.T) {
		block := unittest.BlockFixture()
		block.Header.ParentView = 10
		block.Header.View = 10

		res, err := flow.NewBlock(flow.UntrustedBlock(*block))
		require.Error(t, err)
		require.Nil(t, res)
		require.Contains(t, err.Error(), "invalid views - block parent view (10) is greater than or equal to block view (10)")
	})
}
