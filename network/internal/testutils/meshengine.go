package testutils

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	mockcomponent "github.com/onflow/flow-go/module/component/mock"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
)

// MeshEngine is a simple engine that is used for testing the correctness of
// driving the engines with libp2p, it simply receives and stores the incoming messages
type MeshEngine struct {
	sync.Mutex
	t        *testing.T
	Con      network.Conduit       // used to directly communicate with the network
	originID flow.Identifier       // used to keep track of the id of the sender of the messages
	Event    chan interface{}      // used to keep track of the events that the node receives
	Channel  chan channels.Channel // used to keep track of the channels that events are Received on
	Received chan struct{}         // used as an indicator on reception of messages for testing
	mockcomponent.Component
}

func NewMeshEngine(t *testing.T, net network.EngineRegistry, cap int, channel channels.Channel) *MeshEngine {
	te := &MeshEngine{
		t:        t,
		Event:    make(chan interface{}, cap),
		Channel:  make(chan channels.Channel, cap),
		Received: make(chan struct{}, cap),
	}

	c2, err := net.Register(channel, te)
	require.NoError(te.t, err)
	te.Con = c2

	return te
}

// SubmitLocal is implemented for a valid type assertion to Engine
// any call to it fails the test
func (e *MeshEngine) SubmitLocal(event interface{}) {
	require.Fail(e.t, "not implemented")
}

// Submit is implemented for a valid type assertion to Engine
// any call to it fails the test
func (e *MeshEngine) Submit(channel channels.Channel, originID flow.Identifier, event interface{}) {
	go func() {
		err := e.Process(channel, originID, event)
		if err != nil {
			require.Fail(e.t, "could not process submitted Event")
		}
	}()
}

// ProcessLocal is implemented for a valid type assertion to Engine
// any call to it fails the test
func (e *MeshEngine) ProcessLocal(event interface{}) error {
	require.Fail(e.t, "not implemented")
	return fmt.Errorf(" unexpected method called")
}

// Process receives an originID and an Event and casts them into the corresponding fields of the
// MeshEngine. It then flags the Received Channel on reception of an Event.
func (e *MeshEngine) Process(channel channels.Channel, originID flow.Identifier, event interface{}) error {
	e.Lock()
	defer e.Unlock()

	// stores the message locally
	e.originID = originID
	e.Channel <- channel
	e.Event <- event
	e.Received <- struct{}{}
	return nil
}
