package p2ptest

import (
	"bufio"
	"context"
	"testing"
	"time"

	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/connmgr"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-libp2p/core/routing"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/internal/p2pfixtures"
	"github.com/onflow/flow-go/network/internal/testutils"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/connection"
	p2pdht "github.com/onflow/flow-go/network/p2p/dht"
	"github.com/onflow/flow-go/network/p2p/p2pbuilder"
	"github.com/onflow/flow-go/network/p2p/scoring"
	"github.com/onflow/flow-go/network/p2p/unicast"
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
	opts ...NodeFixtureParameterOption,
) (p2p.LibP2PNode, flow.Identity) {
	// default parameters
	parameters := &NodeFixtureParameters{
		HandlerFunc: func(network.Stream) {},
		Unicasts:    nil,
		Key:         NetworkingKeyFixtures(t),
		Address:     unittest.DefaultAddress,
		Logger:      unittest.Logger().Level(zerolog.ErrorLevel),
		Role:        flow.RoleCollection,
	}

	for _, opt := range opts {
		opt(parameters)
	}

	identity := unittest.IdentityFixture(
		unittest.WithNetworkingKey(parameters.Key.PublicKey()),
		unittest.WithAddress(parameters.Address),
		unittest.WithRole(parameters.Role))

	logger := parameters.Logger.With().Hex("node_id", logging.ID(identity.NodeID)).Logger()

	noopMetrics := metrics.NewNoopCollector()
	connManager := connection.NewConnManager(logger, noopMetrics)
	resourceManager := testutils.NewResourceManager(t)

	builder := p2pbuilder.NewNodeBuilder(
		logger,
		metrics.NewNoopCollector(),
		parameters.Address,
		parameters.Key,
		sporkID,
		p2pbuilder.DefaultResourceManagerConfig()).
		SetConnectionManager(connManager).
		SetRoutingSystem(func(c context.Context, h host.Host) (routing.Routing, error) {
			return p2pdht.NewDHT(c, h,
				protocol.ID(unicast.FlowDHTProtocolIDPrefix+sporkID.String()+"/"+dhtPrefix),
				logger,
				noopMetrics,
				parameters.DhtOptions...,
			)
		}).
		SetResourceManager(resourceManager).
		SetCreateNode(p2pbuilder.DefaultCreateNodeFunc)

	if parameters.ConnGater != nil {
		builder.SetConnectionGater(parameters.ConnGater)
	}

	if parameters.PeerScoringEnabled {
		scoreOptionParams := make([]scoring.PeerScoreParamsOption, 0)
		if parameters.AppSpecificScore != nil {
			scoreOptionParams = append(scoreOptionParams, scoring.WithAppSpecificScoreFunction(parameters.AppSpecificScore))
		}
		builder.EnableGossipSubPeerScoring(parameters.IdProvider, scoreOptionParams...)
	}

	if parameters.UpdateInterval != 0 {
		require.NotNil(t, parameters.PeerProvider)
		builder.SetPeerManagerOptions(parameters.ConnectionPruning, parameters.UpdateInterval)
	}

	if parameters.GossipSubFactory != nil && parameters.GossipSubConfig != nil {
		builder.SetGossipSubFactory(parameters.GossipSubFactory, parameters.GossipSubConfig)
	}

	n, err := builder.Build()
	require.NoError(t, err)

	err = n.WithDefaultUnicastProtocol(parameters.HandlerFunc, parameters.Unicasts)
	require.NoError(t, err)

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
	HandlerFunc        network.StreamHandler
	Unicasts           []unicast.ProtocolName
	Key                crypto.PrivateKey
	Address            string
	DhtOptions         []dht.Option
	Role               flow.Role
	Logger             zerolog.Logger
	PeerScoringEnabled bool
	IdProvider         module.IdentityProvider
	AppSpecificScore   func(peer.ID) float64 // overrides GossipSub scoring for sake of testing.
	ConnectionPruning  bool                  // peer manager parameter
	UpdateInterval     time.Duration         // peer manager parameter
	PeerProvider       p2p.PeersProvider     // peer manager parameter
	ConnGater          connmgr.ConnectionGater
	GossipSubFactory   p2pbuilder.GossipSubFactoryFunc
	GossipSubConfig    p2pbuilder.GossipSubAdapterConfigFunc
}

func WithPeerScoringEnabled(idProvider module.IdentityProvider) NodeFixtureParameterOption {
	return func(p *NodeFixtureParameters) {
		p.PeerScoringEnabled = true
		p.IdProvider = idProvider
	}
}

func WithDefaultStreamHandler(handler network.StreamHandler) NodeFixtureParameterOption {
	return func(p *NodeFixtureParameters) {
		p.HandlerFunc = handler
	}
}

func WithPeerManagerEnabled(connectionPruning bool, updateInterval time.Duration, peerProvider p2p.PeersProvider) NodeFixtureParameterOption {
	return func(p *NodeFixtureParameters) {
		p.ConnectionPruning = connectionPruning
		p.UpdateInterval = updateInterval
		p.PeerProvider = peerProvider
	}
}

func WithPreferredUnicasts(unicasts []unicast.ProtocolName) NodeFixtureParameterOption {
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

func WithConnectionGater(connGater connmgr.ConnectionGater) NodeFixtureParameterOption {
	return func(p *NodeFixtureParameters) {
		p.ConnGater = connGater
	}
}

func WithRole(role flow.Role) NodeFixtureParameterOption {
	return func(p *NodeFixtureParameters) {
		p.Role = role
	}
}

func WithAppSpecificScore(score func(peer.ID) float64) NodeFixtureParameterOption {
	return func(p *NodeFixtureParameters) {
		p.AppSpecificScore = score
	}
}

func WithLogger(logger zerolog.Logger) NodeFixtureParameterOption {
	return func(p *NodeFixtureParameters) {
		p.Logger = logger
	}
}

// NodesFixture is a test fixture that creates a number of libp2p nodes with the given callback function for stream handling.
// It returns the nodes and their identities.
func NodesFixture(t *testing.T, sporkID flow.Identifier, dhtPrefix string, count int, opts ...NodeFixtureParameterOption) ([]p2p.LibP2PNode,
	flow.IdentityList) {
	var nodes []p2p.LibP2PNode

	// creating nodes
	var identities flow.IdentityList
	for i := 0; i < count; i++ {
		// create a node on localhost with a random port assigned by the OS
		node, identity := NodeFixture(t, sporkID, dhtPrefix, opts...)
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

		if peerManager := node.PeerManagerComponent(); peerManager != (*connection.PeerManager)(nil) {
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

// EnsureConnected ensures that the given nodes are connected to each other.
// It fails the test if any of the nodes is not connected to any other node.
func EnsureConnected(t *testing.T, ctx context.Context, nodes []p2p.LibP2PNode) {
	for _, node := range nodes {
		for _, other := range nodes {
			if node == other {
				continue
			}
			require.NoError(t, node.Host().Connect(ctx, other.Host().Peerstore().PeerInfo(other.Host().ID())))
			require.Equal(t, node.Host().Network().Connectedness(other.Host().ID()), network.Connected)
		}
	}
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
// Note: EnsureConnected() must be called to connect all nodes before calling this function.
func EnsurePubsubMessageExchange(t *testing.T, ctx context.Context, nodes []p2p.LibP2PNode, messageFactory func() (interface{}, channels.Topic)) {
	_, topic := messageFactory()

	subs := make([]p2p.Subscription, len(nodes))
	for i, node := range nodes {
		ps, err := node.Subscribe(
			topic,
			validator.TopicValidator(
				unittest.Logger(),
				unittest.AllowAllPeerFilter()))
		require.NoError(t, err)
		subs[i] = ps
	}

	// let subscriptions propagate
	time.Sleep(1 * time.Second)

	channel, ok := channels.ChannelFromTopic(topic)
	require.True(t, ok)

	for _, node := range nodes {
		// creates a unique message to be published by the node
		msg, _ := messageFactory()
		data := p2pfixtures.MustEncodeEvent(t, msg, channel)
		require.NoError(t, node.Publish(ctx, topic, data))

		// wait for the message to be received by all nodes
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		p2pfixtures.SubsMustReceiveMessage(t, ctx, data, subs)
		cancel()
	}
}
