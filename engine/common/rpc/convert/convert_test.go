package convert_test

import (
	"bytes"
	"math/rand"
	"testing"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/encoding/ccf"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	execproto "github.com/onflow/flow/protobuf/go/flow/execution"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/testutils"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/onflow/flow-go/utils/unittest/generator"
)

func TestConvertTransaction(t *testing.T) {
	tx := unittest.TransactionBodyFixture()

	msg := convert.TransactionToMessage(tx)
	converted, err := convert.MessageToTransaction(msg, flow.Testnet.Chain())
	assert.Nil(t, err)

	assert.Equal(t, tx, converted)
	assert.Equal(t, tx.ID(), converted.ID())
}

func TestConvertAccountKey(t *testing.T) {
	privateKey, _ := unittest.AccountKeyDefaultFixture()
	accountKey := privateKey.PublicKey(fvm.AccountKeyWeightThreshold)

	// Explicitly test if Revoked is properly converted
	accountKey.Revoked = true

	msg, err := convert.AccountKeyToMessage(accountKey)
	assert.Nil(t, err)

	converted, err := convert.MessageToAccountKey(msg)
	assert.Nil(t, err)

	assert.Equal(t, accountKey, *converted)
	assert.Equal(t, accountKey.PublicKey, converted.PublicKey)
	assert.Equal(t, accountKey.Revoked, converted.Revoked)
}

func TestConvertEvents(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		messages := convert.EventsToMessages(nil)
		assert.Len(t, messages, 0)
	})

	t.Run("simple", func(t *testing.T) {

		txID := unittest.IdentifierFixture()
		event := unittest.EventFixture(flow.EventAccountCreated, 2, 3, txID, 0)

		messages := convert.EventsToMessages([]flow.Event{event})

		require.Len(t, messages, 1)

		message := messages[0]

		require.Equal(t, event.EventIndex, message.EventIndex)
		require.Equal(t, event.TransactionIndex, message.TransactionIndex)
		require.Equal(t, event.Payload, message.Payload)
		require.Equal(t, event.TransactionID[:], message.TransactionId)
		require.Equal(t, string(event.Type), message.Type)
	})

	t.Run("convert event from ccf format", func(t *testing.T) {
		cadenceValue, err := cadence.NewValue(2)
		require.NoError(t, err)
		ccfPayload, err := ccf.Encode(cadenceValue)
		require.NoError(t, err)
		jsonPayload, err := jsoncdc.Encode(cadenceValue)
		require.NoError(t, err)
		txID := unittest.IdentifierFixture()
		ccfEvent := unittest.EventFixture(
			flow.EventAccountCreated, 2, 3, txID, 0)
		ccfEvent.Payload = ccfPayload
		jsonEvent := unittest.EventFixture(
			flow.EventAccountCreated, 2, 3, txID, 0)
		jsonEvent.Payload = jsonPayload
		message := convert.EventToMessage(ccfEvent)
		convertedEvent, err := convert.MessageToEventFromVersion(message, execproto.EventEncodingVersion_CCF_V0)
		assert.NoError(t, err)
		assert.Equal(t, jsonEvent, *convertedEvent)
	})

	t.Run("convert event from json cdc format", func(t *testing.T) {
		cadenceValue, err := cadence.NewValue(2)
		require.NoError(t, err)
		txID := unittest.IdentifierFixture()
		jsonEvent := unittest.EventFixture(
			flow.EventAccountCreated, 2, 3, txID, 0)
		jsonPayload, err := jsoncdc.Encode(cadenceValue)
		require.NoError(t, err)
		jsonEvent.Payload = jsonPayload
		message := convert.EventToMessage(jsonEvent)
		convertedEvent, err := convert.MessageToEventFromVersion(message, execproto.EventEncodingVersion_JSON_CDC_V0)
		assert.NoError(t, err)
		assert.Equal(t, jsonEvent, *convertedEvent)
	})

	t.Run("convert payload from ccf to jsoncdc", func(t *testing.T) {
		// Round trip conversion check
		cadenceValue, err := cadence.NewValue(2)
		require.NoError(t, err)
		ccfPayload, err := ccf.Encode(cadenceValue)
		require.NoError(t, err)
		txID := unittest.IdentifierFixture()
		ccfEvent := unittest.EventFixture(
			flow.EventAccountCreated, 2, 3, txID, 0)
		ccfEvent.Payload = ccfPayload

		jsonEvent := unittest.EventFixture(
			flow.EventAccountCreated, 2, 3, txID, 0)
		jsonPayload, err := jsoncdc.Encode(cadenceValue)
		require.NoError(t, err)
		jsonEvent.Payload = jsonPayload

		res, err := convert.CcfEventToJsonEvent(ccfEvent)
		require.NoError(t, err)
		require.Equal(t, jsonEvent, *res)
	})
}

// TestConvertBlockExecutionData checks if conversions between BlockExecutionData and it's fields are consistent.
func TestConvertBlockExecutionData(t *testing.T) {
	// Initialize the BlockExecutionData object
	numChunks := 5
	ced := make([]*execution_data.ChunkExecutionData, numChunks)
	bed := &execution_data.BlockExecutionData{
		BlockID:             unittest.IdentifierFixture(),
		ChunkExecutionDatas: ced,
	}

	// Fill the chunk execution datas with trie updates, collections, and events
	minSerializedSize := uint64(10 * execution_data.DefaultMaxBlobSize)
	for i := 0; i < numChunks; i++ {
		// the service chunk sometimes does not have any trie updates
		if i == numChunks-1 {
			tx1 := unittest.TransactionBodyFixture()
			// proposal key and payer are empty addresses for service tx
			tx1.ProposalKey.Address = flow.EmptyAddress
			tx1.Payer = flow.EmptyAddress
			bed.ChunkExecutionDatas[i] = &execution_data.ChunkExecutionData{
				Collection: &flow.Collection{Transactions: []*flow.TransactionBody{&tx1}},
			}
			continue
		}

		// Initialize collection
		tx1 := unittest.TransactionBodyFixture()
		tx2 := unittest.TransactionBodyFixture()
		col := &flow.Collection{Transactions: []*flow.TransactionBody{&tx1, &tx2}}

		// Initialize events
		header := unittest.BlockHeaderFixture()
		events := unittest.BlockEventsFixture(header, 5).Events

		chunk := &execution_data.ChunkExecutionData{
			Collection: col,
			Events:     events,
			TrieUpdate: testutils.TrieUpdateFixture(5, 1, 8),
		}
		size := 1

		// Fill the TrieUpdate with data
	inner:
		for {
			buf := &bytes.Buffer{}
			require.NoError(t, execution_data.DefaultSerializer.Serialize(buf, chunk))

			if buf.Len() >= int(minSerializedSize) {
				break inner
			}

			v := make([]byte, size)
			_, _ = rand.Read(v)

			k, err := chunk.TrieUpdate.Payloads[0].Key()
			require.NoError(t, err)

			chunk.TrieUpdate.Payloads[0] = ledger.NewPayload(k, v)
			size *= 2
		}
		bed.ChunkExecutionDatas[i] = chunk
	}

	t.Run("chunk execution data conversions", func(t *testing.T) {
		chunkMsg, err := convert.ChunkExecutionDataToMessage(bed.ChunkExecutionDatas[0])
		assert.Nil(t, err)

		chunkReConverted, err := convert.MessageToChunkExecutionData(chunkMsg, flow.Testnet.Chain())
		assert.Nil(t, err)
		assert.Equal(t, bed.ChunkExecutionDatas[0], chunkReConverted)
	})

	t.Run("block execution data conversions", func(t *testing.T) {
		blockMsg, err := convert.BlockExecutionDataToMessage(bed)
		assert.Nil(t, err)

		bedReConverted, err := convert.MessageToBlockExecutionData(blockMsg, flow.Testnet.Chain())
		assert.Nil(t, err)
		assert.Equal(t, bed, bedReConverted)
	})
}

func TestConvertBlockExecutionDataEventPayloads(t *testing.T) {
	// generators will produce identical event payloads (before encoding)
	ccfEventGenerator := generator.EventGenerator(generator.WithEncoding(generator.EncodingCCF))
	jsonEventsGenerator := generator.EventGenerator(generator.WithEncoding(generator.EncodingJSON))

	ccfEvents := make([]flow.Event, 0, 3)
	jsonEvents := make([]flow.Event, 0, 3)
	for i := 0; i < 3; i++ {
		ccfEvents = append(ccfEvents, ccfEventGenerator.New())
		jsonEvents = append(jsonEvents, jsonEventsGenerator.New())
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
			events := convert.MessagesToEvents(chunk.Events)
			for i, e := range events {
				assert.Equal(t, ccfEvents[i], e)

				_, err := ccf.Decode(nil, e.Payload)
				require.NoError(t, err)
			}
		}
	})

	t.Run("converted event payloads are encoded in jsoncdc", func(t *testing.T) {
		err = convert.BlockExecutionDataEventPayloadsToJson(execDataMessage)
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
