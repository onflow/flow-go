package test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	mockery "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/codec"
	"github.com/onflow/flow-go/network/internal/messageutils"
	"github.com/onflow/flow-go/network/internal/testutils"
	"github.com/onflow/flow-go/network/message"
	"github.com/onflow/flow-go/network/mocknetwork"
	"github.com/onflow/flow-go/network/slashing"
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

	// senderMW is the mw that will be sending the message
	senderMW network.Middleware
	// senderID the identity on the mw sending the message
	senderID *flow.Identity
	// receiverMW is the mw that will be sending the message
	receiverMW network.Middleware
	// receiverID the identity on the mw sending the message
	receiverID *flow.Identity
	// providers id providers generated at beginning of a test run
	providers []*testutils.UpdatableIDProvider
	// cancel is the cancel func from the context that was used to start the middlewares in a test run
	cancel context.CancelFunc
	// waitCh is the channel used to wait for the middleware to perform authorization and invoke the slashing
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
	// this ch will allow us to wait until the expected method call happens before shutting down middleware
	u.waitCh = make(chan struct{})
}

func (u *UnicastAuthorizationTestSuite) TearDownTest() {
	u.stopMiddlewares()
}

// setupMiddlewaresAndProviders will setup 2 middlewares that will be used as a sender and receiver in each suite test.
func (u *UnicastAuthorizationTestSuite) setupMiddlewaresAndProviders(slashingViolationsConsumer slashing.ViolationsConsumer) {
	ids, libP2PNodes, _ := testutils.GenerateIDs(u.T(), u.logger, 2)
	mws, providers := testutils.GenerateMiddlewares(u.T(), u.logger, ids, libP2PNodes, unittest.NetworkCodec(), slashingViolationsConsumer)
	require.Len(u.T(), ids, 2)
	require.Len(u.T(), providers, 2)
	require.Len(u.T(), mws, 2)

	u.senderID = ids[0]
	u.senderMW = mws[0]
	u.receiverID = ids[1]
	u.receiverMW = mws[1]
	u.providers = providers
}

// startMiddlewares will start both sender and receiver middlewares with an irrecoverable signaler context and set the context cancel func.
func (u *UnicastAuthorizationTestSuite) startMiddlewares(overlay *mocknetwork.Overlay) {
	ctx, cancel := context.WithCancel(context.Background())
	mwCtx, _ := irrecoverable.WithSignaler(ctx)

	u.senderMW.SetOverlay(overlay)
	u.senderMW.Start(mwCtx)
	<-u.senderMW.Ready()

	u.receiverMW.SetOverlay(overlay)
	u.receiverMW.Start(mwCtx)
	<-u.receiverMW.Ready()

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
	expectedViolation := &slashing.Violation{
		Identity:  nilID, // because the peer will be unverified this identity will be nil
		PeerID:    expectedSenderPeerID.String(),
		MsgType:   "",                          // message will not be decoded before OnSenderEjectedError is logged, we won't log message type
		Channel:   channels.TestNetworkChannel, // message will not be decoded before OnSenderEjectedError is logged, we won't log peer ID
		IsUnicast: true,
		Err:       validator.ErrIdentityUnverified,
	}
	slashingViolationsConsumer.On(
		"OnUnAuthorizedSenderError",
		expectedViolation,
	).Once().Run(func(args mockery.Arguments) {
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
	defer overlay.AssertNotCalled(u.T(), "Receive", u.senderID.NodeID, mock.AnythingOfType("*message.Message"))

	u.startMiddlewares(overlay)

	msg, _, _ := messageutils.CreateMessage(u.T(), u.senderID.NodeID, u.receiverID.NodeID, testChannel, "hello")

	// send message via unicast
	err = u.senderMW.SendDirect(msg, u.receiverID.NodeID)
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

	expectedViolation := &slashing.Violation{
		Identity:  u.senderID, // we expect this method to be called with the ejected identity
		PeerID:    expectedSenderPeerID.String(),
		MsgType:   "",                          // message will not be decoded before OnSenderEjectedError is logged, we won't log message type
		Channel:   channels.TestNetworkChannel, // message will not be decoded before OnSenderEjectedError is logged, we won't log peer ID
		IsUnicast: true,
		Err:       validator.ErrSenderEjected,
	}
	slashingViolationsConsumer.On(
		"OnSenderEjectedError",
		expectedViolation,
	).Once().Run(func(args mockery.Arguments) {
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
	defer overlay.AssertNotCalled(u.T(), "Receive", u.senderID.NodeID, mock.AnythingOfType("*message.Message"))

	u.startMiddlewares(overlay)

	msg, _, _ := messageutils.CreateMessage(u.T(), u.senderID.NodeID, u.receiverID.NodeID, testChannel, "hello")

	// send message via unicast
	err = u.senderMW.SendDirect(msg, u.receiverID.NodeID)
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

	expectedViolation := &slashing.Violation{
		Identity:  u.senderID,
		PeerID:    expectedSenderPeerID.String(),
		MsgType:   message.TestMessage,
		Channel:   channels.ConsensusCommittee,
		IsUnicast: true,
		Err:       message.ErrUnauthorizedMessageOnChannel,
	}

	slashingViolationsConsumer.On(
		"OnUnAuthorizedSenderError",
		expectedViolation,
	).Once().Run(func(args mockery.Arguments) {
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

	msg, _, _ := messageutils.CreateMessage(u.T(), u.senderID.NodeID, u.receiverID.NodeID, testChannel, "hello")

	// set channel ID to an unauthorized channel for TestMessage
	msg.ChannelID = channels.ConsensusCommittee.String()

	// send message via unicast
	err = u.senderMW.SendDirect(msg, u.receiverID.NodeID)
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

	invalidMessageCode := byte('X')

	var nilID *flow.Identity
	expectedViolation := &slashing.Violation{
		Identity:  nilID,
		PeerID:    expectedSenderPeerID.String(),
		MsgType:   "",
		Channel:   channels.TestNetworkChannel,
		IsUnicast: true,
		Err:       codec.NewUnknownMsgCodeErr(invalidMessageCode),
	}

	slashingViolationsConsumer.On(
		"OnUnknownMsgTypeError",
		expectedViolation,
	).Once().Run(func(args mockery.Arguments) {
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

	msg, _, _ := messageutils.CreateMessage(u.T(), u.senderID.NodeID, u.receiverID.NodeID, testChannel, "hello")

	// manipulate message code byte
	msg.Payload[0] = invalidMessageCode

	// send message via unicast
	err = u.senderMW.SendDirect(msg, u.receiverID.NodeID)
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

	var nilID *flow.Identity
	expectedViolation := &slashing.Violation{
		Identity:  nilID,
		PeerID:    expectedSenderPeerID.String(),
		MsgType:   "",
		Channel:   channels.TestNetworkChannel,
		IsUnicast: true,
		//NOTE: in this test the message code does not match the underlying message type causing the codec to fail to unmarshal the message when decoding.
		Err: codec.NewMsgUnmarshalErr(modifiedMessageCode, message.DKGMessage, fmt.Errorf("cbor: found unknown field at map element index 0")),
	}

	slashingViolationsConsumer.On(
		"OnInvalidMsgError",
		expectedViolation,
	).Once().Run(func(args mockery.Arguments) {
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

	msg, _, _ := messageutils.CreateMessage(u.T(), u.senderID.NodeID, u.receiverID.NodeID, testChannel, "hello")

	// manipulate message code byte
	msg.Payload[0] = modifiedMessageCode

	// send message via unicast
	err = u.senderMW.SendDirect(msg, u.receiverID.NodeID)
	require.NoError(u.T(), err)

	// wait for slashing violations consumer mock to invoke run func and close ch if expected method call happens
	unittest.RequireCloseBefore(u.T(), u.waitCh, u.channelCloseDuration, "could close ch on time")
}

// TestUnicastAuthorization_PublicChannel tests that messages sent via unicast on a public channel are not rejected for any reason.
func (u *UnicastAuthorizationTestSuite) TestUnicastAuthorization_PublicChannel() {
	// setup mock slashing violations consumer and middlewares
	slashingViolationsConsumer := mocknetwork.NewViolationsConsumer(u.T())
	u.setupMiddlewaresAndProviders(slashingViolationsConsumer)

	msg, expectedMsg, payload := messageutils.CreateMessage(u.T(), u.senderID.NodeID, u.receiverID.NodeID, testChannel, "hello")

	overlay := mocknetwork.NewOverlay(u.T())
	overlay.On("Identities").Maybe().Return(func() flow.IdentityList {
		return u.providers[0].Identities(filter.Any)
	})
	overlay.On("Topology").Maybe().Return(func() flow.IdentityList {
		return u.providers[0].Identities(filter.Any)
	}, nil)
	overlay.On("Identity", mock.AnythingOfType("peer.ID")).Return(u.senderID, true)

	// we should receive the message on our overlay, at this point close the waitCh
	overlay.On("Receive", u.senderID.NodeID, expectedMsg, payload).Return(nil).
		Once().
		Run(func(args mockery.Arguments) {
			close(u.waitCh)
		})

	u.startMiddlewares(overlay)

	// send message via unicast
	err := u.senderMW.SendDirect(msg, u.receiverID.NodeID)
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

	expectedViolation := &slashing.Violation{
		Identity:  u.senderID,
		PeerID:    expectedSenderPeerID.String(),
		MsgType:   "BlockProposal",
		Channel:   channels.ConsensusCommittee,
		IsUnicast: true,
		Err:       message.ErrUnauthorizedUnicastOnChannel,
	}

	slashingViolationsConsumer.On(
		"OnUnauthorizedUnicastOnChannel",
		expectedViolation,
	).Once().Run(func(args mockery.Arguments) {
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

	// messages.BlockProposal is not authorized to be sent via unicast over the ConsensusCommittee channel
	payload := unittest.ProposalFixture()
	msg, _, _ := messageutils.CreateMessageWithPayload(u.T(), u.senderID.NodeID, u.receiverID.NodeID, channels.ConsensusCommittee, payload)

	// send message via unicast
	err = u.senderMW.SendDirect(msg, u.receiverID.NodeID)
	require.NoError(u.T(), err)

	// wait for slashing violations consumer mock to invoke run func and close ch if expected method call happens
	unittest.RequireCloseBefore(u.T(), u.waitCh, u.channelCloseDuration, "could close ch on time")
}
