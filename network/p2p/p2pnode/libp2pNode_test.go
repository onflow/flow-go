package p2pnode_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	mockmodule "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/internal/p2pfixtures"
	"github.com/onflow/flow-go/network/internal/p2putils"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/p2pnode"
	p2ptest "github.com/onflow/flow-go/network/p2p/test"
	"github.com/onflow/flow-go/network/p2p/unicast/protocols"
	"github.com/onflow/flow-go/network/p2p/utils"
	validator "github.com/onflow/flow-go/network/validator/pubsub"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestMultiAddress evaluates correct translations from
// dns and ip4 to libp2p multi-address
func TestMultiAddress(t *testing.T) {
	key := p2pfixtures.NetworkingKeyFixtures(t)

	tt := []struct {
		identity     *flow.Identity
		multiaddress string
	}{
		{ // ip4 test case
			identity:     unittest.IdentityFixture(unittest.WithNetworkingKey(key.PublicKey()), unittest.WithAddress("172.16.254.1:72")),
			multiaddress: "/ip4/172.16.254.1/tcp/72",
		},
		{ // dns test case
			identity:     unittest.IdentityFixture(unittest.WithNetworkingKey(key.PublicKey()), unittest.WithAddress("consensus:2222")),
			multiaddress: "/dns4/consensus/tcp/2222",
		},
		{ // dns test case
			identity:     unittest.IdentityFixture(unittest.WithNetworkingKey(key.PublicKey()), unittest.WithAddress("flow.com:3333")),
			multiaddress: "/dns4/flow.com/tcp/3333",
		},
	}

	for _, tc := range tt {
		ip, port, _, err := p2putils.NetworkingInfo(*tc.identity)
		require.NoError(t, err)

		actualAddress := utils.MultiAddressStr(ip, port)
		assert.Equal(t, tc.multiaddress, actualAddress, "incorrect multi-address translation")
	}

}

// TestSingleNodeLifeCycle evaluates correct lifecycle translation from start to stop the node
func TestSingleNodeLifeCycle(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)
	idProvider := mockmodule.NewIdentityProvider(t)
	node, _ := p2ptest.NodeFixture(
		t,
		unittest.IdentifierFixture(),
		"test_single_node_life_cycle",
		idProvider,
	)

	node.Start(signalerCtx)
	unittest.RequireComponentsReadyBefore(t, 100*time.Millisecond, node)

	cancel()
	unittest.RequireComponentsDoneBefore(t, 100*time.Millisecond, node)
}

// TestGetPeerInfo evaluates the deterministic translation between the nodes address and
// their libp2p info. It generates an address, and checks whether repeated translations
// yields the same info or not.
func TestGetPeerInfo(t *testing.T) {
	for i := 0; i < 10; i++ {
		key := p2pfixtures.NetworkingKeyFixtures(t)

		// creates node-i identity
		identity := unittest.IdentityFixture(unittest.WithNetworkingKey(key.PublicKey()), unittest.WithAddress("1.1.1.1:0"))

		// translates node-i address into info
		info, err := utils.PeerAddressInfo(*identity)
		require.NoError(t, err)

		// repeats the translation for node-i
		for j := 0; j < 10; j++ {
			rinfo, err := utils.PeerAddressInfo(*identity)
			require.NoError(t, err)
			assert.Equal(t, rinfo.String(), info.String(), "inconsistent id generated")
		}
	}
}

// TestAddPeers checks if nodes can be added as peers to a given node
func TestAddPeers(t *testing.T) {
	count := 3
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)
	idProvider := unittest.NewUpdatableIDProvider(flow.IdentityList{})

	nodes, identities := p2ptest.NodesFixture(t, unittest.IdentifierFixture(), "test_add_peers", count, idProvider)
	p2ptest.StartNodes(t, signalerCtx, nodes, 100*time.Millisecond)
	defer p2ptest.StopNodes(t, nodes, cancel, 100*time.Millisecond)

	// add the remaining nodes to the first node as its set of peers
	for _, identity := range identities[1:] {
		peerInfo, err := utils.PeerAddressInfo(*identity)
		require.NoError(t, err)
		require.NoError(t, nodes[0].AddPeer(ctx, peerInfo))
	}

	// Checks if both of the other nodes have been added as peers to the first node
	assert.Len(t, nodes[0].Host().Network().Peers(), count-1)
}

// TestRemovePeers checks if nodes can be removed as peers from a given node
func TestRemovePeers(t *testing.T) {
	count := 3
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)
	idProvider := unittest.NewUpdatableIDProvider(flow.IdentityList{})
	// create nodes
	nodes, identities := p2ptest.NodesFixture(t, unittest.IdentifierFixture(), "test_remove_peers", count, idProvider)
	peerInfos, errs := utils.PeerInfosFromIDs(identities)
	assert.Len(t, errs, 0)

	p2ptest.StartNodes(t, signalerCtx, nodes, 100*time.Millisecond)
	defer p2ptest.StopNodes(t, nodes, cancel, 100*time.Millisecond)

	// add nodes two and three to the first node as its peers
	for _, pInfo := range peerInfos[1:] {
		require.NoError(t, nodes[0].AddPeer(ctx, pInfo))
	}

	// check if all other nodes have been added as peers to the first node
	assert.Len(t, nodes[0].Host().Network().Peers(), count-1)

	// disconnect from each peer and assert that the connection no longer exists
	for _, pInfo := range peerInfos[1:] {
		require.NoError(t, nodes[0].RemovePeer(pInfo.ID))
		assert.Equal(t, network.NotConnected, nodes[0].Host().Network().Connectedness(pInfo.ID))
	}
}

func TestConnGater(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)

	sporkID := unittest.IdentifierFixture()
	idProvider := mockmodule.NewIdentityProvider(t)

	node1Peers := unittest.NewProtectedMap[peer.ID, struct{}]()
	node1, identity1 := p2ptest.NodeFixture(
		t,
		sporkID,
		t.Name(),
		idProvider,
		p2ptest.WithConnectionGater(p2ptest.NewConnectionGater(idProvider, func(pid peer.ID) error {
			if !node1Peers.Has(pid) {
				return fmt.Errorf("peer id not found: %s", pid.String())
			}
			return nil
		})))
	idProvider.On("ByPeerID", node1.Host().ID()).Return(&identity1, true).Maybe()

	p2ptest.StartNode(t, signalerCtx, node1, 100*time.Millisecond)
	defer p2ptest.StopNode(t, node1, cancel, 100*time.Millisecond)

	node1Info, err := utils.PeerAddressInfo(identity1)
	assert.NoError(t, err)

	node2Peers := unittest.NewProtectedMap[peer.ID, struct{}]()
	node2, identity2 := p2ptest.NodeFixture(
		t,
		sporkID, t.Name(),
		idProvider,
		p2ptest.WithConnectionGater(p2ptest.NewConnectionGater(idProvider, func(pid peer.ID) error {
			if !node2Peers.Has(pid) {
				return fmt.Errorf("id not found: %s", pid.String())
			}
			return nil
		})))
	idProvider.On("ByPeerID", node2.Host().ID()).Return(&identity2,

		true).Maybe()

	p2ptest.StartNode(t, signalerCtx, node2, 100*time.Millisecond)
	defer p2ptest.StopNode(t, node2, cancel, 100*time.Millisecond)

	node2Info, err := utils.PeerAddressInfo(identity2)
	assert.NoError(t, err)

	node1.Host().Peerstore().AddAddrs(node2Info.ID, node2Info.Addrs, peerstore.PermanentAddrTTL)
	node2.Host().Peerstore().AddAddrs(node1Info.ID, node1Info.Addrs, peerstore.PermanentAddrTTL)

	_, err = node1.CreateStream(ctx, node2Info.ID)
	assert.Error(t, err, "connection should not be possible")

	_, err = node2.CreateStream(ctx, node1Info.ID)
	assert.Error(t, err, "connection should not be possible")

	node1Peers.Add(node2Info.ID, struct{}{})
	_, err = node1.CreateStream(ctx, node2Info.ID)
	assert.Error(t, err, "connection should not be possible")

	node2Peers.Add(node1Info.ID, struct{}{})
	_, err = node1.CreateStream(ctx, node2Info.ID)
	assert.NoError(t, err, "connection should not be blocked")
}

// TestNode_HasSubscription checks that when a node subscribes to a topic HasSubscription should return true.
func TestNode_HasSubscription(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)
	idProvider := unittest.NewUpdatableIDProvider(flow.IdentityList{})
	sporkID := unittest.IdentifierFixture()
	node, _ := p2ptest.NodeFixture(t, sporkID, "test_has_subscription", idProvider)

	p2ptest.StartNode(t, signalerCtx, node, 100*time.Millisecond)
	defer p2ptest.StopNode(t, node, cancel, 100*time.Millisecond)

	logger := unittest.Logger()

	topicValidator := validator.TopicValidator(logger, func(id peer.ID) error {
		return nil
	})

	// create test topic
	topic := channels.TopicFromChannel(channels.TestNetworkChannel, unittest.IdentifierFixture())
	_, err := node.Subscribe(topic, topicValidator)
	require.NoError(t, err)

	require.True(t, node.HasSubscription(topic))

	// create topic with no subscription
	topic = channels.TopicFromChannel(channels.ConsensusCommittee, unittest.IdentifierFixture())
	require.False(t, node.HasSubscription(topic))
}

// TestCreateStream_SinglePairwiseConnection ensures that despite the number of concurrent streams created from peer -> peer, only a single
// connection will ever be created between two peers on initial peer dialing and subsequent streams will reuse that connection.
func TestCreateStream_SinglePairwiseConnection(t *testing.T) {
	sporkId := unittest.IdentifierFixture()
	nodeCount := 3
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)
	idProvider := unittest.NewUpdatableIDProvider(flow.IdentityList{})
	nodes, ids := p2ptest.NodesFixture(t,
		sporkId,
		"test_create_stream_single_pairwise_connection",
		nodeCount,
		idProvider,
		p2ptest.WithDefaultResourceManager())
	idProvider.SetIdentities(ids)

	p2ptest.StartNodes(t, signalerCtx, nodes, 100*time.Millisecond)
	defer p2ptest.StopNodes(t, nodes, cancel, 100*time.Millisecond)

	ctxWithTimeout, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	done := make(chan struct{})
	numOfStreamsPerNode := 100 // create large number of streams per node per connection to ensure the resource manager does not cause starvation of resources
	expectedTotalNumOfStreams := 600

	// create a number of streams concurrently between each node
	streams := make(chan network.Stream, expectedTotalNumOfStreams)

	go createConcurrentStreams(t, ctxWithTimeout, nodes, ids, numOfStreamsPerNode, streams, done)
	unittest.RequireCloseBefore(t, done, 5*time.Second, "could not create streams on time")
	require.Len(t, streams, expectedTotalNumOfStreams, fmt.Sprintf("expected %d total number of streams created got %d", expectedTotalNumOfStreams, len(streams)))

	// ensure only a single connection exists between all nodes
	ensureSinglePairwiseConnection(t, nodes)
	close(streams)
	for s := range streams {
		_ = s.Close()
	}
}

// TestCreateStream_SinglePeerDial ensures that the unicast manager only attempts to dial a peer once, retries dialing a peer the expected max amount of times when an
// error is encountered and retries creating the stream the expected max amount of times when unicast.ErrDialInProgress is encountered.
func TestCreateStream_SinglePeerDial(t *testing.T) {
	createStreamRetries := atomic.NewInt64(0)
	dialPeerRetries := atomic.NewInt64(0)
	hook := zerolog.HookFunc(func(e *zerolog.Event, level zerolog.Level, message string) {
		if level == zerolog.WarnLevel {
			switch {
			case strings.Contains(message, "retrying create stream, dial to peer in progress"):
				createStreamRetries.Inc()
			case strings.Contains(message, "retrying peer dialing"):
				dialPeerRetries.Inc()
			}
		}
	})
	logger := zerolog.New(os.Stdout).Level(zerolog.InfoLevel).Hook(hook)
	idProvider := mockmodule.NewIdentityProvider(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)

	sporkID := unittest.IdentifierFixture()

	// mock metrics we expected only a single call to CreateStream to initiate the dialing to the peer, which will result in 3 failed attempts
	// the next call to CreateStream will encounter a DialInProgress error which will result in 3 failed attempts
	m := mockmodule.NewNetworkMetrics(t)
	m.On("OnPeerDialFailure", mock.Anything, 3).Once()
	m.On("OnStreamCreationFailure", mock.Anything, mock.Anything).Twice().Run(func(args mock.Arguments) {
		attempts := args.Get(1).(int)
		// We expect OnCreateStream to be called twice: once in each separate call to CreateStream. The first call that initializes
		// the peer dialing should not attempt to retry CreateStream because all peer dialing attempts will be made which will not
		// return the DialInProgress err that kicks off the CreateStream retries so we expect attempts to be 1 in this case. In the
		// second call to CreateStream we expect all 3 attempts to be made as we wait for the DialInProgress to complete, in this case
		// we expect attempts to be 3. Thus we only expect this method to be called twice with either 1 or 3 attempts.
		require.False(t, attempts != 1 && attempts != 3, fmt.Sprintf("expected either 1 or 3 attempts got %d", attempts))
	})

	sender, id1 := p2ptest.NodeFixture(
		t,
		sporkID,
		t.Name(),
		idProvider,
		p2ptest.WithConnectionGater(p2ptest.NewConnectionGater(idProvider, func(pid peer.ID) error {
			// avoid connection gating outbound messages on sender
			return nil
		})),
		// add very small delay so that when the sender attempts to create multiple streams
		// the func fails fast before the first routine can finish the peer dialing retries
		// this prevents us from making another call to dial peer
		p2ptest.WithCreateStreamRetryDelay(10*time.Millisecond),
		p2ptest.WithLogger(logger),
		p2ptest.WithMetricsCollector(m))

	receiver, id2 := p2ptest.NodeFixture(
		t,
		sporkID,
		t.Name(),
		idProvider,
		p2ptest.WithConnectionGater(p2ptest.NewConnectionGater(idProvider, func(pid peer.ID) error {
			// connection gate all incoming connections forcing the senders unicast manager to perform retries
			return fmt.Errorf("gate keep")
		})),
		p2ptest.WithCreateStreamRetryDelay(10*time.Millisecond),
		p2ptest.WithLogger(logger))

	idProvider.On("ByPeerID", sender.Host().ID()).Return(&id1, true).Maybe()
	idProvider.On("ByPeerID", receiver.Host().ID()).Return(&id2, true).Maybe()

	p2ptest.StartNodes(t, signalerCtx, []p2p.LibP2PNode{sender, receiver}, 100*time.Millisecond)
	defer p2ptest.StopNodes(t, []p2p.LibP2PNode{sender, receiver}, cancel, 100*time.Millisecond)

	var wg sync.WaitGroup
	wg.Add(2)
	// attempt to create two concurrent streams
	go func() {
		defer wg.Done()
		_, err := sender.CreateStream(ctx, receiver.Host().ID())
		require.Error(t, err)
	}()
	go func() {
		defer wg.Done()
		_, err := sender.CreateStream(ctx, receiver.Host().ID())
		require.Error(t, err)
	}()

	unittest.RequireReturnsBefore(t, wg.Wait, 3*time.Second, "cannot create streams on time")

	// we expect a single routine to start attempting to dial thus the number of retries
	// before failure should be at most p2pnode.MaxConnectAttempt
	expectedNumOfDialRetries := int64(p2pnode.MaxConnectAttempt)
	// we expect the second routine to retry creating a stream p2pnode.MaxConnectAttempt when dialing is in progress
	expectedCreateStreamRetries := int64(p2pnode.MaxConnectAttempt)
	require.Equal(t, expectedNumOfDialRetries, dialPeerRetries.Load(), fmt.Sprintf("expected %d dial peer retries got %d", expectedNumOfDialRetries, dialPeerRetries.Load()))
	require.Equal(t, expectedCreateStreamRetries, createStreamRetries.Load(), fmt.Sprintf("expected %d dial peer retries got %d", expectedCreateStreamRetries, createStreamRetries.Load()))
}

// TestCreateStream_InboundConnResourceLimit ensures that the setting the resource limit config for
// PeerDefaultLimits.ConnsInbound restricts the number of inbound connections created from a peer to the configured value.
// NOTE: If this test becomes flaky, it indicates a violation of the single inbound connection guarantee.
// In such cases the test should not be quarantined but requires immediate resolution.
func TestCreateStream_InboundConnResourceLimit(t *testing.T) {
	idProvider := mockmodule.NewIdentityProvider(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)

	sporkID := unittest.IdentifierFixture()

	sender, id1 := p2ptest.NodeFixture(
		t,
		sporkID,
		t.Name(),
		idProvider,
		p2ptest.WithDefaultResourceManager(),
		p2ptest.WithCreateStreamRetryDelay(10*time.Millisecond))

	receiver, id2 := p2ptest.NodeFixture(
		t,
		sporkID,
		t.Name(),
		idProvider,
		p2ptest.WithDefaultResourceManager(),
		p2ptest.WithCreateStreamRetryDelay(10*time.Millisecond))

	idProvider.On("ByPeerID", sender.Host().ID()).Return(&id1, true).Maybe()
	idProvider.On("ByPeerID", receiver.Host().ID()).Return(&id2, true).Maybe()

	p2ptest.StartNodes(t, signalerCtx, []p2p.LibP2PNode{sender, receiver}, 100*time.Millisecond)
	defer p2ptest.StopNodes(t, []p2p.LibP2PNode{sender, receiver}, cancel, 100*time.Millisecond)

	p2ptest.LetNodesDiscoverEachOther(t, signalerCtx, []p2p.LibP2PNode{sender, receiver}, flow.IdentityList{&id1, &id2})

	var allStreamsCreated sync.WaitGroup
	// at this point both nodes have discovered each other and we can now create an
	// arbitrary number of streams from sender -> receiver. This will force libp2p
	// to create multiple streams concurrently and attempt to reuse the single pairwise
	// connection. If more than one connection is established while creating the conccurent
	// streams this indicates a bug in the libp2p PeerBaseLimitConnsInbound limit.
	defaultProtocolID := protocols.FlowProtocolID(sporkID)
	expectedNumOfStreams := int64(50)
	for i := int64(0); i < expectedNumOfStreams; i++ {
		allStreamsCreated.Add(1)
		go func() {
			defer allStreamsCreated.Done()
			_, err := sender.Host().NewStream(ctx, receiver.Host().ID(), defaultProtocolID)
			require.NoError(t, err)
		}()
	}

	unittest.RequireReturnsBefore(t, allStreamsCreated.Wait, 2*time.Second, "could not create streams on time")
	require.Len(t, receiver.Host().Network().ConnsToPeer(sender.Host().ID()), 1)
	actualNumOfStreams := p2putils.CountStream(sender.Host(), receiver.Host().ID(), defaultProtocolID, network.DirOutbound)
	require.Equal(t, expectedNumOfStreams, int64(actualNumOfStreams), fmt.Sprintf("expected to create %d number of streams got %d", expectedNumOfStreams, actualNumOfStreams))
}

// createStreams will attempt to create n number of streams concurrently between each combination of node pairs.
func createConcurrentStreams(t *testing.T, ctx context.Context, nodes []p2p.LibP2PNode, ids flow.IdentityList, n int, streams chan network.Stream, done chan struct{}) {
	defer close(done)
	var wg sync.WaitGroup
	for _, this := range nodes {
		for i, other := range nodes {
			if this == other {
				continue
			}

			pInfo, err := utils.PeerAddressInfo(*ids[i])
			require.NoError(t, err)
			this.Host().Peerstore().AddAddrs(pInfo.ID, pInfo.Addrs, peerstore.AddressTTL)

			for j := 0; j < n; j++ {
				wg.Add(1)
				go func(sender p2p.LibP2PNode) {
					defer wg.Done()
					s, err := sender.CreateStream(ctx, pInfo.ID)
					require.NoError(t, err)
					streams <- s
				}(this)
			}
		}
		// brief sleep to prevent sender and receiver dialing each other at the same time if separate goroutines resulting
		// in 2 connections 1 created by each node, this happens because we are calling CreateStream concurrently.
		time.Sleep(500 * time.Millisecond)
	}
	wg.Wait()
}

// ensureSinglePairwiseConnection ensure each node in the list has exactly one connection to every other node in the list.
func ensureSinglePairwiseConnection(t *testing.T, nodes []p2p.LibP2PNode) {
	for _, this := range nodes {
		for _, other := range nodes {
			if this == other {
				continue
			}
			require.Len(t, this.Host().Network().ConnsToPeer(other.Host().ID()), 1)
		}
	}
}
