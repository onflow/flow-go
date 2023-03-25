package network

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/mocknetwork"
)

type Conduit struct {
	mocknetwork.Conduit
	net     *Network
	channel channels.Channel
}

var _ network.Conduit = (*Conduit)(nil)

// Publish sends a message on this mock network, invoking any callback that has
// been specified. This will panic if no callback is found.
func (c *Conduit) Publish(event interface{}, targetIDs ...flow.Identifier) error {
	if c.net.publishFunc != nil {
		return c.net.publishFunc(c.channel, event, targetIDs...)
	}
	panic("Publish called but no callback function was found.")
}

// ReportMisbehavior reports the misbehavior of a node on sending a message to the current node that appears valid
// based on the networking layer but is considered invalid by the current node based on the Flow protocol.
// This method is a no-op in the test helper implementation.
func (c *Conduit) ReportMisbehavior(_ network.MisbehaviorReport) {
	// no-op
}
