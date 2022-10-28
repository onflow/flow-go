package p2pfixtures

import (
	"bytes"
	"context"
	"net"
	"testing"
	"time"

	addrutil "github.com/libp2p/go-addr-util"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-libp2p/core/routing"
	"github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	pubsub "github.com/yhassanzadeh13/go-libp2p-pubsub"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/internal/p2putils"
	"github.com/onflow/flow-go/network/internal/testutils"
	"github.com/onflow/flow-go/network/message"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/connection"
	p2pdht "github.com/onflow/flow-go/network/p2p/dht"
	"github.com/onflow/flow-go/network/p2p/keyutils"
	"github.com/onflow/flow-go/network/p2p/p2pbuilder"

	"github.com/onflow/flow-go/network/p2p/scoring"
	"github.com/onflow/flow-go/network/p2p/unicast"
	"github.com/onflow/flow-go/network/p2p/utils"
	"github.com/onflow/flow-go/utils/logging"
	"github.com/onflow/flow-go/utils/unittest"
)

// Creating a node fixture with defaultAddress lets libp2p runs it on an
// allocated port by OS. So after fixture created, its address would be
// "0.0.0.0:<selected-port-by-os>
const defaultAddress = "0.0.0.0:0"

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
		Address:     defaultAddress,
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

	builder := p2pbuilder.NewNodeBuilder(logger, parameters.Address, parameters.Key, sporkID).
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

	if parameters.PeerFilter != nil {
		filters := []p2p.PeerFilter{parameters.PeerFilter}
		// set parameters.peerFilter as the default peerFilter for both callbacks
		connGater := connection.NewConnGater(
			logger,
			connection.WithOnInterceptPeerDialFilters(filters),
			connection.WithOnInterceptSecuredFilters(filters))
		builder.SetConnectionGater(connGater)
	}

	if parameters.PeerScoringEnabled {
		scoreOptionParams := make([]scoring.PeerScoreParamsOption, 0)
		if parameters.AppSpecificScore != nil {
			scoreOptionParams = append(scoreOptionParams, scoring.WithAppSpecificScoreFunction(parameters.AppSpecificScore))
		}
		builder.EnableGossipSubPeerScoring(parameters.IdProvider, scoreOptionParams...)
	}

	n, err := builder.Build()
	require.NoError(t, err)

	err = n.WithDefaultUnicastProtocol(parameters.HandlerFunc, parameters.Unicasts)
	require.NoError(t, err)

	// get the actual IP and port that have been assigned by the subsystem
	ip, port, err := n.GetIPPort()
	require.NoError(t, err)
	identity.Address = ip + ":" + port
	return n, *identity
}

type NodeFixtureParameters struct {
	HandlerFunc        network.StreamHandler
	Unicasts           []unicast.ProtocolName
	Key                crypto.PrivateKey
	Address            string
	DhtOptions         []dht.Option
	PeerFilter         p2p.PeerFilter
	Role               flow.Role
	Logger             zerolog.Logger
	PeerScoringEnabled bool
	IdProvider         module.IdentityProvider
	AppSpecificScore   func(peer.ID) float64 // overrides GossipSub scoring for sake of testing.
}

type NodeFixtureParameterOption func(*NodeFixtureParameters)

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

func WithPeerFilter(filter p2p.PeerFilter) NodeFixtureParameterOption {
	return func(p *NodeFixtureParameters) {
		p.PeerFilter = filter
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

// StartNodes start all nodes in the input slice using the provided context, timing out if nodes are
// not all Ready() before duration expires
func StartNodes(t *testing.T, ctx irrecoverable.SignalerContext, nodes []p2p.LibP2PNode, timeout time.Duration) {
	rdas := make([]module.ReadyDoneAware, 0, len(nodes))
	for _, node := range nodes {
		node.Start(ctx)
		rdas = append(rdas, node)
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

// SilentNodeFixture returns a TCP listener and a node which never replies
func SilentNodeFixture(t *testing.T) (net.Listener, flow.Identity) {
	key := NetworkingKeyFixtures(t)

	lst, err := net.Listen("tcp4", ":0")
	require.NoError(t, err)

	addr, err := manet.FromNetAddr(lst.Addr())
	require.NoError(t, err)

	addrs := []multiaddr.Multiaddr{addr}
	addrs, err = addrutil.ResolveUnspecifiedAddresses(addrs, nil)
	require.NoError(t, err)

	go acceptAndHang(t, lst)

	ip, port, err := p2putils.IPPortFromMultiAddress(addrs...)
	require.NoError(t, err)

	identity := unittest.IdentityFixture(unittest.WithNetworkingKey(key.PublicKey()), unittest.WithAddress(ip+":"+port))
	return lst, *identity
}

func acceptAndHang(t *testing.T, l net.Listener) {
	conns := make([]net.Conn, 0, 10)
	for {
		c, err := l.Accept()
		if err != nil {
			break
		}
		if c != nil {
			conns = append(conns, c)
		}
	}
	for _, c := range conns {
		require.NoError(t, c.Close())
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

type nodeOpt func(p2pbuilder.NodeBuilder)

func WithSubscriptionFilter(filter pubsub.SubscriptionFilter) nodeOpt {
	return func(builder p2pbuilder.NodeBuilder) {
		builder.SetSubscriptionFilter(filter)
	}
}

func CreateNode(t *testing.T, nodeID flow.Identifier, networkKey crypto.PrivateKey, sporkID flow.Identifier, logger zerolog.Logger, opts ...nodeOpt) p2p.LibP2PNode {
	builder := p2pbuilder.NewNodeBuilder(logger, "0.0.0.0:0", networkKey, sporkID).
		SetRoutingSystem(func(c context.Context, h host.Host) (routing.Routing, error) {
			return p2pdht.NewDHT(c, h, unicast.FlowDHTProtocolID(sporkID), zerolog.Nop(), metrics.NewNoopCollector())
		}).
		SetResourceManager(testutils.NewResourceManager(t))

	for _, opt := range opts {
		opt(builder)
	}

	libp2pNode, err := builder.Build()
	require.NoError(t, err)

	return libp2pNode
}

// PeerIdFixture creates a random and unique peer ID (libp2p node ID).
func PeerIdFixture(t *testing.T) peer.ID {
	key, err := generateNetworkingKey(unittest.IdentifierFixture())
	require.NoError(t, err)

	pubKey, err := keyutils.LibP2PPublicKeyFromFlow(key.PublicKey())
	require.NoError(t, err)

	peerID, err := peer.IDFromPublicKey(pubKey)
	require.NoError(t, err)

	return peerID
}

// generateNetworkingKey generates a Flow ECDSA key using the given seed
func generateNetworkingKey(s flow.Identifier) (crypto.PrivateKey, error) {
	seed := make([]byte, crypto.KeyGenSeedMinLenECDSASecp256k1)
	copy(seed, s[:])
	return crypto.GeneratePrivateKey(crypto.ECDSASecp256k1, seed)
}

// PeerIdsFixture creates random and unique peer IDs (libp2p node IDs).
func PeerIdsFixture(t *testing.T, n int) []peer.ID {
	peerIDs := make([]peer.ID, n)
	for i := 0; i < n; i++ {
		peerIDs[i] = PeerIdFixture(t)
	}
	return peerIDs
}

// MustEncodeEvent encodes and returns the given event and fails the test if it faces any issue while encoding.
func MustEncodeEvent(t *testing.T, v interface{}, channel channels.Channel) []byte {
	bz, err := unittest.NetworkCodec().Encode(v)
	require.NoError(t, err)

	msg := message.Message{
		ChannelID: channel.String(),
		Payload:   bz,
	}
	data, err := msg.Marshal()
	require.NoError(t, err)

	return data
}

// SubMustReceiveMessage checks that the subscription have received the given message within the given timeout by the context.
func SubMustReceiveMessage(t *testing.T, ctx context.Context, expectedMessage []byte, sub *pubsub.Subscription) {
	received := make(chan struct{})
	go func() {
		msg, err := sub.Next(ctx)
		require.NoError(t, err)
		require.Equal(t, expectedMessage, msg.Data)
		close(received)
	}()

	select {
	case <-received:
		return
	case <-ctx.Done():
		require.Fail(t, "timeout on receiving expected pubsub message")
	}
}

// SubsMustReceiveMessage checks that all subscriptions receive the given message within the given timeout by the context.
func SubsMustReceiveMessage(t *testing.T, ctx context.Context, expectedMessage []byte, subs []*pubsub.Subscription) {
	for _, sub := range subs {
		SubMustReceiveMessage(t, ctx, expectedMessage, sub)
	}
}

// SubMustNeverReceiveAnyMessage checks that the subscription never receives any message within the given timeout by the context.
func SubMustNeverReceiveAnyMessage(t *testing.T, ctx context.Context, sub *pubsub.Subscription) {
	timeouted := make(chan struct{})
	go func() {
		_, err := sub.Next(ctx)
		require.Error(t, err)
		require.ErrorIs(t, err, context.DeadlineExceeded)
		close(timeouted)
	}()

	// wait for the timeout, we choose the timeout to be long enough to make sure that
	// on a happy path the timeout never happens, and short enough to make sure that
	// the test doesn't take too long in case of a failure.
	unittest.RequireCloseBefore(t, timeouted, 10*time.Second, "timeout did not happen on receiving expected pubsub message")
}

// HasSubReceivedMessage checks that the subscription have received the given message within the given timeout by the context.
// It returns true if the subscription has received the message, false otherwise.
func HasSubReceivedMessage(t *testing.T, ctx context.Context, expectedMessage []byte, sub *pubsub.Subscription) bool {
	received := make(chan struct{})
	go func() {
		msg, err := sub.Next(ctx)
		if err != nil {
			require.ErrorIs(t, err, context.DeadlineExceeded)
			return
		}
		if !bytes.Equal(expectedMessage, msg.Data) {
			return
		}
		close(received)
	}()

	select {
	case <-received:
		return true
	case <-ctx.Done():
		return false
	}
}

// SubsMustNeverReceiveAnyMessage checks that all subscriptions never receive any message within the given timeout by the context.
func SubsMustNeverReceiveAnyMessage(t *testing.T, ctx context.Context, subs []*pubsub.Subscription) {
	for _, sub := range subs {
		SubMustNeverReceiveAnyMessage(t, ctx, sub)
	}
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

	// wait for all nodes to discover each other
	time.Sleep(time.Second)
}
