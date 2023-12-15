package cohort1

import (
	"context"
	"fmt"
	"math/rand"
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
	"github.com/onflow/flow-go/network/p2p/p2pnet"
	p2ptest "github.com/onflow/flow-go/network/p2p/test"
	"github.com/onflow/flow-go/network/p2p/unicast/ratelimit"
	"github.com/onflow/flow-go/network/p2p/utils/ratelimiter"
	"github.com/onflow/flow-go/utils/unittest"
)

// libp2p emits a call to `Protect` with a topic-specific tag upon establishing each peering connection in a GossipSub mesh, see:
// https://github.com/libp2p/go-libp2p-pubsub/blob/master/tag_tracer.go
// One way to make sure such a mesh has formed, asynchronously, in unit tests, is to wait for libp2p.GossipSub such calls,
// and that's what we do with tagsObserver.
// Usage:
// The tagsObserver struct observes the OnNext, OnError, and OnComplete events related to peer tags.
// A channel 'tags' is used to communicate these tags, and the observer is subscribed to the observable connection manager.
// Advantages:
// Using tag observables helps understand the connectivity between different peers,
// and can be valuable in testing scenarios where network connectivity is critical.
// Issues:
//   - Deviation from Production Code: This tag observation might be unique to the test environment,
//     and therefore not reflect the behavior of the production code.
//   - Mask Issues in the Production Environment: The observables are tied to testing and might
//     lead to behaviors or errors that are masked or not evident within the actual production environment.
//
// TODO: Evaluate the necessity of tag observables in this test. Consider addressing the deviation from
// production code and potential mask issues in the production environment. Evaluate the possibility
// of removing this part eventually.
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

// TODO: eventually this should be moved to the p2pnet package.
type NetworkTestSuite struct {
	suite.Suite
	sync.RWMutex
	size        int // used to determine number of networks under test
	libP2PNodes []p2p.LibP2PNode
	networks    []*p2pnet.Network
	obs         chan string // used to keep track of Protect events tagged by pubsub messages
	ids         []*flow.Identity
	metrics     *metrics.NoopCollector // no-op performance monitoring simulation
	logger      zerolog.Logger
	providers   []*unittest.UpdatableIDProvider
	sporkId     flow.Identifier
	mwCancel    context.CancelFunc
	mwCtx       irrecoverable.SignalerContext
}

// TestNetworkTestSuit runs all the test methods in this test suit
func TestNetworkTestSuite(t *testing.T) {
	// should not run in parallel, some tests are stateful.
	suite.Run(t, new(NetworkTestSuite))
}

// SetupTest initiates the test setups prior to each test
func (suite *NetworkTestSuite) SetupTest() {
	suite.logger = unittest.Logger()

	suite.size = 2 // operates on two networks
	suite.metrics = metrics.NewNoopCollector()

	// create and start the networks and inject a connection observer
	peerChannel := make(chan string)
	ob := tagsObserver{
		tags: peerChannel,
		log:  suite.logger,
	}

	suite.sporkId = unittest.IdentifierFixture()

	libP2PNodes := make([]p2p.LibP2PNode, 0)
	identities := make(flow.IdentityList, 0)
	tagObservables := make([]observable.Observable, 0)
	idProvider := unittest.NewUpdatableIDProvider(flow.IdentityList{})
	defaultFlowConfig, err := config.DefaultConfig()
	require.NoError(suite.T(), err)
	defaultFlowConfig.NetworkConfig.Unicast.UnicastManager.CreateStreamBackoffDelay = 1 * time.Millisecond

	opts := []p2ptest.NodeFixtureParameterOption{p2ptest.WithUnicastHandlerFunc(nil)}

	for i := 0; i < suite.size; i++ {
		connManager, err := testutils.NewTagWatchingConnManager(
			unittest.Logger(),
			metrics.NewNoopCollector(),
			&defaultFlowConfig.NetworkConfig.ConnectionManager)
		require.NoError(suite.T(), err)

		opts = append(opts,
			p2ptest.WithConnectionManager(connManager),
			p2ptest.WithRole(flow.RoleExecution),
			p2ptest.OverrideFlowConfig(defaultFlowConfig)) // to suppress exponential backoff
		node, nodeId := p2ptest.NodeFixture(suite.T(),
			suite.sporkId,
			suite.T().Name(),
			idProvider,
			opts...)
		libP2PNodes = append(libP2PNodes, node)
		identities = append(identities, &nodeId)
		tagObservables = append(tagObservables, connManager)
	}
	idProvider.SetIdentities(identities)

	suite.ids = identities
	suite.libP2PNodes = libP2PNodes

	suite.networks, suite.providers = testutils.NetworksFixture(suite.T(), suite.sporkId, suite.ids, suite.libP2PNodes)
	for _, observableConnMgr := range tagObservables {
		observableConnMgr.Subscribe(&ob)
	}
	suite.obs = peerChannel

	require.Len(suite.Suite.T(), tagObservables, suite.size)
	require.Len(suite.Suite.T(), suite.ids, suite.size)

	ctx, cancel := context.WithCancel(context.Background())
	suite.mwCancel = cancel

	suite.mwCtx = irrecoverable.NewMockSignalerContext(suite.T(), ctx)

	testutils.StartNodes(suite.mwCtx, suite.T(), suite.libP2PNodes)

	for i, net := range suite.networks {
		unittest.RequireComponentsReadyBefore(suite.T(), 1*time.Second, libP2PNodes[i])
		net.Start(suite.mwCtx)
		unittest.RequireComponentsReadyBefore(suite.T(), 1*time.Second, net)
	}
}

func (suite *NetworkTestSuite) TearDownTest() {
	suite.mwCancel()

	testutils.StopComponents(suite.T(), suite.networks, 1*time.Second)
	testutils.StopComponents(suite.T(), suite.libP2PNodes, 1*time.Second)
	suite.libP2PNodes = nil
	suite.ids = nil
	suite.size = 0
}

// TestUpdateNodeAddresses tests that the UpdateNodeAddresses method correctly updates
// the addresses of the staked network participants.
func (suite *NetworkTestSuite) TestUpdateNodeAddresses() {
	ctx, cancel := context.WithCancel(suite.mwCtx)
	irrecoverableCtx := irrecoverable.NewMockSignalerContext(suite.T(), ctx)

	// create a new staked identity
	ids, libP2PNodes := testutils.LibP2PNodeForNetworkFixture(suite.T(), suite.sporkId, 1)
	idProvider := unittest.NewUpdatableIDProvider(ids)
	networkCfg := testutils.NetworkConfigFixture(
		suite.T(),
		*ids[0],
		idProvider,
		suite.sporkId,
		libP2PNodes[0])
	newNet, err := p2pnet.NewNetwork(networkCfg)
	require.NoError(suite.T(), err)
	require.Len(suite.T(), ids, 1)
	newId := ids[0]

	// start up nodes and peer managers
	testutils.StartNodes(irrecoverableCtx, suite.T(), libP2PNodes)
	defer testutils.StopComponents(suite.T(), libP2PNodes, 1*time.Second)

	newNet.Start(irrecoverableCtx)
	defer testutils.StopComponents(suite.T(), []network.EngineRegistry{newNet}, 1*time.Second)
	unittest.RequireComponentsReadyBefore(suite.T(), 1*time.Second, newNet)

	idList := flow.IdentityList(append(suite.ids, newId))

	// needed to enable ID translation
	suite.providers[0].SetIdentities(idList)

	// unicast should fail to send because no address is known yet for the new identity
	con, err := suite.networks[0].Register(channels.TestNetworkChannel, &mocknetwork.MessageProcessor{})
	require.NoError(suite.T(), err)
	err = con.Unicast(&libp2pmessage.TestMessage{
		Text: "TestUpdateNodeAddresses",
	}, newId.NodeID)
	require.True(suite.T(), strings.Contains(err.Error(), swarm.ErrNoAddresses.Error()))

	// update the addresses
	suite.networks[0].UpdateNodeAddresses()

	// now the message should send successfully
	err = con.Unicast(&libp2pmessage.TestMessage{
		Text: "TestUpdateNodeAddresses",
	}, newId.NodeID)
	require.NoError(suite.T(), err)

	cancel()
	unittest.RequireComponentsReadyBefore(suite.T(), 1*time.Second, newNet)
}

func (suite *NetworkTestSuite) TestUnicastRateLimit_Messages() {
	unittest.SkipUnless(suite.T(), unittest.TEST_FLAKY, "flaky")
	// limiter limit will be set to 5 events/sec the 6th event per interval will be rate limited
	limit := rate.Limit(5)

	// burst per interval
	burst := 5

	for _, net := range suite.networks {
		require.NoError(suite.T(), net.Subscribe(channels.TestNetworkChannel))
	}

	messageRateLimiter := ratelimiter.NewRateLimiter(limit, burst, 3*time.Second)

	// we only expect messages from the first networks on the test suite
	expectedPID, err := unittest.PeerIDFromFlowID(suite.ids[0])
	require.NoError(suite.T(), err)

	// the onRateLimit call back we will use to keep track of how many times a rate limit happens.
	rateLimits := atomic.NewUint64(0)

	onRateLimit := func(peerID peer.ID, role, msgType, topic, reason string) {
		require.Equal(suite.T(), reason, ratelimit.ReasonMessageCount.String())
		require.Equal(suite.T(), expectedPID, peerID)
		// update hook calls
		rateLimits.Inc()
	}

	// setup rate limit distributor that will be used to track the number of rate limits via the onRateLimit callback.
	consumer := testutils.NewRateLimiterConsumer(onRateLimit)
	distributor := ratelimit.NewUnicastRateLimiterDistributor()
	distributor.AddConsumer(consumer)

	opts := []ratelimit.RateLimitersOption{ratelimit.WithMessageRateLimiter(messageRateLimiter), ratelimit.WithNotifier(distributor), ratelimit.WithDisabledRateLimiting(false)}
	rateLimiters := ratelimit.NewRateLimiters(opts...)

	defaultFlowConfig, err := config.DefaultConfig()
	require.NoError(suite.T(), err)
	defaultFlowConfig.NetworkConfig.Unicast.UnicastManager.CreateStreamBackoffDelay = 1 * time.Millisecond

	idProvider := unittest.NewUpdatableIDProvider(suite.ids)
	ids, libP2PNodes := testutils.LibP2PNodeForNetworkFixture(suite.T(),
		suite.sporkId,
		1,
		p2ptest.WithUnicastRateLimitDistributor(distributor),
		p2ptest.OverrideFlowConfig(defaultFlowConfig), // to suppress exponential backoff
		p2ptest.WithConnectionGater(p2ptest.NewConnectionGater(idProvider, func(pid peer.ID) error {
			if messageRateLimiter.IsRateLimited(pid) {
				return fmt.Errorf("rate-limited peer")
			}
			return nil
		})))
	idProvider.SetIdentities(append(suite.ids, ids...))

	netCfg := testutils.NetworkConfigFixture(
		suite.T(),
		*ids[0],
		idProvider,
		suite.sporkId,
		libP2PNodes[0])
	newNet, err := p2pnet.NewNetwork(
		netCfg,
		p2pnet.WithUnicastRateLimiters(rateLimiters),
		p2pnet.WithPeerManagerFilters(testutils.IsRateLimitedPeerFilter(messageRateLimiter)))
	require.NoError(suite.T(), err)

	require.Len(suite.T(), ids, 1)
	newId := ids[0]
	idList := flow.IdentityList(append(suite.ids, newId))

	suite.providers[0].SetIdentities(idList)

	ctx, cancel := context.WithCancel(suite.mwCtx)
	irrecoverableCtx := irrecoverable.NewMockSignalerContext(suite.T(), ctx)
	testutils.StartNodes(irrecoverableCtx, suite.T(), libP2PNodes)
	defer testutils.StopComponents(suite.T(), libP2PNodes, 1*time.Second)
	testutils.StartNetworks(irrecoverableCtx, suite.T(), []network.EngineRegistry{newNet})

	calls := atomic.NewUint64(0)
	ch := make(chan struct{})
	// registers an engine on the new network
	newEngine := &mocknetwork.MessageProcessor{}
	_, err = newNet.Register(channels.TestNetworkChannel, newEngine)
	require.NoError(suite.T(), err)
	newEngine.On("Process", channels.TestNetworkChannel, suite.ids[0].NodeID, mockery.Anything).Run(func(args mockery.Arguments) {
		calls.Inc()
		if calls.Load() >= 5 {
			close(ch)
		}
	}).Return(nil)

	// needed to enable ID translation
	suite.providers[0].SetIdentities(idList)

	// update the addresses
	suite.networks[0].UpdateNodeAddresses()

	// add our sender node as a direct peer to our receiving node, this allows us to ensure
	// that connections to peers that are rate limited are completely prune. IsConnected will
	// return true only if the node is a direct peer of the other, after rate limiting this direct
	// peer should be removed by the peer manager.
	p2ptest.LetNodesDiscoverEachOther(suite.T(), ctx, []p2p.LibP2PNode{libP2PNodes[0], suite.libP2PNodes[0]}, flow.IdentityList{ids[0], suite.ids[0]})
	p2ptest.TryConnectionAndEnsureConnected(suite.T(), ctx, []p2p.LibP2PNode{libP2PNodes[0], suite.libP2PNodes[0]})

	con0, err := suite.networks[0].Register(channels.TestNetworkChannel, &mocknetwork.MessageProcessor{})
	require.NoError(suite.T(), err)

	// with the rate limit configured to 5 msg/sec we send 10 messages at once and expect the rate limiter
	// to be invoked at-least once. We send 10 messages due to the flakiness that is caused by async stream
	// handling of streams.
	for i := 0; i < 10; i++ {
		err = con0.Unicast(&libp2pmessage.TestMessage{
			Text: fmt.Sprintf("hello-%d", i),
		}, newId.NodeID)
		require.NoError(suite.T(), err)
	}
	// wait for all rate limits before shutting down network
	unittest.RequireCloseBefore(suite.T(), ch, 100*time.Millisecond, "could not stop rate limit test ch on time")

	// sleep for 1 seconds to allow connection pruner to prune connections
	time.Sleep(2 * time.Second)

	// ensure connection to rate limited peer is pruned
	p2ptest.EnsureNotConnectedBetweenGroups(suite.T(), ctx, []p2p.LibP2PNode{libP2PNodes[0]}, []p2p.LibP2PNode{suite.libP2PNodes[0]})
	p2pfixtures.EnsureNoStreamCreationBetweenGroups(suite.T(), ctx, []p2p.LibP2PNode{libP2PNodes[0]}, []p2p.LibP2PNode{suite.libP2PNodes[0]})

	// eventually the rate limited node should be able to reconnect and send messages
	require.Eventually(suite.T(), func() bool {
		err = con0.Unicast(&libp2pmessage.TestMessage{
			Text: "hello",
		}, newId.NodeID)
		return err == nil
	}, 3*time.Second, 100*time.Millisecond)

	// shutdown our network so that each message can be processed
	cancel()
	unittest.RequireCloseBefore(suite.T(), libP2PNodes[0].Done(), 100*time.Millisecond, "could not stop libp2p node on time")
	unittest.RequireCloseBefore(suite.T(), newNet.Done(), 100*time.Millisecond, "could not stop network on time")

	// expect our rate limited peer callback to be invoked once
	require.True(suite.T(), rateLimits.Load() > 0)
}

func (suite *NetworkTestSuite) TestUnicastRateLimit_Bandwidth() {
	// limiter limit will be set up to 1000 bytes/sec
	limit := rate.Limit(1000)

	// burst per interval
	burst := 1000

	// we only expect messages from the first network on the test suite
	expectedPID, err := unittest.PeerIDFromFlowID(suite.ids[0])
	require.NoError(suite.T(), err)

	// setup bandwidth rate limiter
	bandwidthRateLimiter := ratelimit.NewBandWidthRateLimiter(limit, burst, 4*time.Second)

	// the onRateLimit call back we will use to keep track of how many times a rate limit happens
	// after 5 rate limits we will close ch.
	ch := make(chan struct{})
	rateLimits := atomic.NewUint64(0)
	onRateLimit := func(peerID peer.ID, role, msgType, topic, reason string) {
		require.Equal(suite.T(), reason, ratelimit.ReasonBandwidth.String())

		// we only expect messages from the first network on the test suite
		require.NoError(suite.T(), err)
		require.Equal(suite.T(), expectedPID, peerID)
		// update hook calls
		rateLimits.Inc()
		close(ch)
	}

	consumer := testutils.NewRateLimiterConsumer(onRateLimit)
	distributor := ratelimit.NewUnicastRateLimiterDistributor()
	distributor.AddConsumer(consumer)
	opts := []ratelimit.RateLimitersOption{ratelimit.WithBandwidthRateLimiter(bandwidthRateLimiter), ratelimit.WithNotifier(distributor), ratelimit.WithDisabledRateLimiting(false)}
	rateLimiters := ratelimit.NewRateLimiters(opts...)

	defaultFlowConfig, err := config.DefaultConfig()
	require.NoError(suite.T(), err)
	defaultFlowConfig.NetworkConfig.Unicast.UnicastManager.CreateStreamBackoffDelay = 1 * time.Millisecond

	idProvider := unittest.NewUpdatableIDProvider(suite.ids)
	// create a new staked identity
	ids, libP2PNodes := testutils.LibP2PNodeForNetworkFixture(suite.T(),
		suite.sporkId,
		1,
		p2ptest.WithUnicastRateLimitDistributor(distributor),
		p2ptest.OverrideFlowConfig(defaultFlowConfig), // to suppress exponential backoff
		p2ptest.WithConnectionGater(p2ptest.NewConnectionGater(idProvider, func(pid peer.ID) error {
			// create connection gater, connection gater will refuse connections from rate limited nodes
			if bandwidthRateLimiter.IsRateLimited(pid) {
				return fmt.Errorf("rate-limited peer")
			}

			return nil
		})))
	idProvider.SetIdentities(append(suite.ids, ids...))
	suite.providers[0].SetIdentities(append(suite.ids, ids...))

	netCfg := testutils.NetworkConfigFixture(
		suite.T(),
		*ids[0],
		idProvider,
		suite.sporkId,
		libP2PNodes[0])
	newNet, err := p2pnet.NewNetwork(
		netCfg,
		p2pnet.WithUnicastRateLimiters(rateLimiters),
		p2pnet.WithPeerManagerFilters(testutils.IsRateLimitedPeerFilter(bandwidthRateLimiter)))
	require.NoError(suite.T(), err)
	require.Len(suite.T(), ids, 1)
	newId := ids[0]

	ctx, cancel := context.WithCancel(suite.mwCtx)
	irrecoverableCtx := irrecoverable.NewMockSignalerContext(suite.T(), ctx)

	testutils.StartNodes(irrecoverableCtx, suite.T(), libP2PNodes)
	defer testutils.StopComponents(suite.T(), libP2PNodes, 1*time.Second)

	testutils.StartNetworks(irrecoverableCtx, suite.T(), []network.EngineRegistry{newNet})
	unittest.RequireComponentsReadyBefore(suite.T(), 1*time.Second, newNet)

	// registers an engine on the new network so that it can receive messages on the TestNetworkChannel
	newEngine := &mocknetwork.MessageProcessor{}
	_, err = newNet.Register(channels.TestNetworkChannel, newEngine)
	require.NoError(suite.T(), err)
	newEngine.On("Process", channels.TestNetworkChannel, suite.ids[0].NodeID, mockery.Anything).Return(nil)

	idList := flow.IdentityList(append(suite.ids, newId))

	// needed to enable ID translation
	suite.providers[0].SetIdentities(idList)

	// update the addresses
	suite.networks[0].UpdateNodeAddresses()

	// add our sender node as a direct peer to our receiving node, this allows us to ensure
	// that connections to peers that are rate limited are completely prune. IsConnected will
	// return true only if the node is a direct peer of the other, after rate limiting this direct
	// peer should be removed by the peer manager.
	p2ptest.LetNodesDiscoverEachOther(suite.T(), ctx, []p2p.LibP2PNode{libP2PNodes[0], suite.libP2PNodes[0]}, flow.IdentityList{ids[0], suite.ids[0]})

	// create message with about 400bytes (300 random bytes + 100bytes message info)
	b := make([]byte, 300)
	for i := range b {
		b[i] = byte('X')
	}
	// send 3 messages at once with a size of 400 bytes each. The third message will be rate limited
	// as it is more than our allowed bandwidth of 1000 bytes.
	con0, err := suite.networks[0].Register(channels.TestNetworkChannel, &mocknetwork.MessageProcessor{})
	require.NoError(suite.T(), err)
	for i := 0; i < 3; i++ {
		err = con0.Unicast(&libp2pmessage.TestMessage{
			Text: string(b),
		}, newId.NodeID)
		require.NoError(suite.T(), err)
	}

	// wait for all rate limits before shutting down network
	unittest.RequireCloseBefore(suite.T(), ch, 100*time.Millisecond, "could not stop on rate limit test ch on time")

	// sleep for 1 seconds to allow connection pruner to prune connections
	time.Sleep(1 * time.Second)

	// ensure connection to rate limited peer is pruned
	p2ptest.EnsureNotConnectedBetweenGroups(suite.T(), ctx, []p2p.LibP2PNode{libP2PNodes[0]}, []p2p.LibP2PNode{suite.libP2PNodes[0]})
	p2pfixtures.EnsureNoStreamCreationBetweenGroups(suite.T(), ctx, []p2p.LibP2PNode{libP2PNodes[0]}, []p2p.LibP2PNode{suite.libP2PNodes[0]})

	// eventually the rate limited node should be able to reconnect and send messages
	require.Eventually(suite.T(), func() bool {
		err = con0.Unicast(&libp2pmessage.TestMessage{
			Text: "",
		}, newId.NodeID)
		return err == nil
	}, 5*time.Second, 100*time.Millisecond)

	// shutdown our network so that each message can be processed
	cancel()
	unittest.RequireComponentsDoneBefore(suite.T(), 5*time.Second, newNet)

	// expect our rate limited peer callback to be invoked once
	require.Equal(suite.T(), uint64(1), rateLimits.Load())
}

func (suite *NetworkTestSuite) createOverlay(provider *unittest.UpdatableIDProvider) *mocknetwork.Overlay {
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

// TestPing sends a message from the first network of the test suit to the last one and checks that the
// last network receives the message and that the message is correctly decoded.
func (suite *NetworkTestSuite) TestPing() {
	receiveWG := sync.WaitGroup{}
	receiveWG.Add(1)
	// extracts sender id based on the mock option
	var err error

	senderNodeIndex := 0
	targetNodeIndex := suite.size - 1

	expectedPayload := "TestPingContentReception"
	// mocks a target engine on the last node of the test suit that will receive the message on the test channel.
	targetEngine := &mocknetwork.MessageProcessor{} // target engine on the last node of the test suit that will receive the message
	_, err = suite.networks[targetNodeIndex].Register(channels.TestNetworkChannel, targetEngine)
	require.NoError(suite.T(), err)
	// target engine must receive the message once with the expected payload
	targetEngine.On("Process", mockery.Anything, mockery.Anything, mockery.Anything).
		Run(func(args mockery.Arguments) {
			receiveWG.Done()

			msgChannel, ok := args[0].(channels.Channel)
			require.True(suite.T(), ok)
			require.Equal(suite.T(), channels.TestNetworkChannel, msgChannel) // channel

			msgOriginID, ok := args[1].(flow.Identifier)
			require.True(suite.T(), ok)
			require.Equal(suite.T(), suite.ids[senderNodeIndex].NodeID, msgOriginID) // sender id

			msgPayload, ok := args[2].(*libp2pmessage.TestMessage)
			require.True(suite.T(), ok)
			require.Equal(suite.T(), expectedPayload, msgPayload.Text) // payload
		}).Return(nil).Once()

	// sends a direct message from first node to the last node
	con0, err := suite.networks[senderNodeIndex].Register(channels.TestNetworkChannel, &mocknetwork.MessageProcessor{})
	require.NoError(suite.T(), err)
	err = con0.Unicast(&libp2pmessage.TestMessage{
		Text: expectedPayload,
	}, suite.ids[targetNodeIndex].NodeID)
	require.NoError(suite.Suite.T(), err)

	unittest.RequireReturnsBefore(suite.T(), receiveWG.Wait, 1*time.Second, "did not receive message")
}

// TestMultiPing_TwoPings sends two messages concurrently from the first network of the test suit to the last one,
// and checks that the last network receives the messages and that the messages are correctly decoded.
func (suite *NetworkTestSuite) TestMultiPing_TwoPings() {
	suite.MultiPing(2)
}

// TestMultiPing_FourPings sends four messages concurrently from the first network of the test suit to the last one,
// and checks that the last network receives the messages and that the messages are correctly decoded.
func (suite *NetworkTestSuite) TestMultiPing_FourPings() {
	suite.MultiPing(4)
}

// TestMultiPing_EightPings sends eight messages concurrently from the first network of the test suit to the last one,
// and checks that the last network receives the messages and that the messages are correctly decoded.
func (suite *NetworkTestSuite) TestMultiPing_EightPings() {
	suite.MultiPing(8)
}

// MultiPing sends count-many distinct messages concurrently from the first network of the test suit to the last one.
// It evaluates the correctness of reception of the content of the messages. Each message must be received by the
// last network of the test suit exactly once.
func (suite *NetworkTestSuite) MultiPing(count int) {
	receiveWG := sync.WaitGroup{}
	sendWG := sync.WaitGroup{}
	senderNodeIndex := 0
	targetNodeIndex := suite.size - 1

	receivedPayloads := unittest.NewProtectedMap[string, struct{}]() // keep track of unique payloads received.

	// regex to extract the payload from the message
	regex := regexp.MustCompile(`^hello from: \d`)

	// creates a conduit on sender to send messages to the target on the test channel.
	con0, err := suite.networks[senderNodeIndex].Register(channels.TestNetworkChannel, &mocknetwork.MessageProcessor{})
	require.NoError(suite.T(), err)

	// mocks a target engine on the last node of the test suit that will receive the message on the test channel.
	targetEngine := &mocknetwork.MessageProcessor{}
	_, err = suite.networks[targetNodeIndex].Register(channels.TestNetworkChannel, targetEngine)
	targetEngine.On("Process", mockery.Anything, mockery.Anything, mockery.Anything).
		Run(func(args mockery.Arguments) {
			receiveWG.Done()

			msgChannel, ok := args[0].(channels.Channel)
			require.True(suite.T(), ok)
			require.Equal(suite.T(), channels.TestNetworkChannel, msgChannel) // channel

			msgOriginID, ok := args[1].(flow.Identifier)
			require.True(suite.T(), ok)
			require.Equal(suite.T(), suite.ids[senderNodeIndex].NodeID, msgOriginID) // sender id

			msgPayload, ok := args[2].(*libp2pmessage.TestMessage)
			require.True(suite.T(), ok)
			// payload
			require.True(suite.T(), regex.MatchString(msgPayload.Text))
			require.False(suite.T(), receivedPayloads.Has(msgPayload.Text)) // payload must be unique
			receivedPayloads.Add(msgPayload.Text, struct{}{})
		}).Return(nil)

	for i := 0; i < count; i++ {
		receiveWG.Add(1)
		sendWG.Add(1)

		expectedPayloadText := fmt.Sprintf("hello from: %d", i)
		require.NoError(suite.T(), err)
		err = con0.Unicast(&libp2pmessage.TestMessage{
			Text: expectedPayloadText,
		}, suite.ids[targetNodeIndex].NodeID)
		require.NoError(suite.Suite.T(), err)

		go func() {
			// sends a direct message from first node to the last node
			err := con0.Unicast(&libp2pmessage.TestMessage{
				Text: expectedPayloadText,
			}, suite.ids[targetNodeIndex].NodeID)
			require.NoError(suite.Suite.T(), err)

			sendWG.Done()
		}()
	}

	unittest.RequireReturnsBefore(suite.T(), sendWG.Wait, 1*time.Second, "could not send unicasts on time")
	unittest.RequireReturnsBefore(suite.T(), receiveWG.Wait, 1*time.Second, "could not receive unicasts on time")
}

// TestEcho sends an echo message from first network to the last network
// the last network echos back the message. The test evaluates the correctness
// of the message reception as well as its content
func (suite *NetworkTestSuite) TestEcho() {
	wg := sync.WaitGroup{}
	// extracts sender id based on the mock option
	var err error

	wg.Add(2)
	first := 0
	last := suite.size - 1
	// mocks a target engine on the first and last nodes of the test suit that will receive the message on the test channel.
	targetEngine1 := &mocknetwork.MessageProcessor{}
	con1, err := suite.networks[first].Register(channels.TestNetworkChannel, targetEngine1)
	require.NoError(suite.T(), err)
	targetEngine2 := &mocknetwork.MessageProcessor{}
	con2, err := suite.networks[last].Register(channels.TestNetworkChannel, targetEngine2)
	require.NoError(suite.T(), err)

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
			require.True(suite.T(), ok)
			require.Equal(suite.T(), channels.TestNetworkChannel, msgChannel) // channel

			msgOriginID, ok := args[1].(flow.Identifier)
			require.True(suite.T(), ok)
			require.Equal(suite.T(), suite.ids[first].NodeID, msgOriginID) // sender id

			msgPayload, ok := args[2].(*libp2pmessage.TestMessage)
			require.True(suite.T(), ok)
			require.Equal(suite.T(), expectedSendMsg, msgPayload.Text) // payload

			// echos back the same message back to the sender
			require.NoError(suite.T(), con2.Unicast(&libp2pmessage.TestMessage{
				Text: expectedReplyMsg,
			}, suite.ids[first].NodeID))
		}).Return(nil).Once()

	// mocks the target engine on the last node of the test suit that will receive the message on the test channel, and
	// echos back the reply message to the sender.
	targetEngine1.On("Process", mockery.Anything, mockery.Anything, mockery.Anything).
		Run(func(args mockery.Arguments) {
			wg.Done()

			msgChannel, ok := args[0].(channels.Channel)
			require.True(suite.T(), ok)
			require.Equal(suite.T(), channels.TestNetworkChannel, msgChannel) // channel

			msgOriginID, ok := args[1].(flow.Identifier)
			require.True(suite.T(), ok)
			require.Equal(suite.T(), suite.ids[last].NodeID, msgOriginID) // sender id

			msgPayload, ok := args[2].(*libp2pmessage.TestMessage)
			require.True(suite.T(), ok)
			require.Equal(suite.T(), expectedReplyMsg, msgPayload.Text) // payload
		}).Return(nil)

	// sends a direct message from first node to the last node
	require.NoError(suite.T(), con1.Unicast(&libp2pmessage.TestMessage{
		Text: expectedSendMsg,
	}, suite.ids[last].NodeID))

	unittest.RequireReturnsBefore(suite.T(), wg.Wait, 5*time.Second, "could not receive unicast on time")
}

// TestMaxMessageSize_Unicast evaluates that invoking Unicast method of the network on a message
// size beyond the permissible unicast message size returns an error.
func (suite *NetworkTestSuite) TestMaxMessageSize_Unicast() {
	first := 0
	last := suite.size - 1

	// creates a network payload beyond the maximum message size
	// Note: networkPayloadFixture considers 1000 bytes as the overhead of the encoded message,
	// so the generated payload is 1000 bytes below the maximum unicast message size.
	// We hence add up 1000 bytes to the input of network payload fixture to make
	// sure that payload is beyond the permissible size.
	payload := testutils.NetworkPayloadFixture(suite.T(), uint(p2pnet.DefaultMaxUnicastMsgSize)+1000)
	event := &libp2pmessage.TestMessage{
		Text: string(payload),
	}

	// sends a direct message from first node to the last node
	con0, err := suite.networks[first].Register(channels.TestNetworkChannel, &mocknetwork.MessageProcessor{})
	require.NoError(suite.T(), err)
	require.Error(suite.T(), con0.Unicast(event, suite.ids[last].NodeID))
}

// TestLargeMessageSize_SendDirect asserts that a ChunkDataResponse is treated as a large message and can be unicasted
// successfully even though it's size is greater than the default message size.
func (suite *NetworkTestSuite) TestLargeMessageSize_SendDirect() {
	sourceIndex := 0
	targetIndex := suite.size - 1
	targetId := suite.ids[targetIndex].NodeID

	// creates a network payload with a size greater than the default max size using a known large message type
	targetSize := uint64(p2pnet.DefaultMaxUnicastMsgSize) + 1000
	event := unittest.ChunkDataResponseMsgFixture(unittest.IdentifierFixture(), unittest.WithApproximateSize(targetSize))

	// expect one message to be received by the target
	ch := make(chan struct{})
	// mocks a target engine on the last node of the test suit that will receive the message on the test channel.
	targetEngine := &mocknetwork.MessageProcessor{}
	_, err := suite.networks[targetIndex].Register(channels.ProvideChunks, targetEngine)
	require.NoError(suite.T(), err)
	targetEngine.On("Process", mockery.Anything, mockery.Anything, mockery.Anything).
		Run(func(args mockery.Arguments) {
			defer close(ch)

			msgChannel, ok := args[0].(channels.Channel)
			require.True(suite.T(), ok)
			require.Equal(suite.T(), channels.ProvideChunks, msgChannel) // channel

			msgOriginID, ok := args[1].(flow.Identifier)
			require.True(suite.T(), ok)
			require.Equal(suite.T(), suite.ids[sourceIndex].NodeID, msgOriginID) // sender id

			msgPayload, ok := args[2].(*messages.ChunkDataResponse)
			require.True(suite.T(), ok)
			require.Equal(suite.T(), event, msgPayload) // payload
		}).Return(nil).Once()

	// sends a direct message from source node to the target node
	con0, err := suite.networks[sourceIndex].Register(channels.ProvideChunks, &mocknetwork.MessageProcessor{})
	require.NoError(suite.T(), err)
	require.NoError(suite.T(), con0.Unicast(event, targetId))

	// check message reception on target
	unittest.RequireCloseBefore(suite.T(), ch, 5*time.Second, "source node failed to send large message to target")
}

// TestMaxMessageSize_Publish evaluates that invoking Publish method of the network on a message
// size beyond the permissible publish message size returns an error.
func (suite *NetworkTestSuite) TestMaxMessageSize_Publish() {
	firstIndex := 0
	lastIndex := suite.size - 1
	lastNodeId := suite.ids[lastIndex].NodeID

	// creates a network payload beyond the maximum message size
	// Note: networkPayloadFixture considers 1000 bytes as the overhead of the encoded message,
	// so the generated payload is 1000 bytes below the maximum publish message size.
	// We hence add up 1000 bytes to the input of network payload fixture to make
	// sure that payload is beyond the permissible size.
	payload := testutils.NetworkPayloadFixture(suite.T(), uint(node.DefaultMaxPubSubMsgSize)+1000)
	event := &libp2pmessage.TestMessage{
		Text: string(payload),
	}
	con0, err := suite.networks[firstIndex].Register(channels.TestNetworkChannel, &mocknetwork.MessageProcessor{})
	require.NoError(suite.T(), err)

	err = con0.Publish(event, lastNodeId)
	require.Error(suite.Suite.T(), err)
	require.ErrorContains(suite.T(), err, "exceeds configured max message size")
}

// TestUnsubscribe tests that an engine can unsubscribe from a topic it was earlier subscribed to and stop receiving
// messages.
func (suite *NetworkTestSuite) TestUnsubscribe() {
	senderIndex := 0
	targetIndex := suite.size - 1
	targetId := suite.ids[targetIndex].NodeID

	msgRcvd := make(chan struct{}, 2)
	msgRcvdFun := func() {
		<-msgRcvd
	}

	targetEngine := &mocknetwork.MessageProcessor{}
	con2, err := suite.networks[targetIndex].Register(channels.TestNetworkChannel, targetEngine)
	require.NoError(suite.T(), err)
	targetEngine.On("Process", mockery.Anything, mockery.Anything, mockery.Anything).
		Run(func(args mockery.Arguments) {
			msgChannel, ok := args[0].(channels.Channel)
			require.True(suite.T(), ok)
			require.Equal(suite.T(), channels.TestNetworkChannel, msgChannel) // channel

			msgOriginID, ok := args[1].(flow.Identifier)
			require.True(suite.T(), ok)
			require.Equal(suite.T(), suite.ids[senderIndex].NodeID, msgOriginID) // sender id

			msgPayload, ok := args[2].(*libp2pmessage.TestMessage)
			require.True(suite.T(), ok)
			require.True(suite.T(), msgPayload.Text == "hello1") // payload, we only expect hello 1 that was sent before unsubscribe.
			msgRcvd <- struct{}{}
		}).Return(nil)

	// first test that when both nodes are subscribed to the channel, the target node receives the message
	con1, err := suite.networks[senderIndex].Register(channels.TestNetworkChannel, &mocknetwork.MessageProcessor{})
	require.NoError(suite.T(), err)

	// set up waiting for suite.size pubsub tags indicating a mesh has formed
	for i := 0; i < suite.size; i++ {
		select {
		case <-suite.obs:
		case <-time.After(2 * time.Second):
			assert.FailNow(suite.T(), "could not receive pubsub tag indicating mesh formed")
		}
	}

	err = con1.Publish(&libp2pmessage.TestMessage{
		Text: string("hello1"),
	}, targetId)
	require.NoError(suite.T(), err)

	unittest.RequireReturnsBefore(suite.T(), msgRcvdFun, 3*time.Second, "message not received")

	// now unsubscribe the target node from the channel
	require.NoError(suite.T(), con2.Close())

	// now publish a new message from the first node
	err = con1.Publish(&libp2pmessage.TestMessage{
		Text: string("hello2"),
	}, targetId)
	assert.NoError(suite.T(), err)

	// assert that the new message is not received by the target node
	unittest.RequireNeverReturnBefore(suite.T(), msgRcvdFun, 2*time.Second, "message received unexpectedly")
}

// TestChunkDataPackMaxMessageSize tests that the max message size for a chunk data pack response is set to the large message size.
func TestChunkDataPackMaxMessageSize(t *testing.T) {
	// creates an outgoing chunk data pack response message (imitating an EN is sending a chunk data pack response to VN).
	msg, err := message.NewOutgoingScope(
		flow.IdentifierList{unittest.IdentifierFixture()},
		channels.TopicFromChannel(channels.ProvideChunks, unittest.IdentifierFixture()),
		&messages.ChunkDataResponse{
			ChunkDataPack: *unittest.ChunkDataPackFixture(unittest.IdentifierFixture()),
			Nonce:         rand.Uint64(),
		},
		unittest.NetworkCodec().Encode,
		message.ProtocolTypeUnicast)
	require.NoError(t, err)

	// get the max message size for the message
	size, err := p2pnet.UnicastMaxMsgSizeByCode(msg.Proto().Payload)
	require.NoError(t, err)
	require.Equal(t, p2pnet.LargeMsgMaxUnicastMsgSize, size)
}
