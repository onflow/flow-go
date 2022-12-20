package test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/net/swarm"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	mockery "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/atomic"
	"golang.org/x/time/rate"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	libp2pmessage "github.com/onflow/flow-go/model/libp2p/message"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/module/observable"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/internal/messageutils"
	"github.com/onflow/flow-go/network/internal/testutils"
	"github.com/onflow/flow-go/network/message"
	"github.com/onflow/flow-go/network/mocknetwork"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/middleware"
	"github.com/onflow/flow-go/network/p2p/p2pnode"
	"github.com/onflow/flow-go/network/p2p/unicast/ratelimit"
	"github.com/onflow/flow-go/network/slashing"
	"github.com/onflow/flow-go/utils/unittest"
)

const testChannel = channels.TestNetworkChannel

// libp2p emits a call to `Protect` with a topic-specific tag upon establishing each peering connection in a GossipSUb mesh, see:
// https://github.com/libp2p/go-libp2p-pubsub/blob/master/tag_tracer.go
// One way to make sure such a mesh has formed, asynchronously, in unit tests, is to wait for libp2p.GossipSubD such calls,
// and that's what we do with tagsObserver.
type tagsObserver struct {
	tags chan string
	log  zerolog.Logger
}

func (co *tagsObserver) OnNext(peertag interface{}) {
	pt, ok := peertag.(testutils.PeerTag)

	if ok {
		co.tags <- fmt.Sprintf("peer: %v tag: %v", pt.Peer, pt.Tag)
	}

}
func (co *tagsObserver) OnError(err error) {
	co.log.Error().Err(err).Msg("Tags Observer closed on an error")
	close(co.tags)
}
func (co *tagsObserver) OnComplete() {
	close(co.tags)
}

type MiddlewareTestSuite struct {
	suite.Suite
	sync.RWMutex
	size      int // used to determine number of middlewares under test
	nodes     []p2p.LibP2PNode
	mws       []network.Middleware // used to keep track of middlewares under test
	ov        []*mocknetwork.Overlay
	obs       chan string // used to keep track of Protect events tagged by pubsub messages
	ids       []*flow.Identity
	metrics   *metrics.NoopCollector // no-op performance monitoring simulation
	logger    zerolog.Logger
	providers []*testutils.UpdatableIDProvider

	mwCancel context.CancelFunc
	mwCtx    irrecoverable.SignalerContext

	slashingViolationsConsumer slashing.ViolationsConsumer
}

// TestMiddlewareTestSuit runs all the test methods in this test suit
func TestMiddlewareTestSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(MiddlewareTestSuite))
}

// SetupTest initiates the test setups prior to each test
func (m *MiddlewareTestSuite) SetupTest() {
	m.logger = unittest.Logger()

	m.size = 2 // operates on two middlewares
	m.metrics = metrics.NewNoopCollector()

	// create and start the middlewares and inject a connection observer
	var obs []observable.Observable
	peerChannel := make(chan string)
	ob := tagsObserver{
		tags: peerChannel,
		log:  m.logger,
	}

	m.slashingViolationsConsumer = mocknetwork.NewViolationsConsumer(m.T())

	m.ids, m.nodes, m.mws, obs, m.providers = testutils.GenerateIDsAndMiddlewares(m.T(),
		m.size,
		m.logger,
		unittest.NetworkCodec(),
		m.slashingViolationsConsumer)

	for _, observableConnMgr := range obs {
		observableConnMgr.Subscribe(&ob)
	}
	m.obs = peerChannel

	require.Len(m.Suite.T(), obs, m.size)
	require.Len(m.Suite.T(), m.ids, m.size)
	require.Len(m.Suite.T(), m.mws, m.size)

	// create the mock overlays
	for i := 0; i < m.size; i++ {
		m.ov = append(m.ov, m.createOverlay(m.providers[i]))
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.mwCancel = cancel

	m.mwCtx = irrecoverable.NewMockSignalerContext(m.T(), ctx)

	testutils.StartNodes(m.mwCtx, m.T(), m.nodes, 100*time.Millisecond)

	for i, mw := range m.mws {
		mw.SetOverlay(m.ov[i])
		mw.Start(m.mwCtx)
		unittest.RequireComponentsReadyBefore(m.T(), 100*time.Millisecond, mw)
		require.NoError(m.T(), mw.Subscribe(testChannel))
	}
}

func (m *MiddlewareTestSuite) TearDownTest() {
	m.mwCancel()

	testutils.StopComponents(m.T(), m.mws, 100*time.Millisecond)
	testutils.StopComponents(m.T(), m.nodes, 100*time.Millisecond)

	m.mws = nil
	m.nodes = nil
	m.ov = nil
	m.ids = nil
	m.size = 0
}

// TestUpdateNodeAddresses tests that the UpdateNodeAddresses method correctly updates
// the addresses of the staked network participants.
func (m *MiddlewareTestSuite) TestUpdateNodeAddresses() {
	ctx, cancel := context.WithCancel(m.mwCtx)
	irrecoverableCtx := irrecoverable.NewMockSignalerContext(m.T(), ctx)

	// create a new staked identity
	ids, libP2PNodes, _ := testutils.GenerateIDs(m.T(), m.logger, 1)

	mws, providers := testutils.GenerateMiddlewares(m.T(), m.logger, ids, libP2PNodes, unittest.NetworkCodec(), m.slashingViolationsConsumer)
	require.Len(m.T(), ids, 1)
	require.Len(m.T(), providers, 1)
	require.Len(m.T(), mws, 1)
	newId := ids[0]
	newMw := mws[0]

	overlay := m.createOverlay(providers[0])
	overlay.On("Receive", m.ids[0].NodeID, mockery.AnythingOfType("*message.Message")).Return(nil)
	newMw.SetOverlay(overlay)

	// start up nodes and peer managers
	testutils.StartNodes(irrecoverableCtx, m.T(), libP2PNodes, 100*time.Millisecond)
	defer testutils.StopComponents(m.T(), libP2PNodes, 100*time.Millisecond)

	newMw.Start(irrecoverableCtx)
	defer testutils.StopComponents(m.T(), mws, 100*time.Millisecond)
	unittest.RequireComponentsReadyBefore(m.T(), 100*time.Millisecond, newMw)

	idList := flow.IdentityList(append(m.ids, newId))

	// needed to enable ID translation
	m.providers[0].SetIdentities(idList)

	msg, _, _ := messageutils.CreateMessage(m.T(), m.ids[0].NodeID, newId.NodeID, testChannel, "TestUpdateNodeAddresses")

	// message should fail to send because no address is known yet
	// for the new identity
	err := m.mws[0].SendDirect(msg, newId.NodeID)
	require.ErrorIs(m.T(), err, swarm.ErrNoAddresses)

	// update the addresses
	m.mws[0].UpdateNodeAddresses()

	// now the message should send successfully
	err = m.mws[0].SendDirect(msg, newId.NodeID)
	require.NoError(m.T(), err)

	cancel()
	unittest.RequireComponentsReadyBefore(m.T(), 100*time.Millisecond, newMw)
}

func (m *MiddlewareTestSuite) TestUnicastRateLimit_Messages() {
	// limiter limit will be set to 5 events/sec the 6th event per interval will be rate limited
	limit := rate.Limit(5)

	// burst per interval
	burst := 5

	// create test time
	testtime := unittest.NewTestTime()

	// setup message rate limiter
	messageRateLimiter := ratelimit.NewMessageRateLimiter(limit, burst, 1, p2p.WithGetTimeNowFunc(testtime.Now))

	// the onUnicastRateLimitedPeerFunc call back we will use to keep track of how many times a rate limit happens
	// after 5 rate limits we will close ch. O
	ch := make(chan struct{})
	rateLimits := atomic.NewUint64(0)
	onRateLimit := func(peerID peer.ID, role, msgType string, topic channels.Topic, reason ratelimit.RateLimitReason) {
		require.Equal(m.T(), reason, ratelimit.ReasonMessageCount)

		// we only expect messages from the first middleware on the test suite
		expectedPID, err := unittest.PeerIDFromFlowID(m.ids[0])
		require.NoError(m.T(), err)
		require.Equal(m.T(), expectedPID, peerID)

		// update hook calls
		rateLimits.Inc()
	}

	rateLimiters := ratelimit.NewRateLimiters(messageRateLimiter,
		&ratelimit.NoopRateLimiter{},
		onRateLimit,
		ratelimit.WithDisabledRateLimiting(false))

	// create a new staked identity
	ids, libP2PNodes, _ := testutils.GenerateIDs(m.T(), m.logger, 1)

	// create middleware
	netmet := mock.NewNetworkMetrics(m.T())
	calls := 0
	netmet.On("NetworkMessageReceived", mockery.Anything, mockery.Anything, mockery.Anything).Times(5).Run(func(args mockery.Arguments) {
		calls++
		if calls == 5 {
			close(ch)
		}
	})
	// we expect 5 messages to be processed the rest will be rate limited
	defer netmet.AssertNumberOfCalls(m.T(), "NetworkMessageReceived", 5)

	mws, providers := testutils.GenerateMiddlewares(m.T(),
		m.logger,
		ids,
		libP2PNodes,
		unittest.NetworkCodec(),
		m.slashingViolationsConsumer,
		testutils.WithUnicastRateLimiters(rateLimiters),
		testutils.WithNetworkMetrics(netmet))

	require.Len(m.T(), ids, 1)
	require.Len(m.T(), providers, 1)
	require.Len(m.T(), mws, 1)
	newId := ids[0]
	newMw := mws[0]

	overlay := m.createOverlay(providers[0])
	overlay.On("Receive", m.ids[0].NodeID, mockery.AnythingOfType("*message.Message")).Return(nil)

	newMw.SetOverlay(overlay)

	ctx, cancel := context.WithCancel(m.mwCtx)
	irrecoverableCtx := irrecoverable.NewMockSignalerContext(m.T(), ctx)

	testutils.StartNodes(irrecoverableCtx, m.T(), libP2PNodes, 100*time.Millisecond)
	defer testutils.StopComponents(m.T(), libP2PNodes, 100*time.Millisecond)

	newMw.Start(irrecoverableCtx)
	unittest.RequireComponentsReadyBefore(m.T(), 100*time.Millisecond, newMw)

	require.NoError(m.T(), newMw.Subscribe(testChannel))

	idList := flow.IdentityList(append(m.ids, newId))

	// needed to enable ID translation
	m.providers[0].SetIdentities(idList)

	// update the addresses
	m.mws[0].UpdateNodeAddresses()

	// for the duration of a simulated second we will send 6 messages. The 6th message will be
	// rate limited.
	start := testtime.Now()
	end := start.Add(time.Second)
	for testtime.Now().Before(end) {
		// a message is sent every 167 milliseconds which equates to around 6 req/sec surpassing our limit
		testtime.Advance(168 * time.Millisecond)

		msg, _, _ := messageutils.CreateMessage(m.T(), m.ids[0].NodeID, newId.NodeID, testChannel, fmt.Sprintf("hello-%s", testtime.Now().String()))
		err := m.mws[0].SendDirect(msg, newId.NodeID)
		require.NoError(m.T(), err)
	}

	// wait for all rate limits before shutting down middleware
	unittest.RequireCloseBefore(m.T(), ch, 100*time.Millisecond, "could not stop on rate limit test ch on time")

	// shutdown our middleware so that each message can be processed
	cancel()
	unittest.RequireCloseBefore(m.T(), libP2PNodes[0].Done(), 100*time.Millisecond, "could not stop libp2p node on time")
	unittest.RequireCloseBefore(m.T(), newMw.Done(), 100*time.Millisecond, "could not stop middleware on time")

	// expect our rate limited peer callback to be invoked once
	require.Equal(m.T(), uint64(1), rateLimits.Load())
}

func (m *MiddlewareTestSuite) TestUnicastRateLimit_Bandwidth() {
	//limiter limit will be set up to 1000 bytes/sec
	limit := rate.Limit(1000)

	//burst per interval
	burst := 1000

	// create test time
	testtime := unittest.NewTestTime()

	// setup bandwidth rate limiter
	bandwidthRateLimiter := ratelimit.NewBandWidthRateLimiter(limit, burst, 1, p2p.WithGetTimeNowFunc(testtime.Now))

	// the onUnicastRateLimitedPeerFunc call back we will use to keep track of how many times a rate limit happens
	// after 5 rate limits we will close ch.
	ch := make(chan struct{})
	rateLimits := atomic.NewUint64(0)
	onRateLimit := func(peerID peer.ID, role, msgType string, topic channels.Topic, reason ratelimit.RateLimitReason) {
		require.Equal(m.T(), reason, ratelimit.ReasonBandwidth)

		// we only expect messages from the first middleware on the test suite
		expectedPID, err := unittest.PeerIDFromFlowID(m.ids[0])
		require.NoError(m.T(), err)
		require.Equal(m.T(), expectedPID, peerID)
		// update hook calls
		rateLimits.Inc()
		close(ch)
	}

	rateLimiters := ratelimit.NewRateLimiters(&ratelimit.NoopRateLimiter{},
		bandwidthRateLimiter,
		onRateLimit,
		ratelimit.WithDisabledRateLimiting(false))

	// create a new staked identity
	ids, libP2PNodes, _ := testutils.GenerateIDs(m.T(), m.logger, 1)

	// create middleware
	opts := testutils.WithUnicastRateLimiters(rateLimiters)
	mws, providers := testutils.GenerateMiddlewares(m.T(), m.logger, ids, libP2PNodes, unittest.NetworkCodec(), m.slashingViolationsConsumer, opts)
	require.Len(m.T(), ids, 1)
	require.Len(m.T(), providers, 1)
	require.Len(m.T(), mws, 1)
	newId := ids[0]
	newMw := mws[0]

	overlay := m.createOverlay(providers[0])
	overlay.On("Receive", m.ids[0].NodeID, mockery.AnythingOfType("*message.Message")).Return(nil)

	newMw.SetOverlay(overlay)

	ctx, cancel := context.WithCancel(m.mwCtx)
	irrecoverableCtx := irrecoverable.NewMockSignalerContext(m.T(), ctx)

	testutils.StartNodes(irrecoverableCtx, m.T(), libP2PNodes, 100*time.Millisecond)
	defer testutils.StopComponents(m.T(), libP2PNodes, 100*time.Millisecond)

	newMw.Start(irrecoverableCtx)
	unittest.RequireComponentsReadyBefore(m.T(), 100*time.Millisecond, newMw)

	require.NoError(m.T(), newMw.Subscribe(testChannel))

	idList := flow.IdentityList(append(m.ids, newId))

	// needed to enable ID translation
	m.providers[0].SetIdentities(idList)

	// create message with about 400bytes (300 random bytes + 100bytes message info)
	b := make([]byte, 300)
	for i := range b {
		b[i] = byte('X')
	}

	msg, _, _ := messageutils.CreateMessage(m.T(), m.ids[0].NodeID, newId.NodeID, testChannel, string(b))

	// update the addresses
	m.mws[0].UpdateNodeAddresses()

	// for the duration of a simulated second we will send 3 messages. Each message is about
	// 400 bytes, the 3rd message will put our limiter over the 1000 byte limit at 1200 bytes. Thus
	// the 3rd message should be rate limited.
	start := testtime.Now()
	end := start.Add(time.Second)
	for testtime.Now().Before(end) {
		err := m.mws[0].SendDirect(msg, newId.NodeID)
		require.NoError(m.T(), err)

		// send 3 messages
		testtime.Advance(334 * time.Millisecond)
	}

	// wait for all rate limits before shutting down middleware
	unittest.RequireCloseBefore(m.T(), ch, 100*time.Millisecond, "could not stop on rate limit test ch on time")

	// shutdown our middleware so that each message can be processed
	cancel()
	unittest.RequireComponentsDoneBefore(m.T(), 100*time.Millisecond, newMw)

	// expect our rate limited peer callback to be invoked once
	require.Equal(m.T(), uint64(1), rateLimits.Load())
}

func (m *MiddlewareTestSuite) createOverlay(provider *testutils.UpdatableIDProvider) *mocknetwork.Overlay {
	overlay := &mocknetwork.Overlay{}
	overlay.On("Identities").Maybe().Return(func() flow.IdentityList {
		return provider.Identities(filter.Any)
	})
	overlay.On("Topology").Maybe().Return(func() flow.IdentityList {
		return provider.Identities(filter.Any)
	}, nil)
	// this test is not testing the topic validator, especially in spoofing,
	// so we always return a valid identity. We only care about the node role for the test TestMaxMessageSize_SendDirect
	// where EN are the only node authorized to send chunk data response.
	identityOpts := unittest.WithRole(flow.RoleExecution)
	overlay.On("Identity", mockery.AnythingOfType("peer.ID")).Maybe().Return(unittest.IdentityFixture(identityOpts), true)
	return overlay
}

// TestPingRawReception tests the middleware for solely the
// reception of a single ping message by a node that is sent from another node
// it does not evaluate the type and content of the message
func (m *MiddlewareTestSuite) TestPingRawReception() {
	msg, _, _ := messageutils.CreateMessage(m.T(), m.ids[0].NodeID, m.ids[1].NodeID, testChannel, "TestPingRawReception")
	m.Ping(msg, mockery.Anything, mockery.Anything, mockery.Anything)
}

// TestPingTypeReception tests the middleware against type of received payload
// upon reception at the receiver side
// it does not evaluate content of the payload
// it does not evaluate anything related to the sender id
func (m *MiddlewareTestSuite) TestPingTypeReception() {
	msg, _, _ := messageutils.CreateMessage(m.T(), m.ids[0].NodeID, m.ids[1].NodeID, testChannel, "TestPingTypeReception")
	m.Ping(msg, mockery.Anything, mockery.AnythingOfType("*message.Message"), mockery.Anything)
}

// TestPingIDType tests the middleware against both the type of sender id
// and content of the payload of the event upon reception at the receiver side
// it does not evaluate the actual value of the sender ID
func (m *MiddlewareTestSuite) TestPingIDType() {
	msg, expectedMsg, decodedPayload := messageutils.CreateMessage(m.T(), m.ids[0].NodeID, m.ids[1].NodeID, testChannel, "TestPingIDType")
	m.Ping(msg, mockery.AnythingOfType("flow.Identifier"), expectedMsg, decodedPayload)
}

// TestPingContentReception tests the middleware against both
// the payload and sender ID of the event upon reception at the receiver side
func (m *MiddlewareTestSuite) TestPingContentReception() {
	msg, expectedMsg, decodedPayload := messageutils.CreateMessage(m.T(), m.ids[0].NodeID, m.ids[1].NodeID, testChannel, "TestPingContentReception")
	m.Ping(msg, m.ids[0].NodeID, expectedMsg, decodedPayload)
}

// TestMultiPing tests the middleware against type of received payload
// of distinct messages that are sent concurrently from a node to another
func (m *MiddlewareTestSuite) TestMultiPing() {
	// one distinct message
	m.MultiPing(1)

	// two distinct messages
	m.MultiPing(2)

	// 10 distinct messages
	m.MultiPing(10)
}

// Ping sends a message from the first middleware of the test suit to the last one
// expectID and expectPayload are what we expect the receiver side to evaluate the
// incoming ping against, it can be mocked or typed data
func (m *MiddlewareTestSuite) Ping(msg *message.Message, expectID, expectedMessage, expectedPayload interface{}) {

	ch := make(chan struct{})
	// extracts sender id based on the mock option
	var err error
	// mocks Overlay.Receive for middleware.Overlay.Receive(*nodeID, payload)
	firstNode := 0
	lastNode := m.size - 1
	m.ov[lastNode].On("Receive", expectID, expectedMessage, expectedPayload).Return(nil).Once().
		Run(func(args mockery.Arguments) {
			ch <- struct{}{}
		})

	// sends a direct message from first node to the last node
	err = m.mws[firstNode].SendDirect(msg, m.ids[lastNode].NodeID)
	require.NoError(m.Suite.T(), err)

	select {
	case <-ch:
	case <-time.After(3 * time.Second):
		assert.Fail(m.T(), "peer 1 failed to send a message to peer 2")
	}

	// evaluates the mock calls
	for i := 1; i < m.size; i++ {
		m.ov[i].AssertExpectations(m.T())
	}

}

// Ping sends count-many distinct messages concurrently from the first middleware of the test suit to the last one
// It evaluates the correctness of reception of the content of the messages, as well as the sender ID
func (m *MiddlewareTestSuite) MultiPing(count int) {
	receiveWG := sync.WaitGroup{}
	sendWG := sync.WaitGroup{}
	// extracts sender id based on the mock option
	// mocks Overlay.Receive for  middleware.Overlay.Receive(*nodeID, payload)
	firstNode := 0
	lastNode := m.size - 1
	for i := 0; i < count; i++ {
		receiveWG.Add(1)
		sendWG.Add(1)
		msg, expectedMsg, expectedPayload := messageutils.CreateMessage(m.T(),
			m.ids[firstNode].NodeID,
			m.ids[lastNode].NodeID,
			testChannel,
			fmt.Sprintf("hello from: %d", i))

		m.ov[lastNode].On("Receive", m.ids[firstNode].NodeID, expectedMsg, expectedPayload).Return(nil).Once().
			Run(func(args mockery.Arguments) {
				payload := args.Get(2).(*libp2pmessage.TestMessage)
				require.Equal(m.T(), expectedPayload.(*libp2pmessage.TestMessage), payload)
				receiveWG.Done()
			})
		go func() {
			// sends a direct message from first node to the last node
			err := m.mws[firstNode].SendDirect(msg, m.ids[lastNode].NodeID)
			require.NoError(m.Suite.T(), err)

			sendWG.Done()
		}()
	}

	unittest.RequireReturnsBefore(m.T(), sendWG.Wait, 1*time.Second, "could not send unicasts on time")
	unittest.RequireReturnsBefore(m.T(), receiveWG.Wait, 1*time.Second, "could not receive unicasts on time")

	// evaluates the mock calls
	for i := 1; i < m.size; i++ {
		m.ov[i].AssertExpectations(m.T())
	}
}

// TestEcho sends an echo message from first middleware to the last middleware
// the last middleware echos back the message. The test evaluates the correctness
// of the message reception as well as its content
func (m *MiddlewareTestSuite) TestEcho() {

	wg := sync.WaitGroup{}
	// extracts sender id based on the mock option
	var err error

	wg.Add(2)
	// mocks Overlay.Receive for middleware.Overlay.Receive(*nodeID, payload)
	first := 0
	last := m.size - 1
	firstNode := m.ids[first].NodeID
	lastNode := m.ids[last].NodeID

	sendMsg, expectedSendMsg, sendPayload := messageutils.CreateMessage(m.T(), firstNode, lastNode, testChannel, "TestEcho")
	expectedSendPayload := sendPayload.(*libp2pmessage.TestMessage)

	replyMsg, expectedReplyMsg, replyPayload := messageutils.CreateMessage(m.T(), lastNode, firstNode, testChannel, "TestEcho response")

	expectedReplyPayload := replyPayload.(*libp2pmessage.TestMessage)

	// last node
	m.ov[last].On("Receive", firstNode, expectedSendMsg, sendPayload).Return(nil).Once().
		Run(func(args mockery.Arguments) {
			wg.Done()
			// echos back the same message back to the sender
			err := m.mws[last].SendDirect(replyMsg, firstNode)
			assert.NoError(m.T(), err)

			payload := args.Get(2).(*libp2pmessage.TestMessage)
			require.Equal(m.T(), expectedSendPayload, payload)
		})

	// first node
	m.ov[first].On("Receive", lastNode, expectedReplyMsg, replyPayload).Return(nil).Once().
		Run(func(args mockery.Arguments) {
			wg.Done()
			payload := args.Get(2).(*libp2pmessage.TestMessage)
			require.Equal(m.T(), expectedReplyPayload, payload)
		})

	// sends a direct message from first node to the last node
	err = m.mws[first].SendDirect(sendMsg, m.ids[last].NodeID)
	require.NoError(m.Suite.T(), err)

	unittest.RequireReturnsBefore(m.T(), wg.Wait, 100*time.Second, "could not receive unicast on time")

	// evaluates the mock calls
	for i := 1; i < m.size; i++ {
		m.ov[i].AssertExpectations(m.T())
	}
}

// TestMaxMessageSize_SendDirect evaluates that invoking SendDirect method of the middleware on a message
// size beyond the permissible unicast message size returns an error.
func (m *MiddlewareTestSuite) TestMaxMessageSize_SendDirect() {
	first := 0
	last := m.size - 1
	firstNode := m.ids[first].NodeID
	lastNode := m.ids[last].NodeID

	// creates a network payload beyond the maximum message size
	// Note: networkPayloadFixture considers 1000 bytes as the overhead of the encoded message,
	// so the generated payload is 1000 bytes below the maximum unicast message size.
	// We hence add up 1000 bytes to the input of network payload fixture to make
	// sure that payload is beyond the permissible size.
	payload := testutils.NetworkPayloadFixture(m.T(), uint(middleware.DefaultMaxUnicastMsgSize)+1000)
	event := &libp2pmessage.TestMessage{
		Text: string(payload),
	}

	msg, _, _ := messageutils.CreateMessageWithPayload(m.T(), firstNode, lastNode, testChannel, event)

	// sends a direct message from first node to the last node
	err := m.mws[first].SendDirect(msg, lastNode)
	require.Error(m.Suite.T(), err)
}

// TestLargeMessageSize_SendDirect asserts that a ChunkDataResponse is treated as a large message and can be unicasted
// successfully even though it's size is greater than the default message size.
func (m *MiddlewareTestSuite) TestLargeMessageSize_SendDirect() {
	sourceIndex := 0
	targetIndex := m.size - 1
	sourceNode := m.ids[sourceIndex].NodeID
	targetNode := m.ids[targetIndex].NodeID
	targetMW := m.mws[targetIndex]

	// subscribe to channels.ProvideChunks so that the message is not dropped
	require.NoError(m.T(), targetMW.Subscribe(channels.ProvideChunks))

	// creates a network payload with a size greater than the default max size using a known large message type
	targetSize := uint64(middleware.DefaultMaxUnicastMsgSize) + 1000
	event := unittest.ChunkDataResponseMsgFixture(unittest.IdentifierFixture(), unittest.WithApproximateSize(targetSize))

	msg, expectedMsg, _ := messageutils.CreateMessageWithPayload(m.T(), sourceNode, targetNode, channels.ProvideChunks, event)

	// expect one message to be received by the target
	ch := make(chan struct{})
	m.ov[targetIndex].On("Receive", sourceNode, expectedMsg, event).Return(nil).Once().
		Run(func(args mockery.Arguments) {
			close(ch)
		})

	// sends a direct message from source node to the target node
	err := m.mws[sourceIndex].SendDirect(msg, targetNode)
	// SendDirect should not error since this is a known large message
	require.NoError(m.Suite.T(), err)

	// check message reception on target
	unittest.RequireCloseBefore(m.T(), ch, 60*time.Second, "source node failed to send large message to target")

	m.ov[targetIndex].AssertExpectations(m.T())
}

// TestMessageFieldsOverriden_SendDirect asserts that OriginID, EventID and Type fields are
// overridden in the message received over unicast
func (m *MiddlewareTestSuite) TestMessageFieldsOverriden_SendDirect() {
	first := 0
	last := m.size - 1
	firstNode := m.ids[first].NodeID
	lastNode := m.ids[last].NodeID

	msg, expected, event := messageutils.CreateMessage(m.T(), firstNode, lastNode, testChannel, "test message")

	fakeID := unittest.IdentifierFixture()
	msg.OriginID = fakeID[:]
	msg.EventID = fakeID[:]
	msg.Type = "messages.ChunkDataResponse"

	// should receive the expected message, not msg
	ch := make(chan struct{})
	m.ov[last].On("Receive", firstNode, expected, event).Return(nil).Once().
		Run(func(args mockery.Arguments) {
			close(ch)
		})

	// sends a direct message from first node to the last node with the modified fields
	err := m.mws[first].SendDirect(msg, lastNode)
	assert.NoError(m.Suite.T(), err)

	// check message reception on target
	unittest.RequireCloseBefore(m.T(), ch, 60*time.Second, "source node failed to send overridden message to target")

	m.ov[last].AssertExpectations(m.T())
}

// TestMaxMessageSize_Publish evaluates that invoking Publish method of the middleware on a message
// size beyond the permissible publish message size returns an error.
func (m *MiddlewareTestSuite) TestMaxMessageSize_Publish() {
	first := 0
	last := m.size - 1
	firstNode := m.ids[first].NodeID
	lastNode := m.ids[last].NodeID

	msg, _, _ := messageutils.CreateMessage(m.T(), firstNode, lastNode, testChannel, "")

	// adds another node as the target id to imitate publishing
	msg.TargetIDs = append(msg.TargetIDs, lastNode[:])

	// creates a network payload beyond the maximum message size
	// Note: networkPayloadFixture considers 1000 bytes as the overhead of the encoded message,
	// so the generated payload is 1000 bytes below the maximum publish message size.
	// We hence add up 1000 bytes to the input of network payload fixture to make
	// sure that payload is beyond the permissible size.
	payload := testutils.NetworkPayloadFixture(m.T(), uint(p2pnode.DefaultMaxPubSubMsgSize)+1000)
	event := &libp2pmessage.TestMessage{
		Text: string(payload),
	}

	codec := unittest.NetworkCodec()
	encodedEvent, err := codec.Encode(event)
	require.NoError(m.T(), err)

	msg.Payload = encodedEvent

	// sends a direct message from first node to the last node
	err = m.mws[first].Publish(msg, testChannel)
	require.Error(m.Suite.T(), err)
}

// TestMessageFieldsOverriden_Publish asserts that OriginID, EventID and Type fields are
// overridden in the message received over pubsub
func (m *MiddlewareTestSuite) TestMessageFieldsOverriden_Publish() {
	first := 0
	last := m.size - 1
	firstNode := m.ids[first].NodeID
	lastNode := m.ids[last].NodeID

	msg, expected, event := messageutils.CreateMessage(m.T(), firstNode, lastNode, testChannel, "test message")

	// adds another node as the target id to imitate publishing
	expected.TargetIDs = append(expected.TargetIDs, lastNode[:])
	msg.TargetIDs = expected.TargetIDs

	fakeID := unittest.IdentifierFixture()
	msg.OriginID = fakeID[:]
	msg.EventID = fakeID[:]
	msg.Type = "messages.ChunkDataResponse"

	// should receive the expected message, not msg
	ch := make(chan struct{})
	m.ov[last].On("Receive", firstNode, expected, event).Return(nil).Once().
		Run(func(args mockery.Arguments) {
			close(ch)
		})

	// set up waiting for m.size pubsub tags indicating a mesh has formed
	for i := 0; i < m.size; i++ {
		select {
		case <-m.obs:
		case <-time.After(2 * time.Second):
			assert.FailNow(m.T(), "could not receive pubsub tag indicating mesh formed")
		}
	}

	// sends a direct message from first node to the last node with the modified fields
	err := m.mws[first].Publish(msg, testChannel)
	assert.NoError(m.Suite.T(), err)

	// check message reception on target
	unittest.RequireCloseBefore(m.T(), ch, 2*time.Second, "source node failed to send overridden message to target")

	m.ov[last].AssertExpectations(m.T())
}

// TestUnsubscribe tests that an engine can unsubscribe from a topic it was earlier subscribed to and stop receiving
// messages.
func (m *MiddlewareTestSuite) TestUnsubscribe() {
	first := 0
	last := m.size - 1
	firstNode := m.ids[first].NodeID
	lastNode := m.ids[last].NodeID

	// set up waiting for m.size pubsub tags indicating a mesh has formed
	for i := 0; i < m.size; i++ {
		select {
		case <-m.obs:
		case <-time.After(2 * time.Second):
			assert.FailNow(m.T(), "could not receive pubsub tag indicating mesh formed")
		}
	}

	msgRcvd := make(chan struct{}, 2)
	msgRcvdFun := func() {
		<-msgRcvd
	}
	message1, _, _ := messageutils.CreateMessage(m.T(), firstNode, lastNode, testChannel, "hello1")

	m.ov[last].On("Receive", firstNode, mockery.Anything, mockery.Anything).Return(nil).Run(func(_ mockery.Arguments) {
		msgRcvd <- struct{}{}
	})

	// first test that when both nodes are subscribed to the channel, the target node receives the message
	err := m.mws[first].Publish(message1, testChannel)
	assert.NoError(m.T(), err)

	unittest.RequireReturnsBefore(m.T(), msgRcvdFun, 2*time.Second, "message not received")

	// now unsubscribe the target node from the channel
	err = m.mws[last].Unsubscribe(testChannel)
	assert.NoError(m.T(), err)

	// create and send a new message on the channel from the origin node
	message2, _, _ := messageutils.CreateMessage(m.T(), firstNode, lastNode, testChannel, "hello2")

	err = m.mws[first].Publish(message2, testChannel)
	assert.NoError(m.T(), err)

	// assert that the new message is not received by the target node
	unittest.RequireNeverReturnBefore(m.T(), msgRcvdFun, 2*time.Second, "message received unexpectedly")
}
