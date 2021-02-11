package dkg

import (
	"github.com/onflow/flow-go/model/messages"
)

// BrokerTunnel allows the DKG MessagingEngine to relay messages to and from a
// loosely-coupled Broker and Controller. The same BrokerTunnel is intended
// to be reused across epochs.
type BrokerTunnel struct {
	MsgChIn  chan messages.DKGMessageIn  // from network engine to broker
	MsgChOut chan messages.DKGMessageOut // from broker to network engine
}

// NewBrokerTunnel instantiates a new BrokerTunnel
func NewBrokerTunnel() *BrokerTunnel {
	return &BrokerTunnel{
		MsgChIn:  make(chan messages.DKGMessageIn),
		MsgChOut: make(chan messages.DKGMessageOut),
	}
}

// SendIn pushes incoming messages in the MsgChIn channel to be received by the
// Broker.
func (t *BrokerTunnel) SendIn(msg messages.DKGMessageIn) {
	t.MsgChIn <- msg
}

// SendOut pushes outcoing messages in the MsgChOut channel to be received and
// forwarded by the network engine.
func (t *BrokerTunnel) SendOut(msg messages.DKGMessageOut) {
	t.MsgChOut <- msg
}
