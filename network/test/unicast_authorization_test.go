package test

import (
	"context"
	"io"
	"reflect"
	"testing"
	"time"

	"github.com/rs/zerolog"
	mockery "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"
	libp2pmessage "github.com/onflow/flow-go/model/libp2p/message"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/codec"
	"github.com/onflow/flow-go/network/internal/testutils"
	"github.com/onflow/flow-go/network/message"
	"github.com/onflow/flow-go/network/mocknetwork"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/p2plogging"
	"github.com/onflow/flow-go/network/p2p/p2pnet"
	"github.com/onflow/flow-go/network/validator"
	"github.com/onflow/flow-go/utils/unittest"
)

// UnicastAuthorizationTestSuite tests that messages sent via unicast that are unauthenticated or unauthorized are correctly rejected. Each test on the test suite
// uses 2 networks, a sender and receiver. A mock slashing violation's consumer is used to assert the messages were rejected. Networks and the cancel func
// are set during each test run inside the test and remove after each test run in the TearDownTest callback.
type UnicastAuthorizationTestSuite struct {
	suite.Suite
	channelCloseDuration time.Duration
	logger               zerolog.Logger

	codec *overridableMessageEncoder

	libP2PNodes []p2p.LibP2PNode
	// senderNetwork is the networking layer instance that will be used to send the message.
	senderNetwork network.EngineRegistry
	// senderID the identity on the mw sending the message
	senderID *flow.Identity
	// receiverNetwork is the networking layer instance that will be used to receive the message.
	receiverNetwork network.EngineRegistry
	// receiverID the identity on the mw sending the message
	receiverID *flow.Identity
	// providers id providers generated at beginning of a test run
	providers []*unittest.UpdatableIDProvider
	// cancel is the cancel func from the context that was used to start the networks in a test run
	cancel  context.CancelFunc
	sporkId flow.Identifier
	// waitCh is the channel used to wait for the networks to perform authorization and invoke the slashing
	//violation's consumer before making mock assertions and cleaning up resources
	waitCh chan struct{}
}

// TestUnicastAuthorizationTestSuite runs all the test methods in this test suit
func TestUnicastAuthorizationTestSuite(t *testing.T) {
	suite.Run(t, new(UnicastAuthorizationTestSuite))
}

func (u *UnicastAuthorizationTestSuite) SetupTest() {
	u.logger = unittest.Logger()
	u.channelCloseDuration = 100 * time.Millisecond
	// this ch will allow us to wait until the expected method call happens before shutting down networks.
	u.waitCh = make(chan struct{})
}

func (u *UnicastAuthorizationTestSuite) TearDownTest() {
	u.stopNetworksAndLibp2pNodes()
}

// setupNetworks will setup the sender and receiver networks with the given slashing violations consumer.
func (u *UnicastAuthorizationTestSuite) setupNetworks(slashingViolationsConsumer network.ViolationsConsumer) {
	u.sporkId = unittest.IdentifierFixture()
	ids, libP2PNodes := testutils.LibP2PNodeForNetworkFixture(u.T(), u.sporkId, 2)
	u.codec = newOverridableMessageEncoder(unittest.NetworkCodec())
	nets, providers := testutils.NetworksFixture(
		u.T(),
		u.sporkId,
		ids,
		libP2PNodes,
		p2pnet.WithCodec(u.codec),
		p2pnet.WithSlashingViolationConsumerFactory(func(_ network.ConduitAdapter) network.ViolationsConsumer {
			return slashingViolationsConsumer
		}))
	require.Len(u.T(), ids, 2)
	require.Len(u.T(), providers, 2)
	require.Len(u.T(), nets, 2)

	u.senderNetwork = nets[0]
	u.receiverNetwork = nets[1]
	u.senderID = ids[0]
	u.receiverID = ids[1]
	u.providers = providers
	u.libP2PNodes = libP2PNodes
}

// startNetworksAndLibp2pNodes will start both sender and receiver networks with an irrecoverable signaler context and set the context cancel func.
func (u *UnicastAuthorizationTestSuite) startNetworksAndLibp2pNodes() {
	ctx, cancel := context.WithCancel(context.Background())
	sigCtx, _ := irrecoverable.WithSignaler(ctx)

	testutils.StartNodes(sigCtx, u.T(), u.libP2PNodes)
	testutils.StartNetworks(sigCtx, u.T(), []network.EngineRegistry{u.senderNetwork, u.receiverNetwork})
	unittest.RequireComponentsReadyBefore(u.T(), 1*time.Second, u.senderNetwork, u.receiverNetwork)

	u.cancel = cancel
}

// stopNetworksAndLibp2pNodes will stop all networks and libp2p nodes and wait for them to stop.
func (u *UnicastAuthorizationTestSuite) stopNetworksAndLibp2pNodes() {
	u.cancel() // cancel context to stop libp2p nodes.

	testutils.StopComponents(u.T(), []network.EngineRegistry{u.senderNetwork, u.receiverNetwork}, 1*time.Second)
	unittest.RequireComponentsDoneBefore(u.T(), 1*time.Second, u.senderNetwork, u.receiverNetwork)
}

// TestUnicastAuthorization_UnstakedPeer tests that messages sent via unicast by an unstaked peer is correctly rejected.
func (u *UnicastAuthorizationTestSuite) TestUnicastAuthorization_UnstakedPeer() {
	slashingViolationsConsumer := mocknetwork.NewViolationsConsumer(u.T())
	u.setupNetworks(slashingViolationsConsumer)

	expectedSenderPeerID, err := unittest.PeerIDFromFlowID(u.senderID)
	require.NoError(u.T(), err)

	var nilID *flow.Identity
	expectedViolation := &network.Violation{
		Identity: nilID, // because the peer will be unverified this identity will be nil
		PeerID:   p2plogging.PeerId(expectedSenderPeerID),
		MsgType:  "",                          // message will not be decoded before OnSenderEjectedError is logged, we won't log message type
		Channel:  channels.TestNetworkChannel, // message will not be decoded before OnSenderEjectedError is logged, we won't log peer ID
		Protocol: message.ProtocolTypeUnicast,
		Err:      validator.ErrIdentityUnverified,
	}
	slashingViolationsConsumer.On("OnUnAuthorizedSenderError", expectedViolation).Return(nil).Once().Run(func(args mockery.Arguments) {
		close(u.waitCh)
	})

	u.startNetworksAndLibp2pNodes()

	// overriding the identity provide of the receiver node to return an empty identity list so that the
	// sender node looks unstaked to its networking layer and hence it sends an UnAuthorizedSenderError upon receiving a message
	// from the sender node
	u.providers[1].SetIdentities(nil)

	_, err = u.receiverNetwork.Register(channels.TestNetworkChannel, &mocknetwork.MessageProcessor{})
	require.NoError(u.T(), err)

	senderCon, err := u.senderNetwork.Register(channels.TestNetworkChannel, &mocknetwork.MessageProcessor{})
	require.NoError(u.T(), err)

	// send message via unicast
	err = senderCon.Unicast(&libp2pmessage.TestMessage{
		Text: string("hello"),
	}, u.receiverID.NodeID)
	require.NoError(u.T(), err)

	// wait for slashing violations consumer mock to invoke run func and close ch if expected method call happens
	unittest.RequireCloseBefore(u.T(), u.waitCh, u.channelCloseDuration, "could close ch on time")
}

// TestUnicastAuthorization_EjectedPeer tests that messages sent via unicast by an ejected peer is correctly rejected.
func (u *UnicastAuthorizationTestSuite) TestUnicastAuthorization_EjectedPeer() {
	slashingViolationsConsumer := mocknetwork.NewViolationsConsumer(u.T())
	u.setupNetworks(slashingViolationsConsumer)
	//NOTE: setup ejected identity
	u.senderID.Ejected = true

	// overriding the identity provide of the receiver node to return the ejected identity so that the
	// sender node looks ejected to its networking layer and hence it sends a SenderEjectedError upon receiving a message
	// from the sender node
	u.providers[1].SetIdentities(flow.IdentityList{u.senderID})

	expectedSenderPeerID, err := unittest.PeerIDFromFlowID(u.senderID)
	require.NoError(u.T(), err)

	expectedViolation := &network.Violation{
		Identity: u.senderID, // we expect this method to be called with the ejected identity
		OriginID: u.senderID.NodeID,
		PeerID:   p2plogging.PeerId(expectedSenderPeerID),
		MsgType:  "",                          // message will not be decoded before OnSenderEjectedError is logged, we won't log message type
		Channel:  channels.TestNetworkChannel, // message will not be decoded before OnSenderEjectedError is logged, we won't log peer ID
		Protocol: message.ProtocolTypeUnicast,
		Err:      validator.ErrSenderEjected,
	}
	slashingViolationsConsumer.On("OnSenderEjectedError", expectedViolation).
		Return(nil).Once().Run(func(args mockery.Arguments) {
		close(u.waitCh)
	})

	u.startNetworksAndLibp2pNodes()

	_, err = u.receiverNetwork.Register(channels.TestNetworkChannel, &mocknetwork.MessageProcessor{})
	require.NoError(u.T(), err)

	senderCon, err := u.senderNetwork.Register(channels.TestNetworkChannel, &mocknetwork.MessageProcessor{})
	require.NoError(u.T(), err)

	// send message via unicast
	err = senderCon.Unicast(&libp2pmessage.TestMessage{
		Text: string("hello"),
	}, u.receiverID.NodeID)
	require.NoError(u.T(), err)

	// wait for slashing violations consumer mock to invoke run func and close ch if expected method call happens
	unittest.RequireCloseBefore(u.T(), u.waitCh, u.channelCloseDuration, "could close ch on time")
}

// TestUnicastAuthorization_UnauthorizedPeer tests that messages sent via unicast by an unauthorized peer is correctly rejected.
func (u *UnicastAuthorizationTestSuite) TestUnicastAuthorization_UnauthorizedPeer() {
	slashingViolationsConsumer := mocknetwork.NewViolationsConsumer(u.T())
	u.setupNetworks(slashingViolationsConsumer)

	expectedSenderPeerID, err := unittest.PeerIDFromFlowID(u.senderID)
	require.NoError(u.T(), err)

	expectedViolation := &network.Violation{
		Identity: u.senderID,
		OriginID: u.senderID.NodeID,
		PeerID:   p2plogging.PeerId(expectedSenderPeerID),
		MsgType:  "*message.TestMessage",
		Channel:  channels.ConsensusCommittee,
		Protocol: message.ProtocolTypeUnicast,
		Err:      message.ErrUnauthorizedMessageOnChannel,
	}

	slashingViolationsConsumer.On("OnUnAuthorizedSenderError", expectedViolation).
		Return(nil).Once().Run(func(args mockery.Arguments) {
		close(u.waitCh)
	})

	u.startNetworksAndLibp2pNodes()

	_, err = u.receiverNetwork.Register(channels.ConsensusCommittee, &mocknetwork.MessageProcessor{})
	require.NoError(u.T(), err)

	senderCon, err := u.senderNetwork.Register(channels.ConsensusCommittee, &mocknetwork.MessageProcessor{})
	require.NoError(u.T(), err)

	// send message via unicast; a test message must only be unicasted on the TestNetworkChannel, not on the ConsensusCommittee channel
	// so we expect an unauthorized sender error
	err = senderCon.Unicast(&libp2pmessage.TestMessage{
		Text: string("hello"),
	}, u.receiverID.NodeID)
	require.NoError(u.T(), err)

	// wait for slashing violations consumer mock to invoke run func and close ch if expected method call happens
	unittest.RequireCloseBefore(u.T(), u.waitCh, u.channelCloseDuration, "could close ch on time")
}

// TestUnicastAuthorization_UnknownMsgCode tests that messages sent via unicast with an unknown message code is correctly rejected.
func (u *UnicastAuthorizationTestSuite) TestUnicastAuthorization_UnknownMsgCode() {
	slashingViolationsConsumer := mocknetwork.NewViolationsConsumer(u.T())
	u.setupNetworks(slashingViolationsConsumer)

	expectedSenderPeerID, err := unittest.PeerIDFromFlowID(u.senderID)
	require.NoError(u.T(), err)

	invalidMessageCode := codec.MessageCode(byte('X'))
	// register a custom encoder that encodes the message with an invalid message code when encoding a string.
	u.codec.RegisterEncoder(reflect.TypeOf(""), func(v interface{}) ([]byte, error) {
		e, err := unittest.NetworkCodec().Encode(&libp2pmessage.TestMessage{
			Text: v.(string),
		})
		require.NoError(u.T(), err)
		// manipulate message code byte
		invalidMessageCode := codec.MessageCode(byte('X'))
		e[0] = invalidMessageCode.Uint8()
		return e, nil
	})

	var nilID *flow.Identity
	expectedViolation := &network.Violation{
		Identity: nilID,
		PeerID:   p2plogging.PeerId(expectedSenderPeerID),
		MsgType:  "",
		Channel:  channels.TestNetworkChannel,
		Protocol: message.ProtocolTypeUnicast,
		Err:      codec.NewUnknownMsgCodeErr(invalidMessageCode),
	}

	slashingViolationsConsumer.On("OnUnknownMsgTypeError", expectedViolation).
		Return(nil).Once().Run(func(args mockery.Arguments) {
		close(u.waitCh)
	})

	u.startNetworksAndLibp2pNodes()

	_, err = u.receiverNetwork.Register(channels.TestNetworkChannel, &mocknetwork.MessageProcessor{})
	require.NoError(u.T(), err)

	senderCon, err := u.senderNetwork.Register(channels.TestNetworkChannel, &mocknetwork.MessageProcessor{})
	require.NoError(u.T(), err)

	// send message via unicast
	err = senderCon.Unicast("hello!", u.receiverID.NodeID)
	require.NoError(u.T(), err)

	// wait for slashing violations consumer mock to invoke run func and close ch if expected method call happens
	unittest.RequireCloseBefore(u.T(), u.waitCh, u.channelCloseDuration, "could close ch on time")
}

// TestUnicastAuthorization_WrongMsgCode tests that messages sent via unicast with a message code that does not match the underlying message type are correctly rejected.
func (u *UnicastAuthorizationTestSuite) TestUnicastAuthorization_WrongMsgCode() {
	slashingViolationsConsumer := mocknetwork.NewViolationsConsumer(u.T())
	u.setupNetworks(slashingViolationsConsumer)

	expectedSenderPeerID, err := unittest.PeerIDFromFlowID(u.senderID)
	require.NoError(u.T(), err)

	modifiedMessageCode := codec.CodeDKGMessage
	// register a custom encoder that overrides the message code when encoding a TestMessage.
	u.codec.RegisterEncoder(reflect.TypeOf(&libp2pmessage.TestMessage{}), func(v interface{}) ([]byte, error) {
		e, err := unittest.NetworkCodec().Encode(v)
		require.NoError(u.T(), err)
		e[0] = modifiedMessageCode.Uint8()
		return e, nil
	})

	expectedViolation := &network.Violation{
		Identity: u.senderID,
		OriginID: u.senderID.NodeID,
		PeerID:   p2plogging.PeerId(expectedSenderPeerID),
		MsgType:  "*messages.DKGMessage",
		Channel:  channels.TestNetworkChannel,
		Protocol: message.ProtocolTypeUnicast,
		Err:      message.ErrUnauthorizedMessageOnChannel,
	}

	slashingViolationsConsumer.On("OnUnAuthorizedSenderError", expectedViolation).
		Return(nil).Once().Run(func(args mockery.Arguments) {
		close(u.waitCh)
	})

	u.startNetworksAndLibp2pNodes()

	_, err = u.receiverNetwork.Register(channels.TestNetworkChannel, &mocknetwork.MessageProcessor{})
	require.NoError(u.T(), err)

	senderCon, err := u.senderNetwork.Register(channels.TestNetworkChannel, &mocknetwork.MessageProcessor{})
	require.NoError(u.T(), err)

	// send message via unicast
	err = senderCon.Unicast(&libp2pmessage.TestMessage{
		Text: string("hello"),
	}, u.receiverID.NodeID)
	require.NoError(u.T(), err)

	// wait for slashing violations consumer mock to invoke run func and close ch if expected method call happens
	unittest.RequireCloseBefore(u.T(), u.waitCh, u.channelCloseDuration, "could close ch on time")
}

// TestUnicastAuthorization_PublicChannel tests that messages sent via unicast on a public channel are not rejected for any reason.
func (u *UnicastAuthorizationTestSuite) TestUnicastAuthorization_PublicChannel() {
	slashingViolationsConsumer := mocknetwork.NewViolationsConsumer(u.T())
	u.setupNetworks(slashingViolationsConsumer)
	u.startNetworksAndLibp2pNodes()

	msg := &libp2pmessage.TestMessage{
		Text: string("hello"),
	}

	// mock a message processor that will receive the message.
	receiverEngine := &mocknetwork.MessageProcessor{}
	receiverEngine.On("Process", channels.PublicPushBlocks, u.senderID.NodeID, msg).Run(
		func(args mockery.Arguments) {
			close(u.waitCh)
		}).Return(nil).Once()
	_, err := u.receiverNetwork.Register(channels.PublicPushBlocks, receiverEngine)
	require.NoError(u.T(), err)

	senderCon, err := u.senderNetwork.Register(channels.PublicPushBlocks, &mocknetwork.MessageProcessor{})
	require.NoError(u.T(), err)

	// send message via unicast
	err = senderCon.Unicast(&libp2pmessage.TestMessage{
		Text: string("hello"),
	}, u.receiverID.NodeID)
	require.NoError(u.T(), err)

	// wait for slashing violations consumer mock to invoke run func and close ch if expected method call happens
	unittest.RequireCloseBefore(u.T(), u.waitCh, u.channelCloseDuration, "could close ch on time")
}

// TestUnicastAuthorization_UnauthorizedUnicastOnChannel tests that messages sent via unicast that are not authorized for unicast are rejected.
func (u *UnicastAuthorizationTestSuite) TestUnicastAuthorization_UnauthorizedUnicastOnChannel() {
	slashingViolationsConsumer := mocknetwork.NewViolationsConsumer(u.T())
	u.setupNetworks(slashingViolationsConsumer)

	// set sender id role to RoleConsensus to avoid unauthorized sender validation error
	u.senderID.Role = flow.RoleConsensus

	expectedSenderPeerID, err := unittest.PeerIDFromFlowID(u.senderID)
	require.NoError(u.T(), err)

	expectedViolation := &network.Violation{
		Identity: u.senderID,
		OriginID: u.senderID.NodeID,
		PeerID:   p2plogging.PeerId(expectedSenderPeerID),
		MsgType:  "*messages.BlockProposal",
		Channel:  channels.ConsensusCommittee,
		Protocol: message.ProtocolTypeUnicast,
		Err:      message.ErrUnauthorizedUnicastOnChannel,
	}

	slashingViolationsConsumer.On("OnUnauthorizedUnicastOnChannel", expectedViolation).
		Return(nil).Once().Run(func(args mockery.Arguments) {
		close(u.waitCh)
	})

	u.startNetworksAndLibp2pNodes()

	_, err = u.receiverNetwork.Register(channels.ConsensusCommittee, &mocknetwork.MessageProcessor{})
	require.NoError(u.T(), err)

	senderCon, err := u.senderNetwork.Register(channels.ConsensusCommittee, &mocknetwork.MessageProcessor{})
	require.NoError(u.T(), err)

	// messages.BlockProposal is not authorized to be sent via unicast over the ConsensusCommittee channel
	payload := unittest.ProposalFixture()
	// send message via unicast
	err = senderCon.Unicast(payload, u.receiverID.NodeID)
	require.NoError(u.T(), err)

	// wait for slashing violations consumer mock to invoke run func and close ch if expected method call happens
	unittest.RequireCloseBefore(u.T(), u.waitCh, u.channelCloseDuration, "could close ch on time")
}

// TestUnicastAuthorization_ReceiverHasNoSubscription tests that messages sent via unicast are rejected on the receiver end if the receiver does not have a subscription
// to the channel of the message.
func (u *UnicastAuthorizationTestSuite) TestUnicastAuthorization_ReceiverHasNoSubscription() {
	slashingViolationsConsumer := mocknetwork.NewViolationsConsumer(u.T())
	u.setupNetworks(slashingViolationsConsumer)

	expectedSenderPeerID, err := unittest.PeerIDFromFlowID(u.senderID)
	require.NoError(u.T(), err)

	expectedViolation := &network.Violation{
		Identity: nil,
		PeerID:   p2plogging.PeerId(expectedSenderPeerID),
		MsgType:  "*message.TestMessage",
		Channel:  channels.TestNetworkChannel,
		Protocol: message.ProtocolTypeUnicast,
		Err:      p2pnet.ErrUnicastMsgWithoutSub,
	}

	slashingViolationsConsumer.On("OnUnauthorizedUnicastOnChannel", expectedViolation).
		Return(nil).Once().Run(func(args mockery.Arguments) {
		close(u.waitCh)
	})

	u.startNetworksAndLibp2pNodes()

	senderCon, err := u.senderNetwork.Register(channels.TestNetworkChannel, &mocknetwork.MessageProcessor{})
	require.NoError(u.T(), err)

	// send message via unicast
	err = senderCon.Unicast(&libp2pmessage.TestMessage{
		Text: string("hello"),
	}, u.receiverID.NodeID)
	require.NoError(u.T(), err)

	// wait for slashing violations consumer mock to invoke run func and close ch if expected method call happens
	unittest.RequireCloseBefore(u.T(), u.waitCh, u.channelCloseDuration, "could close ch on time")
}

// TestUnicastAuthorization_ReceiverHasSubscription tests that messages sent via unicast are processed on the receiver end if the receiver does have a subscription
// to the channel of the message.
func (u *UnicastAuthorizationTestSuite) TestUnicastAuthorization_ReceiverHasSubscription() {
	slashingViolationsConsumer := mocknetwork.NewViolationsConsumer(u.T())
	u.setupNetworks(slashingViolationsConsumer)
	u.startNetworksAndLibp2pNodes()

	msg := &messages.EntityRequest{
		EntityIDs: unittest.IdentifierListFixture(10),
	}

	// both sender and receiver must have an authorized role to send and receive messages on the ConsensusCommittee channel.
	u.senderID.Role = flow.RoleConsensus
	u.receiverID.Role = flow.RoleExecution

	receiverEngine := &mocknetwork.MessageProcessor{}
	receiverEngine.On("Process", channels.RequestReceiptsByBlockID, u.senderID.NodeID, msg).Run(
		func(args mockery.Arguments) {
			close(u.waitCh)
		}).Return(nil).Once()
	_, err := u.receiverNetwork.Register(channels.RequestReceiptsByBlockID, receiverEngine)
	require.NoError(u.T(), err)

	senderCon, err := u.senderNetwork.Register(channels.RequestReceiptsByBlockID, &mocknetwork.MessageProcessor{})
	require.NoError(u.T(), err)

	// send message via unicast
	err = senderCon.Unicast(msg, u.receiverID.NodeID)
	require.NoError(u.T(), err)

	// wait for slashing violations consumer mock to invoke run func and close ch if expected method call happens
	unittest.RequireCloseBefore(u.T(), u.waitCh, u.channelCloseDuration, "could close ch on time")
}

// overridableMessageEncoder is a codec that allows to override the encoder for a specific type only for sake of testing.
// We specifically use this to override the encoder for the TestMessage type to encode it with an invalid message code.
type overridableMessageEncoder struct {
	codec           network.Codec
	specificEncoder map[reflect.Type]func(interface{}) ([]byte, error)
}

var _ network.Codec = (*overridableMessageEncoder)(nil)

func newOverridableMessageEncoder(codec network.Codec) *overridableMessageEncoder {
	return &overridableMessageEncoder{
		codec:           codec,
		specificEncoder: make(map[reflect.Type]func(interface{}) ([]byte, error)),
	}
}

// RegisterEncoder registers an encoder for a specific type, overriding the default encoder for that type.
func (u *overridableMessageEncoder) RegisterEncoder(t reflect.Type, encoder func(interface{}) ([]byte, error)) {
	u.specificEncoder[t] = encoder
}

// NewEncoder creates a new encoder.
func (u *overridableMessageEncoder) NewEncoder(w io.Writer) network.Encoder {
	return u.codec.NewEncoder(w)
}

// NewDecoder creates a new decoder.
func (u *overridableMessageEncoder) NewDecoder(r io.Reader) network.Decoder {
	return u.codec.NewDecoder(r)
}

// Encode encodes a value into a byte slice. If a specific encoder is registered for the type of the value, it will be used.
// Otherwise, the default encoder will be used.
func (u *overridableMessageEncoder) Encode(v interface{}) ([]byte, error) {
	if encoder, ok := u.specificEncoder[reflect.TypeOf(v)]; ok {
		return encoder(v)
	}
	return u.codec.Encode(v)
}

// Decode decodes a byte slice into a value. It uses the default decoder.
func (u *overridableMessageEncoder) Decode(data []byte) (interface{}, error) {
	return u.codec.Decode(data)
}
