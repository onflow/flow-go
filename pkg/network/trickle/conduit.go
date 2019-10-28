// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package trickle

// Conduit is a helper of the overlay layer which functions as an accessor for
// sending messages within a single engine process. It sends all messages to
// what can be considered a bus reserved for that specific engine.
type Conduit struct {
	engineID uint8
	send     SendFunc
}

// SendFunc is a function that submits the given event for the given engine to
// the overlay network, which should take care of delivering it to the given
// recipients.
type SendFunc func(uint8, interface{}, ...string) error

// Send will submit a message for delivery on the engine bus that is reserved
// for messages of the engine it was initialized with.
func (c *Conduit) Send(event interface{}, targetIDs ...string) error {
	return c.send(c.engineID, event, targetIDs...)
}
