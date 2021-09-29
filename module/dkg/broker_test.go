package dkg

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	msg "github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/local"
	"github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

// variables that are used throughout the tests
var (
	orig          = 0                     // message sender
	dest          = 1                     // message destination
	msgb          = []byte("hello world") // message content
	dkgInstanceID = "flow-testnet-42"     // dkg instance identifier
)

func initCommittee(n int) (identities flow.IdentityList, locals []module.Local) {
	privateStakingKeys, _ := unittest.StakingKeys(n)
	for i, key := range privateStakingKeys {
		id := unittest.IdentityFixture(unittest.WithStakingPubKey(key.PublicKey()))
		identities = append(identities, id)
		local, _ := local.New(id, privateStakingKeys[i])
		locals = append(locals, local)
	}
	return identities, locals
}

// TestImplementsDKGBroker ensures that Broker implements the DKGBroker
// interface.
func TestImplementsDKGBroker(t *testing.T) {
	var _ module.DKGBroker = (*Broker)(nil)
}

// TestPrivateSend_Valid checks that the broker correctly converts the message
// destination parameter (index in committee list) to the corresponding
// public Identifier, and successfully sends a DKG message to the intended
// recipient through the tunnel.
func TestPrivateSend_Valid(t *testing.T) {
	committee, locals := initCommittee(2)

	// sender broker
	sender := NewBroker(
		zerolog.Logger{},
		dkgInstanceID,
		committee,
		locals[orig],
		orig,
		&mock.DKGContractClient{},
		NewBrokerTunnel(),
	)

	// expected DKGMessageOut
	expectedMsg := msg.PrivDKGMessageOut{
		DKGMessage: msg.NewDKGMessage(
			orig,
			msgb,
			dkgInstanceID,
		),
		DestID: committee[dest].NodeID,
	}

	// launch a background routine to capture messages sent through the tunnel,
	// and require that the expected message is sent withing 1 second.
	doneCh := make(chan struct{})
	go func() {
		msg := <-sender.tunnel.MsgChOut
		require.Equal(t, expectedMsg, msg)
		close(doneCh)

	}()

	sender.PrivateSend(dest, msgb)

	unittest.RequireCloseBefore(t, doneCh, 50*time.Millisecond, "message not sent")
}

// TestPrivateSend_IndexOutOfRange checks that PrivateSend discards messages if
// the message destination parameter is out of range with respect to the
// committee list.
func TestPrivateSend_IndexOutOfRange(t *testing.T) {
	committee, locals := initCommittee(2)

	// sender broker
	sender := NewBroker(
		zerolog.Logger{},
		dkgInstanceID,
		committee,
		locals[orig],
		orig,
		&mock.DKGContractClient{},
		NewBrokerTunnel(),
	)

	// Launch a background routine to capture messages sent through the tunnel.
	// No messages should be received because we are only sending invalid ones.
	doneCh := make(chan struct{})
	go func() {
		for {
			<-sender.tunnel.MsgChOut
			close(doneCh)
		}
	}()

	// try providing destination indexes that are out of range
	sender.PrivateSend(2, msgb)
	sender.PrivateSend(-1, msgb)

	unittest.RequireNeverClosedWithin(t, doneCh, 50*time.Millisecond, "no invalid message should be sent")
}

// TestReceivePrivateMessage_Valid checks that a valid incoming DKG message is
// correctly matched with origin's Identifier, and that the message is forwarded
// to the message channel.
func TestReceivePrivateMessage_Valid(t *testing.T) {
	committee, locals := initCommittee(2)

	// receiving broker
	receiver := NewBroker(
		zerolog.Logger{},
		dkgInstanceID,
		committee,
		locals[dest],
		dest,
		&mock.DKGContractClient{},
		NewBrokerTunnel(),
	)

	expectedMsg := msg.NewDKGMessage(
		orig,
		msgb,
		dkgInstanceID,
	)

	// launch a background routine to capture messages forwared to the private
	// message channel
	doneCh := make(chan struct{})
	go func() {
		msgCh := receiver.GetPrivateMsgCh()
		for {
			msg := <-msgCh
			require.Equal(t, expectedMsg, msg)
			close(doneCh)
		}
	}()

	// simulate receiving an incoming message through the broker
	receiver.tunnel.SendIn(
		msg.PrivDKGMessageIn{
			DKGMessage: expectedMsg,
			OriginID:   committee[orig].NodeID,
		},
	)

	unittest.RequireCloseBefore(t, doneCh, 50*time.Millisecond, "message not received")
}

// TestProcessPrivateMessage_InvalidOrigin checks that incoming DKG messages are
// discarded if their origin is invalid, or if there is a discrepancy between
// the origin defined in the message, and the network identifier of the origin
// (as provided by the network utilities).
func TestProcessPrivateMessage_InvalidOrigin(t *testing.T) {
	committee, locals := initCommittee(2)

	// receiving broker
	receiver := NewBroker(
		zerolog.Logger{},
		dkgInstanceID,
		committee,
		locals[dest],
		dest,
		&mock.DKGContractClient{},
		NewBrokerTunnel(),
	)

	// Launch a background routine to capture messages forwared to the private
	// message channel. No messages should be received because we are only
	// sending invalid ones.
	doneCh := make(chan struct{})
	go func() {
		msgCh := receiver.GetPrivateMsgCh()
		for {
			<-msgCh
			close(doneCh)
		}
	}()

	// check that the Message's Orig field is not out of index
	badIndexes := []int{-1, 2}
	for _, badIndex := range badIndexes {
		dkgMsg := msg.NewDKGMessage(
			badIndex,
			msgb,
			dkgInstanceID,
		)
		// simulate receiving an incoming message with bad Origin index field
		// through the broker
		receiver.tunnel.SendIn(
			msg.PrivDKGMessageIn{
				DKGMessage: dkgMsg,
				OriginID:   committee[orig].NodeID,
			},
		)
	}

	// check that the Message's Orig field matches the sender's network
	// identifier
	dkgMsg := msg.NewDKGMessage(
		orig,
		msgb,
		dkgInstanceID,
	)
	// simulate receiving an incoming message through the broker
	receiver.tunnel.SendIn(
		msg.PrivDKGMessageIn{
			DKGMessage: dkgMsg,
			OriginID:   unittest.IdentifierFixture(),
		},
	)

	unittest.RequireNeverClosedWithin(t, doneCh, 50*time.Millisecond, "no invalid incoming message should be forwarded")
}

// TestBroadcastMessage checks that the broker correctly wraps the message
// data in a DKGMessage (with origin and epochCounter), and that it calls the
// dkg contract client.
func TestBroadcastMessage(t *testing.T) {
	committee, locals := initCommittee(2)

	// sender
	sender := NewBroker(
		zerolog.Logger{},
		dkgInstanceID,
		committee,
		locals[orig],
		orig,
		&mock.DKGContractClient{},
		NewBrokerTunnel(),
	)

	expectedMsg, err := sender.prepareBroadcastMessage(msgb)
	require.NoError(t, err)

	// check that the dkg contract client is called with the expected message
	contractClient := &mock.DKGContractClient{}
	contractClient.On("Broadcast", expectedMsg).
		Return(nil).
		Once()
	sender.dkgContractClient = contractClient

	sender.Broadcast(msgb)
	unittest.AssertClosesBefore(t, sender.unit.Done(), time.Second)
	contractClient.AssertExpectations(t)
}

// TestPoll checks that the broker correctly calls the smart contract to fetch
// broadcast messages, and forwards the messages to the broadcast channel.
func TestPoll(t *testing.T) {
	committee, locals := initCommittee(2)

	sender := NewBroker(
		zerolog.Logger{},
		dkgInstanceID,
		committee,
		locals[orig],
		orig,
		&mock.DKGContractClient{},
		NewBrokerTunnel(),
	)

	recipient := NewBroker(
		zerolog.Logger{},
		dkgInstanceID,
		committee,
		locals[dest],
		dest,
		&mock.DKGContractClient{},
		NewBrokerTunnel(),
	)

	blockID := unittest.IdentifierFixture()
	bcastMsgs := []msg.BroadcastDKGMessage{}
	expectedMsgs := []msg.DKGMessage{}
	for i := 0; i < 3; i++ {
		bmsg, err := sender.prepareBroadcastMessage([]byte(fmt.Sprintf("msg%d", i)))
		require.NoError(t, err)
		bcastMsgs = append(bcastMsgs, bmsg)
		expectedMsgs = append(expectedMsgs, bmsg.DKGMessage)
	}

	// check that the dkg contract client is called correctly
	contractClient := &mock.DKGContractClient{}
	contractClient.On("ReadBroadcast", recipient.messageOffset, blockID).
		Return(bcastMsgs, nil).
		Once()
	sender.dkgContractClient = contractClient

	// launch a background routine to capture messages forwarded to the msgCh
	receivedMsgs := []msg.DKGMessage{}
	doneCh := make(chan struct{})
	go func() {
		msgCh := sender.GetBroadcastMsgCh()
		for {
			msg := <-msgCh
			receivedMsgs = append(receivedMsgs, msg)
			if len(receivedMsgs) == len(bcastMsgs) {
				close(doneCh)
			}
		}
	}()

	err := sender.Poll(blockID)
	require.NoError(t, err)

	// check that the contract has been correctly called
	contractClient.AssertExpectations(t)

	// check that the messages have been received and forwarded to the msgCh
	unittest.AssertClosesBefore(t, doneCh, time.Second)
	require.Equal(t, expectedMsgs, receivedMsgs)

	// check that the message offset has been incremented
	require.Equal(t, uint(len(bcastMsgs)), sender.messageOffset)
}

// TestLogHook checks that the Disqualify and FlagMisbehaviour functions call a
// Warn log, and that we can hook a logger to react to such logs.
func TestLogHook(t *testing.T) {
	committee, locals := initCommittee(2)

	hookCalls := 0

	hook := zerolog.HookFunc(func(e *zerolog.Event, level zerolog.Level, message string) {
		if level == zerolog.WarnLevel {
			hookCalls++
		}
	})
	logger := zerolog.New(os.Stdout).Level(zerolog.WarnLevel).Hook(hook)

	// sender
	sender := NewBroker(
		logger,
		dkgInstanceID,
		committee,
		locals[orig],
		orig,
		&mock.DKGContractClient{},
		NewBrokerTunnel(),
	)

	sender.Disqualify(1, "testing")
	sender.FlagMisbehavior(1, "test")
	require.Equal(t, 2, hookCalls)
}
