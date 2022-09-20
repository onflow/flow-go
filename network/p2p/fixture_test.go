package p2p_test

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	addrutil "github.com/libp2p/go-addr-util"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-libp2p/core/routing"
	"github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	fcrypto "github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/unicast"
	"github.com/onflow/flow-go/network/test"
	"github.com/onflow/flow-go/utils/unittest"
)

// Workaround for https://github.com/stretchr/testify/pull/808
const ticksForAssertEventually = 10 * time.Millisecond

// Creating a node fixture with defaultAddress lets libp2p runs it on an
// allocated port by OS. So after fixture created, its address would be
// "0.0.0.0:<selected-port-by-os>
const defaultAddress = "0.0.0.0:0"

type nodeFixtureParameters struct {
	handlerFunc network.StreamHandler
	unicasts    []unicast.ProtocolName
	key         fcrypto.PrivateKey
	address     string
	dhtOptions  []dht.Option
	peerFilter  p2p.PeerFilter
	role        flow.Role
	logger      zerolog.Logger
}

type nodeFixtureParameterOption func(*nodeFixtureParameters)

func withDefaultStreamHandler(handler network.StreamHandler) nodeFixtureParameterOption {
	return func(p *nodeFixtureParameters) {
		p.handlerFunc = handler
	}
}

func withPreferredUnicasts(unicasts []unicast.ProtocolName) nodeFixtureParameterOption {
	return func(p *nodeFixtureParameters) {
		p.unicasts = unicasts
	}
}

func withNetworkingPrivateKey(key fcrypto.PrivateKey) nodeFixtureParameterOption {
	return func(p *nodeFixtureParameters) {
		p.key = key
	}
}

func withNetworkingAddress(address string) nodeFixtureParameterOption {
	return func(p *nodeFixtureParameters) {
		p.address = address
	}
}

func withDHTOptions(opts ...dht.Option) nodeFixtureParameterOption {
	return func(p *nodeFixtureParameters) {
		p.dhtOptions = opts
	}
}

func withPeerFilter(filter p2p.PeerFilter) nodeFixtureParameterOption {
	return func(p *nodeFixtureParameters) {
		p.peerFilter = filter
	}
}

func withRole(role flow.Role) nodeFixtureParameterOption {
	return func(p *nodeFixtureParameters) {
		p.role = role
	}
}

func withLogger(logger zerolog.Logger) nodeFixtureParameterOption {
	return func(p *nodeFixtureParameters) {
		p.logger = logger
	}
}

// nodeFixture is a test fixture that creates a single libp2p node with the given key, spork id, and options.
// It returns the node and its identity.
func nodeFixture(
	t *testing.T,
	sporkID flow.Identifier,
	dhtPrefix string,
	opts ...nodeFixtureParameterOption,
) (*p2p.Node, flow.Identity) {
	// default parameters
	parameters := &nodeFixtureParameters{
		handlerFunc: func(network.Stream) {},
		unicasts:    nil,
		key:         generateNetworkingKey(t),
		address:     defaultAddress,
		logger:      unittest.Logger().Level(zerolog.ErrorLevel),
	}

	for _, opt := range opts {
		opt(parameters)
	}

	identity := unittest.IdentityFixture(
		unittest.WithNetworkingKey(parameters.key.PublicKey()),
		unittest.WithAddress(parameters.address),
		unittest.WithRole(parameters.role))

	noopMetrics := metrics.NewNoopCollector()
	connManager := p2p.NewConnManager(parameters.logger, noopMetrics)
	resourceManager := test.NewResourceManager(t)

	builder := p2p.NewNodeBuilder(parameters.logger, parameters.address, parameters.key, sporkID).
		SetConnectionManager(connManager).
		SetPubSub(pubsub.NewGossipSub).
		SetRoutingSystem(func(c context.Context, h host.Host) (routing.Routing, error) {
			return p2p.NewDHT(c, h,
				protocol.ID(unicast.FlowDHTProtocolIDPrefix+sporkID.String()+"/"+dhtPrefix),
				parameters.logger,
				noopMetrics,
				parameters.dhtOptions...,
			)
		}).
		SetResourceManager(resourceManager)

	if parameters.peerFilter != nil {
		filters := []p2p.PeerFilter{parameters.peerFilter}
		// set parameters.peerFilter as the default peerFilter for both callbacks
		connGater := p2p.NewConnGater(parameters.logger, p2p.WithOnInterceptPeerDialFilters(filters), p2p.WithOnInterceptSecuredFilters(filters))
		builder.SetConnectionGater(connGater)
	}

	n, err := builder.Build()
	require.NoError(t, err)

	err = n.WithDefaultUnicastProtocol(parameters.handlerFunc, parameters.unicasts)
	require.NoError(t, err)

	require.Eventuallyf(t, func() bool {
		ip, p, err := n.GetIPPort()
		return err == nil && ip != "" && p != ""
	}, 3*time.Second, ticksForAssertEventually, fmt.Sprintf("could not start node %s", identity.NodeID))

	// get the actual IP and port that have been assigned by the subsystem
	ip, port, err := n.GetIPPort()
	require.NoError(t, err)
	identity.Address = ip + ":" + port
	return n, *identity
}

// startNodes start all nodes in the input slice using the provided context, timing out if nodes are
// not all Ready() before duration expires
func startNodes(t *testing.T, ctx irrecoverable.SignalerContext, nodes []*p2p.Node, timeout time.Duration) {
	rdas := make([]module.ReadyDoneAware, 0, len(nodes))
	for _, node := range nodes {
		node.Start(ctx)
		rdas = append(rdas, node)
	}
	unittest.RequireComponentsReadyBefore(t, timeout, rdas...)
}

// startNode start a single node using the provided context, timing out if nodes are not all Ready()
// before duration expires
func startNode(t *testing.T, ctx irrecoverable.SignalerContext, node *p2p.Node, timeout time.Duration) {
	node.Start(ctx)
	unittest.RequireComponentsReadyBefore(t, timeout, node)
}

// stopNodes stops all nodes in the input slice using the provided cancel func, timing out if nodes are
// not all Done() before duration expires
func stopNodes(t *testing.T, nodes []*p2p.Node, cancel context.CancelFunc, timeout time.Duration) {
	cancel()
	for _, node := range nodes {
		unittest.RequireComponentsDoneBefore(t, timeout, node)
	}
}

// stopNode stops a single node using the provided cancel func, timing out if nodes are not all Done()
// before duration expires
func stopNode(t *testing.T, node *p2p.Node, cancel context.CancelFunc, timeout time.Duration) {
	cancel()
	unittest.RequireComponentsDoneBefore(t, timeout, node)
}

// generateNetworkingKey is a test helper that generates a ECDSA flow key pair.
func generateNetworkingKey(t *testing.T) fcrypto.PrivateKey {
	seed := unittest.SeedFixture(48)
	key, err := fcrypto.GeneratePrivateKey(fcrypto.ECDSASecp256k1, seed)
	require.NoError(t, err)
	return key
}

// silentNodeFixture returns a TCP listener and a node which never replies
func silentNodeFixture(t *testing.T) (net.Listener, flow.Identity) {
	key := generateNetworkingKey(t)

	lst, err := net.Listen("tcp4", ":0")
	require.NoError(t, err)

	addr, err := manet.FromNetAddr(lst.Addr())
	require.NoError(t, err)

	addrs := []multiaddr.Multiaddr{addr}
	addrs, err = addrutil.ResolveUnspecifiedAddresses(addrs, nil)
	require.NoError(t, err)

	go acceptAndHang(t, lst)

	ip, port, err := p2p.IPPortFromMultiAddress(addrs...)
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

// nodesFixture is a test fixture that creates a number of libp2p nodes with the given callback function for stream handling.
// It returns the nodes and their identities.
func nodesFixture(
	t *testing.T,
	sporkID flow.Identifier,
	dhtPrefix string,
	count int,
	opts ...nodeFixtureParameterOption,
) ([]*p2p.Node, flow.IdentityList) {
	var nodes []*p2p.Node

	// creating nodes
	var identities flow.IdentityList
	for i := 0; i < count; i++ {
		// create a node on localhost with a random port assigned by the OS
		node, identity := nodeFixture(t, sporkID, dhtPrefix, opts...)
		nodes = append(nodes, node)
		identities = append(identities, &identity)
	}

	return nodes, identities
}
