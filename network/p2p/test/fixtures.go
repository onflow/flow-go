package p2ptest

import (
	"bufio"
	"context"
	crand "math/rand"
	"sync"
	"testing"
	"time"

	dht "github.com/libp2p/go-libp2p-kad-dht"
	pb "github.com/libp2p/go-libp2p-pubsub/pb"
	"github.com/libp2p/go-libp2p/core/connmgr"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-libp2p/core/routing"
	discoveryBackoff "github.com/libp2p/go-libp2p/p2p/discovery/backoff"
	"github.com/rs/zerolog"
	mockery "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/rand"

	"github.com/onflow/crypto"
	"github.com/onflow/flow-go/config"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	flownet "github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/internal/p2pfixtures"
	"github.com/onflow/flow-go/network/message"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/connection"
	p2pdht "github.com/onflow/flow-go/network/p2p/dht"
	mockp2p "github.com/onflow/flow-go/network/p2p/mock"
	"github.com/onflow/flow-go/network/p2p/p2pbuilder"
	p2pconfig "github.com/onflow/flow-go/network/p2p/p2pbuilder/config"
	"github.com/onflow/flow-go/network/p2p/tracer"
	"github.com/onflow/flow-go/network/p2p/unicast/protocols"
	"github.com/onflow/flow-go/network/p2p/utils"
	validator "github.com/onflow/flow-go/network/validator/pubsub"
	"github.com/onflow/flow-go/utils/logging"
	"github.com/onflow/flow-go/utils/unittest"
)

const (
	// libp2pNodeStartupTimeout is the timeout for starting a libp2p node in tests. Note that the
	// timeout has been selected to be large enough to allow for the node to start up on a CI even when
	// the test is run in parallel with other tests. Hence, no further increase of the timeout is
	// expected to be necessary. Any failure to start a node within this timeout is likely to be
	// caused by a bug in the code.
	libp2pNodeStartupTimeout = 10 * time.Second
	// libp2pNodeStartupTimeout is the timeout for starting a libp2p node in tests. Note that the
	// timeout has been selected to be large enough to allow for the node to start up on a CI even when
	// the test is run in parallel with other tests. Hence, no further increase of the timeout is
	// expected to be necessary. Any failure to start a node within this timeout is likely to be
	// caused by a bug in the code.
	libp2pNodeShutdownTimeout = 10 * time.Second

	// topicIDFixtureLen is the length of the topic ID fixture for testing.
	topicIDFixtureLen = 10
	// messageIDFixtureLen is the length of the message ID fixture for testing.
	messageIDFixtureLen = 10
)

// NetworkingKeyFixtures is a test helper that generates a ECDSA flow key pair.
func NetworkingKeyFixtures(t *testing.T) crypto.PrivateKey {
	seed := unittest.SeedFixture(48)
	key, err := crypto.GeneratePrivateKey(crypto.ECDSASecp256k1, seed)
	require.NoError(t, err)
	return key
}

// NodeFixture is a test fixture that creates a single libp2p node with the given key, spork id, and options.
// It returns the node and its identity.
func NodeFixture(t *testing.T,
	sporkID flow.Identifier,
	dhtPrefix string,
	idProvider module.IdentityProvider,
	opts ...NodeFixtureParameterOption) (p2p.LibP2PNode, flow.Identity) {
	defaultFlowConfig, err := config.DefaultConfig()
	require.NoError(t, err)

	logger := unittest.Logger()
	require.NotNil(t, idProvider)
	connectionGater := NewConnectionGater(idProvider, func(p peer.ID) error {
		return nil
	})
	require.NotNil(t, connectionGater)

	meshTracerCfg := &tracer.GossipSubMeshTracerConfig{
		Logger:                             unittest.Logger(),
		Metrics:                            metrics.NewNoopCollector(),
		IDProvider:                         idProvider,
		LoggerInterval:                     time.Second,
		HeroCacheMetricsFactory:            metrics.NewNoopHeroCacheMetricsFactory(),
		RpcSentTrackerCacheSize:            defaultFlowConfig.NetworkConfig.GossipSubConfig.RPCSentTrackerCacheSize,
		RpcSentTrackerWorkerQueueCacheSize: defaultFlowConfig.NetworkConfig.GossipSubConfig.RPCSentTrackerQueueCacheSize,
		RpcSentTrackerNumOfWorkers:         defaultFlowConfig.NetworkConfig.GossipSubConfig.RpcSentTrackerNumOfWorkers,
	}

	parameters := &NodeFixtureParameters{
		NetworkingType: flownet.PrivateNetwork,
		HandlerFunc:    func(network.Stream) {},
		Unicasts:       nil,
		Key:            NetworkingKeyFixtures(t),
		Address:        unittest.DefaultAddress,
		Logger:         logger,
		Role:           flow.RoleCollection,
		IdProvider:     idProvider,
		MetricsCfg: &p2pconfig.MetricsConfig{
			HeroCacheFactory: metrics.NewNoopHeroCacheMetricsFactory(),
			Metrics:          metrics.NewNoopCollector(),
		},
		ResourceManager:                  &network.NullResourceManager{},
		GossipSubPeerScoreTracerInterval: defaultFlowConfig.NetworkConfig.GossipSubConfig.ScoreTracerInterval,
		ConnGater:                        connectionGater,
		PeerManagerConfig:                PeerManagerConfigFixture(), // disabled by default
		FlowConfig:                       defaultFlowConfig,
		PubSubTracer:                     tracer.NewGossipSubMeshTracer(meshTracerCfg),
		UnicastConfig: &p2pconfig.UnicastConfig{
			UnicastConfig: defaultFlowConfig.NetworkConfig.UnicastConfig,
		},
	}

	for _, opt := range opts {
		opt(parameters)
	}

	identity := unittest.IdentityFixture(unittest.WithNetworkingKey(parameters.Key.PublicKey()),
		unittest.WithAddress(parameters.Address),
		unittest.WithRole(parameters.Role))

	logger = parameters.Logger.With().Hex("node_id", logging.ID(identity.NodeID)).Logger()

	connManager, err := connection.NewConnManager(logger, parameters.MetricsCfg.Metrics, &parameters.FlowConfig.NetworkConfig.ConnectionManagerConfig)
	require.NoError(t, err)

	builder := p2pbuilder.NewNodeBuilder(
		logger,
		parameters.MetricsCfg,
		parameters.NetworkingType,
		parameters.Address,
		parameters.Key,
		sporkID,
		parameters.IdProvider,
		defaultFlowConfig.NetworkConfig.GossipSubConfig.GossipSubScoringRegistryConfig,
		&parameters.FlowConfig.NetworkConfig.ResourceManager,
		&parameters.FlowConfig.NetworkConfig.GossipSubConfig,
		parameters.PeerManagerConfig,
		&p2p.DisallowListCacheConfig{
			MaxSize: uint32(1000),
			Metrics: metrics.NewNoopCollector(),
		},
		parameters.PubSubTracer,
		parameters.UnicastConfig).
		SetConnectionManager(connManager).
		SetCreateNode(p2pbuilder.DefaultCreateNodeFunc).
		SetResourceManager(parameters.ResourceManager)

	if parameters.DhtOptions != nil && (parameters.Role != flow.RoleAccess && parameters.Role != flow.RoleExecution) {
		require.Fail(t, "DHT should not be enabled for non-access and non-execution nodes")
	}

	if parameters.Role == flow.RoleAccess || parameters.Role == flow.RoleExecution {
		// Only access and execution nodes need to run DHT;
		// Access nodes and execution nodes need DHT to run a blob service.
		// Moreover, access nodes run a DHT to let un-staked (public) access nodes find each other on the public network.
		builder.SetRoutingSystem(func(ctx context.Context, host host.Host) (routing.Routing, error) {
			return p2pdht.NewDHT(ctx,
				host,
				protocol.ID(protocols.FlowDHTProtocolIDPrefix+sporkID.String()+"/"+dhtPrefix),
				logger,
				parameters.MetricsCfg.Metrics,
				parameters.DhtOptions...)
		})
	}

	if parameters.GossipSubRpcInspectorSuiteFactory != nil {
		builder.OverrideDefaultRpcInspectorSuiteFactory(parameters.GossipSubRpcInspectorSuiteFactory)
	}

	if parameters.ResourceManager != nil {
		builder.SetResourceManager(parameters.ResourceManager)
	}

	if parameters.ConnGater != nil {
		builder.SetConnectionGater(parameters.ConnGater)
	}

	if parameters.PeerScoringEnabled {
		builder.EnableGossipSubScoringWithOverride(parameters.PeerScoringConfigOverride)
	}

	if parameters.GossipSubFactory != nil && parameters.GossipSubConfig != nil {
		builder.SetGossipSubFactory(parameters.GossipSubFactory, parameters.GossipSubConfig)
	}

	if parameters.ConnManager != nil {
		builder.SetConnectionManager(parameters.ConnManager)
	}

	if parameters.PubSubTracer != nil {
		builder.SetGossipSubTracer(parameters.PubSubTracer)
	}

	builder.SetGossipSubScoreTracerInterval(parameters.GossipSubPeerScoreTracerInterval)

	n, err := builder.Build()
	require.NoError(t, err)

	if parameters.HandlerFunc != nil {
		err = n.WithDefaultUnicastProtocol(parameters.HandlerFunc, parameters.Unicasts)
		require.NoError(t, err)
	}

	// get the actual IP and port that have been assigned by the subsystem
	ip, port, err := n.GetIPPort()
	require.NoError(t, err)
	identity.Address = ip + ":" + port

	if parameters.PeerProvider != nil {
		n.WithPeersProvider(parameters.PeerProvider)
	}

	return n, *identity
}

type NodeFixtureParameterOption func(*NodeFixtureParameters)

type NodeFixtureParameters struct {
	HandlerFunc                       network.StreamHandler
	NetworkingType                    flownet.NetworkingType
	Unicasts                          []protocols.ProtocolName
	UnicastConfig                     *p2pconfig.UnicastConfig
	Key                               crypto.PrivateKey
	Address                           string
	DhtOptions                        []dht.Option
	Role                              flow.Role
	Logger                            zerolog.Logger
	PeerScoringEnabled                bool
	IdProvider                        module.IdentityProvider
	PeerScoringConfigOverride         *p2p.PeerScoringConfigOverride
	PeerManagerConfig                 *p2pconfig.PeerManagerConfig
	PeerProvider                      p2p.PeersProvider // peer manager parameter
	ConnGater                         p2p.ConnectionGater
	ConnManager                       connmgr.ConnManager
	GossipSubFactory                  p2p.GossipSubFactoryFunc
	GossipSubConfig                   p2p.GossipSubAdapterConfigFunc
	MetricsCfg                        *p2pconfig.MetricsConfig
	ResourceManager                   network.ResourceManager
	PubSubTracer                      p2p.PubSubTracer
	GossipSubPeerScoreTracerInterval  time.Duration // intervals at which the peer score is updated and logged.
	GossipSubRpcInspectorSuiteFactory p2p.GossipSubRpcInspectorSuiteFactoryFunc
	FlowConfig                        *config.FlowConfig
}

func WithUnicastRateLimitDistributor(distributor p2p.UnicastRateLimiterDistributor) NodeFixtureParameterOption {
	return func(p *NodeFixtureParameters) {
		p.UnicastConfig.RateLimiterDistributor = distributor
	}
}

func OverrideGossipSubRpcInspectorSuiteFactory(factory p2p.GossipSubRpcInspectorSuiteFactoryFunc) NodeFixtureParameterOption {
	return func(p *NodeFixtureParameters) {
		p.GossipSubRpcInspectorSuiteFactory = factory
	}
}

func OverrideFlowConfig(cfg *config.FlowConfig) NodeFixtureParameterOption {
	return func(p *NodeFixtureParameters) {
		p.FlowConfig = cfg
	}
}

func WithCreateStreamRetryDelay(delay time.Duration) NodeFixtureParameterOption {
	return func(p *NodeFixtureParameters) {
		p.UnicastConfig.CreateStreamBackoffDelay = delay
	}
}

// EnablePeerScoringWithOverride enables peer scoring for the GossipSub pubsub system with the given override.
// Any existing peer scoring config attribute that is set in the override will override the default peer scoring config.
// Anything that is left to nil or zero value in the override will be ignored and the default value will be used.
// Note: it is not recommended to override the default peer scoring config in production unless you know what you are doing.
// Default Use Tip: use p2p.PeerScoringConfigNoOverride as the argument to this function to enable peer scoring without any override.
// Args:
//   - PeerScoringConfigOverride: override for the peer scoring config- Recommended to use p2p.PeerScoringConfigNoOverride for production or when
//     you don't want to override the default peer scoring config.
//
// Returns:
// - NodeFixtureParameterOption: a function that can be passed to the NodeFixture function to enable peer scoring.
func EnablePeerScoringWithOverride(override *p2p.PeerScoringConfigOverride) NodeFixtureParameterOption {
	return func(p *NodeFixtureParameters) {
		p.PeerScoringEnabled = true
		p.PeerScoringConfigOverride = override
	}
}

func WithGossipSubTracer(tracer p2p.PubSubTracer) NodeFixtureParameterOption {
	return func(p *NodeFixtureParameters) {
		p.PubSubTracer = tracer
	}
}

func WithDefaultStreamHandler(handler network.StreamHandler) NodeFixtureParameterOption {
	return func(p *NodeFixtureParameters) {
		p.HandlerFunc = handler
	}
}

func WithPeerManagerEnabled(cfg *p2pconfig.PeerManagerConfig, peerProvider p2p.PeersProvider) NodeFixtureParameterOption {
	return func(p *NodeFixtureParameters) {
		p.PeerManagerConfig = cfg
		p.PeerProvider = peerProvider
	}
}

func WithPreferredUnicasts(unicasts []protocols.ProtocolName) NodeFixtureParameterOption {
	return func(p *NodeFixtureParameters) {
		p.Unicasts = unicasts
	}
}

func WithNetworkingPrivateKey(key crypto.PrivateKey) NodeFixtureParameterOption {
	return func(p *NodeFixtureParameters) {
		p.Key = key
	}
}

func WithNetworkingAddress(address string) NodeFixtureParameterOption {
	return func(p *NodeFixtureParameters) {
		p.Address = address
	}
}

func WithDHTOptions(opts ...dht.Option) NodeFixtureParameterOption {
	return func(p *NodeFixtureParameters) {
		p.DhtOptions = opts
	}
}

func WithConnectionGater(connGater p2p.ConnectionGater) NodeFixtureParameterOption {
	return func(p *NodeFixtureParameters) {
		p.ConnGater = connGater
	}
}

func WithConnectionManager(connManager connmgr.ConnManager) NodeFixtureParameterOption {
	return func(p *NodeFixtureParameters) {
		p.ConnManager = connManager
	}
}

func WithRole(role flow.Role) NodeFixtureParameterOption {
	return func(p *NodeFixtureParameters) {
		p.Role = role
	}
}

func WithPeerScoreParamsOption(cfg *p2p.PeerScoringConfigOverride) NodeFixtureParameterOption {
	return func(p *NodeFixtureParameters) {
		p.PeerScoringConfigOverride = cfg
	}
}

func WithLogger(logger zerolog.Logger) NodeFixtureParameterOption {
	return func(p *NodeFixtureParameters) {
		p.Logger = logger
	}
}

func WithMetricsCollector(metrics module.NetworkMetrics) NodeFixtureParameterOption {
	return func(p *NodeFixtureParameters) {
		p.MetricsCfg.Metrics = metrics
	}
}

func WithPeerScoreTracerInterval(interval time.Duration) NodeFixtureParameterOption {
	return func(p *NodeFixtureParameters) {
		p.GossipSubPeerScoreTracerInterval = interval
	}
}

// WithDefaultResourceManager sets the resource manager to nil, which will cause the node to use the default resource manager.
// Otherwise, it uses the resource manager provided by the test (the infinite resource manager).
func WithDefaultResourceManager() NodeFixtureParameterOption {
	return func(p *NodeFixtureParameters) {
		p.ResourceManager = nil
	}
}

// WithResourceManager sets the resource manager to the provided resource manager.
// Otherwise, it uses the resource manager provided by the test (the infinite resource manager).
func WithResourceManager(resourceManager network.ResourceManager) NodeFixtureParameterOption {
	return func(p *NodeFixtureParameters) {
		p.ResourceManager = resourceManager
	}
}

func WithUnicastHandlerFunc(handler network.StreamHandler) NodeFixtureParameterOption {
	return func(p *NodeFixtureParameters) {
		p.HandlerFunc = handler
	}
}

// PeerManagerConfigFixture is a test fixture that sets the default config for the peer manager.
func PeerManagerConfigFixture(opts ...func(*p2pconfig.PeerManagerConfig)) *p2pconfig.PeerManagerConfig {
	cfg := &p2pconfig.PeerManagerConfig{
		ConnectionPruning: true,
		UpdateInterval:    1 * time.Second,
		ConnectorFactory:  connection.DefaultLibp2pBackoffConnectorFactory(),
	}
	for _, opt := range opts {
		opt(cfg)
	}
	return cfg
}

// WithZeroJitterAndZeroBackoff is a test fixture that sets the default config for the peer manager.
// It uses a backoff connector with zero jitter and zero backoff.
func WithZeroJitterAndZeroBackoff(t *testing.T) func(*p2pconfig.PeerManagerConfig) {
	return func(cfg *p2pconfig.PeerManagerConfig) {
		cfg.ConnectorFactory = func(host host.Host) (p2p.Connector, error) {
			cacheSize := 100
			dialTimeout := time.Minute * 2
			backoff := discoveryBackoff.NewExponentialBackoff(1*time.Second, 1*time.Hour, func(_, _, _ time.Duration, _ *crand.Rand) time.Duration {
				return 0 // no jitter
			}, time.Second, 1, 0, crand.NewSource(crand.Int63()))
			backoffConnector, err := discoveryBackoff.NewBackoffConnector(host, cacheSize, dialTimeout, backoff)
			require.NoError(t, err)
			return backoffConnector, nil
		}
	}
}

// NodesFixture is a test fixture that creates a number of libp2p nodes with the given callback function for stream handling.
// It returns the nodes and their identities.
func NodesFixture(t *testing.T,
	sporkID flow.Identifier,
	dhtPrefix string,
	count int,
	idProvider module.IdentityProvider,
	opts ...NodeFixtureParameterOption) ([]p2p.LibP2PNode, flow.IdentityList) {
	var nodes []p2p.LibP2PNode

	// creating nodes
	var identities flow.IdentityList
	for i := 0; i < count; i++ {
		// create a node on localhost with a random port assigned by the OS
		node, identity := NodeFixture(t, sporkID, dhtPrefix, idProvider, opts...)
		nodes = append(nodes, node)
		identities = append(identities, &identity)
	}

	return nodes, identities
}

// StartNodes start all nodes in the input slice using the provided context, timing out if nodes are
// not all Ready() before duration expires
func StartNodes(t *testing.T, ctx irrecoverable.SignalerContext, nodes []p2p.LibP2PNode) {
	rdas := make([]module.ReadyDoneAware, 0, len(nodes))
	for _, node := range nodes {
		node.Start(ctx)
		rdas = append(rdas, node)

		if peerManager := node.PeerManagerComponent(); peerManager != nil {
			// we need to start the peer manager post the node startup (if such component exists).
			peerManager.Start(ctx)
			rdas = append(rdas, peerManager)
		}
	}
	for _, r := range rdas {
		// Any failure to start a node within this timeout is likely to be caused by a bug in the code.
		unittest.RequireComponentsReadyBefore(t, libp2pNodeStartupTimeout, r)
	}
}

// StartNode start a single node using the provided context, timing out if nodes are not all Ready()
// before duration expires, (i.e., 2 seconds).
// Args:
// - t: testing.T- the test object.
// - ctx: context to use.
// - node: node to start.
func StartNode(t *testing.T, ctx irrecoverable.SignalerContext, node p2p.LibP2PNode) {
	node.Start(ctx)
	// Any failure to start a node within this timeout is likely to be caused by a bug in the code.
	unittest.RequireComponentsReadyBefore(t, libp2pNodeStartupTimeout, node)
}

// StopNodes stops all nodes in the input slice using the provided cancel func, timing out if nodes are
// not all Done() before duration expires (i.e., 5 seconds).
// Args:
// - t: testing.T- the test object.
// - nodes: nodes to stop.
// - cancel: cancel func, the function first cancels the context and then waits for the nodes to be done.
func StopNodes(t *testing.T, nodes []p2p.LibP2PNode, cancel context.CancelFunc) {
	cancel()
	for _, node := range nodes {
		// Any failure to start a node within this timeout is likely to be caused by a bug in the code.
		unittest.RequireComponentsDoneBefore(t, libp2pNodeShutdownTimeout, node)
	}
}

// StopNode stops a single node using the provided cancel func, timing out if nodes are not all Done()
// before duration expires, (i.e., 2 seconds).
// Args:
// - t: testing.T- the test object.
// - node: node to stop.
// - cancel: cancel func, the function first cancels the context and then waits for the nodes to be done.
func StopNode(t *testing.T, node p2p.LibP2PNode, cancel context.CancelFunc) {
	cancel()
	// Any failure to start a node within this timeout is likely to be caused by a bug in the code.
	unittest.RequireComponentsDoneBefore(t, libp2pNodeShutdownTimeout, node)
}

// StreamHandlerFixture returns a stream handler that writes the received message to the given channel.
func StreamHandlerFixture(t *testing.T) (func(s network.Stream), chan string) {
	ch := make(chan string, 1) // channel to receive messages

	return func(s network.Stream) {
		rw := bufio.NewReadWriter(bufio.NewReader(s), bufio.NewWriter(s))
		str, err := rw.ReadString('\n')
		require.NoError(t, err)
		ch <- str
	}, ch
}

// LetNodesDiscoverEachOther connects all nodes to each other on the pubsub mesh.
func LetNodesDiscoverEachOther(t *testing.T, ctx context.Context, nodes []p2p.LibP2PNode, ids flow.IdentityList) {
	for _, node := range nodes {
		for i, other := range nodes {
			if node == other {
				continue
			}
			otherPInfo, err := utils.PeerAddressInfo(*ids[i])
			require.NoError(t, err)
			require.NoError(t, node.ConnectToPeer(ctx, otherPInfo))
		}
	}
}

// TryConnectionAndEnsureConnected tries connecting nodes to each other and ensures that the given nodes are connected to each other.
// It fails the test if any of the nodes is not connected to any other node.
func TryConnectionAndEnsureConnected(t *testing.T, ctx context.Context, nodes []p2p.LibP2PNode) {
	for _, node := range nodes {
		for _, other := range nodes {
			if node == other {
				continue
			}
			require.NoError(t, node.Host().Connect(ctx, other.Host().Peerstore().PeerInfo(other.ID())))
			// the other node should be connected to this node
			require.Equal(t, node.Host().Network().Connectedness(other.ID()), network.Connected)
			// at least one connection should be established
			require.True(t, len(node.Host().Network().ConnsToPeer(other.ID())) > 0)
		}
	}
}

// RequireConnectedEventually ensures eventually that the given nodes are already connected to each other.
// It fails the test if any of the nodes is not connected to any other node.
// Args:
// - nodes: the nodes to check
// - tick: the tick duration
// - timeout: the timeout duration
func RequireConnectedEventually(t *testing.T, nodes []p2p.LibP2PNode, tick time.Duration, timeout time.Duration) {
	require.Eventually(t, func() bool {
		for _, node := range nodes {
			for _, other := range nodes {
				if node == other {
					continue
				}
				if node.Host().Network().Connectedness(other.ID()) != network.Connected {
					return false
				}
				if len(node.Host().Network().ConnsToPeer(other.ID())) == 0 {
					return false
				}
			}
		}
		return true
	}, timeout, tick)
}

// RequireEventuallyNotConnected ensures eventually that the given groups of nodes are not connected to each other.
// It fails the test if any of the nodes from groupA is connected to any of the nodes from groupB.
// Args:
// - groupA: the first group of nodes
// - groupB: the second group of nodes
// - tick: the tick duration
// - timeout: the timeout duration
func RequireEventuallyNotConnected(t *testing.T, groupA []p2p.LibP2PNode, groupB []p2p.LibP2PNode, tick time.Duration, timeout time.Duration) {
	require.Eventually(t, func() bool {
		for _, node := range groupA {
			for _, other := range groupB {
				if node.Host().Network().Connectedness(other.ID()) == network.Connected {
					return false
				}
				if len(node.Host().Network().ConnsToPeer(other.ID())) > 0 {
					return false
				}
			}
		}
		return true
	}, timeout, tick)
}

// EnsureStreamCreationInBothDirections ensure that between each pair of nodes in the given list, a stream is created in both directions.
func EnsureStreamCreationInBothDirections(t *testing.T, ctx context.Context, nodes []p2p.LibP2PNode) {
	for _, this := range nodes {
		for _, other := range nodes {
			if this == other {
				continue
			}
			// stream creation should pass without error
			err := this.OpenProtectedStream(ctx, other.ID(), t.Name(), func(stream network.Stream) error {
				// do nothing
				require.NotNil(t, stream)
				return nil
			})
			require.NoError(t, err)

		}
	}
}

// EnsurePubsubMessageExchange ensures that the given connected nodes exchange the given message on the given channel through pubsub.
// Args:
//   - nodes: the nodes to exchange messages
//   - ctx: the context- the test will fail if the context expires.
//   - topic: the topic to exchange messages on
//   - count: the number of messages to exchange from each node.
//   - messageFactory: a function that creates a unique message to be published by the node.
//     The function should return a different message each time it is called.
//
// Note-1: this function assumes a timeout of 5 seconds for each message to be received.
// Note-2: TryConnectionAndEnsureConnected() must be called to connect all nodes before calling this function.
func EnsurePubsubMessageExchange(t *testing.T, ctx context.Context, nodes []p2p.LibP2PNode, topic channels.Topic, count int, messageFactory func() interface{}) {
	subs := make([]p2p.Subscription, len(nodes))
	for i, node := range nodes {
		ps, err := node.Subscribe(topic, validator.TopicValidator(unittest.Logger(), unittest.AllowAllPeerFilter()))
		require.NoError(t, err)
		subs[i] = ps
	}

	// let subscriptions propagate
	time.Sleep(1 * time.Second)

	for _, node := range nodes {
		for i := 0; i < count; i++ {
			// creates a unique message to be published by the node
			payload := messageFactory()
			outgoingMessageScope, err := message.NewOutgoingScope(flow.IdentifierList{unittest.IdentifierFixture()},
				topic,
				payload,
				unittest.NetworkCodec().Encode,
				message.ProtocolTypePubSub)
			require.NoError(t, err)
			require.NoError(t, node.Publish(ctx, outgoingMessageScope))

			// wait for the message to be received by all nodes
			ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
			expectedReceivedData, err := outgoingMessageScope.Proto().Marshal()
			require.NoError(t, err)
			p2pfixtures.SubsMustReceiveMessage(t, ctx, expectedReceivedData, subs)
			cancel()
		}
	}
}

// EnsurePubsubMessageExchangeFromNode ensures that the given node exchanges the given message on the given channel through pubsub with the other nodes.
// Args:
//   - node: the node to exchange messages
//
// - ctx: the context- the test will fail if the context expires.
// - sender: the node that sends the message to the other node.
// - receiverNode: the node that receives the message from the other node.
// - receiverIdentifier: the identifier of the receiver node.
// - topic: the topic to exchange messages on.
// - count: the number of messages to exchange from `sender` to `receiver`.
// - messageFactory: a function that creates a unique message to be published by the node.
func EnsurePubsubMessageExchangeFromNode(t *testing.T,
	ctx context.Context,
	sender p2p.LibP2PNode,
	receiverNode p2p.LibP2PNode,
	receiverIdentifier flow.Identifier,
	topic channels.Topic,
	count int,
	messageFactory func() interface{}) {
	_, err := sender.Subscribe(topic, validator.TopicValidator(unittest.Logger(), unittest.AllowAllPeerFilter()))
	require.NoError(t, err)

	toSub, err := receiverNode.Subscribe(topic, validator.TopicValidator(unittest.Logger(), unittest.AllowAllPeerFilter()))
	require.NoError(t, err)

	// let subscriptions propagate
	time.Sleep(1 * time.Second)

	for i := 0; i < count; i++ {
		// creates a unique message to be published by the node
		payload := messageFactory()
		outgoingMessageScope, err := message.NewOutgoingScope(flow.IdentifierList{receiverIdentifier},
			topic,
			payload,
			unittest.NetworkCodec().Encode,
			message.ProtocolTypePubSub)
		require.NoError(t, err)
		require.NoError(t, sender.Publish(ctx, outgoingMessageScope))

		// wait for the message to be received by all nodes
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		expectedReceivedData, err := outgoingMessageScope.Proto().Marshal()
		require.NoError(t, err)
		p2pfixtures.SubsMustReceiveMessage(t, ctx, expectedReceivedData, []p2p.Subscription{toSub})
		cancel()
	}
}

// EnsureNotConnectedBetweenGroups ensures no connection exists between the given groups of nodes.
func EnsureNotConnectedBetweenGroups(t *testing.T, ctx context.Context, groupA []p2p.LibP2PNode, groupB []p2p.LibP2PNode) {
	// ensure no connection from group A to group B
	p2pfixtures.EnsureNotConnected(t, ctx, groupA, groupB)
	// ensure no connection from group B to group A
	p2pfixtures.EnsureNotConnected(t, ctx, groupB, groupA)
}

// EnsureNoPubsubMessageExchange ensures that the no pubsub message is exchanged "from" the given nodes "to" the given nodes.
// Args:
//   - from: the nodes that send messages to the other group but their message must not be received by the other group.
//
// - to: the nodes that are the target of the messages sent by the other group ("from") but must not receive any message from them.
// - topic: the topic to exchange messages on.
// - count: the number of messages to exchange from each node.
// - messageFactory: a function that creates a unique message to be published by the node.
func EnsureNoPubsubMessageExchange(t *testing.T,
	ctx context.Context,
	from []p2p.LibP2PNode,
	to []p2p.LibP2PNode,
	toIdentifiers flow.IdentifierList,
	topic channels.Topic,
	count int,
	messageFactory func() interface{}) {
	subs := make([]p2p.Subscription, len(to))
	tv := validator.TopicValidator(unittest.Logger(), unittest.AllowAllPeerFilter())
	var err error
	for _, node := range from {
		_, err = node.Subscribe(topic, tv)
		require.NoError(t, err)
	}

	for i, node := range to {
		s, err := node.Subscribe(topic, tv)
		require.NoError(t, err)
		subs[i] = s
	}

	// let subscriptions propagate
	time.Sleep(1 * time.Second)

	wg := &sync.WaitGroup{}
	for _, node := range from {
		node := node // capture range variable
		for i := 0; i < count; i++ {
			wg.Add(1)
			go func() {
				// creates a unique message to be published by the node.

				payload := messageFactory()
				outgoingMessageScope, err := message.NewOutgoingScope(toIdentifiers, topic, payload, unittest.NetworkCodec().Encode, message.ProtocolTypePubSub)
				require.NoError(t, err)
				require.NoError(t, node.Publish(ctx, outgoingMessageScope))

				ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
				p2pfixtures.SubsMustNeverReceiveAnyMessage(t, ctx, subs)
				cancel()
				wg.Done()
			}()
		}
	}

	// we wait for 5 seconds at most for the messages to be exchanged, hence we wait for a total of 6 seconds here to ensure
	// that the goroutines are done in a timely manner.
	unittest.RequireReturnsBefore(t, wg.Wait, 6*time.Second, "timed out waiting for messages to be exchanged")
}

// EnsureNoPubsubExchangeBetweenGroups ensures that no pubsub message is exchanged between the given groups of nodes.
// Args:
// - t: *testing.T instance
// - ctx: context.Context instance
// - groupANodes: first group of nodes- no message should be exchanged from any node of this group to the other group.
// - groupAIdentifiers: identifiers of the nodes in the first group.
// - groupBNodes: second group of nodes- no message should be exchanged from any node of this group to the other group.
// - groupBIdentifiers: identifiers of the nodes in the second group.
// - topic: pubsub topic- no message should be exchanged on this topic.
// - count: number of messages to be exchanged- no message should be exchanged.
// - messageFactory: function to create a unique message to be published by the node.
func EnsureNoPubsubExchangeBetweenGroups(t *testing.T,
	ctx context.Context,
	groupANodes []p2p.LibP2PNode,
	groupAIdentifiers flow.IdentifierList,
	groupBNodes []p2p.LibP2PNode,
	groupBIdentifiers flow.IdentifierList,
	topic channels.Topic,
	count int,
	messageFactory func() interface{}) {
	// ensure no message exchange from group A to group B
	EnsureNoPubsubMessageExchange(t, ctx, groupANodes, groupBNodes, groupBIdentifiers, topic, count, messageFactory)
	// ensure no message exchange from group B to group A
	EnsureNoPubsubMessageExchange(t, ctx, groupBNodes, groupANodes, groupAIdentifiers, topic, count, messageFactory)
}

// PeerIdSliceFixture returns a slice of random peer IDs for testing.
// peer ID is the identifier of a node on the libp2p network.
// Args:
// - t: *testing.T instance
// - n: number of peer IDs to generate
// Returns:
// - peer.IDSlice: slice of peer IDs
func PeerIdSliceFixture(t *testing.T, n int) peer.IDSlice {
	ids := make([]peer.ID, n)
	for i := 0; i < n; i++ {
		ids[i] = unittest.PeerIdFixture(t)
	}
	return ids
}

// NewConnectionGater creates a new connection gater for testing with given allow listing filter.
func NewConnectionGater(idProvider module.IdentityProvider, allowListFilter p2p.PeerFilter) p2p.ConnectionGater {
	filters := []p2p.PeerFilter{allowListFilter}
	return connection.NewConnGater(unittest.Logger(), idProvider, connection.WithOnInterceptPeerDialFilters(filters), connection.WithOnInterceptSecuredFilters(filters))
}

// MockInspectorNotificationDistributorReadyDoneAware mocks the Ready and Done methods of the distributor to return a channel that is already closed,
// so that the distributor is considered ready and done when the test needs.
func MockInspectorNotificationDistributorReadyDoneAware(d *mockp2p.GossipSubInspectorNotificationDistributor) {
	d.On("Start", mockery.Anything).Return().Maybe()
	d.On("Ready").Return(func() <-chan struct{} {
		ch := make(chan struct{})
		close(ch)
		return ch
	}()).Maybe()
	d.On("Done").Return(func() <-chan struct{} {
		ch := make(chan struct{})
		close(ch)
		return ch
	}()).Maybe()
}

// GossipSubRpcFixtures returns a slice of random message IDs for testing.
// Args:
// - t: *testing.T instance
// - count: number of message IDs to generate
// Returns:
// - []string: slice of message IDs.
// Note: evey other parameters that are not explicitly set are set to 10. This function suites applications that need to generate a large number of RPC messages with
// filled random data. For a better control over the generated data, use GossipSubRpcFixture.
func GossipSubRpcFixtures(t *testing.T, count int) []*pb.RPC {
	c := 10
	rpcs := make([]*pb.RPC, 0)
	for i := 0; i < count; i++ {
		rpcs = append(rpcs,
			GossipSubRpcFixture(t,
				c,
				WithPrune(c, GossipSubTopicIdFixture()),
				WithGraft(c, GossipSubTopicIdFixture()),
				WithIHave(c, c, GossipSubTopicIdFixture()),
				WithIWant(c, c)))
	}
	return rpcs
}

// GossipSubRpcFixture returns a random GossipSub RPC message. An RPC message is the GossipSub-level message that is exchanged between nodes.
// It contains individual messages, subscriptions, and control messages.
// Args:
// - t: *testing.T instance
// - msgCnt: number of messages to generate
// - opts: options to customize control messages (not having an option means no control message).
// Returns:
// - *pb.RPC: a random GossipSub RPC message
// Note: the message is not signed.
func GossipSubRpcFixture(t *testing.T, msgCnt int, opts ...GossipSubCtrlOption) *pb.RPC {
	rand.Seed(uint64(time.Now().UnixNano()))

	// creates a random number of Subscriptions
	numSubscriptions := 10
	topicIdSize := 10
	subscriptions := make([]*pb.RPC_SubOpts, numSubscriptions)
	for i := 0; i < numSubscriptions; i++ {
		subscribe := rand.Intn(2) == 1
		topicID := unittest.RandomStringFixture(t, topicIdSize)
		subscriptions[i] = &pb.RPC_SubOpts{
			Subscribe: &subscribe,
			Topicid:   &topicID,
		}
	}

	// generates random messages
	messages := make([]*pb.Message, msgCnt)
	for i := 0; i < msgCnt; i++ {
		messages[i] = GossipSubMessageFixture(t)
	}

	// Create a Control Message
	controlMessages := GossipSubCtrlFixture(opts...)

	// Create the RPC
	rpc := &pb.RPC{
		Subscriptions: subscriptions,
		Publish:       messages,
		Control:       controlMessages,
	}

	return rpc
}

type GossipSubCtrlOption func(*pb.ControlMessage)

// GossipSubCtrlFixture returns a ControlMessage with the given options.
func GossipSubCtrlFixture(opts ...GossipSubCtrlOption) *pb.ControlMessage {
	msg := &pb.ControlMessage{}
	for _, opt := range opts {
		opt(msg)
	}
	return msg
}

// WithIHave adds iHave control messages of the given size and number to the control message.
func WithIHave(msgCount, msgIDCount int, topicId string) GossipSubCtrlOption {
	return func(msg *pb.ControlMessage) {
		iHaves := make([]*pb.ControlIHave, msgCount)
		for i := 0; i < msgCount; i++ {
			iHaves[i] = &pb.ControlIHave{
				TopicID:    &topicId,
				MessageIDs: GossipSubMessageIdsFixture(msgIDCount),
			}
		}
		msg.Ihave = iHaves
	}
}

// WithIWant adds iWant control messages of the given size and number to the control message.
// The message IDs are generated randomly.
// Args:
//
//	msgCount: number of iWant messages to add.
//	msgIdsPerIWant: number of message IDs to add to each iWant message.
//
// Returns:
// A GossipSubCtrlOption that adds iWant messages to the control message.
// Example: WithIWant(2, 3) will add 2 iWant messages, each with 3 message IDs.
func WithIWant(iWantCount int, msgIdsPerIWant int) GossipSubCtrlOption {
	return func(msg *pb.ControlMessage) {
		iWants := make([]*pb.ControlIWant, iWantCount)
		for i := 0; i < iWantCount; i++ {
			iWants[i] = &pb.ControlIWant{
				MessageIDs: GossipSubMessageIdsFixture(msgIdsPerIWant),
			}
		}
		msg.Iwant = iWants
	}
}

// WithGraft adds GRAFT control messages with given topicID to the control message.
func WithGraft(msgCount int, topicId string) GossipSubCtrlOption {
	return func(msg *pb.ControlMessage) {
		grafts := make([]*pb.ControlGraft, msgCount)
		for i := 0; i < msgCount; i++ {
			grafts[i] = &pb.ControlGraft{
				TopicID: &topicId,
			}
		}
		msg.Graft = grafts
	}
}

// WithPrune adds PRUNE control messages with given topicID to the control message.
func WithPrune(msgCount int, topicId string) GossipSubCtrlOption {
	return func(msg *pb.ControlMessage) {
		prunes := make([]*pb.ControlPrune, msgCount)
		for i := 0; i < msgCount; i++ {
			prunes[i] = &pb.ControlPrune{
				TopicID: &topicId,
			}
		}
		msg.Prune = prunes
	}
}

// gossipSubMessageIdFixture returns a random gossipSub message ID.
func gossipSubMessageIdFixture() string {
	// TODO: messageID length should be a parameter.
	return unittest.GenerateRandomStringWithLen(messageIDFixtureLen)
}

// GossipSubTopicIdFixture returns a random gossipSub topic ID.
func GossipSubTopicIdFixture() string {
	// TODO: topicID length should be a parameter.
	return unittest.GenerateRandomStringWithLen(topicIDFixtureLen)
}

// GossipSubMessageIdsFixture returns a slice of random gossipSub message IDs of the given size.
func GossipSubMessageIdsFixture(count int) []string {
	msgIds := make([]string, count)
	for i := 0; i < count; i++ {
		msgIds[i] = gossipSubMessageIdFixture()
	}
	return msgIds
}

// GossipSubMessageFixture returns a random gossipSub message; this contains a single pubsub message that is exchanged between nodes.
// The message is generated randomly.
// Args:
// - t: *testing.T instance
// Returns:
// - *pb.Message: a random gossipSub message
// Note: the message is not signed.
func GossipSubMessageFixture(t *testing.T) *pb.Message {
	byteSize := 100
	topic := unittest.RandomStringFixture(t, byteSize)
	return &pb.Message{
		From:      unittest.RandomBytes(byteSize),
		Data:      unittest.RandomBytes(byteSize),
		Seqno:     unittest.RandomBytes(byteSize),
		Topic:     &topic,
		Signature: unittest.RandomBytes(byteSize),
		Key:       unittest.RandomBytes(byteSize),
	}
}
