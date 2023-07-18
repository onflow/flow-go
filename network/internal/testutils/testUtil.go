package testutils

import (
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/config"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	libp2pmessage "github.com/onflow/flow-go/model/libp2p/message"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/id"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/module/observable"
	"github.com/onflow/flow-go/network"
	alspmgr "github.com/onflow/flow-go/network/alsp/manager"
	netcache "github.com/onflow/flow-go/network/cache"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/codec/cbor"
	"github.com/onflow/flow-go/network/netconf"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/conduit"
	"github.com/onflow/flow-go/network/p2p/connection"
	"github.com/onflow/flow-go/network/p2p/middleware"
	"github.com/onflow/flow-go/network/p2p/subscription"
	p2ptest "github.com/onflow/flow-go/network/p2p/test"
	"github.com/onflow/flow-go/network/p2p/translator"
	"github.com/onflow/flow-go/utils/unittest"
)

var sporkID = unittest.IdentifierFixture()

// RateLimitConsumer p2p.RateLimiterConsumer fixture that invokes a callback when rate limit event is consumed.
type RateLimitConsumer struct {
	callback func(pid peer.ID, role, msgType, topic, reason string) // callback func that will be invoked on rate limit
}

func (r *RateLimitConsumer) OnRateLimitedPeer(pid peer.ID, role, msgType, topic, reason string) {
	r.callback(pid, role, msgType, topic, reason)
}

type PeerTag struct {
	Peer peer.ID
	Tag  string
}

// TagWatchingConnManager implements connection.ConnManager struct, and manages connections with tags. It
// also maintains a set of observers that it notifies when a tag is added or removed from a peer.
type TagWatchingConnManager struct {
	*connection.ConnManager
	observers map[observable.Observer]struct{}
	obsLock   sync.RWMutex
}

// Subscribe allows an observer to subscribe to receive notifications when a tag is added or removed from a peer.
func (tw *TagWatchingConnManager) Subscribe(observer observable.Observer) {
	tw.obsLock.Lock()
	defer tw.obsLock.Unlock()
	var void struct{}
	tw.observers[observer] = void
}

// Unsubscribe allows an observer to unsubscribe from receiving notifications.
func (tw *TagWatchingConnManager) Unsubscribe(observer observable.Observer) {
	tw.obsLock.Lock()
	defer tw.obsLock.Unlock()
	delete(tw.observers, observer)
}

// Protect adds a tag to a peer. It also notifies all observers that a tag has been added to a peer.
func (tw *TagWatchingConnManager) Protect(id peer.ID, tag string) {
	tw.obsLock.RLock()
	defer tw.obsLock.RUnlock()
	tw.ConnManager.Protect(id, tag)
	for obs := range tw.observers {
		go obs.OnNext(PeerTag{Peer: id, Tag: tag})
	}
}

// Unprotect removes a tag from a peer. It also notifies all observers that a tag has been removed from a peer.
func (tw *TagWatchingConnManager) Unprotect(id peer.ID, tag string) bool {
	tw.obsLock.RLock()
	defer tw.obsLock.RUnlock()
	res := tw.ConnManager.Unprotect(id, tag)
	for obs := range tw.observers {
		go obs.OnNext(PeerTag{Peer: id, Tag: tag})
	}
	return res
}

// NewTagWatchingConnManager creates a new TagWatchingConnManager with the given config. It returns an error if the config is invalid.
func NewTagWatchingConnManager(log zerolog.Logger, metrics module.LibP2PConnectionMetrics, config *netconf.ConnectionManagerConfig) (*TagWatchingConnManager, error) {
	cm, err := connection.NewConnManager(log, metrics, config)
	if err != nil {
		return nil, fmt.Errorf("could not create connection manager: %w", err)
	}

	return &TagWatchingConnManager{
		ConnManager: cm,
		observers:   make(map[observable.Observer]struct{}),
		obsLock:     sync.RWMutex{},
	}, nil
}

// LibP2PNodeForMiddlewareFixture is a test helper that generate flow identities with a valid port and libp2p nodes.
// Note that the LibP2PNode created by this fixture is meant to used with a middleware component.
// If you want to create a standalone LibP2PNode without network and middleware components, please use p2ptest.NodeFixture.
// Args:
//
//	t: testing.T- the test object
//
// n: int - number of nodes to create
// opts: []p2ptest.NodeFixtureParameterOption - options to configure the nodes
// Returns:
//
//	flow.IdentityList - list of identities created for the nodes, one for each node.
//
// []p2p.LibP2PNode - list of libp2p nodes created.
// []observable.Observable - list of observables created for each node.
func LibP2PNodeForMiddlewareFixture(t *testing.T, n int, opts ...p2ptest.NodeFixtureParameterOption) (flow.IdentityList, []p2p.LibP2PNode, []observable.Observable) {

	libP2PNodes := make([]p2p.LibP2PNode, 0)
	identities := make(flow.IdentityList, 0)
	tagObservables := make([]observable.Observable, 0)
	idProvider := unittest.NewUpdatableIDProvider(flow.IdentityList{})
	defaultFlowConfig, err := config.DefaultConfig()
	require.NoError(t, err)

	opts = append(opts, p2ptest.WithUnicastHandlerFunc(nil))

	for i := 0; i < n; i++ {
		// TODO: generating a tag watching connection manager can be moved to a separate function, as only a few tests need this.
		// For the rest of tests, the node can run on the default connection manager without setting and option.
		connManager, err := NewTagWatchingConnManager(unittest.Logger(), metrics.NewNoopCollector(), &defaultFlowConfig.NetworkConfig.ConnectionManagerConfig)
		require.NoError(t, err)

		opts = append(opts, p2ptest.WithConnectionManager(connManager))
		node, nodeId := p2ptest.NodeFixture(t,
			sporkID,
			t.Name(),
			idProvider,
			opts...)
		libP2PNodes = append(libP2PNodes, node)
		identities = append(identities, &nodeId)
		tagObservables = append(tagObservables, connManager)
	}
	idProvider.SetIdentities(identities)
	return identities, libP2PNodes, tagObservables
}

// MiddlewareConfigFixture is a test helper that generates a middleware config for testing.
// Args:
// - t: the test instance.
// Returns:
// - a middleware config.
func MiddlewareConfigFixture(t *testing.T) *middleware.Config {
	return &middleware.Config{
		Logger:                unittest.Logger(),
		BitSwapMetrics:        metrics.NewNoopCollector(),
		RootBlockID:           sporkID,
		UnicastMessageTimeout: middleware.DefaultUnicastTimeout,
		Codec:                 unittest.NetworkCodec(),
	}
}

// MiddlewareFixtures is a test helper that generates middlewares with the given identities and libp2p nodes.
// It also generates a list of UpdatableIDProvider that can be used to update the identities of the middlewares.
// The number of identities and libp2p nodes must be the same.
// Args:
// - identities: a list of flow identities that correspond to the libp2p nodes.
// - libP2PNodes: a list of libp2p nodes that correspond to the identities.
// - cfg: the middleware config.
// - opts: a list of middleware option functions.
// Returns:
// - a list of middlewares - one for each identity.
// - a list of UpdatableIDProvider - one for each identity.
func MiddlewareFixtures(t *testing.T, identities flow.IdentityList, libP2PNodes []p2p.LibP2PNode, cfg *middleware.Config, consumer network.ViolationsConsumer, opts ...middleware.OptionFn) ([]network.Middleware, []*unittest.UpdatableIDProvider) {
	require.Equal(t, len(identities), len(libP2PNodes))

	mws := make([]network.Middleware, len(identities))
	idProviders := make([]*unittest.UpdatableIDProvider, len(identities))

	for i := 0; i < len(identities); i++ {
		i := i
		cfg.Libp2pNode = libP2PNodes[i]
		cfg.FlowId = identities[i].NodeID
		idProviders[i] = unittest.NewUpdatableIDProvider(identities)
		cfg.IdTranslator = translator.NewIdentityProviderIDTranslator(idProviders[i])
		mws[i] = middleware.NewMiddleware(cfg, opts...)
		mws[i].SetSlashingViolationsConsumer(consumer)
	}
	return mws, idProviders
}

// NetworksFixture generates the network for the given middlewares
func NetworksFixture(t *testing.T,
	ids flow.IdentityList,
	mws []network.Middleware) []network.Network {

	count := len(ids)
	nets := make([]network.Network, 0)

	for i := 0; i < count; i++ {

		params := NetworkConfigFixture(t, *ids[i], ids, mws[i])
		net, err := p2p.NewNetwork(params)
		require.NoError(t, err)

		nets = append(nets, net)
	}

	return nets
}

func NetworkConfigFixture(
	t *testing.T,
	myId flow.Identity,
	allIds flow.IdentityList,
	mw network.Middleware,
	opts ...p2p.NetworkConfigOption) *p2p.NetworkConfig {

	me := mock.NewLocal(t)
	me.On("NodeID").Return(myId.NodeID).Maybe()
	me.On("NotMeFilter").Return(filter.Not(filter.HasNodeID(me.NodeID()))).Maybe()
	me.On("Address").Return(myId.Address).Maybe()

	defaultFlowConfig, err := config.DefaultConfig()
	require.NoError(t, err)

	receiveCache := netcache.NewHeroReceiveCache(
		defaultFlowConfig.NetworkConfig.NetworkReceivedMessageCacheSize,
		unittest.Logger(),
		metrics.NewNoopCollector())
	subMgr := subscription.NewChannelSubscriptionManager(mw)
	params := &p2p.NetworkConfig{
		Logger:              unittest.Logger(),
		Codec:               unittest.NetworkCodec(),
		Me:                  me,
		MiddlewareFactory:   func() (network.Middleware, error) { return mw, nil },
		Topology:            unittest.NetworkTopology(),
		SubscriptionManager: subMgr,
		Metrics:             metrics.NewNoopCollector(),
		IdentityProvider:    id.NewFixedIdentityProvider(allIds),
		ReceiveCache:        receiveCache,
		ConduitFactory:      conduit.NewDefaultConduitFactory(),
		AlspCfg: &alspmgr.MisbehaviorReportManagerConfig{
			Logger:                  unittest.Logger(),
			SpamRecordCacheSize:     defaultFlowConfig.NetworkConfig.AlspConfig.SpamRecordCacheSize,
			SpamReportQueueSize:     defaultFlowConfig.NetworkConfig.AlspConfig.SpamReportQueueSize,
			HeartBeatInterval:       defaultFlowConfig.NetworkConfig.AlspConfig.HearBeatInterval,
			AlspMetrics:             metrics.NewNoopCollector(),
			HeroCacheMetricsFactory: metrics.NewNoopHeroCacheMetricsFactory(),
		},
	}

	for _, opt := range opts {
		opt(params)
	}

	return params
}

// GenerateEngines generates MeshEngines for the given networks
func GenerateEngines(t *testing.T, nets []network.Network) []*MeshEngine {
	count := len(nets)
	engs := make([]*MeshEngine, count)
	for i, n := range nets {
		eng := NewMeshEngine(t, n, 100, channels.TestNetworkChannel)
		engs[i] = eng
	}
	return engs
}

// StartNodesAndNetworks starts the provided networks and libp2p nodes, returning the irrecoverable error channel.
// Arguments:
// - ctx: the irrecoverable context to use for starting the nodes and networks.
// - t: the test object.
// - nodes: the libp2p nodes to start.
// - nets: the networks to start.
// - timeout: the timeout to use for waiting for the nodes and networks to start.
//
// This function fails the test if the nodes or networks do not start within the given timeout.
func StartNodesAndNetworks(ctx irrecoverable.SignalerContext, t *testing.T, nodes []p2p.LibP2PNode, nets []network.Network, timeout time.Duration) {
	StartNetworks(ctx, t, nets, timeout)

	// start up nodes and Peer managers
	StartNodes(ctx, t, nodes, timeout)
}

// StartNetworks starts the provided networks using the provided irrecoverable context
// Arguments:
// - ctx: the irrecoverable context to use for starting the networks.
// - t: the test object.
// - nets: the networks to start.
// - duration: the timeout to use for waiting for the networks to start.
//
// This function fails the test if the networks do not start within the given timeout.
func StartNetworks(ctx irrecoverable.SignalerContext, t *testing.T, nets []network.Network, duration time.Duration) {
	// start up networks (this will implicitly start middlewares)
	for _, net := range nets {
		net.Start(ctx)
		unittest.RequireComponentsReadyBefore(t, duration, net)
	}
}

// StartNodes starts the provided nodes and their peer managers using the provided irrecoverable context
func StartNodes(ctx irrecoverable.SignalerContext, t *testing.T, nodes []p2p.LibP2PNode, duration time.Duration) {
	for _, node := range nodes {
		node.Start(ctx)
		unittest.RequireComponentsReadyBefore(t, duration, node)

		pm := node.PeerManagerComponent()
		pm.Start(ctx)
		unittest.RequireComponentsReadyBefore(t, duration, pm)
	}
}

// StopComponents stops ReadyDoneAware instances in parallel and fails the test if they could not be stopped within the
// duration.
func StopComponents[R module.ReadyDoneAware](t *testing.T, rda []R, duration time.Duration) {
	comps := make([]module.ReadyDoneAware, 0, len(rda))
	for _, c := range rda {
		comps = append(comps, c)
	}

	unittest.RequireComponentsDoneBefore(t, duration, comps...)
}

// OptionalSleep introduces a sleep to allow nodes to heartbeat and discover each other (only needed when using PubSub)
func OptionalSleep(send ConduitSendWrapperFunc) {
	sendFuncName := runtime.FuncForPC(reflect.ValueOf(send).Pointer()).Name()
	if strings.Contains(sendFuncName, "Multicast") || strings.Contains(sendFuncName, "Publish") {
		time.Sleep(2 * time.Second)
	}
}

// NetworkPayloadFixture creates a blob of random bytes with the given size (in bytes) and returns it.
// The primary goal of utilizing this helper function is to apply stress tests on the network layer by
// sending large messages to transmit.
func NetworkPayloadFixture(t *testing.T, size uint) []byte {
	// reserves 1000 bytes for the message headers, encoding overhead, and libp2p message overhead.
	overhead := 1000
	require.Greater(t, int(size), overhead, "could not generate message below size threshold")
	emptyEvent := &libp2pmessage.TestMessage{
		Text: "",
	}

	// encodes the message
	codec := cbor.NewCodec()
	empty, err := codec.Encode(emptyEvent)
	require.NoError(t, err)

	// max possible payload size
	payloadSize := int(size) - overhead - len(empty)
	payload := make([]byte, payloadSize)

	// populates payload with random bytes
	for i := range payload {
		payload[i] = 'a' // a utf-8 char that translates to 1-byte when converted to a string
	}

	event := emptyEvent
	event.Text = string(payload)
	// encode Event the way the network would encode it to get the size of the message
	// just to do the size check
	encodedEvent, err := codec.Encode(event)
	require.NoError(t, err)

	require.InDelta(t, len(encodedEvent), int(size), float64(overhead))

	return payload
}

// IsRateLimitedPeerFilter returns a p2p.PeerFilter that will return an error if the peer is rate limited.
func IsRateLimitedPeerFilter(rateLimiter p2p.RateLimiter) p2p.PeerFilter {
	return func(p peer.ID) error {
		if rateLimiter.IsRateLimited(p) {
			return fmt.Errorf("peer is rate limited")
		}
		return nil
	}
}

// NewRateLimiterConsumer returns a p2p.RateLimiterConsumer fixture that will invoke the callback provided.
func NewRateLimiterConsumer(callback func(pid peer.ID, role, msgType, topic, reason string)) p2p.RateLimiterConsumer {
	return &RateLimitConsumer{callback}
}
