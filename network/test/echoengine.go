package test

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/libp2p/message"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/network"
)

// EchoEngine is a simple engine that is used for testing the correctness of
// driving the engines with libp2p, in addition to receiving and storing incoming messages
// it also echos them back
type EchoEngine struct {
	sync.RWMutex
	t        *testing.T
	con      network.Conduit        // used to directly communicate with the network
	originID flow.Identifier        // used to keep track of the id of the sender of the messages
	event    chan interface{}       // used to keep track of the events that the node receives
	channel  chan network.Channel   // used to keep track of the channels that events are received on
	received chan struct{}          // used as an indicator on reception of messages for testing
	echomsg  string                 // used as a fix string to be included in the reply echos
	seen     map[string]int         // used to track the seen events
	echo     bool                   // used to enable or disable echoing back the recvd message
	send     ConduitSendWrapperFunc // used to provide play and plug wrapper around its conduit
}

func NewEchoEngine(t *testing.T, net module.Network, cap int, channel network.Channel, echo bool, send ConduitSendWrapperFunc) *EchoEngine {
	te := &EchoEngine{
		t:        t,
		echomsg:  "this is an echo",
		event:    make(chan interface{}, cap),
		channel:  make(chan network.Channel, cap),
		received: make(chan struct{}, cap),
		seen:     make(map[string]int),
		echo:     echo,
		send:     send,
	}

	c2, err := net.Register(channel, te)
	require.NoError(te.t, err)
	te.con = c2

	return te
}

// SubmitLocal is implemented for a valid type assertion to Engine
// any call to it fails the test
func (te *EchoEngine) SubmitLocal(event interface{}) {
	require.Fail(te.t, "not implemented")
}

// Submit is implemented for a valid type assertion to Engine
// any call to it fails the test
func (te *EchoEngine) Submit(channel network.Channel, originID flow.Identifier, event interface{}) {
	go func() {
		err := te.Process(channel, originID, event)
		if err != nil {
			require.Fail(te.t, "could not process submitted event")
		}
	}()
}

// ProcessLocal is implemented for a valid type assertion to Engine
// any call to it fails the test
func (te *EchoEngine) ProcessLocal(event interface{}) error {
	require.Fail(te.t, "not implemented")
	return fmt.Errorf(" unexpected method called")
}

// Process receives an originID and an event and casts them into the corresponding fields of the
// EchoEngine. It then flags the received channel on reception of an event.
// It also sends back an echo of the message to the origin ID
func (te *EchoEngine) Process(channel network.Channel, originID flow.Identifier, event interface{}) error {
	te.Lock()
	defer te.Unlock()
	te.originID = originID
	te.event <- event
	te.channel <- channel
	te.received <- struct{}{}

	// asserting event as string
	lip2pEvent, ok := (event).(*message.TestMessage)
	require.True(te.t, ok, "could not assert event as TestMessage")

	// checks for duplication
	strEvent := lip2pEvent.Text

	// marks event as seen
	te.seen[strEvent]++

	// avoids endless circulation of echos by filtering echoing back the echo messages
	if strings.HasPrefix(strEvent, te.echomsg) {
		return nil
	}

	// do not echo back if not needed
	if !te.echo {
		return nil
	}

	// sends a echo back
	msg := &message.TestMessage{
		Text: fmt.Sprintf("%s: %s", te.echomsg, strEvent),
	}

	// Sends the message through the conduit's send wrapper
	// Note: instead of directly interacting with the conduit, we use
	// the wrapper function here, to enable this same implementation
	// gets tested with different Conduit methods, i.e., publish, multicast, and unicast.
	err := te.send(msg, te.con, originID)
	// we allow one dial failure on echo due to connection tear down
	// specially when the test is for a single message and not echo
	// the sender side may close the connection as it does not expect any echo
	if err != nil && !strings.Contains(err.Error(), "failed to dial") {
		require.Fail(te.t, fmt.Sprintf("could not submit echo back to network: %s", err))
	}
	return nil
}
