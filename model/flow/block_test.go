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

func TestBlockID_Malleability(t *testing.T) {
	block := unittest.FullBlockFixture()
	block.SetPayload(unittest.PayloadFixture(unittest.WithAllTheFixins))
	receiptGenerator := func() (receipts flow.ExecutionReceiptMetaList, results flow.ExecutionResultList) {
		receipt := unittest.ExecutionReceiptFixture(
			unittest.WithResult(unittest.ExecutionResultFixture(unittest.WithServiceEvents(3))),
			unittest.WithSpocks(unittest.SignaturesFixture(3)),
		)
		receipts = flow.ExecutionReceiptMetaList{receipt.Meta()}
		results = flow.ExecutionResultList{&receipt.ExecutionResult}
		return receipts, results
	}
	unittest.RequireEntityNonMalleable(t, &block,
		unittest.WithFieldGenerator("Header.Timestamp", func() time.Time { return time.Now().UTC() }),
		unittest.WithFieldGenerator("Payload.Seals", func() []*flow.Seal {
			block.Payload.Seals = unittest.Seal.Fixtures(3)
			block.SetPayload(*block.Payload)
			return block.Payload.Seals
		}),
		unittest.WithFieldGenerator("Payload.Guarantees", func() []*flow.CollectionGuarantee {
			block.Payload.Guarantees = unittest.CollectionGuaranteesFixture(4)
			block.SetPayload(*block.Payload)
			return block.Payload.Guarantees
		}),
		unittest.WithFieldGenerator("Payload.Receipts", func() flow.ExecutionReceiptMetaList {
			block.Payload.Receipts, _ = receiptGenerator()
			block.SetPayload(*block.Payload)
			return block.Payload.Receipts
		}),
		unittest.WithFieldGenerator("Payload.Results", func() flow.ExecutionResultList {
			_, block.Payload.Results = receiptGenerator()
			block.SetPayload(*block.Payload)
			return block.Payload.Results
		}),
		unittest.WithFieldGenerator("Payload.ProtocolStateID", func() flow.Identifier {
			id := unittest.IdentifierFixture()
			block.Payload.ProtocolStateID = id
			block.SetPayload(*block.Payload)
			return id
		}),
	)
}

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
	assert.Equal(t, block, decoded)
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
	assert.Equal(t, block, decoded)
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
