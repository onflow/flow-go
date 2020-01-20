package test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/flow"
	libp2pmodel "github.com/dapperlabs/flow-go/model/libp2p/message"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/network"
)

// StubEngine is a simple engine that is used for testing the correctness of
// driving the engines with libp2p
type StubEngine struct {
	t        *testing.T
	con      network.Conduit  // used to directly communicate with the network
	originID flow.Identifier  // used to keep track of the id of the sender of the messages
	event    chan interface{} // used to keep track of the events that the node receives
	received chan struct{}    // used as an indicator on reception of messages for testing
	echomsg  string           // used as a fix string to be included in the reply echos

}

func NewEngine(t *testing.T, net module.Network, cap int, engineID uint8) *StubEngine {
	te := &StubEngine{
		t:        t,
		echomsg:  "this is an echo",
		event:    make(chan interface{}, cap),
		received: make(chan struct{}, cap),
	}

	c2, err := net.Register(engineID, te)
	require.NoError(te.t, err)
	te.con = c2

	return te
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
	lip2pEvent, ok := (event).(*libp2pmodel.Echo)
	require.True(te.t, ok, "could not assert event as Echo")
	strEvent := lip2pEvent.Text
	// avoids endless circulation of echos by filtering echoing back the echo messages
	if strings.HasPrefix(strEvent, te.echomsg) {
		return nil
	}

	// sends a echo back
	msg := &libp2pmodel.Echo{
		Text: fmt.Sprintf("%s: %s", te.echomsg, strEvent),
	}
	err := te.con.Submit(msg, originID)
	require.NoError(te.t, err, "could not submit echo back to network ")

	return nil
}
