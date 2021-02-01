package processor

import (
	"math/rand"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
	msg "github.com/onflow/flow-go/model/messages"
	module "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/network/mocknetwork"
	"github.com/onflow/flow-go/utils/unittest"
)

// variables that are used throughout the tests
var (
	committee = unittest.IdentifierListFixture(2) // dkg nodes
	orig      = 0                                 // message sender
	dest      = 1                                 // message destination
	msgb      = []byte("hello world")             // message content
)

// Helper function to initialise an engine with the default committee, a mock
// conduit for private messages, and a mock dkg contract client for broadcast
// messages.
func createTestEngine(t *testing.T, nodeID flow.Identifier) *Engine {

	// define epoch ID
	epochCounter := rand.Uint64()

	// setup mock conduit
	conduit := &mocknetwork.Conduit{}
	network := new(module.Network)
	network.On("Register", mock.Anything, mock.Anything).
		Return(conduit, nil).
		Once()

		// setup local with nodeID
	me := new(module.Local)
	me.On("NodeID").Return(nodeID)

	engine, err := New(
		zerolog.Logger{},
		network,
		me,
		make(chan msg.DKGMessage),
		committee,
		&module.DKGContractClient{},
		epochCounter,
	)
	require.NoError(t, err)

	return engine
}

// TestImplementsDKGProcessor ensures that Engine implements the DKGProcessor
// interface of the crypto package.
func TestImplementsDKGProcessor(t *testing.T) {
	var _ crypto.DKGProcessor = (*Engine)(nil)
}

// TestPrivateSend_Valid checks that the processor correctly converts the
// message destination parameter (index in committee list) to the corresponding
// public Identifier, and successfully sends a DKG message to the intended
// recipient through the network conduit.
func TestPrivateSend_Valid(t *testing.T) {

	// sender engine
	engine := createTestEngine(t, committee[orig])

	// expected DKGMessage
	expectedMsg := msg.NewDKGMessage(
		orig,
		msgb,
		engine.GetEpoch(),
		engine.GetPhase(),
	)

	// override the conduit to check that the Unicast call matches the expected
	// message and destination ID
	conduit := &mocknetwork.Conduit{}
	conduit.On("Unicast", expectedMsg, committee[dest]).
		Return(nil).
		Once()
	engine.conduit = conduit

	engine.PrivateSend(dest, msgb)
	conduit.AssertExpectations(t)
}

// TestPrivateSend_IndexOutOfRange checks that PrivateSend discards messages if
// the message destination parameter is out of range with respect to the
// committee list.
func TestPrivateSend_IndexOutOfRange(t *testing.T) {

	// sender engine
	engine := createTestEngine(t, committee[orig])

	// override the conduit to check that Unicast is never called
	conduit := &mocknetwork.Conduit{}
	conduit.On("Unicast", mock.Anything, mock.Anything).
		Return(nil)
	engine.conduit = conduit

	// try providing destination indexes that are out of range
	engine.PrivateSend(2, msgb)
	engine.PrivateSend(-1, msgb)

	// make sure the unicast method is never called
	conduit.AssertNotCalled(t, "Unicast", mock.Anything, mock.Anything)
}

// TestProcessMessage_Valid checks that a valid incoming DKG message is
// correctly matched with origin's Identifier, and that the message is forwarded
// to the message channel.
func TestProcessMessage_Valid(t *testing.T) {

	// destination engine
	engine := createTestEngine(t, committee[dest])

	expectedMsg := msg.NewDKGMessage(
		orig,
		msgb,
		engine.GetEpoch(),
		engine.GetPhase(),
	)

	// launch a background routine to capture messages forwarded to the msgCh
	var receivedMsg msg.DKGMessage
	doneCh := make(chan struct{})
	go func() {
		receivedMsg = <-engine.msgCh
		close(doneCh)
	}()

	err := engine.Process(committee[orig], expectedMsg)
	require.NoError(t, err)

	// check that the message has been received and forwarded to the msgCh
	unittest.AssertReturnsBefore(
		t,
		func() {
			<-doneCh
		},
		time.Second)
	require.Equal(t, expectedMsg, receivedMsg)
}

// TestProcessMessage checks that incoming DKG messages are discarded with an
// error if their origin is invalid, or if there is a discrepancy between the
// origin defined in the message, and the network identifier of the origin (as
// provided by the network utilities).
func TestProcessMessage_InvalidOrigin(t *testing.T) {

	// destination engine
	engine := createTestEngine(t, committee[dest])

	// check that the Message's Orig field is not out of index
	badIndexes := []int{-1, 2}
	for _, badIndex := range badIndexes {
		dkgMsg := msg.NewDKGMessage(
			badIndex,
			msgb,
			engine.GetEpoch(),
			engine.GetPhase(),
		)
		err := engine.Process(unittest.IdentifierFixture(), dkgMsg)
		require.Error(t, err)
	}

	// check that the Message's Orig field matches the sender's network
	// identifier
	dkgMsg := msg.NewDKGMessage(
		orig,
		msgb,
		engine.GetEpoch(),
		engine.GetPhase(),
	)
	err := engine.Process(unittest.IdentifierFixture(), dkgMsg)
	require.Error(t, err)
}

// TestBroadcastMessage checks that the processor correctly wraps the message
// data in a DKGMessage (with origin and epochCounter), and that it calls the
// dkg contract client.
func TestBroadcastMessage(t *testing.T) {

	// sender engine
	engine := createTestEngine(t, committee[orig])

	expectedMsg := msg.NewDKGMessage(
		orig,
		msgb,
		engine.GetEpoch(),
		engine.GetPhase(),
	)

	// check that the dkg contract client is called with the expected message
	contractClient := &module.DKGContractClient{}
	contractClient.On("Broadcast", expectedMsg).
		Return(nil).
		Once()
	engine.dkgContractClient = contractClient

	engine.Broadcast(msgb)
	contractClient.AssertExpectations(t)
}

// TestReadMessages checks that the engine correctly calls the smart contract
// to fetch broadcast messages, and forwards the messages to the msgCh.
func TestReadMessages(t *testing.T) {

	engine := createTestEngine(t, committee[orig])
	blockID := unittest.IdentifierFixture()
	expectedMsgs := []msg.DKGMessage{
		msg.NewDKGMessage(
			orig,
			msgb,
			engine.GetEpoch(),
			engine.GetPhase(),
		),
		msg.NewDKGMessage(
			orig,
			msgb,
			engine.GetEpoch(),
			engine.GetPhase(),
		),
		msg.NewDKGMessage(
			orig,
			msgb,
			engine.GetEpoch(),
			engine.GetPhase(),
		),
	}

	// check that the dkg contract client is called correctly
	contractClient := &module.DKGContractClient{}
	contractClient.On("ReadBroadcast", blockID, engine.GetEpoch(), engine.GetPhase(), engine.GetOffset()).
		Return(expectedMsgs, nil).
		Once()
	engine.dkgContractClient = contractClient

	// launch a background routine to capture messages forwarded to the msgCh
	receivedMsgs := []msg.DKGMessage{}
	doneCh := make(chan struct{})
	go func() {
		for {
			msg := <-engine.msgCh
			receivedMsgs = append(receivedMsgs, msg)
			if len(receivedMsgs) == len(expectedMsgs) {
				close(doneCh)
			}
		}
	}()

	err := engine.fetchBroadcastMessages(blockID)
	require.NoError(t, err)

	// check that the contract has been correctly called
	contractClient.AssertExpectations(t)

	// check that the messages have been received and forwarded to the msgCh
	unittest.AssertReturnsBefore(
		t,
		func() {
			<-doneCh
		},
		time.Second)
	require.Equal(t, expectedMsgs, receivedMsgs)
}
