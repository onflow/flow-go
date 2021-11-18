package p2p

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	addrutil "github.com/libp2p/go-addr-util"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fcrypto "github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network/mocknetwork"
	"github.com/onflow/flow-go/network/p2p/dns"
	"github.com/onflow/flow-go/network/p2p/unicast"
	"github.com/onflow/flow-go/utils/unittest"
)

// Workaround for https://github.com/stretchr/testify/pull/808
const tickForAssertEventually = 100 * time.Millisecond

// Creating a node fixture with defaultAddress lets libp2p runs it on an
// allocated port by OS. So after fixture created, its address would be
// "0.0.0.0:<selected-port-by-os>
const defaultAddress = "0.0.0.0:0"

var rootBlockID = unittest.IdentifierFixture()

type nodeFixtureParameters struct {
	handlerFunc network.StreamHandler
	unicasts    []unicast.ProtocolName
	allowList   bool
}

type nodeFixtureParameterOption func(*nodeFixtureParameters)

func withDefaultStreamHandler(handler network.StreamHandler) nodeFixtureParameterOption {
	return func(p *nodeFixtureParameters) {
		p.handlerFunc = handler
	}
}

func withAllowListEnabled() nodeFixtureParameterOption {
	return func(p *nodeFixtureParameters) {
		p.allowList = true
	}
}

func withPreferredUnicasts(unicasts []unicast.ProtocolName) nodeFixtureParameterOption {
	return func(p *nodeFixtureParameters) {
		p.unicasts = unicasts
	}
}

// nodeFixture creates a single LibP2PNodes with the given key, root block id, and callback function for stream handling.
// It returns the nodes and their identities.
func nodeFixture(t *testing.T,
	log zerolog.Logger,
	key fcrypto.PrivateKey,
	rootID flow.Identifier,
	address string,
	opts ...nodeFixtureParameterOption) (*Node, flow.Identity) {
	identity := unittest.IdentityFixture(unittest.WithNetworkingKey(key.PublicKey()), unittest.WithAddress(address))

	parameters := &nodeFixtureParameters{
		handlerFunc: func(network.Stream) {},
		allowList:   false,
		unicasts:    nil,
	}

	for _, opt := range opts {
		opt(parameters)
	}

	pingInfoProvider, _, _, _ := mockPingInfoProvider()

	// dns resolver
	resolver := dns.NewResolver(metrics.NewNoopCollector())
	unittest.RequireCloseBefore(t, resolver.Ready(), 10*time.Millisecond, "could not start resolver")

	noopMetrics := metrics.NewNoopCollector()
	connManager := NewConnManager(log, noopMetrics)

	builder := NewDefaultLibP2PNodeBuilder(identity.NodeID, address, key).
		SetRootBlockID(rootID).
		SetConnectionManager(connManager).
		SetPingInfoProvider(pingInfoProvider).
		SetResolver(resolver).
		SetTopicValidation(false).
		SetLogger(log)

	if parameters.allowList {
		connGater := NewConnGater(log)
		builder.SetConnectionGater(connGater)
	}

	ctx := context.Background()
	n, err := builder.Build(ctx)
	require.NoError(t, err)

	err = n.WithDefaultUnicastProtocol(parameters.handlerFunc, nil)
	require.NoError(t, err)

	require.Eventuallyf(t, func() bool {
		ip, p, err := n.GetIPPort()
		return err == nil && ip != "" && p != ""
	}, 3*time.Second, tickForAssertEventually, fmt.Sprintf("could not start node %s", identity.NodeID.String()))

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

// nodesFixture creates a number of LibP2PNodes with the given callback function for stream handling.
// It returns the nodes and their identities.
func nodesFixture(t *testing.T, count int, opts ...nodeFixtureParameterOption) ([]*Node, flow.IdentityList) {
	// keeps track of errors on creating a node
	var err error
	var nodes []*Node

	defer func() {
		if err != nil && nodes != nil {
			// stops all nodes upon an error in starting even one single node
			stopNodes(t, nodes)
		}
	}()

	// creating nodes
	var identities flow.IdentityList
	for i := 0; i < count; i++ {
		// create a node on localhost with a random port assigned by the OS
		key := generateNetworkingKey(t)
		node, identity := nodeFixture(t,
			unittest.Logger(),
			key,
			rootBlockID,
			defaultAddress,
			opts...)
		nodes = append(nodes, node)
		identities = append(identities, &identity)
	}
	return nodes, identities
}
