package libp2p

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// TransmitFunc is a function that reliably sends the specified message
// to the specified recipients over the specified channel.
type TransmitFunc func(channelID uint8, message interface{}, recipientIDs ...flow.Identifier) error

// SendFunc is a function that reliably sends the specified message to
// the specified number of recipients selected from the specified subset.
type SendFunc func(channelID uint8, message interface{}, num uint, selector flow.IdentityFilter) error

// PublishFunc is a function that reliably broadcasts the specified message
// to all participants on the given channel.
type PublishFunc func(channelID uint8, message interface{}, selector flow.IdentityFilter) error

// Conduit is a helper of the overlay layer which functions as an accessor for
// sending messages within a single engine process. It sends all messages to
// what can be considered a bus reserved for that specific engine.
type Conduit struct {
	channelID uint8
	transmit  TransmitFunc
	send      SendFunc
	publish   PublishFunc
}

func (c *Conduit) Transmit(message interface{}, recipientIDs ...flow.Identifier) error {
	return c.transmit(c.channelID, message, recipientIDs...)
}

func (c *Conduit) Send(message interface{}, num uint, selector flow.IdentityFilter) error {
	return c.send(c.channelID, message, num, selector)
}

func (c *Conduit) Publish(message interface{}, selector flow.IdentityFilter) error {
	return c.publish(c.channelID, message)
}
