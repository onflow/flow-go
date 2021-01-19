package test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/network"
)

// MeshEngine is a simple engine that is used for testing the correctness of
// driving the engines with libp2p, it simply receives and stores the incoming messages
type MeshEngine struct {
	t        *testing.T
	con      network.Conduit  // used to directly communicate with the network
	originID flow.Identifier  // used to keep track of the id of the sender of the messages
	event    chan interface{} // used to keep track of the events that the node receives
	received chan struct{}    // used as an indicator on reception of messages for testing
}

func NewMeshEngine(t *testing.T, net module.Network, cap int, channel network.Channel) *MeshEngine {
	te := &MeshEngine{
		t:        t,
		event:    make(chan interface{}, cap),
		received: make(chan struct{}, cap),
	}

	c2, err := net.Register(channel, te)
	require.NoError(te.t, err)
	te.con = c2

	return te
}

// SubmitLocal is implemented for a valid type assertion to Engine
// any call to it fails the test
func (e *MeshEngine) SubmitLocal(event interface{}) {
	require.Fail(e.t, "not implemented")
}

// Submit is implemented for a valid type assertion to Engine
// any call to it fails the test
func (e *MeshEngine) Submit(originID flow.Identifier, event interface{}) {
	go func() {
		err := e.Process(originID, event)
		if err != nil {
			require.Fail(e.t, "could not process submitted event")
		}
	}()
}

// ProcessLocal is implemented for a valid type assertion to Engine
// any call to it fails the test
func (e *MeshEngine) ProcessLocal(event interface{}) error {
	require.Fail(e.t, "not implemented")
	return fmt.Errorf(" unexpected method called")
}

// Process receives an originID and an event and casts them into the corresponding fields of the
// MeshEngine. It then flags the received channel on reception of an event.
func (e *MeshEngine) Process(originID flow.Identifier, event interface{}) error {
	// stores the message locally
	e.originID = originID
	e.event <- event
	e.received <- struct{}{}
	return nil
}
