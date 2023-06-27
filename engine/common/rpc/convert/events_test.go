package convert_test

import (
	"testing"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/encoding/ccf"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	execproto "github.com/onflow/flow/protobuf/go/flow/execution"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

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
