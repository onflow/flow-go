package test

import (
	"bytes"
	"context"
	"fmt"
	"regexp"
	"strings"
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
	"github.com/onflow/flow-go/module/observable"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/internal/p2pfixtures"
	"github.com/onflow/flow-go/network/internal/testutils"
	"github.com/onflow/flow-go/network/message"
	"github.com/onflow/flow-go/network/mocknetwork"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/middleware"
	"github.com/onflow/flow-go/network/p2p/p2pnode"
	p2ptest "github.com/onflow/flow-go/network/p2p/test"
	"github.com/onflow/flow-go/network/p2p/unicast/ratelimit"
	"github.com/onflow/flow-go/network/p2p/utils/ratelimiter"
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
	providers []*unittest.UpdatableIDProvider

	mwCancel                   context.CancelFunc
	mwCtx                      irrecoverable.SignalerContext
	slashingViolationsConsumer network.ViolationsConsumer
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
	m.ids, m.nodes, obs = testutils.LibP2PNodeForMiddlewareFixture(m.T(), m.size)
	m.mws, m.providers = testutils.MiddlewareFixtures(m.T(), m.ids, m.nodes, testutils.MiddlewareConfigFixture(m.T()), m.slashingViolationsConsumer)
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
	ids, libP2PNodes, _ := testutils.LibP2PNodeForMiddlewareFixture(m.T(), 1)
	mws, providers := testutils.MiddlewareFixtures(m.T(), ids, libP2PNodes, testutils.MiddlewareConfigFixture(m.T()), m.slashingViolationsConsumer)
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

	outMsg, err := network.NewOutgoingScope(
		flow.IdentifierList{newId.NodeID},
		testChannel,
		&libp2pmessage.TestMessage{
			Text: "TestUpdateNodeAddresses",
		},
		unittest.NetworkCodec().Encode,
		message.ProtocolTypeUnicast)
	require.NoError(m.T(), err)
	// message should fail to send because no address is known yet
	// for the new identity
	err = m.mws[0].SendDirect(outMsg)
	require.True(m.T(), strings.Contains(err.Error(), swarm.ErrNoAddresses.Error()))

	// update the addresses
	m.mws[0].UpdateNodeAddresses()

	// now the message should send successfully
	err = m.mws[0].SendDirect(outMsg)
	require.NoError(m.T(), err)

	cancel()
	unittest.RequireComponentsReadyBefore(m.T(), 100*time.Millisecond, newMw)
}

func (m *MiddlewareTestSuite) TestUnicastRateLimit_Messages() {
	// limiter limit will be set to 5 events/sec the 6th event per interval will be rate limited
	limit := rate.Limit(5)

	// burst per interval
	burst := 5

	messageRateLimiter := ratelimiter.NewRateLimiter(limit, burst, 3*time.Second)

	// we only expect messages from the first middleware on the test suite
	expectedPID, err := unittest.PeerIDFromFlowID(m.ids[0])
	require.NoError(m.T(), err)

	// the onRateLimit call back we will use to keep track of how many times a rate limit happens.
	rateLimits := atomic.NewUint64(0)

	onRateLimit := func(peerID peer.ID, role, msgType, topic, reason string) {
		require.Equal(m.T(), reason, ratelimit.ReasonMessageCount.String())
		require.Equal(m.T(), expectedPID, peerID)
		// update hook calls
		rateLimits.Inc()
	}

	// setup rate limit distributor that will be used to track the number of rate limits via the onRateLimit callback.
	consumer := testutils.NewRateLimiterConsumer(onRateLimit)
	distributor := ratelimit.NewUnicastRateLimiterDistributor()
	distributor.AddConsumer(consumer)

	opts := []ratelimit.RateLimitersOption{ratelimit.WithMessageRateLimiter(messageRateLimiter), ratelimit.WithNotifier(distributor), ratelimit.WithDisabledRateLimiting(false)}
	rateLimiters := ratelimit.NewRateLimiters(opts...)

	idProvider := unittest.NewUpdatableIDProvider(m.ids)

	ids, libP2PNodes, _ := testutils.LibP2PNodeForMiddlewareFixture(m.T(),
		1,
		p2ptest.WithUnicastRateLimitDistributor(distributor),
		p2ptest.WithConnectionGater(p2ptest.NewConnectionGater(idProvider, func(pid peer.ID) error {
			if messageRateLimiter.IsRateLimited(pid) {
				return fmt.Errorf("rate-limited peer")
			}
			return nil
		})))
	idProvider.SetIdentities(append(m.ids, ids...))

	// create middleware
	mws, providers := testutils.MiddlewareFixtures(m.T(),
		ids,
		libP2PNodes,
		testutils.MiddlewareConfigFixture(m.T()),
		m.slashingViolationsConsumer,
		middleware.WithUnicastRateLimiters(rateLimiters),
		middleware.WithPeerManagerFilters([]p2p.PeerFilter{testutils.IsRateLimitedPeerFilter(messageRateLimiter)}))

	require.Len(m.T(), ids, 1)
	require.Len(m.T(), providers, 1)
	require.Len(m.T(), mws, 1)
	newId := ids[0]
	newMw := mws[0]
	idList := flow.IdentityList(append(m.ids, newId))

	providers[0].SetIdentities(idList)

	overlay := m.createOverlay(providers[0])

	calls := atomic.NewUint64(0)
	ch := make(chan struct{})
	overlay.On("Receive", mockery.AnythingOfType("*network.IncomingMessageScope")).Return(nil).Run(func(args mockery.Arguments) {
		calls.Inc()
		if calls.Load() >= 5 {
			close(ch)
		}
	})

	newMw.SetOverlay(overlay)

	ctx, cancel := context.WithCancel(m.mwCtx)
	irrecoverableCtx := irrecoverable.NewMockSignalerContext(m.T(), ctx)

	testutils.StartNodes(irrecoverableCtx, m.T(), libP2PNodes, 100*time.Millisecond)
	defer testutils.StopComponents(m.T(), libP2PNodes, 100*time.Millisecond)

	newMw.Start(irrecoverableCtx)
	unittest.RequireComponentsReadyBefore(m.T(), 100*time.Millisecond, newMw)

	require.NoError(m.T(), newMw.Subscribe(testChannel))

	// needed to enable ID translation
	m.providers[0].SetIdentities(idList)

	// update the addresses
	m.mws[0].UpdateNodeAddresses()

	// add our sender node as a direct peer to our receiving node, this allows us to ensure
	// that connections to peers that are rate limited are completely prune. IsConnected will
	// return true only if the node is a direct peer of the other, after rate limiting this direct
	// peer should be removed by the peer manager.
	p2ptest.LetNodesDiscoverEachOther(m.T(), ctx, []p2p.LibP2PNode{libP2PNodes[0], m.nodes[0]}, flow.IdentityList{ids[0], m.ids[0]})
	p2ptest.TryConnectionAndEnsureConnected(m.T(), ctx, []p2p.LibP2PNode{libP2PNodes[0], m.nodes[0]})

	// with the rate limit configured to 5 msg/sec we send 10 messages at once and expect the rate limiter
	// to be invoked at-least once. We send 10 messages due to the flakiness that is caused by async stream
	// handling of streams.
	for i := 0; i < 10; i++ {
		msg, err := network.NewOutgoingScope(
			flow.IdentifierList{newId.NodeID},
			testChannel,
			&libp2pmessage.TestMessage{
				Text: fmt.Sprintf("hello-%d", i),
			},
			unittest.NetworkCodec().Encode,
			message.ProtocolTypeUnicast)
		require.NoError(m.T(), err)
		err = m.mws[0].SendDirect(msg)
		require.NoError(m.T(), err)
	}
	// wait for all rate limits before shutting down middleware
	unittest.RequireCloseBefore(m.T(), ch, 100*time.Millisecond, "could not stop rate limit test ch on time")

	// sleep for 1 seconds to allow connection pruner to prune connections
	time.Sleep(1 * time.Second)

	// ensure connection to rate limited peer is pruned
	p2ptest.EnsureNotConnectedBetweenGroups(m.T(), ctx, []p2p.LibP2PNode{libP2PNodes[0]}, []p2p.LibP2PNode{m.nodes[0]})
	p2pfixtures.EnsureNoStreamCreationBetweenGroups(m.T(), ctx, []p2p.LibP2PNode{libP2PNodes[0]}, []p2p.LibP2PNode{m.nodes[0]})

	// eventually the rate limited node should be able to reconnect and send messages
	require.Eventually(m.T(), func() bool {
		msg, err := network.NewOutgoingScope(
			flow.IdentifierList{newId.NodeID},
			testChannel,
			&libp2pmessage.TestMessage{
				Text: "hello",
			},
			unittest.NetworkCodec().Encode,
			message.ProtocolTypeUnicast)
		require.NoError(m.T(), err)
		return m.mws[0].SendDirect(msg) == nil
	}, 3*time.Second, 100*time.Millisecond)

	// shutdown our middleware so that each message can be processed
	cancel()
	unittest.RequireCloseBefore(m.T(), libP2PNodes[0].Done(), 100*time.Millisecond, "could not stop libp2p node on time")
	unittest.RequireCloseBefore(m.T(), newMw.Done(), 100*time.Millisecond, "could not stop middleware on time")

	// expect our rate limited peer callback to be invoked once
	require.True(m.T(), rateLimits.Load() > 0)
}

func (m *MiddlewareTestSuite) TestUnicastRateLimit_Bandwidth() {
	//limiter limit will be set up to 1000 bytes/sec
	limit := rate.Limit(1000)

	//burst per interval
	burst := 1000

	// we only expect messages from the first middleware on the test suite
	expectedPID, err := unittest.PeerIDFromFlowID(m.ids[0])
	require.NoError(m.T(), err)

	// setup bandwidth rate limiter
	bandwidthRateLimiter := ratelimit.NewBandWidthRateLimiter(limit, burst, 4*time.Second)

	// the onRateLimit call back we will use to keep track of how many times a rate limit happens
	// after 5 rate limits we will close ch.
	ch := make(chan struct{})
	rateLimits := atomic.NewUint64(0)
	onRateLimit := func(peerID peer.ID, role, msgType, topic, reason string) {
		require.Equal(m.T(), reason, ratelimit.ReasonBandwidth.String())

		// we only expect messages from the first middleware on the test suite
		require.NoError(m.T(), err)
		require.Equal(m.T(), expectedPID, peerID)
		// update hook calls
		rateLimits.Inc()
		close(ch)
	}

	consumer := testutils.NewRateLimiterConsumer(onRateLimit)
	distributor := ratelimit.NewUnicastRateLimiterDistributor()
	distributor.AddConsumer(consumer)
	opts := []ratelimit.RateLimitersOption{ratelimit.WithBandwidthRateLimiter(bandwidthRateLimiter), ratelimit.WithNotifier(distributor), ratelimit.WithDisabledRateLimiting(false)}
	rateLimiters := ratelimit.NewRateLimiters(opts...)

	idProvider := unittest.NewUpdatableIDProvider(m.ids)
	// create a new staked identity
	ids, libP2PNodes, _ := testutils.LibP2PNodeForMiddlewareFixture(m.T(),
		1,
		p2ptest.WithUnicastRateLimitDistributor(distributor),
		p2ptest.WithConnectionGater(p2ptest.NewConnectionGater(idProvider, func(pid peer.ID) error {
			// create connection gater, connection gater will refuse connections from rate limited nodes
			if bandwidthRateLimiter.IsRateLimited(pid) {
				return fmt.Errorf("rate-limited peer")
			}

			return nil
		})))
	idProvider.SetIdentities(append(m.ids, ids...))

	// create middleware
	mws, providers := testutils.MiddlewareFixtures(m.T(),
		ids,
		libP2PNodes,
		testutils.MiddlewareConfigFixture(m.T()),
		m.slashingViolationsConsumer,
		middleware.WithUnicastRateLimiters(rateLimiters),
		middleware.WithPeerManagerFilters([]p2p.PeerFilter{testutils.IsRateLimitedPeerFilter(bandwidthRateLimiter)}))
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

	msg, err := network.NewOutgoingScope(
		flow.IdentifierList{newId.NodeID},
		testChannel,
		&libp2pmessage.TestMessage{
			Text: string(b),
		},
		unittest.NetworkCodec().Encode,
		message.ProtocolTypeUnicast)
	require.NoError(m.T(), err)

	// update the addresses
	m.mws[0].UpdateNodeAddresses()

	// add our sender node as a direct peer to our receiving node, this allows us to ensure
	// that connections to peers that are rate limited are completely prune. IsConnected will
	// return true only if the node is a direct peer of the other, after rate limiting this direct
	// peer should be removed by the peer manager.
	p2ptest.LetNodesDiscoverEachOther(m.T(), ctx, []p2p.LibP2PNode{libP2PNodes[0], m.nodes[0]}, flow.IdentityList{ids[0], m.ids[0]})

	// send 3 messages at once with a size of 400 bytes each. The third message will be rate limited
	// as it is more than our allowed bandwidth of 1000 bytes.
	for i := 0; i < 3; i++ {
		err = m.mws[0].SendDirect(msg)
		require.NoError(m.T(), err)
	}

	// wait for all rate limits before shutting down middleware
	unittest.RequireCloseBefore(m.T(), ch, 100*time.Millisecond, "could not stop on rate limit test ch on time")

	// sleep for 1 seconds to allow connection pruner to prune connections
	time.Sleep(1 * time.Second)

	// ensure connection to rate limited peer is pruned
	p2ptest.EnsureNotConnectedBetweenGroups(m.T(), ctx, []p2p.LibP2PNode{libP2PNodes[0]}, []p2p.LibP2PNode{m.nodes[0]})
	p2pfixtures.EnsureNoStreamCreationBetweenGroups(m.T(), ctx, []p2p.LibP2PNode{libP2PNodes[0]}, []p2p.LibP2PNode{m.nodes[0]})

	// eventually the rate limited node should be able to reconnect and send messages
	require.Eventually(m.T(), func() bool {
		msg, err = network.NewOutgoingScope(
			flow.IdentifierList{newId.NodeID},
			testChannel,
			&libp2pmessage.TestMessage{
				Text: "",
			},
			unittest.NetworkCodec().Encode,
			message.ProtocolTypeUnicast)
		require.NoError(m.T(), err)
		return m.mws[0].SendDirect(msg) == nil
	}, 3*time.Second, 100*time.Millisecond)

	// shutdown our middleware so that each message can be processed
	cancel()
	unittest.RequireComponentsDoneBefore(m.T(), 100*time.Millisecond, newMw)

	// expect our rate limited peer callback to be invoked once
	require.Equal(m.T(), uint64(1), rateLimits.Load())
}

func (m *MiddlewareTestSuite) createOverlay(provider *unittest.UpdatableIDProvider) *mocknetwork.Overlay {
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

// TestPing sends a message from the first middleware of the test suit to the last one and checks that the
// last middleware receives the message and that the message is correctly decoded.
func (m *MiddlewareTestSuite) TestPing() {
	receiveWG := sync.WaitGroup{}
	receiveWG.Add(1)
	// extracts sender id based on the mock option
	var err error

	// mocks Overlay.Receive for middleware.Overlay.Receive(*nodeID, payload)
	firstNodeIndex := 0
	lastNodeIndex := m.size - 1

	expectedPayload := "TestPingContentReception"
	msg, err := network.NewOutgoingScope(
		flow.IdentifierList{m.ids[lastNodeIndex].NodeID},
		testChannel,
		&libp2pmessage.TestMessage{
			Text: expectedPayload,
		},
		unittest.NetworkCodec().Encode,
		message.ProtocolTypeUnicast)
	require.NoError(m.T(), err)

	m.ov[lastNodeIndex].On("Receive", mockery.Anything).Return(nil).Once().
		Run(func(args mockery.Arguments) {
			receiveWG.Done()

			msg, ok := args[0].(*network.IncomingMessageScope)
			require.True(m.T(), ok)

			require.Equal(m.T(), testChannel, msg.Channel())                                              // channel
			require.Equal(m.T(), m.ids[firstNodeIndex].NodeID, msg.OriginId())                            // sender id
			require.Equal(m.T(), m.ids[lastNodeIndex].NodeID, msg.TargetIDs()[0])                         // target id
			require.Equal(m.T(), message.ProtocolTypeUnicast, msg.Protocol())                             // protocol
			require.Equal(m.T(), expectedPayload, msg.DecodedPayload().(*libp2pmessage.TestMessage).Text) // payload
		})

	// sends a direct message from first node to the last node
	err = m.mws[firstNodeIndex].SendDirect(msg)
	require.NoError(m.Suite.T(), err)

	unittest.RequireReturnsBefore(m.T(), receiveWG.Wait, 1000*time.Millisecond, "did not receive message")

	// evaluates the mock calls
	for i := 1; i < m.size; i++ {
		m.ov[i].AssertExpectations(m.T())
	}

}

// MultiPing sends count-many distinct messages concurrently from the first middleware of the test suit to the last one.
// It evaluates the correctness of reception of the content of the messages. Each message must be received by the
// last middleware of the test suit exactly once.
func (m *MiddlewareTestSuite) MultiPing(count int) {
	receiveWG := sync.WaitGroup{}
	sendWG := sync.WaitGroup{}
	// extracts sender id based on the mock option
	// mocks Overlay.Receive for  middleware.Overlay.Receive(*nodeID, payload)
	firstNodeIndex := 0
	lastNodeIndex := m.size - 1

	receivedPayloads := unittest.NewProtectedMap[string, struct{}]() // keep track of unique payloads received.

	// regex to extract the payload from the message
	regex := regexp.MustCompile(`^hello from: \d`)

	for i := 0; i < count; i++ {
		receiveWG.Add(1)
		sendWG.Add(1)

		expectedPayloadText := fmt.Sprintf("hello from: %d", i)
		msg, err := network.NewOutgoingScope(
			flow.IdentifierList{m.ids[lastNodeIndex].NodeID},
			testChannel,
			&libp2pmessage.TestMessage{
				Text: expectedPayloadText,
			},
			unittest.NetworkCodec().Encode,
			message.ProtocolTypeUnicast)
		require.NoError(m.T(), err)

		m.ov[lastNodeIndex].On("Receive", mockery.Anything).Return(nil).Once().
			Run(func(args mockery.Arguments) {
				receiveWG.Done()

				msg, ok := args[0].(*network.IncomingMessageScope)
				require.True(m.T(), ok)

				require.Equal(m.T(), testChannel, msg.Channel())                      // channel
				require.Equal(m.T(), m.ids[firstNodeIndex].NodeID, msg.OriginId())    // sender id
				require.Equal(m.T(), m.ids[lastNodeIndex].NodeID, msg.TargetIDs()[0]) // target id
				require.Equal(m.T(), message.ProtocolTypeUnicast, msg.Protocol())     // protocol

				// payload
				decodedPayload := msg.DecodedPayload().(*libp2pmessage.TestMessage).Text
				require.True(m.T(), regex.MatchString(decodedPayload))
				require.False(m.T(), receivedPayloads.Has(decodedPayload)) // payload must be unique
				receivedPayloads.Add(decodedPayload, struct{}{})
			})
		go func() {
			// sends a direct message from first node to the last node
			err := m.mws[firstNodeIndex].SendDirect(msg)
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

	// message sent from first node to the last node.
	expectedSendMsg := "TestEcho"
	sendMsg, err := network.NewOutgoingScope(
		flow.IdentifierList{lastNode},
		testChannel,
		&libp2pmessage.TestMessage{
			Text: expectedSendMsg,
		},
		unittest.NetworkCodec().Encode,
		message.ProtocolTypeUnicast)
	require.NoError(m.T(), err)

	// reply from last node to the first node.
	expectedReplyMsg := "TestEcho response"
	replyMsg, err := network.NewOutgoingScope(
		flow.IdentifierList{firstNode},
		testChannel,
		&libp2pmessage.TestMessage{
			Text: expectedReplyMsg,
		},
		unittest.NetworkCodec().Encode,
		message.ProtocolTypeUnicast)
	require.NoError(m.T(), err)

	// last node
	m.ov[last].On("Receive", mockery.Anything).Return(nil).Once().
		Run(func(args mockery.Arguments) {
			wg.Done()

			// sanity checks the message content.
			msg, ok := args[0].(*network.IncomingMessageScope)
			require.True(m.T(), ok)

			require.Equal(m.T(), testChannel, msg.Channel())                                              // channel
			require.Equal(m.T(), m.ids[first].NodeID, msg.OriginId())                                     // sender id
			require.Equal(m.T(), lastNode, msg.TargetIDs()[0])                                            // target id
			require.Equal(m.T(), message.ProtocolTypeUnicast, msg.Protocol())                             // protocol
			require.Equal(m.T(), expectedSendMsg, msg.DecodedPayload().(*libp2pmessage.TestMessage).Text) // payload
			// event id
			eventId, err := network.EventId(msg.Channel(), msg.Proto().Payload)
			require.NoError(m.T(), err)
			require.True(m.T(), bytes.Equal(eventId, msg.EventID()))

			// echos back the same message back to the sender
			err = m.mws[last].SendDirect(replyMsg)
			assert.NoError(m.T(), err)
		})

	// first node
	m.ov[first].On("Receive", mockery.Anything).Return(nil).Once().
		Run(func(args mockery.Arguments) {
			wg.Done()
			// sanity checks the message content.
			msg, ok := args[0].(*network.IncomingMessageScope)
			require.True(m.T(), ok)

			require.Equal(m.T(), testChannel, msg.Channel())                                               // channel
			require.Equal(m.T(), m.ids[last].NodeID, msg.OriginId())                                       // sender id
			require.Equal(m.T(), firstNode, msg.TargetIDs()[0])                                            // target id
			require.Equal(m.T(), message.ProtocolTypeUnicast, msg.Protocol())                              // protocol
			require.Equal(m.T(), expectedReplyMsg, msg.DecodedPayload().(*libp2pmessage.TestMessage).Text) // payload
			// event id
			eventId, err := network.EventId(msg.Channel(), msg.Proto().Payload)
			require.NoError(m.T(), err)
			require.True(m.T(), bytes.Equal(eventId, msg.EventID()))
		})

	// sends a direct message from first node to the last node
	err = m.mws[first].SendDirect(sendMsg)
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

	msg, err := network.NewOutgoingScope(
		flow.IdentifierList{lastNode},
		testChannel,
		event,
		unittest.NetworkCodec().Encode,
		message.ProtocolTypeUnicast)
	require.NoError(m.T(), err)

	// sends a direct message from first node to the last node
	err = m.mws[first].SendDirect(msg)
	require.Error(m.Suite.T(), err)
}

// TestLargeMessageSize_SendDirect asserts that a ChunkDataResponse is treated as a large message and can be unicasted
// successfully even though it's size is greater than the default message size.
func (m *MiddlewareTestSuite) TestLargeMessageSize_SendDirect() {
	sourceIndex := 0
	targetIndex := m.size - 1
	targetNode := m.ids[targetIndex].NodeID
	targetMW := m.mws[targetIndex]

	// subscribe to channels.ProvideChunks so that the message is not dropped
	require.NoError(m.T(), targetMW.Subscribe(channels.ProvideChunks))

	// creates a network payload with a size greater than the default max size using a known large message type
	targetSize := uint64(middleware.DefaultMaxUnicastMsgSize) + 1000
	event := unittest.ChunkDataResponseMsgFixture(unittest.IdentifierFixture(), unittest.WithApproximateSize(targetSize))

	msg, err := network.NewOutgoingScope(
		flow.IdentifierList{targetNode},
		channels.ProvideChunks,
		event,
		unittest.NetworkCodec().Encode,
		message.ProtocolTypeUnicast)
	require.NoError(m.T(), err)

	// expect one message to be received by the target
	ch := make(chan struct{})
	m.ov[targetIndex].On("Receive", mockery.Anything).Return(nil).Once().
		Run(func(args mockery.Arguments) {
			msg, ok := args[0].(*network.IncomingMessageScope)
			require.True(m.T(), ok)

			require.Equal(m.T(), channels.ProvideChunks, msg.Channel())
			require.Equal(m.T(), m.ids[sourceIndex].NodeID, msg.OriginId())
			require.Equal(m.T(), targetNode, msg.TargetIDs()[0])
			require.Equal(m.T(), message.ProtocolTypeUnicast, msg.Protocol())

			eventId, err := network.EventId(msg.Channel(), msg.Proto().Payload)
			require.NoError(m.T(), err)
			require.True(m.T(), bytes.Equal(eventId, msg.EventID()))
			close(ch)
		})

	// sends a direct message from source node to the target node
	err = m.mws[sourceIndex].SendDirect(msg)
	// SendDirect should not error since this is a known large message
	require.NoError(m.Suite.T(), err)

	// check message reception on target
	unittest.RequireCloseBefore(m.T(), ch, 60*time.Second, "source node failed to send large message to target")

	m.ov[targetIndex].AssertExpectations(m.T())
}

// TestMaxMessageSize_Publish evaluates that invoking Publish method of the middleware on a message
// size beyond the permissible publish message size returns an error.
func (m *MiddlewareTestSuite) TestMaxMessageSize_Publish() {
	first := 0
	last := m.size - 1
	lastNode := m.ids[last].NodeID

	// creates a network payload beyond the maximum message size
	// Note: networkPayloadFixture considers 1000 bytes as the overhead of the encoded message,
	// so the generated payload is 1000 bytes below the maximum publish message size.
	// We hence add up 1000 bytes to the input of network payload fixture to make
	// sure that payload is beyond the permissible size.
	payload := testutils.NetworkPayloadFixture(m.T(), uint(p2pnode.DefaultMaxPubSubMsgSize)+1000)
	event := &libp2pmessage.TestMessage{
		Text: string(payload),
	}
	msg, err := network.NewOutgoingScope(
		flow.IdentifierList{lastNode},
		testChannel,
		event,
		unittest.NetworkCodec().Encode,
		message.ProtocolTypePubSub)
	require.NoError(m.T(), err)

	// sends a direct message from first node to the last node
	err = m.mws[first].Publish(msg)
	require.Error(m.Suite.T(), err)
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

	message1, err := network.NewOutgoingScope(
		flow.IdentifierList{lastNode},
		testChannel,
		&libp2pmessage.TestMessage{
			Text: string("hello1"),
		},
		unittest.NetworkCodec().Encode,
		message.ProtocolTypeUnicast)
	require.NoError(m.T(), err)

	m.ov[last].On("Receive", mockery.Anything).Return(nil).Run(func(args mockery.Arguments) {
		msg, ok := args[0].(*network.IncomingMessageScope)
		require.True(m.T(), ok)
		require.Equal(m.T(), firstNode, msg.OriginId())
		msgRcvd <- struct{}{}
	})

	// first test that when both nodes are subscribed to the channel, the target node receives the message
	err = m.mws[first].Publish(message1)
	assert.NoError(m.T(), err)

	unittest.RequireReturnsBefore(m.T(), msgRcvdFun, 2*time.Second, "message not received")

	// now unsubscribe the target node from the channel
	err = m.mws[last].Unsubscribe(testChannel)
	assert.NoError(m.T(), err)

	// create and send a new message on the channel from the origin node
	message2, err := network.NewOutgoingScope(
		flow.IdentifierList{lastNode},
		testChannel,
		&libp2pmessage.TestMessage{
			Text: string("hello2"),
		},
		unittest.NetworkCodec().Encode,
		message.ProtocolTypeUnicast)
	require.NoError(m.T(), err)

	err = m.mws[first].Publish(message2)
	assert.NoError(m.T(), err)

	// assert that the new message is not received by the target node
	unittest.RequireNeverReturnBefore(m.T(), msgRcvdFun, 2*time.Second, "message received unexpectedly")
}
