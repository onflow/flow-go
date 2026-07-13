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
	"github.com/onflow/flow-go/utils/unittest/fixtures"
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
				executor := systemcontracts.SystemContractsForChain(chain.ChainID()).ScheduledTransactionExecutor.Address
				assert.Equal(t, tx.Authorizers, []flow.Address{executor})
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

// TestProcessCallbacksTransactionHash tests that the hash of the ProcessCallbacksTransaction does not change.
func TestProcessCallbacksTransactionHash(t *testing.T) {
	t.Parallel()

	expectedHashes := []chainHash{
		{chainId: "flow-mainnet", expectedHash: "a9caece21b073a85cdfa8e27c6781426025ab67d7018b9afe388a18cc293e14f"},
		{chainId: "flow-testnet", expectedHash: "af35ecd8b485e41ed9fa68580557ced336cf46789e4c93af0378ea9812cc1a4b"},
		{chainId: "flow-previewnet", expectedHash: "7a8b24b172d27d3174cfc8ad40f4412f2483b669b05ea4b25f7cbba6a1decbfb"},
		{chainId: "flow-emulator", expectedHash: "4b58ffb851c3ce5d98922c9b3e9ab0a858de84912d1697aa38bef95e23cee5d4"},
	}

	var actualHashes []chainHash
	for _, expected := range expectedHashes {
		chain := flow.ChainID(expected.chainId)
		tx, err := blueprints.ProcessCallbacksTransaction(chain.Chain())
		require.NoError(t, err)
		actualHashes = append(actualHashes, chainHash{chainId: expected.chainId, expectedHash: tx.ID().String()})
	}

	require.Equal(t, expectedHashes, actualHashes,
		"Hashes of the ProcessCallbacksTransaction have changed.\n"+
			"Update the expected hashes with the following values:\n%s", formatHashes(actualHashes))
}

// TestExecuteCallbacksTransactionHash tests that the hash of the ExecuteCallbacksTransaction does not change
// for a given set of deterministic inputs.
func TestExecuteCallbacksTransactionHash(t *testing.T) {
	t.Parallel()

	const id = 42
	const effort = 1000

	expectedHashes := []chainHash{
		{chainId: "flow-mainnet", expectedHash: "cae4adc3eb92ee67a47754e3e7095e4402249aa482be19b800de601ce4cd0d32"},
		{chainId: "flow-testnet", expectedHash: "5ede5d3a9698685a98027a855cd2711968b21e1ed3b3e479eab8893ead883817"},
		{chainId: "flow-previewnet", expectedHash: "8fc04e2eb6672e75588ea1210be5d74b9453167dec82a0f5f78afd27a0e28f8b"},
		{chainId: "flow-emulator", expectedHash: "0ee4841eee4519049af0440ac7ab553adf4fa62d5835eed63029f73482a1ff54"},
	}

	var actualHashes []chainHash
	for _, expected := range expectedHashes {
		chain := flow.ChainID(expected.chainId)
		tx, err := blueprints.ExecuteCallbacksTransaction(chain.Chain(), id, effort)
		require.NoError(t, err)
		actualHashes = append(actualHashes, chainHash{chainId: expected.chainId, expectedHash: tx.ID().String()})
	}

	require.Equal(t, expectedHashes, actualHashes,
		"Hashes of the ExecuteCallbacksTransaction have changed.\n"+
			"Update the expected hashes with the following values:\n%s", formatHashes(actualHashes))
}

// TestExecuteCallbacksTransactionsHash tests that the hashes of transactions produced by
// ExecuteCallbacksTransactions do not change for a deterministic set of events.
func TestExecuteCallbacksTransactionsHash(t *testing.T) {
	t.Parallel()

	type chainTxHashes struct {
		chainId        string
		expectedHashes []string
	}

	expected := []chainTxHashes{
		{chainId: "flow-mainnet", expectedHashes: []string{
			"9f7294a3490c0e1967022e03b485c28d9ac846ba81cd1689339f4b30996fa8e4",
			"9a2e265f5df74caa80ef02121965b5e9f5626cee1585e3c8ac8cc84d7eceb901",
			"e5d1f9d103d6e03751d0387bddfc586d07b8b9dc5cb9fa9d145742cdcd3e8bbd",
		}},
		{chainId: "flow-testnet", expectedHashes: []string{
			"e2fc0bc9264e0a70250f69bfb0d60d8b9fb9b85292177f2ceaad75e00d03a51e",
			"852f2f4c7a62e71770845d23abec273356595aa81dd1997b22d5b626e0baf821",
			"3f8caada9afc30ca4ea85bb5181cfdad0d2f86db9767e1c3a7603c2f6bc30ee6",
		}},
		{chainId: "flow-previewnet", expectedHashes: []string{
			"e34efc26d3cfb235fb505924eaaa94e409d1aef1b0ed465d11c0a561b346a5ac",
			"07c93c8fddb9fdd4354518ae4ad4eb8131d2e8edab5bf84867cd9c4189f965fe",
			"6d25de124ba1046c11d5b4e68a01e34dbd14d1ea152e69d543f0836cfc0ce501",
		}},
		{chainId: "flow-emulator", expectedHashes: []string{
			"109794396aa22e43b9f18d955e9f8bd814727286dedd8eb0d27c9505255ee2ef",
			"71d3019b3356d2358f670ef6b6167d19ab8534d641b7b0163798300fdea59ae9",
			"286a2b217c23683d32510067ea213ca3bc693b9fb96db71f2ac07ab5cac2fb56",
		}},
	}

	for _, exp := range expected {
		chainID := flow.ChainID(exp.chainId)
		gen := fixtures.NewGeneratorSuite(fixtures.WithSeed(42), fixtures.WithChainID(chainID))
		events := gen.PendingExecutionEvents().List(3)

		txs, err := blueprints.ExecuteCallbacksTransactions(chainID.Chain(), events)
		require.NoError(t, err)
		require.Len(t, txs, len(exp.expectedHashes), "chain %s: unexpected number of transactions", exp.chainId)

		for i, tx := range txs {
			require.Equal(t, exp.expectedHashes[i], tx.ID().String(),
				"chain %s tx[%d] hash changed", exp.chainId, i)
		}
	}
}
