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
	max := t.maxInboundPeerStream
	if t.maxInboundStreamProtocol > max {
		max = t.maxInboundStreamProtocol
	}
	if t.maxInboundStreamPeerProtocol > max {
		max = t.maxInboundStreamPeerProtocol
	}
	if t.maxInboundStreamTransient > max {
		max = t.maxInboundStreamTransient
	}
	if t.maxInboundStreamSystem > max {
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
	unittest.SkipUnless(t,
		unittest.TEST_TODO,
		"max inbound stream peer protocol is not preserved; can be partially due to count steam not counting inbound streams on a protocol")
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
	base := baseCreateStreamInboundStreamResourceLimitConfig()
	base.maxInboundStreamSystem = math.MaxInt
	testCreateStreamInboundStreamResourceLimits(t, base)
}

func TestCreateStream_DefaultConfigWithUnknownProtocol(t *testing.T) {
	unittest.SkipUnless(t,
		unittest.TEST_TODO,
		"limits are not enforced when using an unknown protocol ID")
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
	unittest.SkipUnless(t,
		unittest.TEST_TODO,
		"max inbound stream peer protocol is not preserved; can be partially due to count steam not counting inbound streams on a protocol")
	base := baseCreateStreamInboundStreamResourceLimitConfig()
	base.maxInboundPeerStream = 10        // each peer can create 10 streams.
	base.maxInboundStreamPeerProtocol = 5 // each peer can create 5 streams on a specific protocol.
	base.maxInboundStreamProtocol = 100   // overall limit is 100 streams on a specific protocol (across all peers).
	base.maxInboundStreamTransient = 1000 // overall limit is 1000 transient streams.
	base.maxInboundStreamSystem = 1000    // overall limit is 1000 system-wide streams.
	testCreateStreamInboundStreamResourceLimits(t, base)
}

func TestCreateStream_ProtocolLimitLessThanPeerProtocolLimit(t *testing.T) {
	// the case where protocol-level limit is lower than the peer-protocol-level limit.
	base := baseCreateStreamInboundStreamResourceLimitConfig()
	base.maxInboundStreamProtocol = 5      // each peer can create 5 streams on a specific protocol.
	base.maxInboundStreamPeerProtocol = 10 // each peer can create 10 streams on a specific protocol (but should still be limited by the protocol-level limit).
	testCreateStreamInboundStreamResourceLimits(t, base)
}

func TestCreateStream_ProtocolLimitGreaterThanPeerProtocolLimit(t *testing.T) {
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

// TestCreateStream_SystemStreamLimit_NotEnforced is a re-production of a hypothetical bug where the system-wide inbound stream limit of libp2p resource management
// was not being enforced. The purpose of this test is to share with the libp2p community as well as to evaluate the existence of the bug on
// future libp2p versions.
// Test scenario works as follows:
//   - We have 30 senders and 1 receiver.
//   - The senders are running with a resource manager that allows infinite number of streams; so that they can create as many streams as they want.
//   - The receiver is running with a resource manager with base limits and no scaling.
//   - The test reads the peer protocol default limits for inbound streams at receiver; say x; which is the limit for the number of inbound streams from each sender on a
//     specific protocol.
//   - Each sender creates x-1 streams to the receiver on a specific protocol. This is done to ensure that the receiver has x-1 streams from each sender; a total of
//     30*(x-1) streams at the receiver.
//   - Test first ensures that numerically 30 * (x - 1) > max system-wide inbound stream limit; i.e., the total number of streams created by all senders is greater than
//     the system-wide limit.
//   - Then each sender creates x - 1 streams concurrently to the receiver.
//   - At the end of the test we ensure that the total number of streams created by all senders is greater than the system-wide limit; which should not be the case if the
//     system-wide limit is being enforced.
func testCreateStreamInboundStreamResourceLimits(t *testing.T, cfg *testPeerLimitConfig) {
	idProvider := mockmodule.NewIdentityProvider(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)

	sporkID := unittest.IdentifierFixture()

	// sender nodes will have infinite stream limit to ensure that they can create as many streams as they want.
	resourceManagerSnd, err := rcmgr.NewResourceManager(rcmgr.NewFixedLimiter(rcmgr.InfiniteLimits))
	require.NoError(t, err)
	senders, senderIds := p2ptest.NodesFixture(t,
		sporkID,
		t.Name(), cfg.nodeCount,
		idProvider,
		p2ptest.WithResourceManager(resourceManagerSnd),
		p2ptest.WithCreateStreamRetryDelay(10*time.Millisecond))

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
		p2ptest.WithCreateStreamRetryDelay(10*time.Millisecond))

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

	// creates max(maxInboundStreamPeerProtocol * nodeCount, maxInboundStreamSystem) streams from each sender to the receiver; breaks as soon as the system-wide limit is reached.
	totalStreamAttempted := int64(0) // total number of stream creation attempts.

	streamListMu := sync.Mutex{}             // mutex to protect the streamsList.
	streamsList := make([]network.Stream, 0) // list of all streams created to avoid garbage collection.
	for sIndex := range senders {
		for i := int64(0); i < int64(cfg.maxInboundStreamPeerProtocol); i++ {
			totalStreamAttempted++
			if totalStreamAttempted >= int64(cfg.maxInboundStreamSystem) {
				// we reached the system-wide limit; no need to create more streams; as stream creation may fail; we re-examine pressure on system-wide limit later.
				break
			}
			allStreamsCreated.Add(1)
			go func(sIndex int) {
				defer allStreamsCreated.Done()
				sender := senders[sIndex]
				s, err := sender.Host().NewStream(ctx, receiver.ID(), protocolID)
				if err != nil {
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

	require.NoError(t, resourceManagerRcv.ViewTransient(func(scope network.ResourceScope) error {
		// number of in-transient streams must be less than or equal to the max transient limit
		require.LessOrEqual(t, int64(scope.Stat().NumStreamsInbound), int64(cfg.maxInboundStreamTransient))

		// number of in-transient streams must be less than or equal the total number of streams created.
		require.LessOrEqual(t, int64(scope.Stat().NumStreamsInbound), int64(len(streamsList)))
		// t.Logf("transient scope; inbound stream count %d; inbound connections; %d", scope.Stat().NumStreamsInbound, scope.Stat().NumConnsInbound)
		return nil
	}))

	require.NoError(t, resourceManagerRcv.ViewSystem(func(scope network.ResourceScope) error {
		t.Logf("system scope; inbound stream count %d; inbound connections; %d", scope.Stat().NumStreamsInbound, scope.Stat().NumConnsInbound)
		return nil
	}))

	totalInboundStreams := 0
	for _, sender := range senders {
		actualNumOfStreams := p2putils.CountStream(receiver.Host(), sender.ID(), p2putils.Direction(network.DirInbound))
		// t.Logf("sender %d has %d streams", i, actualNumOfStreams)
		require.LessOrEqual(t, int64(actualNumOfStreams), int64(cfg.maxInboundPeerStream))
		totalInboundStreams += actualNumOfStreams
	}
	// sanity check; the total number of inbound streams must be less than or equal to the system-wide limit.
	// TODO: this must be a hard equal check; but falls short; to be shared with libp2p community.
	// Failing at this line means the system-wide limit is not being enforced.
	require.LessOrEqual(t, totalInboundStreams, cfg.maxInboundStreamSystem)

	// now the stress testing with each sender making `maxInboundStreamSystem` concurrent streams to the receiver.
	for sIndex := range senders {
		for i := int64(0); i < int64(cfg.maxInboundStreamSystem); i++ {
			allStreamsCreated.Add(1)
			go func(sIndex int) {
				defer allStreamsCreated.Done()
				sender := senders[sIndex]
				// we don't care about the error here; as we are trying to create more streams than the system-wide limit; so we expect some of the stream creations to fail.
				_, _ = sender.Host().NewStream(ctx, receiver.ID(), protocolID)
			}(sIndex)
		}
	}

	unittest.RequireReturnsBefore(t, allStreamsCreated.Wait, 2*time.Second, "could not create (stress-testing) streams on time")

	totalInboundStreams = 0
	for _, sender := range senders {
		actualNumOfStreams := p2putils.CountStream(receiver.Host(), sender.ID(), p2putils.Direction(network.DirInbound), p2putils.Protocol(""))
		require.LessOrEqual(t, actualNumOfStreams, cfg.maxInboundPeerStream)
		require.LessOrEqual(t, actualNumOfStreams, cfg.maxInboundStreamPeerProtocol)
		totalInboundStreams += actualNumOfStreams
	}
	// sanity check; the total number of inbound streams must be less than or equal to the system-wide limit.
	// TODO: this must be a hard equal check; but falls short; to be shared with libp2p community.
	// Failing at this line means the system-wide limit is not being enforced.
	require.LessOrEqual(t, totalInboundStreams, cfg.maxInboundStreamSystem)
	require.LessOrEqual(t, totalInboundStreams, cfg.maxInboundStreamTransient)
}
