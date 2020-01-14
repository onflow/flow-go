package test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/flow"
	libp2p2 "github.com/dapperlabs/flow-go/network/gossip/libp2p"
)

// StubEngine is a simple engine that is used for testing the correctness of
// driving the engines with libp2p
type StubEngine struct {
	t        *testing.T
	net      libp2p2.Network // used to communicate with the network layer
	con      libp2p2.Conduit // used to directly communicate with the network
	originID flow.Identifier
	event    chan interface{} // used to keep track of the events that the node receives
	received chan struct{}    // used as an indicator on reception of messages for testing
	echomsg  string           // used as a fix string to be included in the reply echos
}

// SubmitLocal is implemented for a valid type assertion to Engine
// any call to it fails the test
func (te *StubEngine) SubmitLocal(event interface{}) {
	require.Fail(te.t, "not implemented")
}

// Submit is implemented for a valid type assertion to Engine
// any call to it fails the test
func (te *StubEngine) Submit(originID flow.Identifier, event interface{}) {
	require.Fail(te.t, "not implemented")
}

// ProcessLocal is implemented for a valid type assertion to Engine
// any call to it fails the test
func (te *StubEngine) ProcessLocal(event interface{}) error {
	require.Fail(te.t, "not implemented")
	return fmt.Errorf(" unexpected method called")
}

// Process receives an originID and an event and casts them into the corresponding fields of the
// StubEngine. It then flags the received channel on reception of an event.
// It also sends back an echo of the message to the origin ID
func (te *StubEngine) Process(originID flow.Identifier, event interface{}) error {
	// stores the message locally
	te.originID = originID
	te.event <- event
	te.received <- struct{}{}

	// asserting event as string
	strEvent, ok := event.(string)
	require.True(te.t, ok, "could not assert event as string")

	// sends a echo back
	msg := fmt.Sprintf("%s: %s", te.echomsg, strEvent)
	err := te.con.Submit(msg, originID)
	require.NoError(te.t, err, "could not submit echo back to network ")

	return nil
}
