package mqueue

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

type Message interface {
	Payload() flow.Entity
}

type message struct {
	payload flow.Entity
}

func (m *message) Payload() flow.Entity {
	return m.payload
}

func FakeMessage() Message {
	return &message{
		payload: unittest.CollectionFixture(1),
	}
}
