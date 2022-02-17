package insecure

import (
	"context"

	"github.com/onflow/flow-go/insecure/proto"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network"
)

// Observer defines part of the behavior of a corruptible conduit factory that allows the conduits it generates pass messages to it.
type Observer interface {
	// Observe is supposed to use by a corruptible conduit to send a received event upward to the factory that
	// originally created that conduit.
	// Boolean return value determines whether the event has successfully processed.
	Observe(context.Context, interface{}, network.Channel, proto.Protocol, uint32, ...flow.Identifier) (bool, error)
}
