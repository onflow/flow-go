package test

import (
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
func (c *ConduitWrapper) Publish(msg interface{}, conduit network.Conduit) error {
	return conduit.Publish(msg)
}
