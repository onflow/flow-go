package p2ptest

import (
	"bufio"
	"context"
	"crypto/rand"
	crand "math/rand"
	"sync"
	"testing"
	"time"

	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/connmgr"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-libp2p/core/routing"
	discoveryBackoff "github.com/libp2p/go-libp2p/p2p/discovery/backoff"
	mh "github.com/multiformats/go-multihash"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/config"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	flownet "github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/internal/p2pfixtures"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/connection"
	p2pdht "github.com/onflow/flow-go/network/p2p/dht"
	"github.com/onflow/flow-go/network/p2p/p2pbuilder"
	p2pconfig "github.com/onflow/flow-go/network/p2p/p2pbuilder/config"
	"github.com/onflow/flow-go/network/p2p/p2pconf"
	"github.com/onflow/flow-go/network/p2p/unicast"
	"github.com/onflow/flow-go/network/p2p/unicast/protocols"
	"github.com/onflow/flow-go/network/p2p/utils"
	validator "github.com/onflow/flow-go/network/validator/pubsub"
	"github.com/onflow/flow-go/utils/logging"
	"github.com/onflow/flow-go/utils/unittest"
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
func NodeFixture(
	t *testing.T,
	sporkID flow.Identifier,
	dhtPrefix string,
	idProvider module.IdentityProvider,
	opts ...NodeFixtureParameterOption,
) (p2p.LibP2PNode, flow.Identity) {
	defaultFlowConfig, err := config.DefaultConfig()
	require.NoError(t, err)

	logger := unittest.Logger().Level(zerolog.WarnLevel)
	require.NotNil(t, idProvider)
	connectionGater := NewConnectionGater(idProvider, func(p peer.ID) error {
		return nil
	})
	require.NotNil(t, connectionGater)
	parameters := &NodeFixtureParameters{
		NetworkingType:         flownet.PrivateNetwork,
		HandlerFunc:            func(network.Stream) {},
		Unicasts:               nil,
		Key:                    NetworkingKeyFixtures(t),
		Address:                unittest.DefaultAddress,
		Logger:                 logger,
		Role:                   flow.RoleCollection,
		CreateStreamRetryDelay: unicast.DefaultRetryDelay,
		IdProvider:             idProvider,
		MetricsCfg: &p2pconfig.MetricsConfig{
			HeroCacheFactory: metrics.NewNoopHeroCacheMetricsFactory(),
			Metrics:          metrics.NewNoopCollector(),
		},
		ResourceManager:                  &network.NullResourceManager{},
		GossipSubPeerScoreTracerInterval: 0, // disabled by default
		ConnGater:                        connectionGater,
		PeerManagerConfig:                PeerManagerConfigFixture(), // disabled by default
		GossipSubRPCInspectorCfg:         &defaultFlowConfig.NetworkConfig.GossipSubRPCInspectorsConfig,
	}

	for _, opt := range opts {
		opt(parameters)
	}

	identity := unittest.IdentityFixture(
		unittest.WithNetworkingKey(parameters.Key.PublicKey()),
		unittest.WithAddress(parameters.Address),
		unittest.WithRole(parameters.Role))

	logger = parameters.Logger.With().Hex("node_id", logging.ID(identity.NodeID)).Logger()

	connManager, err := connection.NewConnManager(logger, parameters.MetricsCfg.Metrics, &defaultFlowConfig.NetworkConfig.ConnectionManagerConfig)
	require.NoError(t, err)

	builder := p2pbuilder.NewNodeBuilder(
		logger,
		parameters.MetricsCfg,
		parameters.NetworkingType,
		parameters.Address,
		parameters.Key,
		sporkID,
		parameters.IdProvider,
		&defaultFlowConfig.NetworkConfig.ResourceManagerConfig,
		parameters.GossipSubRPCInspectorCfg,
		parameters.PeerManagerConfig,
		&p2p.DisallowListCacheConfig{
			MaxSize: uint32(1000),
			Metrics: metrics.NewNoopCollector(),
		}).
		SetConnectionManager(connManager).
		SetRoutingSystem(func(c context.Context, h host.Host) (routing.Routing, error) {
			return p2pdht.NewDHT(c, h,
				protocol.ID(protocols.FlowDHTProtocolIDPrefix+sporkID.String()+"/"+dhtPrefix),
				logger,
				parameters.MetricsCfg.Metrics,
				parameters.DhtOptions...,
			)
		}).
		SetCreateNode(p2pbuilder.DefaultCreateNodeFunc).
		SetStreamCreationRetryInterval(parameters.CreateStreamRetryDelay).
		SetResourceManager(parameters.ResourceManager)

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

	if parameters.UnicastRateLimitDistributor != nil {
		builder.SetRateLimiterDistributor(parameters.UnicastRateLimitDistributor)
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
	CreateStreamRetryDelay            time.Duration
	UnicastRateLimitDistributor       p2p.UnicastRateLimiterDistributor
	GossipSubRpcInspectorSuiteFactory p2p.GossipSubRpcInspectorSuiteFactoryFunc
	GossipSubRPCInspectorCfg          *p2pconf.GossipSubRPCInspectorsConfig
}

func WithUnicastRateLimitDistributor(distributor p2p.UnicastRateLimiterDistributor) NodeFixtureParameterOption {
	return func(p *NodeFixtureParameters) {
		p.UnicastRateLimitDistributor = distributor
	}
}

func OverrideGossipSubRpcInspectorSuiteFactory(factory p2p.GossipSubRpcInspectorSuiteFactoryFunc) NodeFixtureParameterOption {
	return func(p *NodeFixtureParameters) {
		p.GossipSubRpcInspectorSuiteFactory = factory
	}
}

func OverrideGossipSubRpcInspectorConfig(cfg *p2pconf.GossipSubRPCInspectorsConfig) NodeFixtureParameterOption {
	return func(p *NodeFixtureParameters) {
		p.GossipSubRPCInspectorCfg = cfg
	}
}

func WithCreateStreamRetryDelay(delay time.Duration) NodeFixtureParameterOption {
	return func(p *NodeFixtureParameters) {
		p.CreateStreamRetryDelay = delay
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
			backoff := discoveryBackoff.NewExponentialBackoff(
				1*time.Second,
				1*time.Hour,
				func(_, _, _ time.Duration, _ *crand.Rand) time.Duration {
					return 0 // no jitter
				},
				time.Second,
				1,
				0,
				crand.NewSource(crand.Int63()),
			)
			backoffConnector, err := discoveryBackoff.NewBackoffConnector(host, cacheSize, dialTimeout, backoff)
			require.NoError(t, err)
			return backoffConnector, nil
		}
	}
}

// NodesFixture is a test fixture that creates a number of libp2p nodes with the given callback function for stream handling.
// It returns the nodes and their identities.
func NodesFixture(t *testing.T, sporkID flow.Identifier, dhtPrefix string, count int, idProvider module.IdentityProvider, opts ...NodeFixtureParameterOption) ([]p2p.LibP2PNode,
	flow.IdentityList) {
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
func StartNodes(t *testing.T, ctx irrecoverable.SignalerContext, nodes []p2p.LibP2PNode, timeout time.Duration) {
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
	unittest.RequireComponentsReadyBefore(t, timeout, rdas...)
}

// StartNode start a single node using the provided context, timing out if nodes are not all Ready()
// before duration expires
func StartNode(t *testing.T, ctx irrecoverable.SignalerContext, node p2p.LibP2PNode, timeout time.Duration) {
	node.Start(ctx)
	unittest.RequireComponentsReadyBefore(t, timeout, node)
}

// StopNodes stops all nodes in the input slice using the provided cancel func, timing out if nodes are
// not all Done() before duration expires
func StopNodes(t *testing.T, nodes []p2p.LibP2PNode, cancel context.CancelFunc, timeout time.Duration) {
	cancel()
	for _, node := range nodes {
		unittest.RequireComponentsDoneBefore(t, timeout, node)
	}
}

// StopNode stops a single node using the provided cancel func, timing out if nodes are not all Done()
// before duration expires
func StopNode(t *testing.T, node p2p.LibP2PNode, cancel context.CancelFunc, timeout time.Duration) {
	cancel()
	unittest.RequireComponentsDoneBefore(t, timeout, node)
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
			require.NoError(t, node.AddPeer(ctx, otherPInfo))
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
			require.NoError(t, node.Host().Connect(ctx, other.Host().Peerstore().PeerInfo(other.Host().ID())))
			// the other node should be connected to this node
			require.Equal(t, node.Host().Network().Connectedness(other.Host().ID()), network.Connected)
			// at least one connection should be established
			require.True(t, len(node.Host().Network().ConnsToPeer(other.Host().ID())) > 0)
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
				if node.Host().Network().Connectedness(other.Host().ID()) != network.Connected {
					return false
				}
				if len(node.Host().Network().ConnsToPeer(other.Host().ID())) == 0 {
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
				if node.Host().Network().Connectedness(other.Host().ID()) == network.Connected {
					return false
				}
				if len(node.Host().Network().ConnsToPeer(other.Host().ID())) > 0 {
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
			s, err := this.CreateStream(ctx, other.Host().ID())
			require.NoError(t, err)
			require.NotNil(t, s)
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

	channel, ok := channels.ChannelFromTopic(topic)
	require.True(t, ok)

	for _, node := range nodes {
		for i := 0; i < count; i++ {
			// creates a unique message to be published by the node
			msg := messageFactory()
			data := p2pfixtures.MustEncodeEvent(t, msg, channel)
			require.NoError(t, node.Publish(ctx, topic, data))

			// wait for the message to be received by all nodes
			ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
			p2pfixtures.SubsMustReceiveMessage(t, ctx, data, subs)
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
// - receiver: the node that receives the message from the other node.
// - topic: the topic to exchange messages on.
// - count: the number of messages to exchange from `sender` to `receiver`.
// - messageFactory: a function that creates a unique message to be published by the node.
func EnsurePubsubMessageExchangeFromNode(t *testing.T, ctx context.Context, sender p2p.LibP2PNode, receiver p2p.LibP2PNode, topic channels.Topic, count int, messageFactory func() interface{}) {
	_, err := sender.Subscribe(topic, validator.TopicValidator(unittest.Logger(), unittest.AllowAllPeerFilter()))
	require.NoError(t, err)

	toSub, err := receiver.Subscribe(topic, validator.TopicValidator(unittest.Logger(), unittest.AllowAllPeerFilter()))
	require.NoError(t, err)

	// let subscriptions propagate
	time.Sleep(1 * time.Second)

	channel, ok := channels.ChannelFromTopic(topic)
	require.True(t, ok)

	for i := 0; i < count; i++ {
		// creates a unique message to be published by the node
		msg := messageFactory()
		data := p2pfixtures.MustEncodeEvent(t, msg, channel)
		require.NoError(t, sender.Publish(ctx, topic, data))

		// wait for the message to be received by all nodes
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		p2pfixtures.SubsMustReceiveMessage(t, ctx, data, []p2p.Subscription{toSub})
		cancel()
	}
}

// PeerIdFixture returns a random peer ID for testing.
// peer ID is the identifier of a node on the libp2p network.
func PeerIdFixture(t *testing.T) peer.ID {
	buf := make([]byte, 16)
	n, err := rand.Read(buf)
	require.NoError(t, err)
	require.Equal(t, 16, n)
	h, err := mh.Sum(buf, mh.SHA2_256, -1)
	require.NoError(t, err)

	return peer.ID(h)
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
func EnsureNoPubsubMessageExchange(t *testing.T, ctx context.Context, from []p2p.LibP2PNode, to []p2p.LibP2PNode, topic channels.Topic, count int, messageFactory func() interface{}) {
	subs := make([]p2p.Subscription, len(to))
	tv := validator.TopicValidator(
		unittest.Logger(),
		unittest.AllowAllPeerFilter())
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
				msg := messageFactory()
				channel, ok := channels.ChannelFromTopic(topic)
				require.True(t, ok)
				data := p2pfixtures.MustEncodeEvent(t, msg, channel)

				// ensure the message is NOT received by any of the nodes.
				require.NoError(t, node.Publish(ctx, topic, data))
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
// - groupA: first group of nodes- no message should be exchanged from any node of this group to the other group.
// - groupB: second group of nodes- no message should be exchanged from any node of this group to the other group.
// - topic: pubsub topic- no message should be exchanged on this topic.
// - count: number of messages to be exchanged- no message should be exchanged.
// - messageFactory: function to create a unique message to be published by the node.
func EnsureNoPubsubExchangeBetweenGroups(t *testing.T, ctx context.Context, groupA []p2p.LibP2PNode, groupB []p2p.LibP2PNode, topic channels.Topic, count int, messageFactory func() interface{}) {
	// ensure no message exchange from group A to group B
	EnsureNoPubsubMessageExchange(t, ctx, groupA, groupB, topic, count, messageFactory)
	// ensure no message exchange from group B to group A
	EnsureNoPubsubMessageExchange(t, ctx, groupB, groupA, topic, count, messageFactory)
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
		ids[i] = PeerIdFixture(t)
	}
	return ids
}

// NewConnectionGater creates a new connection gater for testing with given allow listing filter.
func NewConnectionGater(idProvider module.IdentityProvider, allowListFilter p2p.PeerFilter) p2p.ConnectionGater {
	filters := []p2p.PeerFilter{allowListFilter}
	return connection.NewConnGater(unittest.Logger(),
		idProvider,
		connection.WithOnInterceptPeerDialFilters(filters),
		connection.WithOnInterceptSecuredFilters(filters))
}
