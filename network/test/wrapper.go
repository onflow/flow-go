package test

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network"
)

// ConduitSendWrapperFunc is a wrapper around the set of methods offered by the
// Conduit (e.g., Publish). This data type is solely introduced at the test level.
// Its primary purpose is to make the same test reusable on different Conduit methods.
type ConduitSendWrapperFunc func(msg interface{}, conduit network.Conduit, targetIDs ...flow.Identifier) error

type ConduitWrapper struct{}

// Publish defines a function that receives a message, conduit of an engine instance, and
// a set target IDs. It then sends the message to the target IDs using the Publish method of conduit.
func (c *ConduitWrapper) Publish(msg interface{}, conduit network.Conduit, targetIDs ...flow.Identifier) error {
	return conduit.Publish(msg, targetIDs...)
}

// Unicast defines a function that receives a message, conduit of an engine instance, and
// a set of target IDs. It then sends the message to the target IDs using individual Unicasts to each
// target in the underlying network.
func (c *ConduitWrapper) Unicast(msg interface{}, conduit network.Conduit, targetIDs ...flow.Identifier) error {
	for _, id := range targetIDs {
		if err := conduit.Unicast(msg, id); err != nil {
			return fmt.Errorf("could not unicast to node ID %x: %w", id, err)
		}
	}
	return nil
}

// Multicast defines a function that receives a message, conduit of an engine instance, and
// a set of target ID. It then sends the message to the target IDs using the Multicast method of conduit.
func (c *ConduitWrapper) Multicast(msg interface{}, conduit network.Conduit, targetIDs ...flow.Identifier) error {
	return conduit.Multicast(msg, uint(len(targetIDs)), targetIDs...)
}
