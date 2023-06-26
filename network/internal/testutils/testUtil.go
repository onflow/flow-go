package testutils

import (
	"context"
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
	p2pNetwork "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	pc "github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-libp2p/core/routing"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/config"
	netconf "github.com/onflow/flow-go/config/network"
	"github.com/onflow/flow-go/crypto"
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
	"github.com/onflow/flow-go/network/mocknetwork"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/conduit"
	"github.com/onflow/flow-go/network/p2p/connection"
	p2pdht "github.com/onflow/flow-go/network/p2p/dht"
	"github.com/onflow/flow-go/network/p2p/middleware"
	"github.com/onflow/flow-go/network/p2p/p2pbuilder"
	p2pconfig "github.com/onflow/flow-go/network/p2p/p2pbuilder/config"
	inspectorbuilder "github.com/onflow/flow-go/network/p2p/p2pbuilder/inspector"
	"github.com/onflow/flow-go/network/p2p/subscription"
	"github.com/onflow/flow-go/network/p2p/translator"
	"github.com/onflow/flow-go/network/p2p/unicast"
	"github.com/onflow/flow-go/network/p2p/unicast/protocols"
	"github.com/onflow/flow-go/network/p2p/unicast/ratelimit"
	"github.com/onflow/flow-go/network/slashing"
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

// GenerateIDs is a test helper that generate flow identities with a valid port and libp2p nodes.
func GenerateIDs(t *testing.T, n int, opts ...func(*optsConfig)) (flow.IdentityList, []p2p.LibP2PNode, []observable.Observable) {
	libP2PNodes := make([]p2p.LibP2PNode, n)
	tagObservables := make([]observable.Observable, n)

	identities := unittest.IdentityListFixture(n, unittest.WithAllRoles())
	idProvider := NewUpdatableIDProvider(identities)
	o := &optsConfig{
		peerUpdateInterval:            connection.DefaultPeerUpdateInterval,
		unicastRateLimiterDistributor: ratelimit.NewUnicastRateLimiterDistributor(),
		connectionGaterFactory: func() p2p.ConnectionGater {
			return NewConnectionGater(idProvider, func(p peer.ID) error {
				return nil
			})
		},
		createStreamRetryInterval: unicast.DefaultRetryDelay,
	}
	for _, opt := range opts {
		opt(o)
	}

	for _, identity := range identities {
		for _, idOpt := range o.idOpts {
			idOpt(identity)
		}
	}

	// generates keys and address for the node
	for i, identity := range identities {
		// generate key
		key, err := generateNetworkingKey(identity.NodeID)
		require.NoError(t, err)

		var opts []nodeBuilderOption

		opts = append(opts, withDHT(o.dhtPrefix, o.dhtOpts...))
		opts = append(opts, withRateLimiterDistributor(o.unicastRateLimiterDistributor))
		opts = append(opts, withConnectionGater(o.connectionGaterFactory()))
		opts = append(opts, withUnicastManagerOpts(o.createStreamRetryInterval))

		libP2PNodes[i], tagObservables[i] = generateLibP2PNode(t, unittest.Logger(), key, idProvider, opts...)

		_, port, err := libP2PNodes[i].GetIPPort()
		require.NoError(t, err)

		identities[i].Address = unittest.IPPort(port)
		identities[i].NetworkPubKey = key.PublicKey()
	}

	return identities, libP2PNodes, tagObservables
}

// GenerateMiddlewares creates and initializes middleware instances for all the identities
func GenerateMiddlewares(t *testing.T, identities flow.IdentityList, libP2PNodes []p2p.LibP2PNode, opts ...func(*optsConfig)) ([]network.Middleware, []*UpdatableIDProvider) {
	mws := make([]network.Middleware, len(identities))
	idProviders := make([]*UpdatableIDProvider, len(identities))
	bitswapmet := metrics.NewNoopCollector()
	o := &optsConfig{
		peerUpdateInterval:  connection.DefaultPeerUpdateInterval,
		unicastRateLimiters: ratelimit.NoopRateLimiters(),
		networkMetrics:      metrics.NewNoopCollector(),
		peerManagerFilters:  []p2p.PeerFilter{},
		cfg: &middleware.Config{
			Logger:                     unittest.Logger(),
			BitSwapMetrics:             bitswapmet,
			RootBlockID:                sporkID,
			UnicastMessageTimeout:      middleware.DefaultUnicastTimeout,
			Codec:                      unittest.NetworkCodec(),
			SlashingViolationsConsumer: mocknetwork.NewViolationsConsumer(t),
		},
	}

	for _, opt := range opts {
		opt(o)
	}

	total := len(identities)
	for i := 0; i < total; i++ {
		i := i
		o.cfg.Libp2pNode = libP2PNodes[i]
		o.cfg.FlowId = identities[i].NodeID
		idProviders[i] = NewUpdatableIDProvider(identities)
		o.cfg.IdTranslator = translator.NewIdentityProviderIDTranslator(idProviders[i])

		mws[i] = middleware.NewMiddleware(o.cfg,
			middleware.WithUnicastRateLimiters(o.unicastRateLimiters),
			middleware.WithPeerManagerFilters(o.peerManagerFilters))
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

// GenerateIDsAndMiddlewares returns nodeIDs, libp2pNodes, middlewares, and observables which can be subscirbed to in order to witness protect events from pubsub
func GenerateIDsAndMiddlewares(t *testing.T, n int, opts ...func(*optsConfig)) (flow.IdentityList, []p2p.LibP2PNode, []network.Middleware) {
	ids, libP2PNodes, _ := GenerateIDs(t, n, opts...)
	mws, _ := GenerateMiddlewares(t, ids, libP2PNodes, opts...)
	return ids, libP2PNodes, mws
}

type optsConfig struct {
	idOpts                        []func(*flow.Identity)
	dhtPrefix                     string
	dhtOpts                       []dht.Option
	unicastRateLimiters           *ratelimit.RateLimiters
	peerUpdateInterval            time.Duration
	networkMetrics                module.NetworkMetrics
	peerManagerFilters            []p2p.PeerFilter
	unicastRateLimiterDistributor p2p.UnicastRateLimiterDistributor
	connectionGaterFactory        func() p2p.ConnectionGater
	createStreamRetryInterval     time.Duration
	cfg                           *middleware.Config
}

func WithUnicastRateLimiterDistributor(distributor p2p.UnicastRateLimiterDistributor) func(*optsConfig) {
	return func(o *optsConfig) {
		o.unicastRateLimiterDistributor = distributor
	}
}

func WithIdentityOpts(idOpts ...func(*flow.Identity)) func(*optsConfig) {
	return func(o *optsConfig) {
		o.idOpts = idOpts
	}
}

func WithDHT(prefix string, dhtOpts ...dht.Option) func(*optsConfig) {
	return func(o *optsConfig) {
		o.dhtPrefix = prefix
		o.dhtOpts = dhtOpts
	}
}

func WithPeerUpdateInterval(interval time.Duration) func(*optsConfig) {
	return func(o *optsConfig) {
		o.peerUpdateInterval = interval
	}
}

func WithPeerManagerFilters(filters ...p2p.PeerFilter) func(*optsConfig) {
	return func(o *optsConfig) {
		o.peerManagerFilters = filters
	}
}

func WithUnicastRateLimiters(limiters *ratelimit.RateLimiters) func(*optsConfig) {
	return func(o *optsConfig) {
		o.unicastRateLimiters = limiters
	}
}

func WithSlashingViolationConsumer(consumer slashing.ViolationsConsumer) func(*optsConfig) {
	return func(o *optsConfig) {
		o.cfg.SlashingViolationsConsumer = consumer
	}
}

func WithConnectionGaterFactory(connectionGaterFactory func() p2p.ConnectionGater) func(*optsConfig) {
	return func(o *optsConfig) {
		o.connectionGaterFactory = connectionGaterFactory
	}
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

type nodeBuilderOption func(p2p.NodeBuilder)

func withDHT(prefix string, dhtOpts ...dht.Option) nodeBuilderOption {
	return func(nb p2p.NodeBuilder) {
		nb.SetRoutingSystem(func(c context.Context, h host.Host) (routing.Routing, error) {
			return p2pdht.NewDHT(c, h, pc.ID(protocols.FlowDHTProtocolIDPrefix+prefix), zerolog.Nop(), metrics.NewNoopCollector(), dhtOpts...)
		})
	}
}

func withRateLimiterDistributor(distributor p2p.UnicastRateLimiterDistributor) nodeBuilderOption {
	return func(nb p2p.NodeBuilder) {
		nb.SetRateLimiterDistributor(distributor)
	}
}

func withConnectionGater(connectionGater p2p.ConnectionGater) nodeBuilderOption {
	return func(nb p2p.NodeBuilder) {
		nb.SetConnectionGater(connectionGater)
	}
}

func withUnicastManagerOpts(delay time.Duration) nodeBuilderOption {
	return func(nb p2p.NodeBuilder) {
		nb.SetStreamCreationRetryInterval(delay)
	}
}

// generateLibP2PNode generates a `LibP2PNode` on localhost using a port assigned by the OS
func generateLibP2PNode(t *testing.T, logger zerolog.Logger, key crypto.PrivateKey, provider *UpdatableIDProvider, opts ...nodeBuilderOption) (p2p.LibP2PNode, observable.Observable) {

	noopMetrics := metrics.NewNoopCollector()
	defaultFlowConfig, err := config.DefaultConfig()
	require.NoError(t, err)

	// Inject some logic to be able to observe connections of this node
	connManager, err := NewTagWatchingConnManager(logger, noopMetrics, &defaultFlowConfig.NetworkConfig.ConnectionManagerConfig)
	require.NoError(t, err)
	met := metrics.NewNoopCollector()

	rpcInspectorSuite, err := inspectorbuilder.NewGossipSubInspectorBuilder(logger, sporkID, &defaultFlowConfig.NetworkConfig.GossipSubConfig.GossipSubRPCInspectorsConfig, provider, met).Build()
	require.NoError(t, err)

	builder := p2pbuilder.NewNodeBuilder(
		logger,
		met,
		unittest.DefaultAddress,
		key,
		sporkID,
		&defaultFlowConfig.NetworkConfig.ResourceManagerConfig,
		&p2pconfig.PeerManagerConfig{
			ConnectionPruning: true,
			UpdateInterval:    1 * time.Second,
			ConnectorFactory:  connection.DefaultLibp2pBackoffConnectorFactory(),
		}, // disable peer manager
		&p2p.DisallowListCacheConfig{
			MaxSize: uint32(1000),
			Metrics: metrics.NewNoopCollector(),
		}).
		SetConnectionManager(connManager).
		SetResourceManager(NewResourceManager(t)).
		SetStreamCreationRetryInterval(unicast.DefaultRetryDelay).
		SetGossipSubRpcInspectorSuite(rpcInspectorSuite)

	for _, opt := range opts {
		opt(builder)
	}

	libP2PNode, err := builder.Build()
	require.NoError(t, err)

	return libP2PNode, connManager
}

// OptionalSleep introduces a sleep to allow nodes to heartbeat and discover each other (only needed when using PubSub)
func OptionalSleep(send ConduitSendWrapperFunc) {
	sendFuncName := runtime.FuncForPC(reflect.ValueOf(send).Pointer()).Name()
	if strings.Contains(sendFuncName, "Multicast") || strings.Contains(sendFuncName, "Publish") {
		time.Sleep(2 * time.Second)
	}
}

// generateNetworkingKey generates a Flow ECDSA key using the given seed
func generateNetworkingKey(s flow.Identifier) (crypto.PrivateKey, error) {
	seed := make([]byte, crypto.KeyGenSeedMinLen)
	copy(seed, s[:])
	return crypto.GeneratePrivateKey(crypto.ECDSASecp256k1, seed)
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

// NewResourceManager creates a new resource manager for testing with no limits.
func NewResourceManager(t *testing.T) p2pNetwork.ResourceManager {
	return &p2pNetwork.NullResourceManager{}
}

// NewConnectionGater creates a new connection gater for testing with given allow listing filter.
func NewConnectionGater(idProvider module.IdentityProvider, allowListFilter p2p.PeerFilter) p2p.ConnectionGater {
	filters := []p2p.PeerFilter{allowListFilter}
	return connection.NewConnGater(unittest.Logger(),
		idProvider,
		connection.WithOnInterceptPeerDialFilters(filters),
		connection.WithOnInterceptSecuredFilters(filters))
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
