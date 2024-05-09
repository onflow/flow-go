package dkg

import (
	"github.com/onflow/flow-go/model/messages"
)

// BrokerTunnel allows the DKG MessagingEngine to relay messages to and from a
// loosely-coupled Broker and Controller. The same BrokerTunnel is intended
// to be reused across epochs (multiple DKG instances). The BrokerTunnel does
// not internally queue messages, so sends through the tunnel are blocking.
type BrokerTunnel struct {
	MsgChIn  chan messages.PrivDKGMessageIn  // from network engine to broker
	MsgChOut chan messages.PrivDKGMessageOut // from broker to network engine
}

// NewBrokerTunnel instantiates a new BrokerTunnel.
func NewBrokerTunnel() *BrokerTunnel {
	return &BrokerTunnel{
		MsgChIn:  make(chan messages.PrivDKGMessageIn),
		MsgChOut: make(chan messages.PrivDKGMessageOut),
	}
}

// SendIn pushes incoming messages in the MsgChIn channel to be received by the Broker.
// This is a blocking call (messages are not queued within the tunnel)
func (t *BrokerTunnel) SendIn(msg messages.PrivDKGMessageIn) {
	t.MsgChIn <- msg
}

// SendOut pushes outbound messages in the MsgChOut channel to be received and
// forwarded by the network engine.
// This is a blocking call (messages are not queued within the tunnel)
func (t *BrokerTunnel) SendOut(msg messages.PrivDKGMessageOut) {
	t.MsgChOut <- msg
}
