package p2p

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	addrutil "github.com/libp2p/go-addr-util"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/routing"
	"github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fcrypto "github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network/mocknetwork"
	"github.com/onflow/flow-go/network/p2p/unicast"
	"github.com/onflow/flow-go/utils/unittest"
)

// Workaround for https://github.com/stretchr/testify/pull/808
const ticksForAssertEventually = 100 * time.Millisecond

// Creating a node fixture with defaultAddress lets libp2p runs it on an
// allocated port by OS. So after fixture created, its address would be
// "0.0.0.0:<selected-port-by-os>
const defaultAddress = "0.0.0.0:0"

type nodeFixtureParameters struct {
	handlerFunc network.StreamHandler
	unicasts    []unicast.ProtocolName
	key         fcrypto.PrivateKey
	address     string
	dhtEnabled  bool
	dhtServer   bool
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

func withDHTNodeEnabled(asServer bool) nodeFixtureParameterOption {
	return func(p *nodeFixtureParameters) {
		p.dhtEnabled = true
		p.dhtServer = asServer
	}
}

// nodeFixture is a test fixture that creates a single libp2p node with the given key, spork id, and options.
// It returns the node and its identity.
func nodeFixture(t *testing.T, ctx context.Context, sporkId flow.Identifier, opts ...nodeFixtureParameterOption) (*Node, flow.Identity) {
	logger := unittest.Logger().Level(zerolog.ErrorLevel)

	// default parameters
	parameters := &nodeFixtureParameters{
		handlerFunc: func(network.Stream) {},
		unicasts:    nil,
		key:         generateNetworkingKey(t),
		address:     defaultAddress,
		dhtServer:   false,
		dhtEnabled:  false,
	}

	for _, opt := range opts {
		opt(parameters)
	}

	identity := unittest.IdentityFixture(
		unittest.WithNetworkingKey(parameters.key.PublicKey()),
		unittest.WithAddress(parameters.address))

	noopMetrics := metrics.NewNoopCollector()
	connManager := NewConnManager(logger, noopMetrics)

	builder := NewNodeBuilder(logger, parameters.address, parameters.key, sporkId).
		SetConnectionManager(connManager)

	if parameters.dhtEnabled {
		builder.SetRoutingSystem(func(c context.Context, h host.Host) (routing.Routing, error) {
			return NewDHT(c, h, unicast.FlowDHTProtocolIDPrefix+"test", AsServer(parameters.dhtServer))
		})
	}

	n, err := builder.Build(ctx)
	require.NoError(t, err)

	err = n.WithDefaultUnicastProtocol(parameters.handlerFunc, parameters.unicasts)
	require.NoError(t, err)

	require.Eventuallyf(t, func() bool {
		ip, p, err := n.GetIPPort()
		return err == nil && ip != "" && p != ""
	}, 3*time.Second, ticksForAssertEventually, fmt.Sprintf("could not start node %s", identity.NodeID.String()))

	// get the actual IP and port that have been assigned by the subsystem
	ip, port, err := n.GetIPPort()
	require.NoError(t, err)
	identity.Address = ip + ":" + port

	return n, *identity
}

func mockPingInfoProvider() (*mocknetwork.PingInfoProvider, string, uint64, uint64) {
	version := "version_1"
	height := uint64(5000)
	view := uint64(10)
	pingInfoProvider := new(mocknetwork.PingInfoProvider)
	pingInfoProvider.On("SoftwareVersion").Return(version)
	pingInfoProvider.On("SealedBlockHeight").Return(height)
	pingInfoProvider.On("HotstuffView").Return(view)
	return pingInfoProvider, version, height, view
}

// stopNodes stop all nodes in the input slice
func stopNodes(t *testing.T, nodes []*Node) {
	for _, n := range nodes {
		stopNode(t, n)
	}
}

func stopNode(t *testing.T, node *Node) {
	done, err := node.Stop()
	assert.NoError(t, err)
	unittest.RequireCloseBefore(t, done, 1*time.Second, "could not stop node on ime")
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

	ip, port, err := IPPortFromMultiAddress(addrs...)
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
func nodesFixture(t *testing.T, ctx context.Context, sporkId flow.Identifier, count int, opts ...nodeFixtureParameterOption) ([]*Node,
	flow.IdentityList) {
	// keeps track of errors on creating a node
	var err error
	var nodes []*Node

	defer func() {
		if err != nil && nodes != nil {
			// stops all nodes upon an error in starting even one single node
			stopNodes(t, nodes)
			t.Fail()
		}
	}()

	// creating nodes
	var identities flow.IdentityList
	for i := 0; i < count; i++ {
		// create a node on localhost with a random port assigned by the OS
		node, identity := nodeFixture(t, ctx, sporkId, opts...)
		nodes = append(nodes, node)
		identities = append(identities, &identity)
	}
	return nodes, identities
}
