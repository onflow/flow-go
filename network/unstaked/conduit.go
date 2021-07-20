package unstaked

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network"
)

type UnstakedConduit struct {
	network.Conduit
	stakedNodeID flow.Identifier
}

func (c *UnstakedConduit) Publish(event interface{}, targetIDs ...flow.Identifier) error {
	return c.Conduit.Unicast(event, c.stakedNodeID)
}

func (c *UnstakedConduit) Unicast(event interface{}, targetID flow.Identifier) error {
	return c.Conduit.Unicast(event, c.stakedNodeID)
}

func (c *UnstakedConduit) Multicast(event interface{}, num uint, targetIDs ...flow.Identifier) error {
	return c.Conduit.Unicast(event, c.stakedNodeID)
}
