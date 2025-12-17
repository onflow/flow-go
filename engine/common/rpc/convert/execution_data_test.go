package convert_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence/encoding/ccf"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/flow/protobuf/go/flow/entities"

	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/ledger/common/testutils"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/executiondatasync/testutil"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/onflow/flow-go/utils/unittest/fixtures"
)

func TestConvertBlockExecutionDataEventPayloads(t *testing.T) {
	// generators will produce identical event payloads (before encoding)
	ccfEvents := unittest.EventGenerator.GetEventsWithEncoding(3, entities.EventEncodingVersion_CCF_V0)
	jsonEvents := make([]flow.Event, len(ccfEvents))
	for i, e := range ccfEvents {
		jsonEvent, err := convert.CcfEventToJsonEvent(e)
		require.NoError(t, err)
		jsonEvents[i] = *jsonEvent
	}

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
			events, err := convert.MessagesToEvents(chunk.Events)
			require.NoError(t, err)

			for i, e := range events {
				require.Equal(t, ccfEvents[i], e)

				res, err := ccf.Decode(nil, e.Payload)
				require.NotNil(t, res)
				require.NoError(t, err)
			}
		}
	})

	t.Run("converted event payloads are encoded in jsoncdc", func(t *testing.T) {
		err = convert.BlockExecutionDataEventPayloadsToVersion(execDataMessage, entities.EventEncodingVersion_JSON_CDC_V0)
		require.NoError(t, err)

		for _, chunk := range execDataMessage.GetChunkExecutionData() {
			events, err := convert.MessagesToEvents(chunk.Events)
			require.NoError(t, err)

			for i, e := range events {
				require.Equal(t, jsonEvents[i], e)

				res, err := jsoncdc.Decode(nil, e.Payload)
				require.NotNil(t, res)
				require.NoError(t, err)
			}
		}
	})
}

func TestConvertBlockExecutionData(t *testing.T) {
	t.Parallel()

	g := fixtures.NewGeneratorSuite()
	tf := testutil.CompleteFixture(t, g, g.Blocks().Fixture())
	blockData := tf.ExecutionData

	msg, err := convert.BlockExecutionDataToMessage(blockData)
	require.NoError(t, err)

	converted, err := convert.MessageToBlockExecutionData(msg, g.ChainID().Chain())
	require.NoError(t, err)

	require.Equal(t, blockData, converted)
	for i, chunk := range blockData.ChunkExecutionDatas {
		if chunk.TrieUpdate == nil {
			require.Nil(t, converted.ChunkExecutionDatas[i].TrieUpdate)
		} else {
			require.True(t, chunk.TrieUpdate.Equals(converted.ChunkExecutionDatas[i].TrieUpdate))
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
				ced.Collection = flow.NewEmptyCollection()
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

			require.Equal(t, ced, chunkReConverted)
			if ced.TrieUpdate == nil {
				require.Nil(t, chunkReConverted.TrieUpdate)
			} else {
				require.True(t, ced.TrieUpdate.Equals(chunkReConverted.TrieUpdate))
			}
		})
	}
}

func TestMessageToRegisterID(t *testing.T) {
	chain := flow.Testnet.Chain()
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
			converted, err := convert.MessageToRegisterID(msg, chain)
			require.NoError(t, err)
			require.Equal(t, test.regID, converted)
		})
	}

	t.Run("nil owner converts to empty string", func(t *testing.T) {
		msg := &entities.RegisterID{
			Owner: nil,
			Key:   []byte("key"),
		}
		converted, err := convert.MessageToRegisterID(msg, chain)
		require.NoError(t, err)
		require.Equal(t, "", converted.Owner)
		require.Equal(t, "key", converted.Key)
	})

	t.Run("nil message returns error", func(t *testing.T) {
		_, err := convert.MessageToRegisterID(nil, chain)
		require.ErrorIs(t, err, convert.ErrEmptyMessage)
	})

	t.Run("invalid address returns error", func(t *testing.T) {
		// addresses for other chains are invalid
		registerID := flow.NewRegisterID(
			unittest.RandomAddressFixtureForChain(flow.Mainnet),
			"key",
		)

		msg := convert.RegisterIDToMessage(registerID)
		_, err := convert.MessageToRegisterID(msg, chain)
		require.Error(t, err)
	})

	t.Run("multiple registerIDs", func(t *testing.T) {
		expected := flow.RegisterIDs{
			flow.UUIDRegisterID(0),
			flow.AccountStatusRegisterID(unittest.AddressFixture()),
			unittest.RegisterIDFixture(),
		}

		messages := make([]*entities.RegisterID, len(expected))
		for i, regID := range expected {
			regID := regID
			messages[i] = convert.RegisterIDToMessage(regID)
			require.Equal(t, regID.Owner, string(messages[i].Owner))
			require.Equal(t, regID.Key, string(messages[i].Key))
		}

		actual, err := convert.MessagesToRegisterIDs(messages, chain)
		require.NoError(t, err)
		require.Equal(t, expected, actual)
	})
}
