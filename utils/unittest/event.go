package unittest

import (
	"encoding/hex"
	"fmt"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/common"
	"github.com/onflow/cadence/encoding/ccf"

	"github.com/onflow/flow-go/model/flow"
)

var Event eventFactory

type eventFactory struct{}

// EventFixture returns an event
func EventFixture(
	eventType flow.EventType,
	transactionIndex uint32,
	eventIndex uint32,
	txID flow.Identifier,
	opts ...func(*flow.Event),
) flow.Event {
	event := &flow.Event{
		Type:             eventType,
		TransactionIndex: transactionIndex,
		EventIndex:       eventIndex,
		TransactionID:    txID,
		Payload:          RandomEventPayloadCCFFixture(),
	}

	for _, opt := range opts {
		opt(event)
	}

	return *event
}

func (f *eventFactory) WithPayload(payload []byte) func(*flow.Event) {
	return func(e *flow.Event) {
		e.Payload = payload
	}
}

func RandomEventPayloadCCFFixture() []byte {
	randomSourceHex := hex.EncodeToString(RandomBytes(100))
	b, err := ccf.Encode(createRandomEvent(randomSourceHex))
	if err != nil {
		panic(err)
	}
	_, err = ccf.Decode(nil, b)
	if err != nil {
		panic(err)
	}
	return b
}

func createRandomEvent(randomSourceHex string) cadence.Event {
	address, err := common.BytesToAddress(RandomAddressFixture().Bytes())
	if err != nil {
		panic(fmt.Sprintf("unexpected error while creating random address: %s", err))
	}
	location := common.NewAddressLocation(nil, address, "TestContract")

	testEventType := cadence.NewEventType(
		location,
		IdentifierFixture().String(),
		[]cadence.Field{
			{
				Identifier: "foo",
				Type:       cadence.StringType,
			},
		},
		nil,
	)

	return cadence.NewEvent([]cadence.Value{
		// randomSource
		cadence.String(randomSourceHex),
	}).WithType(testEventType)
}
