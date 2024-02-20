package p2pnode_test

import (
	"context"
	"fmt"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/network"
	rcmgr "github.com/libp2p/go-libp2p/p2p/host/resource-manager"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/config"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	mockmodule "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/network/internal/p2putils"
	"github.com/onflow/flow-go/network/p2p"
	p2ptest "github.com/onflow/flow-go/network/p2p/test"
	"github.com/onflow/flow-go/network/p2p/unicast/protocols"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestCreateStream_InboundConnResourceLimit ensures that the setting the resource limit config for
// PeerDefaultLimits.ConnsInbound restricts the number of inbound connections created from a peer to the configured value.
// NOTE: If this test becomes flaky, it indicates a violation of the single inbound connection guarantee.
// In such cases the test should not be quarantined but requires immediate resolution.
func TestCreateStream_InboundConnResourceLimit(t *testing.T) {
	idProvider := mockmodule.NewIdentityProvider(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)

	cfg, err := config.DefaultConfig()
	require.NoError(t, err)
	cfg.NetworkConfig.Unicast.UnicastManager.CreateStreamBackoffDelay = 10 * time.Millisecond

	sporkID := unittest.IdentifierFixture()

	sender, id1 := p2ptest.NodeFixture(
		t,
		sporkID,
		t.Name(),
		idProvider,
		p2ptest.WithDefaultResourceManager(),
		p2ptest.OverrideFlowConfig(cfg))

	receiver, id2 := p2ptest.NodeFixture(
		t,
		sporkID,
		t.Name(),
		idProvider,
		p2ptest.WithDefaultResourceManager(),
		p2ptest.OverrideFlowConfig(cfg))

	idProvider.On("ByPeerID", sender.ID()).Return(&id1, true).Maybe()
	idProvider.On("ByPeerID", receiver.ID()).Return(&id2, true).Maybe()

	p2ptest.StartNodes(t, signalerCtx, []p2p.LibP2PNode{sender, receiver})
	defer p2ptest.StopNodes(t, []p2p.LibP2PNode{sender, receiver}, cancel)

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
			require.NoError(t, sender.Host().Connect(ctx, receiver.Host().Peerstore().PeerInfo(receiver.ID())))
			_, err := sender.Host().NewStream(ctx, receiver.ID(), defaultProtocolID)
			require.NoError(t, err)
		}()
	}

	unittest.RequireReturnsBefore(t, allStreamsCreated.Wait, 2*time.Second, "could not create streams on time")
	require.Len(t, receiver.Host().Network().ConnsToPeer(sender.ID()), 1)
	actualNumOfStreams := p2putils.CountStream(sender.Host(), receiver.ID(), p2putils.Protocol(defaultProtocolID), p2putils.Direction(network.DirOutbound))
	require.Equal(t,
		expectedNumOfStreams,
		int64(actualNumOfStreams),
		fmt.Sprintf("expected to create %d number of streams got %d", expectedNumOfStreams, actualNumOfStreams))
}

type testPeerLimitConfig struct {
	// nodeCount is the number of nodes in the test.
	nodeCount int

	// maxInboundPeerStream is the maximum number of inbound streams from a single peer to the receiver.
	maxInboundPeerStream int

	// maxInboundStreamProtocol is the maximum number of inbound streams at the receiver using a specific protocol; it accumulates all streams from all senders.
	maxInboundStreamProtocol int

	// maxInboundStreamPeerProtocol is the maximum number of inbound streams at the receiver from a single peer using a specific protocol.
	maxInboundStreamPeerProtocol int

	// maxInboundStreamTransient is the maximum number of inbound transient streams at the receiver; it accumulates all streams from all senders across all protocols.
	// transient streams are those that are not associated fully with a peer and protocol.
	maxInboundStreamTransient int

	// maxInboundStreamSystem is the maximum number of inbound streams at the receiver; it accumulates all streams from all senders across all protocols.
	maxInboundStreamSystem int

	// unknownProtocol when set to true will cause senders to use an unknown protocol ID when creating streams.
	unknownProtocol bool
}

// maxLimit returns the maximum limit across all limits.
func (t testPeerLimitConfig) maxLimit() int {
	max := 0
	if t.maxInboundPeerStream > max && t.maxInboundPeerStream != math.MaxInt {
		max = t.maxInboundPeerStream
	}
	if t.maxInboundStreamProtocol > max && t.maxInboundStreamProtocol != math.MaxInt {
		max = t.maxInboundStreamProtocol
	}
	if t.maxInboundStreamPeerProtocol > max && t.maxInboundStreamPeerProtocol != math.MaxInt {
		max = t.maxInboundStreamPeerProtocol
	}
	if t.maxInboundStreamTransient > max && t.maxInboundStreamTransient != math.MaxInt {
		max = t.maxInboundStreamTransient
	}
	if t.maxInboundStreamSystem > max && t.maxInboundStreamSystem != math.MaxInt {
		max = t.maxInboundStreamSystem
	}
	return max
}

// baseCreateStreamInboundStreamResourceLimitConfig returns a testPeerLimitConfig with default values.
func baseCreateStreamInboundStreamResourceLimitConfig() *testPeerLimitConfig {
	return &testPeerLimitConfig{
		nodeCount:                    10,
		maxInboundPeerStream:         100,
		maxInboundStreamProtocol:     100,
		maxInboundStreamPeerProtocol: 100,
		maxInboundStreamTransient:    100,
		maxInboundStreamSystem:       100,
	}
}

func TestCreateStream_DefaultConfig(t *testing.T) {
	testCreateStreamInboundStreamResourceLimits(t, baseCreateStreamInboundStreamResourceLimitConfig())
}

func TestCreateStream_MinPeerLimit(t *testing.T) {
	base := baseCreateStreamInboundStreamResourceLimitConfig()
	base.maxInboundPeerStream = 1
	testCreateStreamInboundStreamResourceLimits(t, base)
}

func TestCreateStream_MaxPeerLimit(t *testing.T) {

	base := baseCreateStreamInboundStreamResourceLimitConfig()
	base.maxInboundPeerStream = math.MaxInt
	testCreateStreamInboundStreamResourceLimits(t, base)
}

func TestCreateStream_MinProtocolLimit(t *testing.T) {
	// max inbound protocol is not preserved; can be partially due to count stream not counting inbound streams on a protocol
	unittest.SkipUnless(t, unittest.TEST_TODO, "broken test")
	base := baseCreateStreamInboundStreamResourceLimitConfig()
	base.maxInboundStreamProtocol = 1
	testCreateStreamInboundStreamResourceLimits(t, base)
}

func TestCreateStream_MaxProtocolLimit(t *testing.T) {
	base := baseCreateStreamInboundStreamResourceLimitConfig()
	base.maxInboundStreamProtocol = math.MaxInt
	testCreateStreamInboundStreamResourceLimits(t, base)
}

func TestCreateStream_MinPeerProtocolLimit(t *testing.T) {
	// max inbound stream peer protocol is not preserved; can be partially due to count stream not counting inbound streams on a protocol
	unittest.SkipUnless(t, unittest.TEST_TODO, "broken test")
	base := baseCreateStreamInboundStreamResourceLimitConfig()
	base.maxInboundStreamPeerProtocol = 1
	testCreateStreamInboundStreamResourceLimits(t, base)
}

func TestCreateStream_MaxPeerProtocolLimit(t *testing.T) {
	base := baseCreateStreamInboundStreamResourceLimitConfig()
	base.maxInboundStreamPeerProtocol = math.MaxInt
	testCreateStreamInboundStreamResourceLimits(t, base)
}

func TestCreateStream_MinTransientLimit(t *testing.T) {
	base := baseCreateStreamInboundStreamResourceLimitConfig()
	base.maxInboundStreamTransient = 1
	testCreateStreamInboundStreamResourceLimits(t, base)
}

func TestCreateStream_MaxTransientLimit(t *testing.T) {
	base := baseCreateStreamInboundStreamResourceLimitConfig()
	base.maxInboundStreamTransient = math.MaxInt
	testCreateStreamInboundStreamResourceLimits(t, base)
}

func TestCreateStream_MinSystemLimit(t *testing.T) {
	base := baseCreateStreamInboundStreamResourceLimitConfig()
	base.maxInboundStreamSystem = 1
	testCreateStreamInboundStreamResourceLimits(t, base)
}

func TestCreateStream_MaxSystemLimit(t *testing.T) {
	// max inbound stream protocol is not preserved; can be partially due to count stream not counting inbound streams on a protocol
	unittest.SkipUnless(t, unittest.TEST_TODO, "broken test")
	base := baseCreateStreamInboundStreamResourceLimitConfig()
	base.maxInboundStreamSystem = math.MaxInt
	testCreateStreamInboundStreamResourceLimits(t, base)
}

func TestCreateStream_DefaultConfigWithUnknownProtocol(t *testing.T) {
	// limits are not enforced when using an unknown protocol ID
	unittest.SkipUnless(t, unittest.TEST_TODO, "broken test")
	base := baseCreateStreamInboundStreamResourceLimitConfig()
	base.unknownProtocol = true
	testCreateStreamInboundStreamResourceLimits(t, base)
}

func TestCreateStream_PeerLimitLessThanPeerProtocolLimit(t *testing.T) {
	// the case where peer-level limit is lower than the peer-protocol-level limit.
	base := baseCreateStreamInboundStreamResourceLimitConfig()
	base.maxInboundPeerStream = 5          // each peer can only create 5 streams.
	base.maxInboundStreamPeerProtocol = 10 // each peer can create 10 streams on a specific protocol (but should still be limited by the peer-level limit).
	testCreateStreamInboundStreamResourceLimits(t, base)
}

func TestCreateStream_PeerLimitGreaterThanPeerProtocolLimit(t *testing.T) {
	// the case where peer-level limit is higher than the peer-protocol-level limit.
	// max inbound stream peer protocol is not preserved; can be partially due to count stream not counting inbound streams on a protocol
	unittest.SkipUnless(t, unittest.TEST_TODO, "broken test")
	base := baseCreateStreamInboundStreamResourceLimitConfig()
	base.maxInboundPeerStream = 10        // each peer can create 10 streams.
	base.maxInboundStreamPeerProtocol = 5 // each peer can create 5 streams on a specific protocol.
	base.maxInboundStreamProtocol = 100   // overall limit is 100 streams on a specific protocol (across all peers).
	base.maxInboundStreamTransient = 1000 // overall limit is 1000 transient streams.
	base.maxInboundStreamSystem = 1000    // overall limit is 1000 system-wide streams.
	testCreateStreamInboundStreamResourceLimits(t, base)
}

func TestCreateStream_ProtocolLimitLessThanPeerProtocolLimit(t *testing.T) {
	// max inbound stream peer protocol is not preserved; can be partially due to count stream not counting inbound streams on a protocol
	unittest.SkipUnless(t,
		unittest.TEST_TODO, "broken test")
	// the case where protocol-level limit is lower than the peer-protocol-level limit.
	base := baseCreateStreamInboundStreamResourceLimitConfig()
	base.maxInboundStreamProtocol = 5      // each peer can create 5 streams on a specific protocol.
	base.maxInboundStreamPeerProtocol = 10 // each peer can create 10 streams on a specific protocol (but should still be limited by the protocol-level limit).
	testCreateStreamInboundStreamResourceLimits(t, base)
}

func TestCreateStream_ProtocolLimitGreaterThanPeerProtocolLimit(t *testing.T) {
	// TODO: with libp2p upgrade to v0.32.2; this test is failing as the peer protocol limit is not being enforced, and
	// rather the protocol limit is being enforced, this test expects each peer not to be allowed more than 5 streams on a specific protocol.
	// However, the maximum number of streams on a specific protocol (and specific protocol) are being enforced instead.
	// A quick investigation shows that it may be due to the way libp2p treats our unicast protocol (it is not a limit-enforcing protocol).
	// But further investigation is required to confirm this.
	unittest.SkipUnless(t, unittest.TEST_TODO, "broken test")
	// the case where protocol-level limit is higher than the peer-protocol-level limit.
	base := baseCreateStreamInboundStreamResourceLimitConfig()
	base.maxInboundStreamProtocol = 10    // overall limit is 10 streams on a specific protocol (across all peers).
	base.maxInboundStreamPeerProtocol = 5 // each peer can create 5 streams on a specific protocol.
	base.maxInboundStreamTransient = 1000 // overall limit is 1000 transient streams.
	base.maxInboundStreamSystem = 1000    // overall limit is 1000 system-wide streams.
	testCreateStreamInboundStreamResourceLimits(t, base)
}

func TestCreateStream_TransientLimitLessThanPeerProtocolLimit(t *testing.T) {
	// the case where transient-level limit is lower than the peer-protocol-level limit.
	base := baseCreateStreamInboundStreamResourceLimitConfig()
	base.maxInboundStreamTransient = 5     // overall limit is 5 transient streams (across all peers).
	base.maxInboundStreamPeerProtocol = 10 // each peer can create 10 streams on a specific protocol (but should still be limited by the transient-level limit).
	testCreateStreamInboundStreamResourceLimits(t, base)
}

// testCreateStreamInboundStreamResourceLimits tests the inbound stream limits for a given testPeerLimitConfig. It creates
// a number of senders and a single receiver. The receiver will have a resource manager with the given limits.
// The senders will have a resource manager with infinite limits to ensure that they can create as many streams as they want.
// The test will create a number of streams from each sender to the receiver. The test will then check that the limits are
// being enforced correctly.
// The number of streams is determined by the maxLimit() of the testPeerLimitConfig, which is the maximum limit across all limits (peer, protocol, transient, system), excluding
// the math.MaxInt limits.
func testCreateStreamInboundStreamResourceLimits(t *testing.T, cfg *testPeerLimitConfig) {
	idProvider := mockmodule.NewIdentityProvider(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)
	sporkID := unittest.IdentifierFixture()

	flowCfg, err := config.DefaultConfig()
	require.NoError(t, err)
	flowCfg.NetworkConfig.Unicast.UnicastManager.CreateStreamBackoffDelay = 10 * time.Millisecond

	// sender nodes will have infinite stream limit to ensure that they can create as many streams as they want.
	resourceManagerSnd, err := rcmgr.NewResourceManager(rcmgr.NewFixedLimiter(rcmgr.InfiniteLimits))
	require.NoError(t, err)
	senders, senderIds := p2ptest.NodesFixture(t,
		sporkID,
		t.Name(), cfg.nodeCount,
		idProvider,
		p2ptest.WithResourceManager(resourceManagerSnd),
		p2ptest.OverrideFlowConfig(flowCfg))

	// receiver node will run with default limits and no scaling.
	limits := rcmgr.DefaultLimits
	libp2p.SetDefaultServiceLimits(&limits)
	l := limits.Scale(0, 0)
	partial := rcmgr.PartialLimitConfig{
		System: rcmgr.ResourceLimits{
			StreamsInbound: rcmgr.LimitVal(cfg.maxInboundStreamSystem),
			ConnsInbound:   rcmgr.LimitVal(cfg.nodeCount),
		},
		Transient: rcmgr.ResourceLimits{
			ConnsInbound:   rcmgr.LimitVal(cfg.nodeCount),
			StreamsInbound: rcmgr.LimitVal(cfg.maxInboundStreamTransient),
		},
		ProtocolDefault: rcmgr.ResourceLimits{
			StreamsInbound: rcmgr.LimitVal(cfg.maxInboundStreamProtocol),
		},
		ProtocolPeerDefault: rcmgr.ResourceLimits{
			StreamsInbound: rcmgr.LimitVal(cfg.maxInboundStreamPeerProtocol),
		},
		PeerDefault: rcmgr.ResourceLimits{
			StreamsInbound: rcmgr.LimitVal(cfg.maxInboundPeerStream),
		},
		Conn: rcmgr.ResourceLimits{
			StreamsInbound: rcmgr.LimitVal(cfg.maxInboundPeerStream),
		},
		Stream: rcmgr.ResourceLimits{
			StreamsInbound: rcmgr.LimitVal(cfg.maxInboundPeerStream),
		},
	}
	l = partial.Build(l)
	resourceManagerRcv, err := rcmgr.NewResourceManager(rcmgr.NewFixedLimiter(l))
	require.NoError(t, err)
	receiver, id2 := p2ptest.NodeFixture(t,
		sporkID,
		t.Name(),
		idProvider,
		p2ptest.WithResourceManager(resourceManagerRcv),
		p2ptest.OverrideFlowConfig(flowCfg))

	for i, sender := range senders {
		idProvider.On("ByPeerID", sender.ID()).Return(senderIds[i], true).Maybe()
	}
	idProvider.On("ByPeerID", receiver.ID()).Return(&id2, true).Maybe()

	nodes := append(senders, receiver)
	ids := append(senderIds, &id2)

	p2ptest.StartNodes(t, signalerCtx, nodes)
	defer p2ptest.StopNodes(t, nodes, cancel)

	p2ptest.LetNodesDiscoverEachOther(t, signalerCtx, nodes, ids)

	var allStreamsCreated sync.WaitGroup

	protocolID := protocols.FlowProtocolID(sporkID)
	if cfg.unknownProtocol {
		protocolID = protocols.FlowProtocolID(unittest.IdentifierFixture())
	}

	loadLimit := cfg.maxLimit()
	require.Greaterf(t, loadLimit, 0, "test limit must be greater than 0; got %d", loadLimit)

	streamListMu := sync.Mutex{}             // mutex to protect the streamsList.
	streamsList := make([]network.Stream, 0) // list of all streams created to avoid garbage collection.
	for sIndex := range senders {
		for i := 0; i < loadLimit; i++ {
			allStreamsCreated.Add(1)
			go func(sIndex int) {
				defer allStreamsCreated.Done()
				sender := senders[sIndex]
				s, err := sender.Host().NewStream(ctx, receiver.ID(), protocolID)
				if err != nil {
					// we don't care about the error here; as we are trying to break a limit; so we expect some of the stream creations to fail.
					return
				}

				require.NotNil(t, s)
				streamListMu.Lock()
				streamsList = append(streamsList, s)
				streamListMu.Unlock()
			}(sIndex)
		}
	}

	unittest.RequireReturnsBefore(t, allStreamsCreated.Wait, 2*time.Second, "could not create streams on time")

	// transient sanity-check
	require.NoError(t, resourceManagerRcv.ViewTransient(func(scope network.ResourceScope) error {
		// number of in-transient streams must be less than or equal to the max transient limit
		require.LessOrEqual(t, int64(scope.Stat().NumStreamsInbound), int64(cfg.maxInboundStreamTransient))

		// number of in-transient streams must be less than or equal the total number of streams created.
		require.LessOrEqual(t, int64(scope.Stat().NumStreamsInbound), int64(len(streamsList)))
		return nil
	}))

	// system-wide limit sanity-check
	require.NoError(t, resourceManagerRcv.ViewSystem(func(scope network.ResourceScope) error {
		require.LessOrEqual(t, int64(scope.Stat().NumStreamsInbound), int64(cfg.maxInboundStreamSystem), "system-wide limit is not being enforced")
		return nil
	}))

	totalInboundStreams := 0
	for _, sender := range senders {
		actualNumOfStreams := p2putils.CountStream(receiver.Host(), sender.ID(), p2putils.Direction(network.DirInbound))
		// number of inbound streams must be less than or equal to the peer-level limit for each sender.
		require.LessOrEqual(t, int64(actualNumOfStreams), int64(cfg.maxInboundPeerStream))
		require.LessOrEqual(t, int64(actualNumOfStreams), int64(cfg.maxInboundStreamPeerProtocol))
		totalInboundStreams += actualNumOfStreams
	}
	// sanity check; the total number of inbound streams must be less than or equal to the system-wide limit.
	// TODO: this must be a hard equal check; but falls short; to be shared with libp2p community.
	// Failing at this line means the system-wide limit is not being enforced.
	require.LessOrEqual(t, totalInboundStreams, cfg.maxInboundStreamSystem)
	// sanity check; the total number of inbound streams must be less than or equal to the protocol-level limit.
	require.LessOrEqual(t, totalInboundStreams, cfg.maxInboundStreamProtocol)
}
