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
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/module/observable"
	"github.com/onflow/flow-go/network"
	alspmgr "github.com/onflow/flow-go/network/alsp/manager"
	netcache "github.com/onflow/flow-go/network/cache"
	"github.com/onflow/flow-go/network/codec/cbor"
	"github.com/onflow/flow-go/network/mocknetwork"
	"github.com/onflow/flow-go/network/netconf"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/conduit"
	"github.com/onflow/flow-go/network/p2p/connection"
	"github.com/onflow/flow-go/network/p2p/p2pnet"
	p2ptest "github.com/onflow/flow-go/network/p2p/test"
	"github.com/onflow/flow-go/network/p2p/translator"
	"github.com/onflow/flow-go/utils/unittest"
)

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
func NewTagWatchingConnManager(log zerolog.Logger, metrics module.LibP2PConnectionMetrics, config *netconf.ConnectionManager) (*TagWatchingConnManager, error) {
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

// LibP2PNodeForNetworkFixture is a test helper that generate flow identities with a valid port and libp2p nodes.
// Note that the LibP2PNode created by this fixture is meant to used with a network component.
// If you want to create a standalone LibP2PNode without network component, please use p2ptest.NodeFixture.
// Args:
//
//	t: testing.T- the test object
//	sporkId: flow.Identifier - the spork id to use for the nodes
//	n: int - number of nodes to create
//
// opts: []p2ptest.NodeFixtureParameterOption - options to configure the nodes
// Returns:
//
//	flow.IdentityList - list of identities created for the nodes, one for each node.
//
// []p2p.LibP2PNode - list of libp2p nodes created.
// TODO: several test cases only need a single node, consider encapsulating this function in a single node fixture.
func LibP2PNodeForNetworkFixture(t *testing.T, sporkId flow.Identifier, n int, opts ...p2ptest.NodeFixtureParameterOption) (flow.IdentityList, []p2p.LibP2PNode) {
	libP2PNodes := make([]p2p.LibP2PNode, 0)
	identities := make(flow.IdentityList, 0)
	idProvider := unittest.NewUpdatableIDProvider(flow.IdentityList{})
	opts = append(opts, p2ptest.WithUnicastHandlerFunc(nil))

	for i := 0; i < n; i++ {
		node, nodeId := p2ptest.NodeFixture(t,
			sporkId,
			t.Name(),
			idProvider,
			opts...)
		libP2PNodes = append(libP2PNodes, node)
		identities = append(identities, &nodeId)
	}
	idProvider.SetIdentities(identities)
	return identities, libP2PNodes
}

// NetworksFixture generates the network for the given libp2p nodes.
func NetworksFixture(t *testing.T,
	sporkId flow.Identifier,
	ids flow.IdentityList,
	libp2pNodes []p2p.LibP2PNode,
	configOpts ...func(*p2pnet.NetworkConfig)) ([]*p2pnet.Network, []*unittest.UpdatableIDProvider) {

	count := len(ids)
	nets := make([]*p2pnet.Network, 0)
	idProviders := make([]*unittest.UpdatableIDProvider, 0)

	for i := 0; i < count; i++ {
		idProvider := unittest.NewUpdatableIDProvider(ids)
		params := NetworkConfigFixture(t, *ids[i], idProvider, sporkId, libp2pNodes[i])

		for _, opt := range configOpts {
			opt(params)
		}

		net, err := p2pnet.NewNetwork(params)
		require.NoError(t, err)

		nets = append(nets, net)
		idProviders = append(idProviders, idProvider)
	}

	return nets, idProviders
}

func NetworkConfigFixture(
	t *testing.T,
	myId flow.Identity,
	idProvider module.IdentityProvider,
	sporkId flow.Identifier,
	libp2pNode p2p.LibP2PNode,
	opts ...p2pnet.NetworkConfigOption) *p2pnet.NetworkConfig {

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
	params := &p2pnet.NetworkConfig{
		Logger:                unittest.Logger(),
		Codec:                 unittest.NetworkCodec(),
		Libp2pNode:            libp2pNode,
		Me:                    me,
		BitSwapMetrics:        metrics.NewNoopCollector(),
		Topology:              unittest.NetworkTopology(),
		Metrics:               metrics.NewNoopCollector(),
		IdentityProvider:      idProvider,
		ReceiveCache:          receiveCache,
		ConduitFactory:        conduit.NewDefaultConduitFactory(),
		SporkId:               sporkId,
		UnicastMessageTimeout: p2pnet.DefaultUnicastTimeout,
		IdentityTranslator:    translator.NewIdentityProviderIDTranslator(idProvider),
		AlspCfg: &alspmgr.MisbehaviorReportManagerConfig{
			Logger:                  unittest.Logger(),
			SpamRecordCacheSize:     defaultFlowConfig.NetworkConfig.AlspConfig.SpamRecordCacheSize,
			SpamReportQueueSize:     defaultFlowConfig.NetworkConfig.AlspConfig.SpamReportQueueSize,
			HeartBeatInterval:       defaultFlowConfig.NetworkConfig.AlspConfig.HearBeatInterval,
			AlspMetrics:             metrics.NewNoopCollector(),
			HeroCacheMetricsFactory: metrics.NewNoopHeroCacheMetricsFactory(),
		},
		SlashingViolationConsumerFactory: func(_ network.ConduitAdapter) network.ViolationsConsumer {
			return mocknetwork.NewViolationsConsumer(t)
		},
	}

	for _, opt := range opts {
		opt(params)
	}

	return params
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
func StartNodesAndNetworks(ctx irrecoverable.SignalerContext, t *testing.T, nodes []p2p.LibP2PNode, nets []network.EngineRegistry) {
	StartNetworks(ctx, t, nets)

	// start up nodes and Peer managers
	StartNodes(ctx, t, nodes)
}

// StartNetworks starts the provided networks using the provided irrecoverable context
// Arguments:
// - ctx: the irrecoverable context to use for starting the networks.
// - t: the test object.
// - nets: the networks to start.
// - duration: the timeout to use for waiting for the networks to start.
//
// This function fails the test if the networks do not start within the given timeout.
func StartNetworks(ctx irrecoverable.SignalerContext, t *testing.T, nets []network.EngineRegistry) {
	for _, net := range nets {
		net.Start(ctx)
		unittest.RequireComponentsReadyBefore(t, 5*time.Second, net)
	}
}

// StartNodes starts the provided nodes and their peer managers using the provided irrecoverable context
func StartNodes(ctx irrecoverable.SignalerContext, t *testing.T, nodes []p2p.LibP2PNode) {
	for _, node := range nodes {
		node.Start(ctx)
		unittest.RequireComponentsReadyBefore(t, 5*time.Second, node)

		pm := node.PeerManagerComponent()
		pm.Start(ctx)
		unittest.RequireComponentsReadyBefore(t, 5*time.Second, pm)
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
