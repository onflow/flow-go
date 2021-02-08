package dkg

import (
	"github.com/onflow/flow-go/model/messages"
)

type BrokerTunnel struct {
	MsgChIn  chan messages.DKGMessageIn
	MsgChOut chan messages.DKGMessageOut
}

func NewBrokerTunnel() *BrokerTunnel {
	return &BrokerTunnel{
		MsgChIn:  make(chan messages.DKGMessageIn),
		MsgChOut: make(chan messages.DKGMessageOut),
	}
}

func (t *BrokerTunnel) SendIn(msg messages.DKGMessageIn) {
	t.MsgChIn <- msg
}

func (t *BrokerTunnel) SendOut(msg messages.DKGMessageOut) {
	t.MsgChOut <- msg
}
