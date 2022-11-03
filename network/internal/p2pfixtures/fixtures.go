package p2pfixtures

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	addrutil "github.com/libp2p/go-addr-util"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/connmgr"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-libp2p/core/routing"
	"github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	flownet "github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/internal/p2putils"
	"github.com/onflow/flow-go/network/internal/testutils"
	"github.com/onflow/flow-go/network/message"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/connection"
	p2pdht "github.com/onflow/flow-go/network/p2p/dht"
	"github.com/onflow/flow-go/network/p2p/keyutils"
	"github.com/onflow/flow-go/network/p2p/p2pbuilder"
	validator "github.com/onflow/flow-go/network/validator/pubsub"

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
}

// AddNodesToEachOthersPeerStore adds the dialing address of all nodes to the peer store of all other nodes.
// However, it does not connect them to each other.
func AddNodesToEachOthersPeerStore(t *testing.T, nodes []p2p.LibP2PNode, ids flow.IdentityList) {
	for _, node := range nodes {
		for i, other := range nodes {
			if node == other {
				continue
			}
			otherPInfo, err := utils.PeerAddressInfo(*ids[i])
			require.NoError(t, err)
			node.Host().Peerstore().AddAddrs(otherPInfo.ID, otherPInfo.Addrs, peerstore.AddressTTL)
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
		}
	}
}

// EnsureNotConnected ensures that no connection exists from "from" nodes to "to" nodes.
func EnsureNotConnected(t *testing.T, ctx context.Context, from []p2p.LibP2PNode, to []p2p.LibP2PNode) {
	for _, node := range from {
		for _, other := range to {
			if node == other {
				require.Fail(t, "overlapping nodes in from and to lists")
			}
			require.Error(t, node.Host().Connect(ctx, other.Host().Peerstore().PeerInfo(other.Host().ID())))
		}
	}
}

// EnsureNotConnectedBetweenGroups ensures no connection exists between the given groups of nodes.
func EnsureNotConnectedBetweenGroups(t *testing.T, ctx context.Context, groupA []p2p.LibP2PNode, groupB []p2p.LibP2PNode) {
	// ensure no connection from group A to group B
	EnsureNotConnected(t, ctx, groupA, groupB)
	// ensure no connection from group B to group A
	EnsureNotConnected(t, ctx, groupB, groupA)
}

// EnsurePubsubMessageExchange ensures that the given nodes exchange the given message on the given channel through pubsub.
func EnsurePubsubMessageExchange(t *testing.T, ctx context.Context, nodes []p2p.LibP2PNode, messageFactory func() (interface{}, channels.Topic)) {
	_, topic := messageFactory()

	subs := make([]*pubsub.Subscription, len(nodes))
	slashingViolationsConsumer := unittest.NetworkSlashingViolationsConsumer(unittest.Logger(), metrics.NewNoopCollector())
	var err error
	for i, node := range nodes {
		subs[i], err = node.Subscribe(
			topic,
			validator.TopicValidator(
				unittest.Logger(),
				unittest.NetworkCodec(),
				slashingViolationsConsumer,
				unittest.AllowAllPeerFilter()))
		require.NoError(t, err)
	}

	// let subscriptions propagate
	time.Sleep(1 * time.Second)

	channel, ok := channels.ChannelFromTopic(topic)
	require.True(t, ok)

	for _, node := range nodes {
		// creates a unique message to be published by the node
		msg, _ := messageFactory()
		data := MustEncodeEvent(t, msg, channel)
		require.NoError(t, node.Publish(ctx, topic, data))

		// wait for the message to be received by all nodes
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		SubsMustReceiveMessage(t, ctx, data, subs)
		cancel()
	}
}

// EnsureNoPubsubMessageExchange ensures that the no pubsub message is exchanged "from" the given nodes "to" the given nodes.
func EnsureNoPubsubMessageExchange(t *testing.T, ctx context.Context, from []p2p.LibP2PNode, to []p2p.LibP2PNode, messageFactory func() (interface{}, channels.Topic)) {
	_, topic := messageFactory()

	subs := make([]*pubsub.Subscription, len(to))
	svc := unittest.NetworkSlashingViolationsConsumer(unittest.Logger(), metrics.NewNoopCollector())
	tv := validator.TopicValidator(
		unittest.Logger(),
		unittest.NetworkCodec(),
		svc,
		unittest.AllowAllPeerFilter())
	var err error
	for _, node := range from {
		_, err = node.Subscribe(topic, tv)
		require.NoError(t, err)
	}

	for i, node := range to {
		subs[i], err = node.Subscribe(topic, tv)
		require.NoError(t, err)
	}

	// let subscriptions propagate
	time.Sleep(1 * time.Second)

	for _, node := range from {
		// creates a unique message to be published by the node.
		msg, _ := messageFactory()
		channel, ok := channels.ChannelFromTopic(topic)
		require.True(t, ok)
		data := MustEncodeEvent(t, msg, channel)

		// ensure the message is NOT received by any of the nodes.
		require.NoError(t, node.Publish(ctx, topic, data))
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		SubsMustNeverReceiveAnyMessage(t, ctx, subs)
		cancel()
	}
}

// EnsureNoPubsubExchangeBetweenGroups ensures that no pubsub message is exchanged between the given groups of nodes.
func EnsureNoPubsubExchangeBetweenGroups(t *testing.T, ctx context.Context, groupA []p2p.LibP2PNode, groupB []p2p.LibP2PNode, messageFactory func() (interface{}, channels.Topic)) {
	// ensure no message exchange from group A to group B
	EnsureNoPubsubMessageExchange(t, ctx, groupA, groupB, messageFactory)
	// ensure no message exchange from group B to group A
	EnsureNoPubsubMessageExchange(t, ctx, groupB, groupA, messageFactory)
}

// EnsureMessageExchangeOverUnicast ensures that the given nodes exchange arbitrary messages on through unicasting (i.e., stream creation).
// It fails the test if any of the nodes does not receive the message from the other nodes.
// The "inbounds" parameter specifies the inbound channel of the nodes on which the messages are received.
// The "messageFactory" parameter specifies the function that creates unique messages to be sent.
func EnsureMessageExchangeOverUnicast(t *testing.T, ctx context.Context, nodes []p2p.LibP2PNode, inbounds []chan string, messageFactory func() string) {
	for _, this := range nodes {
		msg := messageFactory()

		// send the message to all other nodes
		for _, other := range nodes {
			if this == other {
				continue
			}
			s, err := this.CreateStream(ctx, other.Host().ID())
			require.NoError(t, err)
			rw := bufio.NewReadWriter(bufio.NewReader(s), bufio.NewWriter(s))
			_, err = rw.WriteString(msg)
			require.NoError(t, err)

			// Flush the stream
			require.NoError(t, rw.Flush())
		}

		// wait for the message to be received by all other nodes
		for i, other := range nodes {
			if this == other {
				continue
			}

			select {
			case rcv := <-inbounds[i]:
				require.Equal(t, msg, rcv)
			case <-time.After(3 * time.Second):
				require.Fail(t, fmt.Sprintf("did not receive message from node %d", i))
			}
		}
	}
}

// EnsureNoStreamCreationBetweenGroups ensures that no stream is created between the given groups of nodes.
func EnsureNoStreamCreationBetweenGroups(t *testing.T, ctx context.Context, groupA []p2p.LibP2PNode, groupB []p2p.LibP2PNode, errorCheckers ...func(*testing.T, error)) {
	// no stream from groupA -> groupB
	EnsureNoStreamCreation(t, ctx, groupA, groupB, errorCheckers...)
	// no stream from groupB -> groupA
	EnsureNoStreamCreation(t, ctx, groupB, groupA, errorCheckers...)
}

// EnsureNoStreamCreation ensures that no stream is created "from" the given nodes "to" the given nodes.
func EnsureNoStreamCreation(t *testing.T, ctx context.Context, from []p2p.LibP2PNode, to []p2p.LibP2PNode, errorCheckers ...func(*testing.T, error)) {
	for _, this := range from {
		for _, other := range to {
			if this == other {
				// should not happen, unless the test is misconfigured.
				require.Fail(t, "node is in both from and to lists")
			}
			// stream creation should fail
			_, err := this.CreateStream(ctx, other.Host().ID())
			require.Error(t, err)
			require.True(t, flownet.IsPeerUnreachableError(err))

			// runs the error checkers if any.
			for _, check := range errorCheckers {
				check(t, err)
			}
		}
	}
}

// EnsureStreamCreation ensures that a stream is created between each of the  "from" nodes to each of the "to" nodes.
func EnsureStreamCreation(t *testing.T, ctx context.Context, from []p2p.LibP2PNode, to []p2p.LibP2PNode) {
	for _, this := range from {
		for _, other := range to {
			if this == other {
				// should not happen, unless the test is misconfigured.
				require.Fail(t, "node is in both from and to lists")
			}
			// stream creation should pass without error
			s, err := this.CreateStream(ctx, other.Host().ID())
			require.NoError(t, err)
			require.NotNil(t, s)
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

// LongStringMessageFactoryFixture returns a function that creates a long unique string message.
func LongStringMessageFactoryFixture(t *testing.T) func() string {
	return func() string {
		msg := "this is an intentionally long MESSAGE to be bigger than buffer size of most of stream compressors"
		require.Greater(t, len(msg), 10, "we must stress test with longer than 10 bytes messages")
		return fmt.Sprintf("%s %d \n", msg, time.Now().UnixNano()) // add timestamp to make sure we don't send the same message twice
	}
}
