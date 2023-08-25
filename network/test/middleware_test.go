package test

import (
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

	"github.com/onflow/flow-go/config"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	libp2pmessage "github.com/onflow/flow-go/model/libp2p/message"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module/id"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/observable"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/internal/p2pfixtures"
	"github.com/onflow/flow-go/network/internal/testutils"
	"github.com/onflow/flow-go/network/mocknetwork"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/middleware"
	"github.com/onflow/flow-go/network/p2p/p2pnode"
	p2ptest "github.com/onflow/flow-go/network/p2p/test"
	"github.com/onflow/flow-go/network/p2p/unicast/ratelimit"
	"github.com/onflow/flow-go/network/p2p/utils/ratelimiter"
	"github.com/onflow/flow-go/utils/unittest"
)

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

// TODO: eventually this should be renamed to NetworkTesTSuite and should be moved to the p2p.network package.
type MiddlewareTestSuite struct {
	suite.Suite
	sync.RWMutex
	size                       int // used to determine number of middlewares under test
	libP2PNodes                []p2p.LibP2PNode
	networks                   []network.Network
	mws                        []network.Middleware // used to keep track of middlewares under test
	obs                        chan string          // used to keep track of Protect events tagged by pubsub messages
	ids                        []*flow.Identity
	metrics                    *metrics.NoopCollector // no-op performance monitoring simulation
	logger                     zerolog.Logger
	providers                  []*unittest.UpdatableIDProvider
	sporkId                    flow.Identifier
	mwCancel                   context.CancelFunc
	mwCtx                      irrecoverable.SignalerContext
	slashingViolationsConsumer network.ViolationsConsumer
}

// TestMiddlewareTestSuit runs all the test methods in this test suit
func TestMiddlewareTestSuite(t *testing.T) {
	// should not run in parallel, some tests are stateful.
	suite.Run(t, new(MiddlewareTestSuite))
}

// SetupTest initiates the test setups prior to each test
func (m *MiddlewareTestSuite) SetupTest() {
	m.logger = unittest.Logger()

	m.size = 2 // operates on two middlewares
	m.metrics = metrics.NewNoopCollector()

	// create and start the middlewares and inject a connection observer
	peerChannel := make(chan string)
	ob := tagsObserver{
		tags: peerChannel,
		log:  m.logger,
	}

	m.slashingViolationsConsumer = mocknetwork.NewViolationsConsumer(m.T())
	m.sporkId = unittest.IdentifierFixture()

	libP2PNodes := make([]p2p.LibP2PNode, 0)
	identities := make(flow.IdentityList, 0)
	tagObservables := make([]observable.Observable, 0)
	idProvider := unittest.NewUpdatableIDProvider(flow.IdentityList{})
	defaultFlowConfig, err := config.DefaultConfig()
	require.NoError(m.T(), err)

	opts := []p2ptest.NodeFixtureParameterOption{p2ptest.WithUnicastHandlerFunc(nil)}

	for i := 0; i < m.size; i++ {
		connManager, err := testutils.NewTagWatchingConnManager(
			unittest.Logger(),
			metrics.NewNoopCollector(),
			&defaultFlowConfig.NetworkConfig.ConnectionManagerConfig)
		require.NoError(m.T(), err)

		opts = append(opts,
			p2ptest.WithConnectionManager(connManager),
			p2ptest.WithRole(flow.RoleExecution))
		node, nodeId := p2ptest.NodeFixture(m.T(),
			m.sporkId,
			m.T().Name(),
			idProvider,
			opts...)
		libP2PNodes = append(libP2PNodes, node)
		identities = append(identities, &nodeId)
		tagObservables = append(tagObservables, connManager)
	}
	idProvider.SetIdentities(identities)

	m.ids = identities
	m.libP2PNodes = libP2PNodes

	m.mws, _ = testutils.MiddlewareFixtures(
		m.T(),
		m.ids,
		m.libP2PNodes,
		testutils.MiddlewareConfigFixture(m.T(), m.sporkId),
		m.slashingViolationsConsumer)

	m.networks, m.providers = testutils.NetworksFixture(m.T(), m.sporkId, m.ids, m.mws)
	for _, observableConnMgr := range tagObservables {
		observableConnMgr.Subscribe(&ob)
	}
	m.obs = peerChannel

	require.Len(m.Suite.T(), tagObservables, m.size)
	require.Len(m.Suite.T(), m.ids, m.size)
	require.Len(m.Suite.T(), m.mws, m.size)

	ctx, cancel := context.WithCancel(context.Background())
	m.mwCancel = cancel

	m.mwCtx = irrecoverable.NewMockSignalerContext(m.T(), ctx)

	testutils.StartNodes(m.mwCtx, m.T(), m.libP2PNodes, 1*time.Second)

	for i, net := range m.networks {
		unittest.RequireComponentsReadyBefore(m.T(), 1*time.Second, libP2PNodes[i])
		net.Start(m.mwCtx)
		unittest.RequireComponentsReadyBefore(m.T(), 1*time.Second, net)
	}
}

func (m *MiddlewareTestSuite) TearDownTest() {
	m.mwCancel()

	testutils.StopComponents(m.T(), m.networks, 1*time.Second)
	testutils.StopComponents(m.T(), m.libP2PNodes, 1*time.Second)

	m.mws = nil
	m.libP2PNodes = nil
	m.ids = nil
	m.size = 0
}

// TestUpdateNodeAddresses tests that the UpdateNodeAddresses method correctly updates
// the addresses of the staked network participants.
func (m *MiddlewareTestSuite) TestUpdateNodeAddresses() {
	ctx, cancel := context.WithCancel(m.mwCtx)
	irrecoverableCtx := irrecoverable.NewMockSignalerContext(m.T(), ctx)

	// create a new staked identity
	ids, libP2PNodes := testutils.LibP2PNodeForMiddlewareFixture(m.T(), m.sporkId, 1)
	idProvider := id.NewFixedIdentityProvider(ids)
	mws, providers := testutils.MiddlewareFixtures(
		m.T(),
		ids,
		libP2PNodes,
		testutils.MiddlewareConfigFixture(m.T(), m.sporkId),
		m.slashingViolationsConsumer)
	networkCfg := testutils.NetworkConfigFixture(
		m.T(),
		*ids[0],
		idProvider,
		m.sporkId,
		mws[0])
	newNet, err := p2p.NewNetwork(networkCfg)
	require.NoError(m.T(), err)

	require.Len(m.T(), ids, 1)
	require.Len(m.T(), providers, 1)
	require.Len(m.T(), mws, 1)
	newId := ids[0]
	newMw := mws[0]

	overlay := m.createOverlay(providers[0])
	overlay.On("Receive", m.ids[0].NodeID, mockery.AnythingOfType("*message.Message")).Return(nil)
	newMw.SetOverlay(overlay)

	// start up nodes and peer managers
	testutils.StartNodes(irrecoverableCtx, m.T(), libP2PNodes, 1*time.Second)
	defer testutils.StopComponents(m.T(), libP2PNodes, 1*time.Second)

	newNet.Start(irrecoverableCtx)
	defer testutils.StopComponents(m.T(), []network.Middleware{newMw}, 1*time.Second)
	unittest.RequireComponentsReadyBefore(m.T(), 1*time.Second, newMw)

	idList := flow.IdentityList(append(m.ids, newId))

	// needed to enable ID translation
	m.providers[0].SetIdentities(idList)

	// unicast should fail to send because no address is known yet for the new identity
	con, err := m.networks[0].Register(channels.TestNetworkChannel, &mocknetwork.MessageProcessor{})
	err = con.Unicast(&libp2pmessage.TestMessage{
		Text: "TestUpdateNodeAddresses",
	}, newId.NodeID)
	require.True(m.T(), strings.Contains(err.Error(), swarm.ErrNoAddresses.Error()))

	// update the addresses
	m.mws[0].UpdateNodeAddresses()

	// now the message should send successfully
	err = con.Unicast(&libp2pmessage.TestMessage{
		Text: "TestUpdateNodeAddresses",
	}, newId.NodeID)
	require.NoError(m.T(), err)

	cancel()
	unittest.RequireComponentsReadyBefore(m.T(), 1*time.Second, newMw)
}

func (m *MiddlewareTestSuite) TestUnicastRateLimit_Messages() {
	// limiter limit will be set to 5 events/sec the 6th event per interval will be rate limited
	limit := rate.Limit(5)

	// burst per interval
	burst := 5

	for _, mw := range m.mws {
		require.NoError(m.T(), mw.Subscribe(channels.TestNetworkChannel))
	}

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
	ids, libP2PNodes := testutils.LibP2PNodeForMiddlewareFixture(m.T(),
		m.sporkId,
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
		testutils.MiddlewareConfigFixture(m.T(), m.sporkId),
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
	overlay.On("Receive", mockery.AnythingOfType("*message.IncomingMessageScope")).Return(nil).Run(func(args mockery.Arguments) {
		calls.Inc()
		if calls.Load() >= 5 {
			close(ch)
		}
	})
	newMw.SetOverlay(overlay)

	ctx, cancel := context.WithCancel(m.mwCtx)
	irrecoverableCtx := irrecoverable.NewMockSignalerContext(m.T(), ctx)

	testutils.StartNodes(irrecoverableCtx, m.T(), libP2PNodes, 1*time.Second)
	defer testutils.StopComponents(m.T(), libP2PNodes, 1*time.Second)

	newMw.Start(irrecoverableCtx)
	unittest.RequireComponentsReadyBefore(m.T(), 1*time.Second, newMw)
	require.NoError(m.T(), newMw.Subscribe(channels.TestNetworkChannel))

	// needed to enable ID translation
	m.providers[0].SetIdentities(idList)

	// update the addresses
	m.mws[0].UpdateNodeAddresses()

	// add our sender node as a direct peer to our receiving node, this allows us to ensure
	// that connections to peers that are rate limited are completely prune. IsConnected will
	// return true only if the node is a direct peer of the other, after rate limiting this direct
	// peer should be removed by the peer manager.
	p2ptest.LetNodesDiscoverEachOther(m.T(), ctx, []p2p.LibP2PNode{libP2PNodes[0], m.libP2PNodes[0]}, flow.IdentityList{ids[0], m.ids[0]})
	p2ptest.TryConnectionAndEnsureConnected(m.T(), ctx, []p2p.LibP2PNode{libP2PNodes[0], m.libP2PNodes[0]})

	con0, err := m.networks[0].Register(channels.TestNetworkChannel, &mocknetwork.MessageProcessor{})
	require.NoError(m.T(), err)

	// with the rate limit configured to 5 msg/sec we send 10 messages at once and expect the rate limiter
	// to be invoked at-least once. We send 10 messages due to the flakiness that is caused by async stream
	// handling of streams.
	for i := 0; i < 10; i++ {
		err = con0.Unicast(&libp2pmessage.TestMessage{
			Text: fmt.Sprintf("hello-%d", i),
		}, newId.NodeID)
		require.NoError(m.T(), err)
	}
	// wait for all rate limits before shutting down middleware
	unittest.RequireCloseBefore(m.T(), ch, 100*time.Millisecond, "could not stop rate limit test ch on time")

	// sleep for 1 seconds to allow connection pruner to prune connections
	time.Sleep(1 * time.Second)

	// ensure connection to rate limited peer is pruned
	p2ptest.EnsureNotConnectedBetweenGroups(m.T(), ctx, []p2p.LibP2PNode{libP2PNodes[0]}, []p2p.LibP2PNode{m.libP2PNodes[0]})
	p2pfixtures.EnsureNoStreamCreationBetweenGroups(m.T(), ctx, []p2p.LibP2PNode{libP2PNodes[0]}, []p2p.LibP2PNode{m.libP2PNodes[0]})

	// eventually the rate limited node should be able to reconnect and send messages
	require.Eventually(m.T(), func() bool {
		err = con0.Unicast(&libp2pmessage.TestMessage{
			Text: "hello",
		}, newId.NodeID)
		return err == nil
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
	ids, libP2PNodes := testutils.LibP2PNodeForMiddlewareFixture(m.T(),
		m.sporkId,
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
		testutils.MiddlewareConfigFixture(m.T(), m.sporkId),
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

	testutils.StartNodes(irrecoverableCtx, m.T(), libP2PNodes, 1*time.Second)
	defer testutils.StopComponents(m.T(), libP2PNodes, 1*time.Second)

	newMw.Start(irrecoverableCtx)
	unittest.RequireComponentsReadyBefore(m.T(), 1*time.Second, newMw)
	require.NoError(m.T(), newMw.Subscribe(channels.TestNetworkChannel))

	idList := flow.IdentityList(append(m.ids, newId))

	// needed to enable ID translation
	m.providers[0].SetIdentities(idList)

	// update the addresses
	m.mws[0].UpdateNodeAddresses()

	// add our sender node as a direct peer to our receiving node, this allows us to ensure
	// that connections to peers that are rate limited are completely prune. IsConnected will
	// return true only if the node is a direct peer of the other, after rate limiting this direct
	// peer should be removed by the peer manager.
	p2ptest.LetNodesDiscoverEachOther(m.T(), ctx, []p2p.LibP2PNode{libP2PNodes[0], m.libP2PNodes[0]}, flow.IdentityList{ids[0], m.ids[0]})

	// create message with about 400bytes (300 random bytes + 100bytes message info)
	b := make([]byte, 300)
	for i := range b {
		b[i] = byte('X')
	}
	// send 3 messages at once with a size of 400 bytes each. The third message will be rate limited
	// as it is more than our allowed bandwidth of 1000 bytes.
	con0, err := m.networks[0].Register(channels.TestNetworkChannel, &mocknetwork.MessageProcessor{})
	require.NoError(m.T(), err)
	for i := 0; i < 3; i++ {
		err = con0.Unicast(&libp2pmessage.TestMessage{
			Text: string(b),
		}, newId.NodeID)
		require.NoError(m.T(), err)
	}

	// wait for all rate limits before shutting down middleware
	unittest.RequireCloseBefore(m.T(), ch, 100*time.Millisecond, "could not stop on rate limit test ch on time")

	// sleep for 1 seconds to allow connection pruner to prune connections
	time.Sleep(1 * time.Second)

	// ensure connection to rate limited peer is pruned
	p2ptest.EnsureNotConnectedBetweenGroups(m.T(), ctx, []p2p.LibP2PNode{libP2PNodes[0]}, []p2p.LibP2PNode{m.libP2PNodes[0]})
	p2pfixtures.EnsureNoStreamCreationBetweenGroups(m.T(), ctx, []p2p.LibP2PNode{libP2PNodes[0]}, []p2p.LibP2PNode{m.libP2PNodes[0]})

	// eventually the rate limited node should be able to reconnect and send messages
	require.Eventually(m.T(), func() bool {
		err = con0.Unicast(&libp2pmessage.TestMessage{
			Text: "",
		}, newId.NodeID)
		return err == nil
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
	// so we always return a valid identity. We only care about the node role for the test TestMaxMessageSize_Unicast
	// where EN are the only node authorized to send chunk data response.
	identityOpts := unittest.WithRole(flow.RoleExecution)
	overlay.On("Identity", mockery.AnythingOfType("peer.ID")).Maybe().Return(unittest.IdentityFixture(identityOpts), true)
	return overlay
}

// TestPing sends a message from the first middleware of the test suit to the last one and checks that the
// last middleware receives the message and that the message is correctly decoded.
func (m *MiddlewareTestSuite) TestPing() {
	receiveWG := sync.WaitGroup{}
	receiveWG.Add(1)
	// extracts sender id based on the mock option
	var err error

	// mocks Overlay.Receive for middleware.Overlay.Receive(*nodeID, payload)
	senderNodeIndex := 0
	targetNodeIndex := m.size - 1

	expectedPayload := "TestPingContentReception"
	// mocks a target engine on the last node of the test suit that will receive the message on the test channel.
	targetEngine := &mocknetwork.MessageProcessor{} // target engine on the last node of the test suit that will receive the message
	_, err = m.networks[targetNodeIndex].Register(channels.TestNetworkChannel, targetEngine)
	require.NoError(m.T(), err)
	// target engine must receive the message once with the expected payload
	targetEngine.On("Process", mockery.Anything, mockery.Anything, mockery.Anything).
		Run(func(args mockery.Arguments) {
			receiveWG.Done()

			msgChannel, ok := args[0].(channels.Channel)
			require.True(m.T(), ok)
			require.Equal(m.T(), channels.TestNetworkChannel, msgChannel) // channel

			msgOriginID, ok := args[1].(flow.Identifier)
			require.True(m.T(), ok)
			require.Equal(m.T(), m.ids[senderNodeIndex].NodeID, msgOriginID) // sender id

			msgPayload, ok := args[2].(*libp2pmessage.TestMessage)
			require.Equal(m.T(), expectedPayload, msgPayload.Text) // payload
		}).Return(nil).Once()

	// sends a direct message from first node to the last node
	con0, err := m.networks[senderNodeIndex].Register(channels.TestNetworkChannel, &mocknetwork.MessageProcessor{})
	require.NoError(m.T(), err)
	err = con0.Unicast(&libp2pmessage.TestMessage{
		Text: expectedPayload,
	}, m.ids[targetNodeIndex].NodeID)
	require.NoError(m.Suite.T(), err)

	unittest.RequireReturnsBefore(m.T(), receiveWG.Wait, 1*time.Second, "did not receive message")
}

// TestMultiPing_TwoPings sends two messages concurrently from the first middleware of the test suit to the last one,
// and checks that the last middleware receives the messages and that the messages are correctly decoded.
func (m *MiddlewareTestSuite) TestMultiPing_TwoPings() {
	m.MultiPing(2)
}

// TestMultiPing_FourPings sends four messages concurrently from the first middleware of the test suit to the last one,
// and checks that the last middleware receives the messages and that the messages are correctly decoded.
func (m *MiddlewareTestSuite) TestMultiPing_FourPings() {
	m.MultiPing(4)
}

// TestMultiPing_EightPings sends eight messages concurrently from the first middleware of the test suit to the last one,
// and checks that the last middleware receives the messages and that the messages are correctly decoded.
func (m *MiddlewareTestSuite) TestMultiPing_EightPings() {
	m.MultiPing(8)
}

// MultiPing sends count-many distinct messages concurrently from the first middleware of the test suit to the last one.
// It evaluates the correctness of reception of the content of the messages. Each message must be received by the
// last middleware of the test suit exactly once.
func (m *MiddlewareTestSuite) MultiPing(count int) {
	receiveWG := sync.WaitGroup{}
	sendWG := sync.WaitGroup{}
	senderNodeIndex := 0
	targetNodeIndex := m.size - 1

	receivedPayloads := unittest.NewProtectedMap[string, struct{}]() // keep track of unique payloads received.

	// regex to extract the payload from the message
	regex := regexp.MustCompile(`^hello from: \d`)

	// creates a conduit on sender to send messages to the target on the test channel.
	con0, err := m.networks[senderNodeIndex].Register(channels.TestNetworkChannel, &mocknetwork.MessageProcessor{})
	require.NoError(m.T(), err)

	// mocks a target engine on the last node of the test suit that will receive the message on the test channel.
	targetEngine := &mocknetwork.MessageProcessor{}
	_, err = m.networks[targetNodeIndex].Register(channels.TestNetworkChannel, targetEngine)
	targetEngine.On("Process", mockery.Anything, mockery.Anything, mockery.Anything).
		Run(func(args mockery.Arguments) {
			receiveWG.Done()

			msgChannel, ok := args[0].(channels.Channel)
			require.True(m.T(), ok)
			require.Equal(m.T(), channels.TestNetworkChannel, msgChannel) // channel

			msgOriginID, ok := args[1].(flow.Identifier)
			require.True(m.T(), ok)
			require.Equal(m.T(), m.ids[senderNodeIndex].NodeID, msgOriginID) // sender id

			msgPayload, ok := args[2].(*libp2pmessage.TestMessage)
			require.True(m.T(), ok)
			// payload
			require.True(m.T(), regex.MatchString(msgPayload.Text))
			require.False(m.T(), receivedPayloads.Has(msgPayload.Text)) // payload must be unique
			receivedPayloads.Add(msgPayload.Text, struct{}{})
		}).Return(nil)

	for i := 0; i < count; i++ {
		receiveWG.Add(1)
		sendWG.Add(1)

		expectedPayloadText := fmt.Sprintf("hello from: %d", i)
		require.NoError(m.T(), err)
		err = con0.Unicast(&libp2pmessage.TestMessage{
			Text: expectedPayloadText,
		}, m.ids[targetNodeIndex].NodeID)
		require.NoError(m.Suite.T(), err)

		go func() {
			// sends a direct message from first node to the last node
			err := con0.Unicast(&libp2pmessage.TestMessage{
				Text: expectedPayloadText,
			}, m.ids[targetNodeIndex].NodeID)
			require.NoError(m.Suite.T(), err)

			sendWG.Done()
		}()
	}

	unittest.RequireReturnsBefore(m.T(), sendWG.Wait, 1*time.Second, "could not send unicasts on time")
	unittest.RequireReturnsBefore(m.T(), receiveWG.Wait, 1*time.Second, "could not receive unicasts on time")
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
	// mocks a target engine on the first and last nodes of the test suit that will receive the message on the test channel.
	targetEngine1 := &mocknetwork.MessageProcessor{}
	con1, err := m.networks[first].Register(channels.TestNetworkChannel, targetEngine1)
	require.NoError(m.T(), err)
	targetEngine2 := &mocknetwork.MessageProcessor{}
	con2, err := m.networks[last].Register(channels.TestNetworkChannel, targetEngine2)
	require.NoError(m.T(), err)

	// message sent from first node to the last node.
	expectedSendMsg := "TestEcho"
	// reply from last node to the first node.
	expectedReplyMsg := "TestEcho response"

	// mocks the target engine on the last node of the test suit that will receive the message on the test channel, and
	// echos back the reply message to the sender.
	targetEngine2.On("Process", mockery.Anything, mockery.Anything, mockery.Anything).
		Run(func(args mockery.Arguments) {
			wg.Done()

			msgChannel, ok := args[0].(channels.Channel)
			require.True(m.T(), ok)
			require.Equal(m.T(), channels.TestNetworkChannel, msgChannel) // channel

			msgOriginID, ok := args[1].(flow.Identifier)
			require.True(m.T(), ok)
			require.Equal(m.T(), m.ids[first].NodeID, msgOriginID) // sender id

			msgPayload, ok := args[2].(*libp2pmessage.TestMessage)
			require.Equal(m.T(), expectedSendMsg, msgPayload.Text) // payload

			// echos back the same message back to the sender
			require.NoError(m.T(), con2.Unicast(&libp2pmessage.TestMessage{
				Text: expectedReplyMsg,
			}, m.ids[first].NodeID))
		}).Return(nil).Once()

	// mocks the target engine on the last node of the test suit that will receive the message on the test channel, and
	// echos back the reply message to the sender.
	targetEngine1.On("Process", mockery.Anything, mockery.Anything, mockery.Anything).
		Run(func(args mockery.Arguments) {
			wg.Done()

			msgChannel, ok := args[0].(channels.Channel)
			require.True(m.T(), ok)
			require.Equal(m.T(), channels.TestNetworkChannel, msgChannel) // channel

			msgOriginID, ok := args[1].(flow.Identifier)
			require.True(m.T(), ok)
			require.Equal(m.T(), m.ids[last].NodeID, msgOriginID) // sender id

			msgPayload, ok := args[2].(*libp2pmessage.TestMessage)
			require.Equal(m.T(), expectedReplyMsg, msgPayload.Text) // payload
		}).Return(nil)

	// sends a direct message from first node to the last node
	require.NoError(m.T(), con1.Unicast(&libp2pmessage.TestMessage{
		Text: expectedSendMsg,
	}, m.ids[last].NodeID))

	unittest.RequireReturnsBefore(m.T(), wg.Wait, 5*time.Second, "could not receive unicast on time")
}

// TestMaxMessageSize_Unicast evaluates that invoking Unicast method of the middleware on a message
// size beyond the permissible unicast message size returns an error.
func (m *MiddlewareTestSuite) TestMaxMessageSize_Unicast() {
	first := 0
	last := m.size - 1

	// creates a network payload beyond the maximum message size
	// Note: networkPayloadFixture considers 1000 bytes as the overhead of the encoded message,
	// so the generated payload is 1000 bytes below the maximum unicast message size.
	// We hence add up 1000 bytes to the input of network payload fixture to make
	// sure that payload is beyond the permissible size.
	payload := testutils.NetworkPayloadFixture(m.T(), uint(middleware.DefaultMaxUnicastMsgSize)+1000)
	event := &libp2pmessage.TestMessage{
		Text: string(payload),
	}

	// sends a direct message from first node to the last node
	con0, err := m.networks[first].Register(channels.TestNetworkChannel, &mocknetwork.MessageProcessor{})
	require.NoError(m.T(), err)
	require.Error(m.T(), con0.Unicast(event, m.ids[last].NodeID))
}

// TestLargeMessageSize_SendDirect asserts that a ChunkDataResponse is treated as a large message and can be unicasted
// successfully even though it's size is greater than the default message size.
func (m *MiddlewareTestSuite) TestLargeMessageSize_SendDirect() {
	sourceIndex := 0
	targetIndex := m.size - 1
	targetId := m.ids[targetIndex].NodeID

	// creates a network payload with a size greater than the default max size using a known large message type
	targetSize := uint64(middleware.DefaultMaxUnicastMsgSize) + 1000
	event := unittest.ChunkDataResponseMsgFixture(unittest.IdentifierFixture(), unittest.WithApproximateSize(targetSize))

	// expect one message to be received by the target
	ch := make(chan struct{})
	// mocks a target engine on the last node of the test suit that will receive the message on the test channel.
	targetEngine := &mocknetwork.MessageProcessor{}
	_, err := m.networks[targetIndex].Register(channels.ProvideChunks, targetEngine)
	require.NoError(m.T(), err)
	targetEngine.On("Process", mockery.Anything, mockery.Anything, mockery.Anything).
		Run(func(args mockery.Arguments) {
			defer close(ch)

			msgChannel, ok := args[0].(channels.Channel)
			require.True(m.T(), ok)
			require.Equal(m.T(), channels.ProvideChunks, msgChannel) // channel

			msgOriginID, ok := args[1].(flow.Identifier)
			require.True(m.T(), ok)
			require.Equal(m.T(), m.ids[sourceIndex].NodeID, msgOriginID) // sender id

			msgPayload, ok := args[2].(*messages.ChunkDataResponse)
			require.True(m.T(), ok)
			require.Equal(m.T(), event, msgPayload) // payload
		}).Return(nil).Once()

	// sends a direct message from source node to the target node
	con0, err := m.networks[sourceIndex].Register(channels.ProvideChunks, &mocknetwork.MessageProcessor{})
	require.NoError(m.T(), err)
	require.NoError(m.T(), con0.Unicast(event, targetId))

	// check message reception on target
	unittest.RequireCloseBefore(m.T(), ch, 5*time.Second, "source node failed to send large message to target")
}

// TestMaxMessageSize_Publish evaluates that invoking Publish method of the middleware on a message
// size beyond the permissible publish message size returns an error.
func (m *MiddlewareTestSuite) TestMaxMessageSize_Publish() {
	firstIndex := 0
	lastIndex := m.size - 1
	lastNodeId := m.ids[lastIndex].NodeID

	// creates a network payload beyond the maximum message size
	// Note: networkPayloadFixture considers 1000 bytes as the overhead of the encoded message,
	// so the generated payload is 1000 bytes below the maximum publish message size.
	// We hence add up 1000 bytes to the input of network payload fixture to make
	// sure that payload is beyond the permissible size.
	payload := testutils.NetworkPayloadFixture(m.T(), uint(p2pnode.DefaultMaxPubSubMsgSize)+1000)
	event := &libp2pmessage.TestMessage{
		Text: string(payload),
	}
	con0, err := m.networks[firstIndex].Register(channels.TestNetworkChannel, &mocknetwork.MessageProcessor{})
	require.NoError(m.T(), err)

	err = con0.Publish(event, lastNodeId)
	require.Error(m.Suite.T(), err)
	require.ErrorContains(m.T(), err, "exceeds configured max message size")
}

// TestUnsubscribe tests that an engine can unsubscribe from a topic it was earlier subscribed to and stop receiving
// messages.
func (m *MiddlewareTestSuite) TestUnsubscribe() {
	senderIndex := 0
	targetIndex := m.size - 1
	targetId := m.ids[targetIndex].NodeID

	msgRcvd := make(chan struct{}, 2)
	msgRcvdFun := func() {
		<-msgRcvd
	}

	targetEngine := &mocknetwork.MessageProcessor{}
	con2, err := m.networks[targetIndex].Register(channels.TestNetworkChannel, targetEngine)
	targetEngine.On("Process", mockery.Anything, mockery.Anything, mockery.Anything).
		Run(func(args mockery.Arguments) {
			msgChannel, ok := args[0].(channels.Channel)
			require.True(m.T(), ok)
			require.Equal(m.T(), channels.TestNetworkChannel, msgChannel) // channel

			msgOriginID, ok := args[1].(flow.Identifier)
			require.True(m.T(), ok)
			require.Equal(m.T(), m.ids[senderIndex].NodeID, msgOriginID) // sender id

			msgPayload, ok := args[2].(*libp2pmessage.TestMessage)
			require.True(m.T(), ok)
			require.True(m.T(), msgPayload.Text == "hello1") // payload, we only expect hello 1 that was sent before unsubscribe.
			msgRcvd <- struct{}{}
		}).Return(nil)

	// first test that when both nodes are subscribed to the channel, the target node receives the message
	con1, err := m.networks[senderIndex].Register(channels.TestNetworkChannel, &mocknetwork.MessageProcessor{})
	require.NoError(m.T(), err)

	// set up waiting for m.size pubsub tags indicating a mesh has formed
	for i := 0; i < m.size; i++ {
		select {
		case <-m.obs:
		case <-time.After(2 * time.Second):
			assert.FailNow(m.T(), "could not receive pubsub tag indicating mesh formed")
		}
	}

	err = con1.Publish(&libp2pmessage.TestMessage{
		Text: string("hello1"),
	}, targetId)
	require.NoError(m.T(), err)

	unittest.RequireReturnsBefore(m.T(), msgRcvdFun, 3*time.Second, "message not received")

	// now unsubscribe the target node from the channel
	require.NoError(m.T(), con2.Close())

	// now publish a new message from the first node
	err = con1.Publish(&libp2pmessage.TestMessage{
		Text: string("hello2"),
	}, targetId)
	assert.NoError(m.T(), err)

	// assert that the new message is not received by the target node
	unittest.RequireNeverReturnBefore(m.T(), msgRcvdFun, 2*time.Second, "message received unexpectedly")
}
