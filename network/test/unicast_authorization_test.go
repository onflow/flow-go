package test

import (
	"context"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	mockery "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
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
	"github.com/onflow/flow-go/network/p2p/middleware"
	"github.com/onflow/flow-go/network/validator"
	"github.com/onflow/flow-go/utils/unittest"
)

// UnicastAuthorizationTestSuite tests that messages sent via unicast that are unauthenticated or unauthorized are correctly rejected. Each test on the test suite
// uses 2 middlewares, a sender and receiver. A mock slashing violation's consumer is used to assert the messages were rejected. Middleware and the cancel func
// are set during each test run inside the test and remove after each test run in the TearDownTest callback.
type UnicastAuthorizationTestSuite struct {
	suite.Suite
	channelCloseDuration time.Duration
	logger               zerolog.Logger

	libP2PNodes []p2p.LibP2PNode
	// senderMW is the mw that will be sending the message
	senderMW network.Middleware
	// senderID the identity on the mw sending the message
	senderID *flow.Identity
	// receiverMW is the mw that will be sending the message
	receiverMW network.Middleware
	// receiverID the identity on the mw sending the message
	receiverID *flow.Identity
	// providers id providers generated at beginning of a test run
	providers []*unittest.UpdatableIDProvider
	// cancel is the cancel func from the context that was used to start the middlewares in a test run
	cancel context.CancelFunc
	// waitCh is the channel used to wait for the middleware to perform authorization and invoke the slashing
	//violation's consumer before making mock assertions and cleaning up resources
	waitCh chan struct{}
}

// TestUnicastAuthorizationTestSuite runs all the test methods in this test suit
func TestUnicastAuthorizationTestSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(UnicastAuthorizationTestSuite))
}

func (u *UnicastAuthorizationTestSuite) SetupTest() {
	u.logger = unittest.Logger()
	u.channelCloseDuration = 100 * time.Millisecond
	// this ch will allow us to wait until the expected method call happens before shutting down middleware
	u.waitCh = make(chan struct{})
}

func (u *UnicastAuthorizationTestSuite) TearDownTest() {
	u.stopMiddlewares()
}

// setupMiddlewaresAndProviders will setup 2 middlewares that will be used as a sender and receiver in each suite test.
func (u *UnicastAuthorizationTestSuite) setupMiddlewaresAndProviders(slashingViolationsConsumer network.ViolationsConsumer) {
	ids, libP2PNodes, _ := testutils.LibP2PNodeForMiddlewareFixture(u.T(), 2)
	cfg := testutils.MiddlewareConfigFixture(u.T())
	mws, providers := testutils.MiddlewareFixtures(u.T(), ids, libP2PNodes, cfg, slashingViolationsConsumer)
	require.Len(u.T(), ids, 2)
	require.Len(u.T(), providers, 2)
	require.Len(u.T(), mws, 2)

	u.senderID = ids[0]
	u.senderMW = mws[0]
	u.receiverID = ids[1]
	u.receiverMW = mws[1]
	u.providers = providers
	u.libP2PNodes = libP2PNodes
}

// startMiddlewares will start both sender and receiver middlewares with an irrecoverable signaler context and set the context cancel func.
func (u *UnicastAuthorizationTestSuite) startMiddlewares(overlay *mocknetwork.Overlay) {
	ctx, cancel := context.WithCancel(context.Background())
	sigCtx, _ := irrecoverable.WithSignaler(ctx)

	testutils.StartNodes(sigCtx, u.T(), u.libP2PNodes, 100*time.Millisecond)

	u.senderMW.SetOverlay(overlay)
	u.senderMW.Start(sigCtx)

	u.receiverMW.SetOverlay(overlay)
	u.receiverMW.Start(sigCtx)

	unittest.RequireComponentsReadyBefore(u.T(), 100*time.Millisecond, u.senderMW, u.receiverMW)

	u.cancel = cancel
}

// stopMiddlewares will stop all middlewares.
func (u *UnicastAuthorizationTestSuite) stopMiddlewares() {
	u.cancel()
	unittest.RequireCloseBefore(u.T(), u.senderMW.Done(), u.channelCloseDuration, "could not stop middleware on time")
	unittest.RequireCloseBefore(u.T(), u.receiverMW.Done(), u.channelCloseDuration, "could not stop middleware on time")
}

// TestUnicastAuthorization_UnstakedPeer tests that messages sent via unicast by an unstaked peer is correctly rejected.
func (u *UnicastAuthorizationTestSuite) TestUnicastAuthorization_UnstakedPeer() {
	// setup mock slashing violations consumer and middlewares
	slashingViolationsConsumer := mocknetwork.NewViolationsConsumer(u.T())
	u.setupMiddlewaresAndProviders(slashingViolationsConsumer)

	expectedSenderPeerID, err := unittest.PeerIDFromFlowID(u.senderID)
	require.NoError(u.T(), err)

	var nilID *flow.Identity
	expectedViolation := &network.Violation{
		Identity: nilID, // because the peer will be unverified this identity will be nil
		PeerID:   expectedSenderPeerID.String(),
		MsgType:  "",                          // message will not be decoded before OnSenderEjectedError is logged, we won't log message type
		Channel:  channels.TestNetworkChannel, // message will not be decoded before OnSenderEjectedError is logged, we won't log peer ID
		Protocol: message.ProtocolTypeUnicast,
		Err:      validator.ErrIdentityUnverified,
	}
	slashingViolationsConsumer.On(
		"OnUnAuthorizedSenderError",
		expectedViolation,
	).Return(nil).Once().Run(func(args mockery.Arguments) {
		close(u.waitCh)
	})

	overlay := mocknetwork.NewOverlay(u.T())
	overlay.On("Identities").Maybe().Return(func() flow.IdentityList {
		return u.providers[0].Identities(filter.Any)
	})
	overlay.On("Topology").Maybe().Return(func() flow.IdentityList {
		return u.providers[0].Identities(filter.Any)
	}, nil)

	//NOTE: return (nil, false) simulating unstaked node
	overlay.On("Identity", mock.AnythingOfType("peer.ID")).Return(nil, false)
	// message will be rejected so assert overlay never receives it
	defer overlay.AssertNotCalled(u.T(), "Receive", mockery.Anything)

	u.startMiddlewares(overlay)

	require.NoError(u.T(), u.receiverMW.Subscribe(testChannel))
	require.NoError(u.T(), u.senderMW.Subscribe(testChannel))

	msg, err := network.NewOutgoingScope(
		flow.IdentifierList{u.receiverID.NodeID},
		testChannel,
		&libp2pmessage.TestMessage{
			Text: string("hello"),
		},
		unittest.NetworkCodec().Encode,
		message.ProtocolTypeUnicast)
	require.NoError(u.T(), err)

	// send message via unicast
	err = u.senderMW.SendDirect(msg)
	require.NoError(u.T(), err)

	// wait for slashing violations consumer mock to invoke run func and close ch if expected method call happens
	unittest.RequireCloseBefore(u.T(), u.waitCh, u.channelCloseDuration, "could close ch on time")
}

// TestUnicastAuthorization_EjectedPeer tests that messages sent via unicast by an ejected peer is correctly rejected.
func (u *UnicastAuthorizationTestSuite) TestUnicastAuthorization_EjectedPeer() {
	// setup mock slashing violations consumer and middlewares
	slashingViolationsConsumer := mocknetwork.NewViolationsConsumer(u.T())
	u.setupMiddlewaresAndProviders(slashingViolationsConsumer)
	//NOTE: setup ejected identity
	u.senderID.Ejected = true

	expectedSenderPeerID, err := unittest.PeerIDFromFlowID(u.senderID)
	require.NoError(u.T(), err)

	expectedViolation := &network.Violation{
		Identity: u.senderID, // we expect this method to be called with the ejected identity
		OriginID: u.senderID.NodeID,
		PeerID:   expectedSenderPeerID.String(),
		MsgType:  "",                          // message will not be decoded before OnSenderEjectedError is logged, we won't log message type
		Channel:  channels.TestNetworkChannel, // message will not be decoded before OnSenderEjectedError is logged, we won't log peer ID
		Protocol: message.ProtocolTypeUnicast,
		Err:      validator.ErrSenderEjected,
	}
	slashingViolationsConsumer.On(
		"OnSenderEjectedError",
		expectedViolation,
	).Return(nil).Once().Run(func(args mockery.Arguments) {
		close(u.waitCh)
	})

	overlay := mocknetwork.NewOverlay(u.T())
	overlay.On("Identities").Maybe().Return(func() flow.IdentityList {
		return u.providers[0].Identities(filter.Any)
	})
	overlay.On("Topology").Maybe().Return(func() flow.IdentityList {
		return u.providers[0].Identities(filter.Any)
	}, nil)
	//NOTE: return ejected identity causing validation to fail
	overlay.On("Identity", mock.AnythingOfType("peer.ID")).Return(u.senderID, true)
	// message will be rejected so assert overlay never receives it
	defer overlay.AssertNotCalled(u.T(), "Receive", mockery.Anything)

	u.startMiddlewares(overlay)

	require.NoError(u.T(), u.receiverMW.Subscribe(testChannel))
	require.NoError(u.T(), u.senderMW.Subscribe(testChannel))

	msg, err := network.NewOutgoingScope(
		flow.IdentifierList{u.receiverID.NodeID},
		testChannel,
		&libp2pmessage.TestMessage{
			Text: string("hello"),
		},
		unittest.NetworkCodec().Encode,
		message.ProtocolTypeUnicast)
	require.NoError(u.T(), err)

	// send message via unicast
	err = u.senderMW.SendDirect(msg)
	require.NoError(u.T(), err)

	// wait for slashing violations consumer mock to invoke run func and close ch if expected method call happens
	unittest.RequireCloseBefore(u.T(), u.waitCh, u.channelCloseDuration, "could close ch on time")
}

// TestUnicastAuthorization_UnauthorizedPeer tests that messages sent via unicast by an unauthorized peer is correctly rejected.
func (u *UnicastAuthorizationTestSuite) TestUnicastAuthorization_UnauthorizedPeer() {
	// setup mock slashing violations consumer and middlewares
	slashingViolationsConsumer := mocknetwork.NewViolationsConsumer(u.T())
	u.setupMiddlewaresAndProviders(slashingViolationsConsumer)

	expectedSenderPeerID, err := unittest.PeerIDFromFlowID(u.senderID)
	require.NoError(u.T(), err)

	expectedViolation := &network.Violation{
		Identity: u.senderID,
		OriginID: u.senderID.NodeID,
		PeerID:   expectedSenderPeerID.String(),
		MsgType:  "*message.TestMessage",
		Channel:  channels.ConsensusCommittee,
		Protocol: message.ProtocolTypeUnicast,
		Err:      message.ErrUnauthorizedMessageOnChannel,
	}

	slashingViolationsConsumer.On(
		"OnUnAuthorizedSenderError",
		expectedViolation,
	).Return(nil).Once().Run(func(args mockery.Arguments) {
		close(u.waitCh)
	})

	overlay := mocknetwork.NewOverlay(u.T())
	overlay.On("Identities").Maybe().Return(func() flow.IdentityList {
		return u.providers[0].Identities(filter.Any)
	})
	overlay.On("Topology").Maybe().Return(func() flow.IdentityList {
		return u.providers[0].Identities(filter.Any)
	}, nil)
	overlay.On("Identity", mock.AnythingOfType("peer.ID")).Return(u.senderID, true)
	// message will be rejected so assert overlay never receives it
	defer overlay.AssertNotCalled(u.T(), "Receive", mockery.Anything)

	u.startMiddlewares(overlay)

	channel := channels.ConsensusCommittee
	require.NoError(u.T(), u.receiverMW.Subscribe(channel))
	require.NoError(u.T(), u.senderMW.Subscribe(channel))

	msg, err := network.NewOutgoingScope(
		flow.IdentifierList{u.receiverID.NodeID},
		channel,
		&libp2pmessage.TestMessage{
			Text: string("hello"),
		},
		unittest.NetworkCodec().Encode,
		message.ProtocolTypeUnicast)
	require.NoError(u.T(), err)

	// send message via unicast
	err = u.senderMW.SendDirect(msg)
	require.NoError(u.T(), err)

	// wait for slashing violations consumer mock to invoke run func and close ch if expected method call happens
	unittest.RequireCloseBefore(u.T(), u.waitCh, u.channelCloseDuration, "could close ch on time")
}

// TestUnicastAuthorization_UnknownMsgCode tests that messages sent via unicast with an unknown message code is correctly rejected.
func (u *UnicastAuthorizationTestSuite) TestUnicastAuthorization_UnknownMsgCode() {
	// setup mock slashing violations consumer and middlewares
	slashingViolationsConsumer := mocknetwork.NewViolationsConsumer(u.T())
	u.setupMiddlewaresAndProviders(slashingViolationsConsumer)

	expectedSenderPeerID, err := unittest.PeerIDFromFlowID(u.senderID)
	require.NoError(u.T(), err)

	invalidMessageCode := codec.MessageCode(byte('X'))

	var nilID *flow.Identity
	expectedViolation := &network.Violation{
		Identity: nilID,
		PeerID:   expectedSenderPeerID.String(),
		MsgType:  "",
		Channel:  channels.TestNetworkChannel,
		Protocol: message.ProtocolTypeUnicast,
		Err:      codec.NewUnknownMsgCodeErr(invalidMessageCode),
	}

	slashingViolationsConsumer.On(
		"OnUnknownMsgTypeError",
		expectedViolation,
	).Return(nil).Once().Run(func(args mockery.Arguments) {
		close(u.waitCh)
	})

	overlay := mocknetwork.NewOverlay(u.T())
	overlay.On("Identities").Maybe().Return(func() flow.IdentityList {
		return u.providers[0].Identities(filter.Any)
	})
	overlay.On("Topology").Maybe().Return(func() flow.IdentityList {
		return u.providers[0].Identities(filter.Any)
	}, nil)
	overlay.On("Identity", mock.AnythingOfType("peer.ID")).Return(u.senderID, true)

	// message will be rejected so assert overlay never receives it
	defer overlay.AssertNotCalled(u.T(), "Receive", u.senderID.NodeID, mock.AnythingOfType("*message.Message"))

	u.startMiddlewares(overlay)

	require.NoError(u.T(), u.receiverMW.Subscribe(testChannel))
	require.NoError(u.T(), u.senderMW.Subscribe(testChannel))

	msg, err := network.NewOutgoingScope(
		flow.IdentifierList{u.receiverID.NodeID},
		testChannel,
		&libp2pmessage.TestMessage{
			Text: "hello",
		},
		// we use a custom encoder that encodes the message with an invalid message code.
		func(msg interface{}) ([]byte, error) {
			e, err := unittest.NetworkCodec().Encode(msg)
			require.NoError(u.T(), err)
			// manipulate message code byte
			e[0] = invalidMessageCode.Uint8()
			return e, nil
		},
		message.ProtocolTypeUnicast)
	require.NoError(u.T(), err)

	// send message via unicast
	err = u.senderMW.SendDirect(msg)
	require.NoError(u.T(), err)

	// wait for slashing violations consumer mock to invoke run func and close ch if expected method call happens
	unittest.RequireCloseBefore(u.T(), u.waitCh, u.channelCloseDuration, "could close ch on time")
}

// TestUnicastAuthorization_WrongMsgCode tests that messages sent via unicast with a message code that does not match the underlying message type are correctly rejected.
func (u *UnicastAuthorizationTestSuite) TestUnicastAuthorization_WrongMsgCode() {
	// setup mock slashing violations consumer and middlewares
	slashingViolationsConsumer := mocknetwork.NewViolationsConsumer(u.T())
	u.setupMiddlewaresAndProviders(slashingViolationsConsumer)

	expectedSenderPeerID, err := unittest.PeerIDFromFlowID(u.senderID)
	require.NoError(u.T(), err)

	modifiedMessageCode := codec.CodeDKGMessage

	expectedViolation := &network.Violation{
		Identity: u.senderID,
		OriginID: u.senderID.NodeID,
		PeerID:   expectedSenderPeerID.String(),
		MsgType:  "*messages.DKGMessage",
		Channel:  channels.TestNetworkChannel,
		Protocol: message.ProtocolTypeUnicast,
		Err:      message.ErrUnauthorizedMessageOnChannel,
	}

	slashingViolationsConsumer.On(
		"OnUnAuthorizedSenderError",
		expectedViolation,
	).Return(nil).Once().Run(func(args mockery.Arguments) {
		close(u.waitCh)
	})

	overlay := mocknetwork.NewOverlay(u.T())
	overlay.On("Identities").Maybe().Return(func() flow.IdentityList {
		return u.providers[0].Identities(filter.Any)
	})
	overlay.On("Topology").Maybe().Return(func() flow.IdentityList {
		return u.providers[0].Identities(filter.Any)
	}, nil)
	overlay.On("Identity", expectedSenderPeerID).Return(u.senderID, true)

	// message will be rejected so assert overlay never receives it
	defer overlay.AssertNotCalled(u.T(), "Receive", u.senderID.NodeID, mock.AnythingOfType("*message.Message"))

	u.startMiddlewares(overlay)

	require.NoError(u.T(), u.receiverMW.Subscribe(testChannel))
	require.NoError(u.T(), u.senderMW.Subscribe(testChannel))

	msg, err := network.NewOutgoingScope(
		flow.IdentifierList{u.receiverID.NodeID},
		testChannel,
		&libp2pmessage.TestMessage{
			Text: "hello",
		},
		// we use a custom encoder that encodes the message with an invalid message code.
		func(msg interface{}) ([]byte, error) {
			e, err := unittest.NetworkCodec().Encode(msg)
			require.NoError(u.T(), err)
			// manipulate message code byte
			e[0] = modifiedMessageCode.Uint8()
			return e, nil
		},
		message.ProtocolTypeUnicast)
	require.NoError(u.T(), err)

	// send message via unicast
	err = u.senderMW.SendDirect(msg)
	require.NoError(u.T(), err)

	// wait for slashing violations consumer mock to invoke run func and close ch if expected method call happens
	unittest.RequireCloseBefore(u.T(), u.waitCh, u.channelCloseDuration, "could close ch on time")
}

// TestUnicastAuthorization_PublicChannel tests that messages sent via unicast on a public channel are not rejected for any reason.
func (u *UnicastAuthorizationTestSuite) TestUnicastAuthorization_PublicChannel() {
	// setup mock slashing violations consumer and middlewares
	slashingViolationsConsumer := mocknetwork.NewViolationsConsumer(u.T())
	u.setupMiddlewaresAndProviders(slashingViolationsConsumer)

	expectedPayload := "hello"
	msg, err := network.NewOutgoingScope(
		flow.IdentifierList{u.receiverID.NodeID},
		testChannel,
		&libp2pmessage.TestMessage{
			Text: expectedPayload,
		},
		unittest.NetworkCodec().Encode,
		message.ProtocolTypeUnicast)
	require.NoError(u.T(), err)

	overlay := mocknetwork.NewOverlay(u.T())
	overlay.On("Identities").Maybe().Return(func() flow.IdentityList {
		return u.providers[0].Identities(filter.Any)
	})
	overlay.On("Topology").Maybe().Return(func() flow.IdentityList {
		return u.providers[0].Identities(filter.Any)
	}, nil)
	overlay.On("Identity", mock.AnythingOfType("peer.ID")).Return(u.senderID, true)

	// we should receive the message on our overlay, at this point close the waitCh
	overlay.On("Receive", mockery.Anything).Return(nil).
		Once().
		Run(func(args mockery.Arguments) {
			close(u.waitCh)

			msg, ok := args[0].(*network.IncomingMessageScope)
			require.True(u.T(), ok)

			require.Equal(u.T(), testChannel, msg.Channel())                                              // channel
			require.Equal(u.T(), u.senderID.NodeID, msg.OriginId())                                       // sender id
			require.Equal(u.T(), u.receiverID.NodeID, msg.TargetIDs()[0])                                 // target id
			require.Equal(u.T(), message.ProtocolTypeUnicast, msg.Protocol())                             // protocol
			require.Equal(u.T(), expectedPayload, msg.DecodedPayload().(*libp2pmessage.TestMessage).Text) // payload
		})

	u.startMiddlewares(overlay)

	require.NoError(u.T(), u.receiverMW.Subscribe(testChannel))
	require.NoError(u.T(), u.senderMW.Subscribe(testChannel))

	// send message via unicast
	err = u.senderMW.SendDirect(msg)
	require.NoError(u.T(), err)

	// wait for slashing violations consumer mock to invoke run func and close ch if expected method call happens
	unittest.RequireCloseBefore(u.T(), u.waitCh, u.channelCloseDuration, "could close ch on time")
}

// TestUnicastAuthorization_UnauthorizedUnicastOnChannel tests that messages sent via unicast that are not authorized for unicast are rejected.
func (u *UnicastAuthorizationTestSuite) TestUnicastAuthorization_UnauthorizedUnicastOnChannel() {
	// setup mock slashing violations consumer and middlewares
	slashingViolationsConsumer := mocknetwork.NewViolationsConsumer(u.T())
	u.setupMiddlewaresAndProviders(slashingViolationsConsumer)

	// set sender id role to RoleConsensus to avoid unauthorized sender validation error
	u.senderID.Role = flow.RoleConsensus

	expectedSenderPeerID, err := unittest.PeerIDFromFlowID(u.senderID)
	require.NoError(u.T(), err)

	expectedViolation := &network.Violation{
		Identity: u.senderID,
		OriginID: u.senderID.NodeID,
		PeerID:   expectedSenderPeerID.String(),
		MsgType:  "*messages.BlockProposal",
		Channel:  channels.ConsensusCommittee,
		Protocol: message.ProtocolTypeUnicast,
		Err:      message.ErrUnauthorizedUnicastOnChannel,
	}

	slashingViolationsConsumer.On(
		"OnUnauthorizedUnicastOnChannel",
		expectedViolation,
	).Return(nil).Return(nil).Once().Run(func(args mockery.Arguments) {
		close(u.waitCh)
	})

	overlay := mocknetwork.NewOverlay(u.T())
	overlay.On("Identities").Maybe().Return(func() flow.IdentityList {
		return u.providers[0].Identities(filter.Any)
	})
	overlay.On("Topology").Maybe().Return(func() flow.IdentityList {
		return u.providers[0].Identities(filter.Any)
	}, nil)
	overlay.On("Identity", expectedSenderPeerID).Return(u.senderID, true)

	// message will be rejected so assert overlay never receives it
	defer overlay.AssertNotCalled(u.T(), "Receive", u.senderID.NodeID, mock.AnythingOfType("*message.Message"))

	u.startMiddlewares(overlay)

	channel := channels.ConsensusCommittee
	require.NoError(u.T(), u.receiverMW.Subscribe(channel))
	require.NoError(u.T(), u.senderMW.Subscribe(channel))

	// messages.BlockProposal is not authorized to be sent via unicast over the ConsensusCommittee channel
	payload := unittest.ProposalFixture()

	msg, err := network.NewOutgoingScope(
		flow.IdentifierList{u.receiverID.NodeID},
		channel,
		payload,
		unittest.NetworkCodec().Encode,
		message.ProtocolTypeUnicast)
	require.NoError(u.T(), err)

	// send message via unicast
	err = u.senderMW.SendDirect(msg)
	require.NoError(u.T(), err)

	// wait for slashing violations consumer mock to invoke run func and close ch if expected method call happens
	unittest.RequireCloseBefore(u.T(), u.waitCh, u.channelCloseDuration, "could close ch on time")
}

// TestUnicastAuthorization_ReceiverHasNoSubscription tests that messages sent via unicast are rejected on the receiver end if the receiver does not have a subscription
// to the channel of the message.
func (u *UnicastAuthorizationTestSuite) TestUnicastAuthorization_ReceiverHasNoSubscription() {
	// setup mock slashing violations consumer and middlewares
	slashingViolationsConsumer := mocknetwork.NewViolationsConsumer(u.T())
	u.setupMiddlewaresAndProviders(slashingViolationsConsumer)

	expectedSenderPeerID, err := unittest.PeerIDFromFlowID(u.senderID)
	require.NoError(u.T(), err)

	expectedViolation := &network.Violation{
		Identity: nil,
		PeerID:   expectedSenderPeerID.String(),
		MsgType:  "*message.TestMessage",
		Channel:  channels.TestNetworkChannel,
		Protocol: message.ProtocolTypeUnicast,
		Err:      middleware.ErrUnicastMsgWithoutSub,
	}

	slashingViolationsConsumer.On(
		"OnUnauthorizedUnicastOnChannel",
		expectedViolation,
	).Return(nil).Return(nil).Once().Run(func(args mockery.Arguments) {
		close(u.waitCh)
	})

	overlay := mocknetwork.NewOverlay(u.T())
	overlay.On("Identities").Maybe().Return(func() flow.IdentityList {
		return u.providers[0].Identities(filter.Any)
	})
	overlay.On("Topology").Maybe().Return(func() flow.IdentityList {
		return u.providers[0].Identities(filter.Any)
	}, nil)

	// message will be rejected so assert overlay never receives it
	defer overlay.AssertNotCalled(u.T(), "Receive", u.senderID.NodeID, mock.AnythingOfType("*message.Message"))

	u.startMiddlewares(overlay)

	channel := channels.TestNetworkChannel

	msg, err := network.NewOutgoingScope(
		flow.IdentifierList{u.receiverID.NodeID},
		channel,
		&libp2pmessage.TestMessage{
			Text: "TestUnicastAuthorization_ReceiverHasNoSubscription",
		},
		unittest.NetworkCodec().Encode,
		message.ProtocolTypeUnicast)
	require.NoError(u.T(), err)

	// send message via unicast
	err = u.senderMW.SendDirect(msg)
	require.NoError(u.T(), err)

	// wait for slashing violations consumer mock to invoke run func and close ch if expected method call happens
	unittest.RequireCloseBefore(u.T(), u.waitCh, u.channelCloseDuration, "could close ch on time")
}

// TestUnicastAuthorization_ReceiverHasSubscription tests that messages sent via unicast are processed on the receiver end if the receiver does have a subscription
// to the channel of the message.
func (u *UnicastAuthorizationTestSuite) TestUnicastAuthorization_ReceiverHasSubscription() {
	// setup mock slashing violations consumer and middlewares
	slashingViolationsConsumer := mocknetwork.NewViolationsConsumer(u.T())
	u.setupMiddlewaresAndProviders(slashingViolationsConsumer)
	channel := channels.RequestReceiptsByBlockID

	msg, err := network.NewOutgoingScope(
		flow.IdentifierList{u.receiverID.NodeID},
		channel,
		&messages.EntityRequest{},
		unittest.NetworkCodec().Encode,
		message.ProtocolTypeUnicast)
	require.NoError(u.T(), err)

	u.senderID.Role = flow.RoleConsensus
	u.receiverID.Role = flow.RoleExecution

	overlay := mocknetwork.NewOverlay(u.T())
	overlay.On("Identities").Maybe().Return(func() flow.IdentityList {
		return u.providers[0].Identities(filter.Any)
	})
	overlay.On("Topology").Maybe().Return(func() flow.IdentityList {
		return u.providers[0].Identities(filter.Any)
	}, nil)
	overlay.On("Identity", mock.AnythingOfType("peer.ID")).Return(u.senderID, true)

	// we should receive the message on our overlay, at this point close the waitCh
	overlay.On("Receive", mockery.Anything).Return(nil).
		Once().
		Run(func(args mockery.Arguments) {
			close(u.waitCh)

			msg, ok := args[0].(*network.IncomingMessageScope)
			require.True(u.T(), ok)

			require.Equal(u.T(), channel, msg.Channel())                      // channel
			require.Equal(u.T(), u.senderID.NodeID, msg.OriginId())           // sender id
			require.Equal(u.T(), u.receiverID.NodeID, msg.TargetIDs()[0])     // target id
			require.Equal(u.T(), message.ProtocolTypeUnicast, msg.Protocol()) // protocol
		})

	u.startMiddlewares(overlay)

	require.NoError(u.T(), u.receiverMW.Subscribe(channel))
	require.NoError(u.T(), u.senderMW.Subscribe(channel))

	// send message via unicast
	err = u.senderMW.SendDirect(msg)
	require.NoError(u.T(), err)

	// wait for slashing violations consumer mock to invoke run func and close ch if expected method call happens
	unittest.RequireCloseBefore(u.T(), u.waitCh, u.channelCloseDuration, "could close ch on time")
}
