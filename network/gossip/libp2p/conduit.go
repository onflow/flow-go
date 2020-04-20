package libp2p

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// SubmitFunc is a function that submits the given event for the given engine to
// the overlay network, which should take care of delivering it to the given
// recipients.
type SubmitFunc func(uint8, interface{}, ...flow.Identifier) error

// Conduit is a helper of the overlay layer which functions as an accessor for
// sending messages within a single engine process. It sends all messages to
// what can be considered a bus reserved for that specific engine.
type Conduit struct {
	channelID uint8
	submit    SubmitFunc
}

// Submit will submit a message for delivery on the engine bus that is reserved
// for messages of the engine it was initialized with.
func (c *Conduit) Submit(event interface{}, targetIDs ...flow.Identifier) error {
	return c.submit(c.channelID, event, targetIDs...)
}
