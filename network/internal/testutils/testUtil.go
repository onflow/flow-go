package testutils

import (
	"context"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/connmgr"
	"github.com/libp2p/go-libp2p/core/host"
	p2pNetwork "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	pc "github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-libp2p/core/routing"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

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
	netcache "github.com/onflow/flow-go/network/cache"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/codec/cbor"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/connection"
	p2pdht "github.com/onflow/flow-go/network/p2p/dht"
	"github.com/onflow/flow-go/network/p2p/middleware"
	"github.com/onflow/flow-go/network/p2p/p2pbuilder"
	"github.com/onflow/flow-go/network/p2p/subscription"
	"github.com/onflow/flow-go/network/p2p/translator"
	"github.com/onflow/flow-go/network/p2p/unicast"
	"github.com/onflow/flow-go/network/p2p/unicast/ratelimit"
	"github.com/onflow/flow-go/network/slashing"
	"github.com/onflow/flow-go/utils/unittest"
)

var sporkID = unittest.IdentifierFixture()

type PeerTag struct {
	Peer peer.ID
	Tag  string
}

type TagWatchingConnManager struct {
	*connection.ConnManager
	observers map[observable.Observer]struct{}
	obsLock   sync.RWMutex
}

func (cwcm *TagWatchingConnManager) Subscribe(observer observable.Observer) {
	cwcm.obsLock.Lock()
	defer cwcm.obsLock.Unlock()
	var void struct{}
	cwcm.observers[observer] = void
}

func (cwcm *TagWatchingConnManager) Unsubscribe(observer observable.Observer) {
	cwcm.obsLock.Lock()
	defer cwcm.obsLock.Unlock()
	delete(cwcm.observers, observer)
}

func (cwcm *TagWatchingConnManager) Protect(id peer.ID, tag string) {
	cwcm.obsLock.RLock()
	defer cwcm.obsLock.RUnlock()
	cwcm.ConnManager.Protect(id, tag)
	for obs := range cwcm.observers {
		go obs.OnNext(PeerTag{Peer: id, Tag: tag})
	}
}

func (cwcm *TagWatchingConnManager) Unprotect(id peer.ID, tag string) bool {
	cwcm.obsLock.RLock()
	defer cwcm.obsLock.RUnlock()
	res := cwcm.ConnManager.Unprotect(id, tag)
	for obs := range cwcm.observers {
		go obs.OnNext(PeerTag{Peer: id, Tag: tag})
	}
	return res
}

func NewTagWatchingConnManager(log zerolog.Logger, idProvider module.IdentityProvider, metrics module.NetworkMetrics) *TagWatchingConnManager {
	cm := connection.NewConnManager(log, metrics)
	return &TagWatchingConnManager{
		ConnManager: cm,
		observers:   make(map[observable.Observer]struct{}),
		obsLock:     sync.RWMutex{},
	}
}

// GenerateIDs is a test helper that generate flow identities with a valid port and libp2p nodes.
func GenerateIDs(t *testing.T, logger zerolog.Logger, n int, opts ...func(*optsConfig)) (flow.IdentityList,
	[]p2p.LibP2PNode,
	[]observable.Observable) {
	libP2PNodes := make([]p2p.LibP2PNode, n)
	tagObservables := make([]observable.Observable, n)

	o := &optsConfig{peerUpdateInterval: connection.DefaultPeerUpdateInterval}
	for _, opt := range opts {
		opt(o)
	}

	identities := unittest.IdentityListFixture(n, unittest.WithAllRoles())

	for _, identity := range identities {
		for _, idOpt := range o.idOpts {
			idOpt(identity)
		}
	}

	idProvider := id.NewFixedIdentityProvider(identities)

	// generates keys and address for the node
	for i, id := range identities {
		// generate key
		key, err := generateNetworkingKey(id.NodeID)
		require.NoError(t, err)

		var opts []nodeBuilderOption

		opts = append(opts, withDHT(o.dhtPrefix, o.dhtOpts...))
		opts = append(opts, withPeerManagerOptions(connection.ConnectionPruningEnabled, o.peerUpdateInterval))

		libP2PNodes[i], tagObservables[i] = generateLibP2PNode(t, logger, key, idProvider, opts...)

		_, port, err := libP2PNodes[i].GetIPPort()
		require.NoError(t, err)

		identities[i].Address = unittest.IPPort(port)
		identities[i].NetworkPubKey = key.PublicKey()
	}

	return identities, libP2PNodes, tagObservables
}

// GenerateMiddlewares creates and initializes middleware instances for all the identities
func GenerateMiddlewares(t *testing.T,
	logger zerolog.Logger,
	identities flow.IdentityList,
	libP2PNodes []p2p.LibP2PNode,
	codec network.Codec,
	consumer slashing.ViolationsConsumer,
	opts ...func(*optsConfig)) ([]network.Middleware, []*UpdatableIDProvider) {
	mws := make([]network.Middleware, len(identities))
	idProviders := make([]*UpdatableIDProvider, len(identities))
	bitswapmet := metrics.NewNoopCollector()
	o := &optsConfig{
		peerUpdateInterval:  connection.DefaultPeerUpdateInterval,
		unicastRateLimiters: ratelimit.NoopRateLimiters(),
		networkMetrics:      metrics.NewNoopCollector(),
	}

	for _, opt := range opts {
		opt(o)
	}

	total := len(identities)
	for i := 0; i < total; i++ {
		// casts libP2PNode instance to a local variable to avoid closure
		node := libP2PNodes[i]
		nodeId := identities[i].NodeID

		idProviders[i] = NewUpdatableIDProvider(identities)

		// creating middleware of nodes
		mws[i] = middleware.NewMiddleware(logger,
			node,
			nodeId,
			o.networkMetrics,
			bitswapmet,
			sporkID,
			middleware.DefaultUnicastTimeout,
			translator.NewIdentityProviderIDTranslator(idProviders[i]),
			codec,
			consumer,
			middleware.WithUnicastRateLimiters(o.unicastRateLimiters))
	}
	return mws, idProviders
}

// GenerateNetworks generates the network for the given middlewares
func GenerateNetworks(t *testing.T,
	log zerolog.Logger,
	ids flow.IdentityList,
	mws []network.Middleware,
	sms []network.SubscriptionManager) []network.Network {
	count := len(ids)
	nets := make([]network.Network, 0)

	for i := 0; i < count; i++ {

		// creates and mocks me
		me := &mock.Local{}
		me.On("NodeID").Return(ids[i].NodeID)
		me.On("NotMeFilter").Return(filter.Not(filter.HasNodeID(me.NodeID())))
		me.On("Address").Return(ids[i].Address)

		receiveCache := netcache.NewHeroReceiveCache(p2p.DefaultReceiveCacheSize, log, metrics.NewNoopCollector())

		// create the network
		net, err := p2p.NewNetwork(&p2p.NetworkParameters{
			Logger:              log,
			Codec:               cbor.NewCodec(),
			Me:                  me,
			MiddlewareFactory:   func() (network.Middleware, error) { return mws[i], nil },
			Topology:            unittest.NetworkTopology(),
			SubscriptionManager: sms[i],
			Metrics:             metrics.NewNoopCollector(),
			IdentityProvider:    id.NewFixedIdentityProvider(ids),
			ReceiveCache:        receiveCache,
		})
		require.NoError(t, err)

		nets = append(nets, net)
	}

	return nets
}

// GenerateIDsAndMiddlewares returns nodeIDs, libp2pNodes, middlewares, and observables which can be subscirbed to in order to witness protect events from pubsub
func GenerateIDsAndMiddlewares(t *testing.T,
	n int,
	logger zerolog.Logger,
	codec network.Codec,
	consumer slashing.ViolationsConsumer,
	opts ...func(*optsConfig)) (flow.IdentityList, []p2p.LibP2PNode, []network.Middleware, []observable.Observable, []*UpdatableIDProvider) {

	ids, libP2PNodes, protectObservables := GenerateIDs(t, logger, n, opts...)
	mws, providers := GenerateMiddlewares(t, logger, ids, libP2PNodes, codec, consumer, opts...)
	return ids, libP2PNodes, mws, protectObservables, providers
}

type optsConfig struct {
	idOpts              []func(*flow.Identity)
	dhtPrefix           string
	dhtOpts             []dht.Option
	unicastRateLimiters *ratelimit.RateLimiters
	peerUpdateInterval  time.Duration
	networkMetrics      module.NetworkMetrics
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

func WithUnicastRateLimiters(limiters *ratelimit.RateLimiters) func(*optsConfig) {
	return func(o *optsConfig) {
		o.unicastRateLimiters = limiters
	}
}

func WithNetworkMetrics(m module.NetworkMetrics) func(*optsConfig) {
	return func(o *optsConfig) {
		o.networkMetrics = m
	}
}

func GenerateIDsMiddlewaresNetworks(t *testing.T,
	n int,
	log zerolog.Logger,
	codec network.Codec,
	consumer slashing.ViolationsConsumer,
	opts ...func(*optsConfig)) (flow.IdentityList, []p2p.LibP2PNode, []network.Middleware, []network.Network, []observable.Observable) {
	ids, libp2pNodes, mws, observables, _ := GenerateIDsAndMiddlewares(t, n, log, codec, consumer, opts...)
	sms := GenerateSubscriptionManagers(t, mws)
	networks := GenerateNetworks(t, log, ids, mws, sms)

	return ids, libp2pNodes, mws, networks, observables
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

// StartNodesAndNetworks starts the provided networks and libp2p nodes, returning the irrecoverable error channel
func StartNodesAndNetworks(ctx irrecoverable.SignalerContext, t *testing.T, nodes []p2p.LibP2PNode, nets []network.Network, duration time.Duration) {
	// start up networks (this will implicitly start middlewares)
	for _, net := range nets {
		net.Start(ctx)
		unittest.RequireComponentsReadyBefore(t, duration, net)
	}

	// start up nodes and Peer managers
	StartNodes(ctx, t, nodes, duration)
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

type nodeBuilderOption func(p2pbuilder.NodeBuilder)

func withDHT(prefix string, dhtOpts ...dht.Option) nodeBuilderOption {
	return func(nb p2pbuilder.NodeBuilder) {
		nb.SetRoutingSystem(func(c context.Context, h host.Host) (routing.Routing, error) {
			return p2pdht.NewDHT(c, h, pc.ID(unicast.FlowDHTProtocolIDPrefix+prefix), zerolog.Nop(), metrics.NewNoopCollector(), dhtOpts...)
		})
	}
}

func withPeerManagerOptions(connectionPruning bool, updateInterval time.Duration) nodeBuilderOption {
	return func(nb p2pbuilder.NodeBuilder) {
		nb.SetPeerManagerOptions(connectionPruning, updateInterval)
	}
}

// generateLibP2PNode generates a `LibP2PNode` on localhost using a port assigned by the OS
func generateLibP2PNode(t *testing.T,
	logger zerolog.Logger,
	key crypto.PrivateKey,
	idProvider module.IdentityProvider,
	opts ...nodeBuilderOption) (p2p.LibP2PNode, observable.Observable) {

	noopMetrics := metrics.NewNoopCollector()

	// Inject some logic to be able to observe connections of this node
	connManager := NewTagWatchingConnManager(logger, idProvider, noopMetrics)

	builder := p2pbuilder.NewNodeBuilder(logger, metrics.NewNoopCollector(), unittest.DefaultAddress, key, sporkID).
		SetConnectionManager(connManager).
		SetResourceManager(NewResourceManager(t))

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
	seed := make([]byte, crypto.KeyGenSeedMinLenECDSASecp256k1)
	copy(seed, s[:])
	return crypto.GeneratePrivateKey(crypto.ECDSASecp256k1, seed)
}

// GenerateSubscriptionManagers creates and returns a ChannelSubscriptionManager for each middleware object.
func GenerateSubscriptionManagers(t *testing.T, mws []network.Middleware) []network.SubscriptionManager {
	require.NotEmpty(t, mws)

	sms := make([]network.SubscriptionManager, len(mws))
	for i, mw := range mws {
		sms[i] = subscription.NewChannelSubscriptionManager(mw)
	}
	return sms
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
	return p2pNetwork.NullResourceManager
}

// NewConnectionGater creates a new connection gater for testing with given allow listing filter.
func NewConnectionGater(allowListFilter p2p.PeerFilter) connmgr.ConnectionGater {
	filters := []p2p.PeerFilter{allowListFilter}
	return connection.NewConnGater(unittest.Logger(),
		connection.WithOnInterceptPeerDialFilters(filters),
		connection.WithOnInterceptSecuredFilters(filters))
}
