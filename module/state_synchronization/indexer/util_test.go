package indexer

import (
	"testing"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/encoding/ccf"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/stdlib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/onflow/flow-go/utils/unittest/generator"
	"github.com/onflow/flow/protobuf/go/flow/entities"
)

// TestFindContractUpdates tests the findContractUpdates function returns all contract updates
func TestFindContractUpdates(t *testing.T) {
	t.Parallel()

	g := generator.EventGenerator(generator.WithEncoding(entities.EventEncodingVersion_CCF_V0))

	expectedAddress1 := unittest.RandomAddressFixture()
	expected1 := common.NewAddressLocation(nil, common.Address(expectedAddress1), "TestContract1")

	expectedAddress2 := unittest.RandomAddressFixture()
	expected2 := common.NewAddressLocation(nil, common.Address(expectedAddress2), "TestContract2")

	events := []flow.Event{
		g.New(), // random event
		contractUpdatedFixture(
			t,
			expected1.Address,
			expected1.Name,
		),
		g.New(), // random event
		g.New(), // random event
		contractUpdatedFixture(
			t,
			expected2.Address,
			expected2.Name,
		),
	}

	updates, err := findContractUpdates(events)
	require.NoError(t, err)

	assert.Len(t, updates, 2)

	_, ok := updates[expected1.ID()]
	assert.Truef(t, ok, "could not find %s", expected1.ID())

	_, ok = updates[expected2.ID()]
	assert.Truef(t, ok, "could not find %s", expected2.ID())
}

func contractUpdatedFixture(t *testing.T, address common.Address, contractName string) flow.Event {
	contractUpdateEventType := &cadence.EventType{
		Location:            stdlib.AccountContractAddedEventType.Location,
		QualifiedIdentifier: stdlib.AccountContractAddedEventType.QualifiedIdentifier(),
		Fields: []cadence.Field{
			{
				Identifier: "address",
				Type:       cadence.AddressType{},
			},
			{
				Identifier: "codeHash",
				Type:       cadence.AddressType{}, // actually a byte slice, but we're ignoring it anyway
			},
			{
				Identifier: "contract",
				Type:       cadence.StringType{},
			},
		},
	}

	contractString, err := cadence.NewString(contractName)
	require.NoError(t, err)

	testEvent := cadence.NewEvent(
		[]cadence.Value{
			cadence.NewAddress(address),
			cadence.NewAddress(flow.EmptyAddress),
			contractString,
		}).WithType(contractUpdateEventType)

	payload, err := ccf.Encode(testEvent)
	require.NoError(t, err)

	return flow.Event{
		Type:             accountContractUpdated,
		TransactionID:    unittest.IdentifierFixture(),
		TransactionIndex: 0,
		EventIndex:       0,
		Payload:          payload,
	}
}
