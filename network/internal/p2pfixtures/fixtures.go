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
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/libp2p/go-libp2p/core/routing"
	"github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/config"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/id"
	"github.com/onflow/flow-go/module/metrics"
	flownet "github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/internal/p2putils"
	"github.com/onflow/flow-go/network/message"
	"github.com/onflow/flow-go/network/p2p"
	p2pdht "github.com/onflow/flow-go/network/p2p/dht"
	"github.com/onflow/flow-go/network/p2p/p2pbuilder"
	p2pconfig "github.com/onflow/flow-go/network/p2p/p2pbuilder/config"
	"github.com/onflow/flow-go/network/p2p/tracer"
	"github.com/onflow/flow-go/network/p2p/unicast/protocols"
	"github.com/onflow/flow-go/network/p2p/utils"
	"github.com/onflow/flow-go/utils/unittest"
)

// NetworkingKeyFixtures is a test helper that generates a ECDSA flow key pair.
func NetworkingKeyFixtures(t *testing.T) crypto.PrivateKey {
	seed := unittest.SeedFixture(48)
	key, err := crypto.GeneratePrivateKey(crypto.ECDSASecp256k1, seed)
	require.NoError(t, err)
	return key
}

// SilentNodeFixture returns a TCP listener and a node which never replies
func SilentNodeFixture(t *testing.T) (net.Listener, flow.Identity) {
	key := NetworkingKeyFixtures(t)

	lst, err := net.Listen("tcp4", unittest.DefaultAddress)
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

type nodeOpt func(p2p.NodeBuilder)

func WithSubscriptionFilter(filter pubsub.SubscriptionFilter) nodeOpt {
	return func(builder p2p.NodeBuilder) {
		builder.SetSubscriptionFilter(filter)
	}
}

// TODO: this should be replaced by node fixture: https://github.com/onflow/flow-go/blob/master/network/p2p/test/fixtures.go
func CreateNode(t *testing.T, networkKey crypto.PrivateKey, sporkID flow.Identifier, logger zerolog.Logger, nodeIds flow.IdentityList, opts ...nodeOpt) p2p.LibP2PNode {
	idProvider := id.NewFixedIdentityProvider(nodeIds)
	defaultFlowConfig, err := config.DefaultConfig()
	require.NoError(t, err)

	meshTracerCfg := &tracer.GossipSubMeshTracerConfig{
		Logger:                             logger,
		Metrics:                            metrics.NewNoopCollector(),
		IDProvider:                         idProvider,
		LoggerInterval:                     defaultFlowConfig.NetworkConfig.GossipSubConfig.LocalMeshLogInterval,
		RpcSentTrackerCacheSize:            defaultFlowConfig.NetworkConfig.GossipSubConfig.RPCSentTrackerCacheSize,
		RpcSentTrackerWorkerQueueCacheSize: defaultFlowConfig.NetworkConfig.GossipSubConfig.RPCSentTrackerQueueCacheSize,
		RpcSentTrackerNumOfWorkers:         defaultFlowConfig.NetworkConfig.GossipSubConfig.RpcSentTrackerNumOfWorkers,
		HeroCacheMetricsFactory:            metrics.NewNoopHeroCacheMetricsFactory(),
		NetworkingType:                     flownet.PublicNetwork,
	}
	meshTracer := tracer.NewGossipSubMeshTracer(meshTracerCfg)
	params := &p2pbuilder.LibP2PNodeBuilderConfig{
		Logger: logger,
		MetricsConfig: &p2pconfig.MetricsConfig{
			HeroCacheFactory: metrics.NewNoopHeroCacheMetricsFactory(),
			Metrics:          metrics.NewNoopCollector(),
		},
		NetworkingType:             flownet.PrivateNetwork,
		Address:                    unittest.DefaultAddress,
		NetworkKey:                 networkKey,
		SporkId:                    sporkID,
		IdProvider:                 idProvider,
		ResourceManagerParams:      &defaultFlowConfig.NetworkConfig.ResourceManager,
		RpcInspectorParams:         &defaultFlowConfig.NetworkConfig.GossipSubRPCInspectorsConfig,
		PeerManagerParams:          p2pconfig.PeerManagerDisableConfig(),
		SubscriptionProviderParams: &defaultFlowConfig.NetworkConfig.GossipSubConfig.SubscriptionProviderConfig,
		DisallowListCacheCfg: &p2p.DisallowListCacheConfig{
			MaxSize: uint32(1000),
			Metrics: metrics.NewNoopCollector(),
		},
		UnicastParams: &p2pconfig.UnicastConfig{
			UnicastConfig: defaultFlowConfig.NetworkConfig.UnicastConfig,
		},
		GossipSubScorePenaltiesParams: &defaultFlowConfig.NetworkConfig.GossipsubScorePenalties,
		ScoringRegistryParams:         &defaultFlowConfig.NetworkConfig.GossipSubScoringRegistryConfig,
	}
	builder, err := p2pbuilder.NewNodeBuilder(params, meshTracer)
	require.NoError(t, err)
	builder.
		SetRoutingSystem(func(c context.Context, h host.Host) (routing.Routing, error) {
			return p2pdht.NewDHT(c, h, protocols.FlowDHTProtocolID(sporkID), zerolog.Nop(), metrics.NewNoopCollector())
		}).
		SetResourceManager(&network.NullResourceManager{}).
		SetGossipSubTracer(meshTracer).
		SetGossipSubScoreTracerInterval(defaultFlowConfig.NetworkConfig.GossipSubConfig.ScoreTracerInterval)

	for _, opt := range opts {
		opt(builder)
	}

	libp2pNode, err := builder.Build()
	require.NoError(t, err)

	return libp2pNode
}

// SubMustNeverReceiveAnyMessage checks that the subscription never receives any message within the given timeout by the context.
func SubMustNeverReceiveAnyMessage(t *testing.T, ctx context.Context, sub p2p.Subscription) {
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
func HasSubReceivedMessage(t *testing.T, ctx context.Context, expectedMessage []byte, sub p2p.Subscription) bool {
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
func SubsMustNeverReceiveAnyMessage(t *testing.T, ctx context.Context, subs []p2p.Subscription) {
	for _, sub := range subs {
		SubMustNeverReceiveAnyMessage(t, ctx, sub)
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

// EnsureNotConnected ensures that no connection exists from "from" nodes to "to" nodes.
func EnsureNotConnected(t *testing.T, ctx context.Context, from []p2p.LibP2PNode, to []p2p.LibP2PNode) {
	for _, this := range from {
		for _, other := range to {
			if this == other {
				require.Fail(t, "overlapping nodes in from and to lists")
			}
			thisId := this.ID()
			// we intentionally do not check the error here, with libp2p v0.24 connection gating at the "InterceptSecured" level
			// does not cause the nodes to complain about the connection being rejected at the dialer side.
			// Hence, we instead check for any trace of the connection being established in the receiver side.
			_ = this.Host().Connect(ctx, other.Host().Peerstore().PeerInfo(other.ID()))
			// ensures that other node has never received a connection from this node.
			require.Equal(t, network.NotConnected, other.Host().Network().Connectedness(thisId))
			require.Empty(t, other.Host().Network().ConnsToPeer(thisId))
		}
	}
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
			err := this.OpenProtectedStream(ctx, other.ID(), t.Name(), func(stream network.Stream) error {
				rw := bufio.NewReadWriter(bufio.NewReader(stream), bufio.NewWriter(stream))
				_, err := rw.WriteString(msg)
				require.NoError(t, err)

				// Flush the stream
				require.NoError(t, rw.Flush())

				return nil
			})
			require.NoError(t, err)

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

			// we intentionally do not check the error here, with libp2p v0.24 connection gating at the "InterceptSecured" level
			// does not cause the nodes to complain about the connection being rejected at the dialer side.
			// Hence, we instead check for any trace of the connection being established in the receiver side.
			otherId := other.ID()
			thisId := this.ID()

			// closes all connections from other node to this node in order to isolate the connection attempt.
			for _, conn := range other.Host().Network().ConnsToPeer(thisId) {
				require.NoError(t, conn.Close())
			}
			require.Empty(t, other.Host().Network().ConnsToPeer(thisId))

			err := this.OpenProtectedStream(ctx, otherId, t.Name(), func(stream network.Stream) error {
				// no-op as the stream is never created.
				return nil
			})
			// ensures that other node has never received a connection from this node.
			require.Equal(t, network.NotConnected, other.Host().Network().Connectedness(thisId))
			// a stream is established on top of a connection, so if there is no connection, there should be no stream.
			require.Empty(t, other.Host().Network().ConnsToPeer(thisId))
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
			err := this.OpenProtectedStream(ctx, other.ID(), t.Name(), func(stream network.Stream) error {
				require.NotNil(t, stream)
				return nil
			})
			require.NoError(t, err)
		}
	}
}

// LongStringMessageFactoryFixture returns a function that creates a long unique string message.
func LongStringMessageFactoryFixture(t *testing.T) func() string {
	return func() string {
		msg := "this is an intentionally long MESSAGE to be bigger than buffer size of most of stream compressors"
		require.Greater(t, len(msg), 10, "we must stress test with longer than 10 bytes messages")
		return fmt.Sprintf("%s %d \n", msg, time.Now().UnixNano()) // add timestamp to make sure we don't send the same message twice
	}
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
func SubMustReceiveMessage(t *testing.T, ctx context.Context, expectedMessage []byte, sub p2p.Subscription) {
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
func SubsMustReceiveMessage(t *testing.T, ctx context.Context, expectedMessage []byte, subs []p2p.Subscription) {
	for _, sub := range subs {
		SubMustReceiveMessage(t, ctx, expectedMessage, sub)
	}
}
