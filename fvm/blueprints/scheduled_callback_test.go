package blueprints_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence"
	cadenceCommon "github.com/onflow/cadence/common"
	"github.com/onflow/cadence/encoding/ccf"

	jsoncdc "github.com/onflow/cadence/encoding/json"

	"github.com/onflow/flow-go/fvm/blueprints"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestProcessCallbacksTransaction(t *testing.T) {
	t.Parallel()

	chain := flow.Mainnet.Chain()
	tx, err := blueprints.ProcessCallbacksTransaction(chain)
	require.NoError(t, err)

	assert.NotNil(t, tx)
	assert.NotEmpty(t, tx.Script)
	require.False(t, strings.Contains(string(tx.Script), `import "FlowTransactionScheduler"`), "should resolve callback scheduler import")
	assert.Equal(t, uint64(flow.DefaultMaxTransactionGasLimit), tx.GasLimit)
	assert.Equal(t, tx.Authorizers, []flow.Address{chain.ServiceAddress()})
	assert.Empty(t, tx.Arguments)
}

func TestExecuteCallbacksTransactions(t *testing.T) {
	t.Parallel()

	chain := flow.Mainnet.Chain()

	tests := []struct {
		name         string
		events       []flow.Event
		expectedTxs  int
		expectError  bool
		errorMessage string
	}{
		{
			name:        "no events",
			events:      []flow.Event{},
			expectedTxs: 0,
			expectError: false,
		},
		{
			name:        "single valid event",
			events:      []flow.Event{createValidCallbackEvent(t, 1, 100)},
			expectedTxs: 1,
			expectError: false,
		},
		{
			name: "multiple valid events",
			events: []flow.Event{
				createValidCallbackEvent(t, 1, 100),
				createValidCallbackEvent(t, 2, 200),
				createValidCallbackEvent(t, 3, 300),
			},
			expectedTxs: 3,
			expectError: false,
		},
		{
			name:        "invalid event type ignored",
			events:      []flow.Event{createInvalidTypeEvent()},
			expectedTxs: 0,
			expectError: false,
		},
		{
			name:        "invalid event payload ignored",
			events:      []flow.Event{createInvalidPayloadEvent()},
			expectedTxs: 0,
			expectError: false,
		},
		{
			name:         "invalid CCF payload",
			events:       []flow.Event{createPendingExecutionEventWithPayload([]byte{0xFF, 0xAB, 0xCD})}, // Invalid CCF bytes
			expectedTxs:  0,
			expectError:  true,
			errorMessage: "failed to decode event",
		},
		{
			name:         "non-cadence event data",
			events:       []flow.Event{createPendingExecutionEventWithEncodedValue(t, cadence.String("not an event"))},
			expectedTxs:  0,
			expectError:  true,
			errorMessage: "event data is not a cadence event",
		},
		{
			name:         "missing id field",
			events:       []flow.Event{createEventWithModifiedField(t, "id", nil)},
			expectedTxs:  0,
			expectError:  true,
			errorMessage: "id is not uint64",
		},
		{
			name:         "missing effort field",
			events:       []flow.Event{createEventWithModifiedField(t, "executionEffort", nil)},
			expectedTxs:  0,
			expectError:  true,
			errorMessage: "effort is not uint64",
		},
		{
			name:        "effort exceeding max gas limit",
			events:      []flow.Event{createValidCallbackEvent(t, 1, flow.DefaultMaxTransactionGasLimit+1000)},
			expectedTxs: 1,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			txs, err := blueprints.ExecuteCallbacksTransactions(chain, tt.events)

			if tt.expectError {
				assert.Errorf(t, err, tt.name)
				assert.Contains(t, err.Error(), tt.errorMessage)
				assert.Nil(t, txs)
				return
			}

			assert.NoError(t, err)
			assert.Len(t, txs, tt.expectedTxs)

			// Verify gas limit for effort exceeding max case
			if tt.name == "effort exceeding max gas limit" && len(txs) > 0 {
				assert.Equal(t, uint64(flow.DefaultMaxTransactionGasLimit), txs[0].GasLimit)
			}

			for i, tx := range txs {
				assert.NotNil(t, tx)
				assert.NotEmpty(t, tx.Script)
				if tt.name != "effort exceeding max gas limit" {
					expectedEffort := uint64(100 * (i + 1)) // Events created with efforts 100, 200, 300
					assert.Equal(t, expectedEffort, tx.GasLimit)
				} else {
					assert.Equal(t, uint64(flow.DefaultMaxTransactionGasLimit), tx.GasLimit)
				}
				assert.Equal(t, tx.Authorizers, []flow.Address{chain.ServiceAddress()})
				assert.Len(t, tx.Arguments, 1)
				assert.NotEmpty(t, tx.Arguments[0])

				t.Logf("Transaction %d: ID arg length: %d, GasLimit: %d",
					i, len(tx.Arguments[0]), tx.GasLimit)
			}
		})
	}
}

func TestExecuteCallbackTransaction(t *testing.T) {
	t.Parallel()

	chain := flow.Mainnet.Chain()

	const id = 123
	const effort = 456
	event := createValidCallbackEvent(t, id, effort)
	txs, err := blueprints.ExecuteCallbacksTransactions(chain, []flow.Event{event})

	require.NoError(t, err)
	require.Len(t, txs, 1)

	tx := txs[0]
	assert.NotNil(t, tx)
	assert.NotEmpty(t, tx.Script)
	require.False(t, strings.Contains(string(tx.Script), `import "FlowTransactionScheduler"`), "should resolve callback scheduler import")
	assert.Equal(t, uint64(effort), tx.GasLimit)
	assert.Len(t, tx.Arguments, 1)

	expectedEncodedID, err := jsoncdc.Encode(cadence.NewUInt64(id))
	require.NoError(t, err)
	assert.Equal(t, tx.Arguments[0], expectedEncodedID)

	assert.Equal(t, tx.GasLimit, uint64(effort))
}

func TestIsPendingExecutionEvent(t *testing.T) {
	t.Parallel()

	chain := flow.Mainnet.Chain()
	env := systemcontracts.SystemContractsForChain(chain.ChainID()).AsTemplateEnv()
	assert.True(t, blueprints.IsPendingExecutionEvent(env, createValidCallbackEvent(t, 1, 100)))
}

func TestParsePendingExecutionEvent(t *testing.T) {
	t.Parallel()

	expectedID := uint64(123)
	expectedEffort := uint64(456)
	event := createValidCallbackEvent(t, expectedID, expectedEffort)

	actualID, actualEffort, err := blueprints.ParsePendingExecutionEvent(event)
	require.NoError(t, err)
	assert.Equal(t, expectedID, uint64(actualID))
	assert.Equal(t, expectedEffort, uint64(actualEffort))
}

func createValidCallbackEvent(t *testing.T, id uint64, effort uint64) flow.Event {
	const processedEventTypeTemplate = "A.%v.FlowTransactionScheduler.PendingExecution"
	env := systemcontracts.SystemContractsForChain(flow.Mainnet.Chain().ChainID()).AsTemplateEnv()
	eventTypeString := fmt.Sprintf(processedEventTypeTemplate, env.FlowTransactionSchedulerAddress)
	loc, err := cadenceCommon.HexToAddress(env.FlowTransactionSchedulerAddress)
	require.NoError(t, err)
	location := cadenceCommon.NewAddressLocation(nil, loc, "PendingExecution")

	eventType := cadence.NewEventType(
		location,
		"PendingExecution",
		[]cadence.Field{
			{Identifier: "id", Type: cadence.UInt64Type},
			{Identifier: "priority", Type: cadence.UInt8Type},
			{Identifier: "executionEffort", Type: cadence.UInt64Type},
			{Identifier: "callbackOwner", Type: cadence.AddressType},
		},
		nil,
	)

	event := cadence.NewEvent(
		[]cadence.Value{
			cadence.NewUInt64(id),
			cadence.NewUInt8(1),
			cadence.NewUInt64(effort),
			cadence.NewAddress([8]byte{}),
		},
	).WithType(eventType)

	payload, err := ccf.Encode(event)
	require.NoError(t, err)

	return flow.Event{
		Type:             flow.EventType(eventTypeString),
		TransactionID:    unittest.IdentifierFixture(),
		TransactionIndex: 0,
		EventIndex:       0,
		Payload:          payload,
	}
}

func createInvalidTypeEvent() flow.Event {
	return flow.Event{
		Type:             flow.EventType("A.0000000000000000.SomeContract.WrongEvent"),
		TransactionID:    unittest.IdentifierFixture(),
		TransactionIndex: 0,
		EventIndex:       0,
		Payload:          []byte("invalid"),
	}
}

func createInvalidPayloadEvent() flow.Event {
	return flow.Event{
		Type:             flow.EventType("A.0000000000000000.FlowTransactionScheduler.PendingExecution"),
		TransactionID:    unittest.IdentifierFixture(),
		TransactionIndex: 0,
		EventIndex:       0,
		Payload:          []byte("not valid ccf"),
	}
}

func createPendingExecutionEventWithPayload(payload []byte) flow.Event {
	const processedEventTypeTemplate = "A.%v.FlowTransactionScheduler.PendingExecution"
	env := systemcontracts.SystemContractsForChain(flow.Mainnet.Chain().ChainID()).AsTemplateEnv()
	eventTypeString := fmt.Sprintf(processedEventTypeTemplate, env.FlowTransactionSchedulerAddress)

	return flow.Event{
		Type:             flow.EventType(eventTypeString),
		TransactionID:    unittest.IdentifierFixture(),
		TransactionIndex: 0,
		EventIndex:       0,
		Payload:          payload,
	}
}

func createPendingExecutionEventWithEncodedValue(t *testing.T, value cadence.Value) flow.Event {
	payload, err := ccf.Encode(value)
	require.NoError(t, err)
	return createPendingExecutionEventWithPayload(payload)
}

func TestSystemCollection(t *testing.T) {
	t.Parallel()

	chain := flow.Mainnet.Chain()

	tests := []struct {
		name            string
		events          []flow.Event
		expectedTxCount int
		errorMessage    string
	}{
		{
			name:            "no events",
			events:          []flow.Event{},
			expectedTxCount: 2, // process + system chunk
		},
		{
			name:            "single valid callback event",
			events:          []flow.Event{createValidCallbackEvent(t, 1, 100)},
			expectedTxCount: 3, // process + execute + system chunk
		},
		{
			name: "multiple valid callback events",
			events: []flow.Event{
				createValidCallbackEvent(t, 1, 100),
				createValidCallbackEvent(t, 2, 200),
				createValidCallbackEvent(t, 3, 300),
			},
			expectedTxCount: 5, // process + 3 executes + system chunk
		},
		{
			name: "mixed events - valid callbacks and invalid types",
			events: []flow.Event{
				createValidCallbackEvent(t, 1, 100),
				createInvalidTypeEvent(),
				createValidCallbackEvent(t, 2, 200),
				createInvalidPayloadEvent(),
			},
			expectedTxCount: 4, // process + 2 executes + system chunk
		},
		{
			name:            "only invalid event types",
			events:          []flow.Event{createInvalidTypeEvent(), createInvalidPayloadEvent()},
			expectedTxCount: 2, // process + system chunk
		},
		{
			name:         "invalid CCF payload in callback event",
			events:       []flow.Event{createPendingExecutionEventWithPayload([]byte{0xFF, 0xAB, 0xCD})},
			errorMessage: "failed to construct execute callbacks transactions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			collection, err := blueprints.SystemCollection(chain, tt.events)

			if tt.errorMessage != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMessage)
				assert.Nil(t, collection)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, collection)

			transactions := collection.Transactions
			assert.Len(t, transactions, tt.expectedTxCount)

			if tt.expectedTxCount > 0 {
				// First transaction should always be the process transaction
				processTx := transactions[0]
				assert.NotNil(t, processTx)
				assert.NotEmpty(t, processTx.Script)
				assert.Equal(t, uint64(flow.DefaultMaxTransactionGasLimit), processTx.GasLimit)
				assert.Equal(t, []flow.Address{chain.ServiceAddress()}, processTx.Authorizers)
				assert.Empty(t, processTx.Arguments)

				// Last transaction should always be the system chunk transaction
				systemChunkTx := transactions[len(transactions)-1]
				assert.NotNil(t, systemChunkTx)
				assert.NotEmpty(t, systemChunkTx.Script)
				assert.Equal(t, []flow.Address{chain.ServiceAddress()}, systemChunkTx.Authorizers)

				// Middle transactions should be execute callback transactions
				executeCount := tt.expectedTxCount - 2 // subtract process and system chunk
				if executeCount > 0 {
					for i := 1; i < len(transactions)-1; i++ {
						executeTx := transactions[i]
						assert.NotNil(t, executeTx)
						assert.NotEmpty(t, executeTx.Script)
						assert.Equal(t, []flow.Address{chain.ServiceAddress()}, executeTx.Authorizers)
						assert.Len(t, executeTx.Arguments, 1)
						assert.NotEmpty(t, executeTx.Arguments[0])
					}
				}
			}

			// Verify collection properties
			assert.NotEmpty(t, collection.ID())
			assert.Equal(t, len(transactions), len(collection.Transactions))
		})
	}
}

func createEventWithModifiedField(t *testing.T, fieldName string, newValue cadence.Value) flow.Event {
	const processedEventTypeTemplate = "A.%v.FlowTransactionScheduler.PendingExecution"
	env := systemcontracts.SystemContractsForChain(flow.Mainnet.Chain().ChainID()).AsTemplateEnv()
	eventTypeString := fmt.Sprintf(processedEventTypeTemplate, env.FlowTransactionSchedulerAddress)
	loc, err := cadenceCommon.HexToAddress(env.FlowTransactionSchedulerAddress)
	require.NoError(t, err)
	location := cadenceCommon.NewAddressLocation(nil, loc, "PendingExecution")

	fields := []cadence.Field{
		{Identifier: "id", Type: cadence.UInt64Type},
		{Identifier: "priority", Type: cadence.UInt8Type},
		{Identifier: "executionEffort", Type: cadence.UInt64Type},
		{Identifier: "callbackOwner", Type: cadence.AddressType},
	}

	values := []cadence.Value{
		cadence.NewUInt64(123),
		cadence.NewUInt8(1),
		cadence.NewUInt64(456),
		cadence.NewAddress([8]byte{}),
	}

	// Handle field modification or removal
	if newValue == nil {
		// Remove the field entirely
		var filteredFields []cadence.Field
		var filteredValues []cadence.Value
		for i, field := range fields {
			if field.Identifier != fieldName {
				filteredFields = append(filteredFields, field)
				filteredValues = append(filteredValues, values[i])
			}
		}
		fields = filteredFields
		values = filteredValues
	} else {
		// Replace the field value with wrong type
		switch fieldName {
		case "id":
			values[0] = newValue
		case "executionEffort":
			values[2] = newValue
		}
	}

	eventType := cadence.NewEventType(
		location,
		"PendingExecution",
		fields,
		nil,
	)

	event := cadence.NewEvent(values).WithType(eventType)

	payload, err := ccf.Encode(event)
	require.NoError(t, err)

	return flow.Event{
		Type:             flow.EventType(eventTypeString),
		TransactionID:    unittest.IdentifierFixture(),
		TransactionIndex: 0,
		EventIndex:       0,
		Payload:          payload,
	}
}
