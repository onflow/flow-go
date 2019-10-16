// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package overlay

// SendFunc is a function that submits the given event for the given engine to
// the overlay network, which should take care of delivering it to the given
// recipients.
type SendFunc func(uint8, interface{}, ...string) error

// Conduit is a helper of the overlay layer which functions as an accessor for
// sending messages within a single engine process. It sends all messages to
// what can be considered a bus reserved for that specific engine.
type Conduit struct {
	code uint8
	send SendFunc
}

// Send will submit a message for delivery on the engine bus that is reserved
// for messages of the engine it was initialized with.
func (c *Conduit) Send(event interface{}, nodes ...string) error {
	return c.send(c.code, event, nodes...)
}
