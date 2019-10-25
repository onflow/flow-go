package mocks

import (
	"github.com/dapperlabs/flow-go/pkg/network/trickle"
)

type MockConduit struct {
	engineID uint8
	send     trickle.SendFunc
}

func (c *MockConduit) Send(event interface{}, targetIDs ...string) error {
	return c.send(c.engineID, event, targetIDs...)
}
