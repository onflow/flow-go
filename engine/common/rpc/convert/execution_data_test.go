package convert_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence/encoding/ccf"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/flow/protobuf/go/flow/entities"

	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/ledger/common/testutils"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/onflow/flow-go/utils/unittest/generator"
)

func TestConvertBlockExecutionDataEventPayloads(t *testing.T) {
	// generators will produce identical event payloads (before encoding)
	ccfEvents := generator.GetEventsWithEncoding(3, entities.EventEncodingVersion_CCF_V0)
	jsonEvents := generator.GetEventsWithEncoding(3, entities.EventEncodingVersion_JSON_CDC_V0)

	// generate BlockExecutionData with CCF encoded events
	executionData := unittest.BlockExecutionDataFixture(
		unittest.WithChunkExecutionDatas(
			unittest.ChunkExecutionDataFixture(t, 1024, unittest.WithChunkEvents(ccfEvents)),
			unittest.ChunkExecutionDataFixture(t, 1024, unittest.WithChunkEvents(ccfEvents)),
		),
	)

	execDataMessage, err := convert.BlockExecutionDataToMessage(executionData)
	require.NoError(t, err)

	t.Run("regular convert does not modify payload encoding", func(t *testing.T) {
		for _, chunk := range execDataMessage.GetChunkExecutionData() {
			events := convert.MessagesToEvents(chunk.Events)
			for i, e := range events {
				assert.Equal(t, ccfEvents[i], e)

				_, err := ccf.Decode(nil, e.Payload)
				require.NoError(t, err)
			}
		}
	})

	t.Run("converted event payloads are encoded in jsoncdc", func(t *testing.T) {
		err = convert.BlockExecutionDataEventPayloadsToVersion(execDataMessage, entities.EventEncodingVersion_JSON_CDC_V0)
		require.NoError(t, err)

		for _, chunk := range execDataMessage.GetChunkExecutionData() {
			events := convert.MessagesToEvents(chunk.Events)
			for i, e := range events {
				assert.Equal(t, jsonEvents[i], e)

				_, err := jsoncdc.Decode(nil, e.Payload)
				require.NoError(t, err)
			}
		}
	})
}

func TestConvertBlockExecutionData(t *testing.T) {
	t.Parallel()

	chain := flow.Testnet.Chain() // this is used by the AddressFixture
	events := unittest.EventsFixture(5)

	chunks := 5
	chunkData := make([]*execution_data.ChunkExecutionData, 0, chunks)
	for i := 0; i < chunks-1; i++ {
		ced := unittest.ChunkExecutionDataFixture(t,
			0, // updates set explicitly to target 160-320KB per chunk
			unittest.WithChunkEvents(events),
			unittest.WithTrieUpdate(testutils.TrieUpdateFixture(5, 32*1024, 64*1024)),
		)

		chunkData = append(chunkData, ced)
	}
	makeServiceTx := func(ced *execution_data.ChunkExecutionData) {
		// proposal key and payer are empty addresses for service tx
		collection := unittest.CollectionFixture(1)
		collection.Transactions[0].ProposalKey.Address = flow.EmptyAddress
		collection.Transactions[0].Payer = flow.EmptyAddress
		ced.Collection = &collection

		// the service chunk sometimes does not have any trie updates
		ced.TrieUpdate = nil
	}
	chunk := unittest.ChunkExecutionDataFixture(t, execution_data.DefaultMaxBlobSize/5, unittest.WithChunkEvents(events), makeServiceTx)
	chunkData = append(chunkData, chunk)

	blockData := unittest.BlockExecutionDataFixture(unittest.WithChunkExecutionDatas(chunkData...))

	msg, err := convert.BlockExecutionDataToMessage(blockData)
	require.NoError(t, err)

	converted, err := convert.MessageToBlockExecutionData(msg, chain)
	require.NoError(t, err)

	assert.Equal(t, blockData, converted)
	for i, chunk := range blockData.ChunkExecutionDatas {
		if chunk.TrieUpdate == nil {
			assert.Nil(t, converted.ChunkExecutionDatas[i].TrieUpdate)
		} else {
			assert.True(t, chunk.TrieUpdate.Equals(converted.ChunkExecutionDatas[i].TrieUpdate))
		}
	}
}

func TestConvertChunkExecutionData(t *testing.T) {
	tests := []struct {
		name string
		fn   func(*testing.T) *execution_data.ChunkExecutionData
	}{
		{
			name: "chunk execution data conversions",
			fn: func(t *testing.T) *execution_data.ChunkExecutionData {
				return unittest.ChunkExecutionDataFixture(t,
					0, // updates set explicitly to target 160-320KB per chunk
					unittest.WithChunkEvents(unittest.EventsFixture(5)),
					unittest.WithTrieUpdate(testutils.TrieUpdateFixture(5, 32*1024, 64*1024)),
				)
			},
		},
		{
			name: "chunk execution data conversions - no events",
			fn: func(t *testing.T) *execution_data.ChunkExecutionData {
				ced := unittest.ChunkExecutionDataFixture(t, 0)
				ced.Events = nil
				return ced
			},
		},
		{
			name: "chunk execution data conversions - no trie update",
			fn: func(t *testing.T) *execution_data.ChunkExecutionData {
				ced := unittest.ChunkExecutionDataFixture(t, 0)
				ced.TrieUpdate = nil
				return ced
			},
		},
		{
			name: "chunk execution data conversions - empty collection",
			fn: func(t *testing.T) *execution_data.ChunkExecutionData {
				ced := unittest.ChunkExecutionDataFixture(t, 0)
				ced.Collection = &flow.Collection{}
				ced.TransactionResults = nil
				return ced
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ced := test.fn(t)

			chunkMsg, err := convert.ChunkExecutionDataToMessage(ced)
			require.NoError(t, err)

			chunkReConverted, err := convert.MessageToChunkExecutionData(chunkMsg, flow.Testnet.Chain())
			require.NoError(t, err)

			assert.Equal(t, ced, chunkReConverted)
			if ced.TrieUpdate == nil {
				assert.Nil(t, chunkReConverted.TrieUpdate)
			} else {
				assert.True(t, ced.TrieUpdate.Equals(chunkReConverted.TrieUpdate))
			}
		})
	}
}

func TestMessageToRegisterIDs(t *testing.T) {
	tests := []struct {
		name  string
		regID flow.RegisterID
	}{
		{
			name:  "service level register id",
			regID: flow.UUIDRegisterID(0),
		},
		{
			name:  "account level register id",
			regID: flow.AccountStatusRegisterID(unittest.AddressFixture()),
		},
		{
			name:  "regular register id",
			regID: unittest.RegisterIDFixture(),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			msg := convert.RegisterIDToMessage(test.regID)
			converted, err := convert.MessageToRegisterID(msg)
			require.NoError(t, err)
			assert.Equal(t, test.regID, converted)
		})
	}

	t.Run("nil owner converts to empty string", func(t *testing.T) {
		msg := &entities.RegisterID{
			Owner: nil,
			Key:   []byte("key"),
		}
		converted, err := convert.MessageToRegisterID(msg)
		require.NoError(t, err)
		assert.Equal(t, "", converted.Owner)
		assert.Equal(t, "key", converted.Key)
	})

	t.Run("nil message returns error", func(t *testing.T) {
		_, err := convert.MessageToRegisterID(nil)
		require.ErrorIs(t, err, convert.ErrEmptyMessage)
	})
}
