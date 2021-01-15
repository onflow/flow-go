package processor

import (
	"math/rand"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gotest.tools/assert"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/dkg"
	module "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/network/mocknetwork"
	"github.com/onflow/flow-go/utils/unittest"
)

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

	// initialize the list of dkg participants
	committee := unittest.IdentifierListFixture(2)

	// define epoch ID
	epochID := rand.Uint64()

	// send a msg from the first node to the second node
	orig := 0
	dest := 1
	msgb := []byte("hello world")

	msg := dkg.NewDKGMessage(
		orig,
		msgb,
		epochID,
	)

	conduit := &mocknetwork.Conduit{}
	conduit.On("Unicast", mock.Anything, mock.Anything).
		Run(
			func(args mock.Arguments) {
				request := args.Get(0).(dkg.DKGMessage)
				destID := args.Get(1).(flow.Identifier)
				assert.Equal(t, committee[dest], destID)
				assert.DeepEqual(t, msg, request)
			},
		).
		Return(nil).
		Once()

	network := new(module.Network)
	network.On("Register", mock.Anything, mock.Anything).
		Return(conduit, nil).
		Once()

	me := new(module.Local)
	me.On("NodeID").Return(committee[orig])

	origEngine, err := New(
		zerolog.Logger{},
		network,
		me,
		make(chan dkg.DKGMessage),
		committee,
		epochID,
	)
	require.NoError(t, err)

	origEngine.PrivateSend(dest, msgb)

	conduit.AssertExpectations(t)
}

// TestPrivateSend_IndexOutOfRange checks that PrivateSend discards messages if
// the message destination parameter is out of range with respect to the
// committee list.
func TestPrivateSend_IndexOutOfRange(t *testing.T) {

	// initialize the list of dkg participants
	committee := unittest.IdentifierListFixture(2)

	// define epoch ID
	epochID := rand.Uint64()

	// send a msg from the first node on the list to second node on the list
	orig := 0
	msgb := []byte("hello world")

	conduit := &mocknetwork.Conduit{}
	network := new(module.Network)
	network.On("Register", mock.Anything, mock.Anything).
		Return(conduit, nil).
		Once()

	me := new(module.Local)
	me.On("NodeID").Return(committee[orig])

	origEngine, err := New(
		zerolog.Logger{},
		network,
		me,
		make(chan dkg.DKGMessage),
		committee,
		epochID,
	)
	require.NoError(t, err)

	origEngine.PrivateSend(2, msgb)
	origEngine.PrivateSend(-1, msgb)

	// make sure the unicast method is never called
	conduit.AssertNotCalled(t, "Unicast", mock.Anything, mock.Anything)
}

// TestProcessDKGMessage_Valid checks that a valid incoming DKG message is
// correctly matched with origin's Identifier, and that the message is forwarded
// to the message channel.
func TestProcessDKGMessage_Valid(t *testing.T) {

	// initialize the list of dkg participants
	committee := unittest.IdentifierListFixture(2)

	// define epoch ID
	epochID := rand.Uint64()

	// send a msg from the first node to the second node
	orig := 0
	dest := 1
	msgb := []byte("hello world")

	msg := dkg.NewDKGMessage(
		orig,
		msgb,
		epochID,
	)

	network := new(module.Network)
	network.On("Register", mock.Anything, mock.Anything).
		Return(nil, nil).
		Once()

	me := new(module.Local)
	me.On("NodeID").Return(committee[dest])

	msgCh := make(chan dkg.DKGMessage)

	engine, err := New(
		zerolog.Logger{},
		network,
		me,
		msgCh,
		committee,
		epochID,
	)
	require.NoError(t, err)

	// launch a background routine to capture messages forwarded to the msgCh
	var receivedMsg dkg.DKGMessage
	doneCh := make(chan struct{})
	go func() {
		receivedMsg = <-msgCh
		close(doneCh)
	}()

	err = engine.Process(committee[orig], msg)
	require.NoError(t, err)

	//check that the message has been received and forwarded to the msgCh
	unittest.AssertReturnsBefore(
		t,
		func() {
			<-doneCh
		},
		time.Second)
	require.Equal(t, msg, receivedMsg)
}

// TestProcessDKGMessage checks that incoming DKG messages are discarded with an
// error if their origin is invalid, or if there is a discrepancy between the
// origin defined in the message, and the network identifier of the origin (as
// provided by the network utilities).
func TestProcessDKGMessage_InvalidOrigin(t *testing.T) {

	// initialize the list of dkg participants
	committee := unittest.IdentifierListFixture(2)

	// define epoch ID
	epochID := rand.Uint64()

	network := new(module.Network)
	network.On("Register", mock.Anything, mock.Anything).
		Return(nil, nil).
		Once()

	me := new(module.Local)
	me.On("NodeID").Return(committee[1])

	msgCh := make(chan dkg.DKGMessage)

	engine, err := New(
		zerolog.Logger{},
		network,
		me,
		msgCh,
		committee,
		epochID,
	)
	require.NoError(t, err)

	// check that the DKGMessage's Orig field is not out of index
	msg := dkg.NewDKGMessage(
		2,
		[]byte{},
		epochID,
	)

	err = engine.Process(unittest.IdentifierFixture(), msg)
	require.Error(t, err)

	msg = dkg.NewDKGMessage(
		-1,
		[]byte{},
		epochID,
	)

	err = engine.Process(unittest.IdentifierFixture(), msg)
	require.Error(t, err)

	// check that the DKGMessage's Orig field matches the sender's network
	// identifier
	msg = dkg.NewDKGMessage(
		0,
		[]byte{},
		epochID,
	)

	err = engine.Process(unittest.IdentifierFixture(), msg)
	require.Error(t, err)
}
