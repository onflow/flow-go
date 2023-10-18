package p2pnode_test

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/network"
	rcmgr "github.com/libp2p/go-libp2p/p2p/host/resource-manager"
	"github.com/stretchr/testify/assert"
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
func TestCreateStream_SystemStreamLimit_NotEnforced(t *testing.T) {
	nodeCount := 30

	idProvider := mockmodule.NewIdentityProvider(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)

	sporkID := unittest.IdentifierFixture()

	resourceManagerSnd, err := rcmgr.NewResourceManager(rcmgr.NewFixedLimiter(rcmgr.InfiniteLimits))
	require.NoError(t, err)
	senders, senderIds := p2ptest.NodesFixture(t, sporkID, t.Name(), nodeCount,
		idProvider,
		p2ptest.WithResourceManager(resourceManagerSnd),
		p2ptest.WithCreateStreamRetryDelay(10*time.Millisecond))

	limits := rcmgr.DefaultLimits
	libp2p.SetDefaultServiceLimits(&limits)

	l := limits.Scale(0, 0)
	resourceManagerRcv, err := rcmgr.NewResourceManager(rcmgr.NewFixedLimiter(l))
	require.NoError(t, err)
	receiver, id2 := p2ptest.NodeFixture(
		t,
		sporkID,
		t.Name(),
		idProvider,
		p2ptest.WithResourceManager(resourceManagerRcv),
		p2ptest.WithCreateStreamRetryDelay(10*time.Millisecond))

	for i, sender := range senders {
		idProvider.On("ByPeerID", sender.ID()).Return(senderIds[i], true).Maybe()
	}
	idProvider.On("ByPeerID", receiver.ID()).Return(&id2, true).Maybe()

	p2ptest.StartNodes(t, signalerCtx, append(senders, receiver))
	defer p2ptest.StopNodes(t, append(senders, receiver), cancel)

	p2ptest.LetNodesDiscoverEachOther(t, signalerCtx, append(senders, receiver), append(senderIds, &id2))

	var allStreamsCreated sync.WaitGroup
	defaultProtocolID := protocols.FlowProtocolID(sporkID)
	maxInboundStreamPerPeer := l.ToPartialLimitConfig().ProtocolPeerDefault.StreamsInbound
	maxSystemInboundStream := l.ToPartialLimitConfig().System.StreamsInbound

	t.Log("max allowed inbound stream from each sender to receiver (per protocol)", maxInboundStreamPerPeer)
	t.Log("max allowed inbound stream across all peers and protocols at receiver (system-wide)", maxSystemInboundStream)

	// sanity check; if each peer creates maxInboundStreamPerPeer-1 streams, and we assume there the maxSystemInboundStream is not enforced; then to validate the hypothesis we need
	// to ensure that (maxInboundStreamPerPeer - 1) * nodeCount > maxSystemInboundStream, i.e., if each peer creates maxInboundStreamPerPeer-1 streams, then the total number of streams
	// end up being greater than the system-wide limit.
	require.Greaterf(t,
		int64(maxInboundStreamPerPeer-1)*int64(nodeCount),
		int64(maxSystemInboundStream),
		"(maxInboundStreamPerPeer - 1) * nodeCount should be greater than maxSystemInboundStream")

	for sIndex := range senders {
		sender := senders[sIndex]
		for i := int64(0); i < int64(maxInboundStreamPerPeer-1); i++ {
			allStreamsCreated.Add(1)
			go func() {
				defer allStreamsCreated.Done()
				_, err := sender.Host().NewStream(ctx, receiver.ID(), defaultProtocolID)
				require.NoError(t, err, "error creating stream")
			}()
		}
	}

	unittest.RequireReturnsBefore(t, allStreamsCreated.Wait, 2*time.Second, "could not create streams on time")

	totalStreams := 0
	for i, sender := range senders {
		actualNumOfStreams := p2putils.CountStream(sender.Host(), receiver.ID(), p2putils.Protocol(defaultProtocolID), p2putils.Direction(network.DirOutbound))
		t.Logf("sender %d has %d streams", i, actualNumOfStreams)
		assert.Equalf(t,
			int64(maxInboundStreamPerPeer-1),
			int64(actualNumOfStreams),
			"expected to create %d number of streams got %d",
			int64(maxInboundStreamPerPeer-1),
			actualNumOfStreams)
		totalStreams += actualNumOfStreams
	}

	// when system-wide limit is not enforced, the total number of streams created by all senders should be greater than the system-wide limit.
	require.Greaterf(t,
		totalStreams,
		l.ToPartialLimitConfig().Stream.StreamsInbound,
		"expected to create more than %d number of streams got %d",
		l.ToPartialLimitConfig().Stream.StreamsInbound,
		totalStreams)

	totalTrackedStreams := 0
	require.NoError(t, resourceManagerRcv.ViewTransient(func(scope network.ResourceScope) error {
		t.Logf("transient scope; inbound stream count %d", scope.Stat().NumStreamsInbound)
		totalTrackedStreams += scope.Stat().NumStreamsInbound
		return nil
	}))

	require.NoError(t, resourceManagerRcv.ViewProtocol(defaultProtocolID, func(scope network.ProtocolScope) error {
		t.Logf("protocol scope for %s; inbound stream count %d", defaultProtocolID, scope.Stat().NumStreamsInbound)
		totalTrackedStreams += scope.Stat().NumStreamsInbound
		return nil
	}))

	for _, sender := range senders {
		require.NoError(t, resourceManagerRcv.ViewPeer(sender.ID(), func(scope network.PeerScope) error {
			t.Logf("peer scope for %s; inbound stream count %d", sender.ID(), scope.Stat().NumStreamsInbound)
			totalTrackedStreams += scope.Stat().NumStreamsInbound
			return nil
		}))
	}

	require.NoError(t, resourceManagerRcv.ViewSystem(func(scope network.ResourceScope) error {
		t.Logf("system scope; inbound stream count %d", scope.Stat().NumStreamsInbound)
		totalTrackedStreams += scope.Stat().NumStreamsInbound
		return nil
	}))

	require.NoError(t, resourceManagerRcv.ViewTransient(func(scope network.ResourceScope) error {
		t.Logf("transient scope; inbound stream count %d", scope.Stat().NumStreamsInbound)
		totalTrackedStreams += scope.Stat().NumStreamsInbound
		return nil
	}))

	require.NoError(t, resourceManagerRcv.ViewProtocol(defaultProtocolID, func(scope network.ProtocolScope) error {
		t.Logf("protocol scope for %s; inbound stream count %d", defaultProtocolID, scope.Stat().NumStreamsInbound)
		totalTrackedStreams += scope.Stat().NumStreamsInbound
		return nil
	}))

	for _, sender := range senders {
		require.NoError(t, resourceManagerRcv.ViewPeer(sender.ID(), func(scope network.PeerScope) error {
			t.Logf("peer scope for %s; inbound stream count %d", sender.ID(), scope.Stat().NumStreamsInbound)
			totalTrackedStreams += scope.Stat().NumStreamsInbound
			return nil
		}))
	}

	t.Logf("total tracked streams %d", totalTrackedStreams)
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
func TestCreateStream_ResourceAllocation(t *testing.T) {
	idProvider := mockmodule.NewIdentityProvider(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)

	sporkID := unittest.IdentifierFixture()

	resourceManagerSnd, err := rcmgr.NewResourceManager(rcmgr.NewFixedLimiter(rcmgr.InfiniteLimits))
	require.NoError(t, err)
	sender, senderId := p2ptest.NodeFixture(t,
		sporkID,
		t.Name(),
		idProvider,
		p2ptest.WithResourceManager(resourceManagerSnd),
		p2ptest.WithCreateStreamRetryDelay(10*time.Millisecond))

	limits := rcmgr.DefaultLimits
	libp2p.SetDefaultServiceLimits(&limits)

	l := limits.Scale(0, 0)
	resourceManagerRcv, err := rcmgr.NewResourceManager(rcmgr.NewFixedLimiter(l))
	require.NoError(t, err)
	receiver, id2 := p2ptest.NodeFixture(t,
		sporkID,
		t.Name(),
		idProvider,
		p2ptest.WithResourceManager(resourceManagerRcv),
		p2ptest.WithCreateStreamRetryDelay(10*time.Millisecond))

	idProvider.On("ByPeerID", sender.ID()).Return(&senderId, true).Maybe()
	idProvider.On("ByPeerID", receiver.ID()).Return(&id2, true).Maybe()

	p2ptest.StartNodes(t, signalerCtx, []p2p.LibP2PNode{sender, receiver})
	defer p2ptest.StopNodes(t, []p2p.LibP2PNode{sender, receiver}, cancel)

	p2ptest.LetNodesDiscoverEachOther(t, signalerCtx, []p2p.LibP2PNode{sender, receiver}, flow.IdentityList{&senderId, &id2})

	defaultProtocolID := protocols.FlowProtocolID(sporkID)
	maxInboundStreamPerPeer := l.ToPartialLimitConfig().ProtocolPeerDefault.StreamsInbound
	maxSystemInboundStream := l.ToPartialLimitConfig().System.StreamsInbound

	t.Log("max allowed inbound stream from each sender to receiver (per protocol)", maxInboundStreamPerPeer)
	t.Log("max protocol peer limit", l.ToPartialLimitConfig().ProtocolDefault.StreamsInbound)
	t.Log("max allowed inbound stream across all peers and protocols at receiver (system-wide)", maxSystemInboundStream)

	for i := 0; i < 100; i++ {
		streamCreated := make(chan struct{})
		go func() {
			err := sender.OpenProtectedStream(ctx, receiver.ID(), t.Name(), func(stream network.Stream) error {
				close(streamCreated)
				<-ctx.Done()
				return nil

			})
			require.NoError(t, err, "error creating stream")
		}()
		<-streamCreated

		t.Logf("created stream %d", i)
		outStreamCnt := p2putils.CountStream(sender.Host(), receiver.ID(), p2putils.Protocol(defaultProtocolID), p2putils.Direction(network.DirOutbound))
		inStreamCnt := p2putils.CountStream(receiver.Host(), sender.ID(), p2putils.Protocol(defaultProtocolID), p2putils.Direction(network.DirInbound))
		t.Logf("outbound stream count %d", outStreamCnt)
		t.Logf("inbound stream count %d", inStreamCnt)
		require.NoError(t, resourceManagerRcv.ViewTransient(func(scope network.ResourceScope) error {
			t.Logf("transient scope; inbound stream count %d; inbound connections; %d", scope.Stat().NumStreamsInbound, scope.Stat().NumConnsInbound)
			return nil
		}))

		require.NoError(t, resourceManagerRcv.ViewProtocol(defaultProtocolID, func(scope network.ProtocolScope) error {
			t.Logf("protocol scope; inbound stream count %d; inbound connections; %d", scope.Stat().NumStreamsInbound, scope.Stat().NumConnsInbound)
			return nil
		}))

		require.NoError(t, resourceManagerRcv.ViewSystem(func(scope network.ResourceScope) error {
			t.Logf("system scope; inbound stream count %d; inbound connections; %d", scope.Stat().NumStreamsInbound, scope.Stat().NumConnsInbound)
			return nil
		}))

	}

	require.NoError(t, resourceManagerRcv.ViewTransient(func(scope network.ResourceScope) error {
		t.Logf("transient scope; inbound stream count %d; inbound connections; %d", scope.Stat().NumStreamsInbound, scope.Stat().NumConnsInbound)
		return nil
	}))

	require.NoError(t, resourceManagerRcv.ViewProtocol(defaultProtocolID, func(scope network.ProtocolScope) error {
		t.Logf("protocol scope; inbound stream count %d; inbound connections; %d", scope.Stat().NumStreamsInbound, scope.Stat().NumConnsInbound)
		return nil
	}))

	require.NoError(t, resourceManagerRcv.ViewSystem(func(scope network.ResourceScope) error {
		t.Logf("system scope; inbound stream count %d; inbound connections; %d", scope.Stat().NumStreamsInbound, scope.Stat().NumConnsInbound)
		return nil
	}))

	// require.NoError(t, resourceManagerRcv.ViewSystem(func(scope network.ResourceScope) error {
	// 	t.Logf("system scope")
	// 	t.Logf("inbound stream count %d", scope.Stat().NumStreamsInbound)
	// 	return nil
	// }))

	// unittest.RequireReturnsBefore(t, allStreamsCreated.Wait, 2*time.Second, "could not create streams on time")
	//
	// totalStreams := 0
	// for i, sender := range senders {
	// 	actualNumOfStreams := p2putils.CountStream(sender.Host(), receiver.ID(), defaultProtocolID, network.DirOutbound)
	// 	t.Logf("sender %d has %d streams", i, actualNumOfStreams)
	// 	assert.Equalf(t,
	// 		int64(maxInboundStreamPerPeer-1),
	// 		int64(actualNumOfStreams),
	// 		"expected to create %d number of streams got %d",
	// 		int64(maxInboundStreamPerPeer-1),
	// 		actualNumOfStreams)
	// 	totalStreams += actualNumOfStreams
	// }
	//
	// // when system-wide limit is not enforced, the total number of streams created by all senders should be greater than the system-wide limit.
	// require.Greaterf(t,
	// 	totalStreams,
	// 	l.ToPartialLimitConfig().Stream.StreamsInbound,
	// 	"expected to create more than %d number of streams got %d",
	// 	l.ToPartialLimitConfig().Stream.StreamsInbound,
	// 	totalStreams)
	//
	// require.NoError(t, resourceManagerRcv.ViewProtocol(defaultProtocolID, func(scope network.ProtocolScope) error {
	// 	t.Logf("protocol scope for %s", defaultProtocolID)
	// 	t.Logf("inbound stream count %d", scope.Stat().NumStreamsInbound)
	// 	return nil
	// }))
	//
	// require.NoError(t, resourceManagerRcv.ViewSystem(func(scope network.ResourceScope) error {
	// 	t.Logf("system scope")
	// 	t.Logf("inbound stream count %d", scope.Stat().NumStreamsInbound)
	// 	return nil
	// }))
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
func TestCreateStream_PeerLimit_Enforced(t *testing.T) {
	nodeCount := 10
	buff := 0
	maxStreamPerPeer := 5
	maxStreamProtocol := nodeCount * maxStreamPerPeer
	maxStreamPeerProtocol := maxStreamPerPeer * maxStreamProtocol
	maxTransient := nodeCount
	maxSystemStream := nodeCount

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
		t.Name(),
		nodeCount,
		idProvider,
		p2ptest.WithResourceManager(resourceManagerSnd),
		p2ptest.WithCreateStreamRetryDelay(10*time.Millisecond))

	// receiver node will run with default limits and no scaling.
	limits := rcmgr.DefaultLimits
	libp2p.SetDefaultServiceLimits(&limits)
	l := limits.Scale(0, 0)
	cfg := rcmgr.PartialLimitConfig{
		System: rcmgr.ResourceLimits{
			StreamsInbound: rcmgr.LimitVal(maxSystemStream),
			ConnsInbound:   rcmgr.LimitVal(nodeCount),
		},
		Transient: rcmgr.ResourceLimits{
			ConnsInbound:   rcmgr.LimitVal(nodeCount),
			StreamsInbound: rcmgr.LimitVal(maxTransient),
		},
		ProtocolDefault: rcmgr.ResourceLimits{
			StreamsInbound: rcmgr.LimitVal(maxStreamProtocol + buff),
		},
		ProtocolPeerDefault: rcmgr.ResourceLimits{
			StreamsInbound: rcmgr.LimitVal(maxStreamPeerProtocol + buff),
		},
		PeerDefault: rcmgr.ResourceLimits{
			StreamsInbound: rcmgr.LimitVal(maxStreamPerPeer + buff),
		},
		Conn: rcmgr.ResourceLimits{
			StreamsInbound: rcmgr.LimitVal(maxStreamPerPeer + buff),
		},
		Stream: rcmgr.ResourceLimits{
			StreamsInbound: rcmgr.LimitVal(maxStreamPerPeer + buff),
		},
	}
	l = cfg.Build(l)
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
	defaultProtocolID := protocols.FlowProtocolID(sporkID)
	// maxTransientPerPeer := l.ToPartialLimitConfig().Transient.StreamsInbound
	//
	// t.Log("max allowed inbound stream from each sender to receiver (per protocol)", maxInboundStreamPerPeer, "max transient", maxTransientPerPeer)

	totalStreamsCreated := int64(0)
	for sIndex := range senders {
		for i := int64(0); i < int64(maxStreamPerPeer); i++ {
			if i >= int64(maxSystemStream) {
				// we reached the system-wide limit; no need to create more streams; as stream creation may fail; we re-examine pressure on system-wide limit later.
				break
			}
			allStreamsCreated.Add(1)
			go func(sIndex int) {
				defer allStreamsCreated.Done()
				sender := senders[sIndex]
				_, err := sender.Host().NewStream(ctx, receiver.ID(), defaultProtocolID)
				require.NoError(t, err, "error creating stream")
				atomic.AddInt64(&totalStreamsCreated, 1)
			}(sIndex)
		}
	}

	unittest.RequireReturnsBefore(t, allStreamsCreated.Wait, 2*time.Second, "could not create streams on time")

	require.NoError(t, resourceManagerRcv.ViewTransient(func(scope network.ResourceScope) error {
		// number of in-transient streams must be less than the max transient limit
		require.Less(t, int64(scope.Stat().NumStreamsInbound), int64(maxTransient))

		// number of in-transient streams must be less than or equal the total number of streams created.
		require.LessOrEqual(t, int64(scope.Stat().NumStreamsInbound), int64(totalStreamsCreated))
		// t.Logf("transient scope; inbound stream count %d; inbound connections; %d", scope.Stat().NumStreamsInbound, scope.Stat().NumConnsInbound)
		return nil
	}))

	require.NoError(t, resourceManagerRcv.ViewSystem(func(scope network.ResourceScope) error {
		t.Logf("system scope; inbound stream count %d; inbound connections; %d", scope.Stat().NumStreamsInbound, scope.Stat().NumConnsInbound)
		return nil
	}))

	for i, sender := range senders {
		actualNumOfStreams := p2putils.CountStream(receiver.Host(), sender.ID(), p2putils.Direction(network.DirInbound))
		t.Logf("sender %d has %d streams", i, actualNumOfStreams)
		// require.Equalf(t,
		// 	int64(maxStreamPerPeer),
		// 	int64(actualNumOfStreams),
		// 	"expected to create %d number of streams got %d",
		// 	int64(maxStreamPerPeer),
		// 	actualNumOfStreams)
	}

	// now the test goes beyond the protocol-peer limit and tries to create one more stream from each sender.
	// this should cause receiver to close all streams from the sender and disconnect from the sender.
	for sIndex := range senders {
		for i := int64(0); i < int64(100); i++ {
			allStreamsCreated.Add(1)
			go func(sIndex int) {
				defer allStreamsCreated.Done()
				sender := senders[sIndex]
				_, _ = sender.Host().NewStream(ctx, receiver.ID(), defaultProtocolID)
			}(sIndex)
		}
	}

	unittest.RequireReturnsBefore(t, allStreamsCreated.Wait, 2*time.Second, "could not create streams on time")

	t.Log("-----")
	total := 0
	for i, sender := range senders {
		actualNumOfStreams := p2putils.CountStream(receiver.Host(), sender.ID(), p2putils.Direction(network.DirInbound))
		t.Logf("sender %d has %d streams", i, actualNumOfStreams)
		// require.Equalf(t,
		// 	int64(0),
		// 	int64(actualNumOfStreams),
		// 	"expected to create %d number of streams got %d",
		// 	int64(0),
		// 	actualNumOfStreams)
		total += actualNumOfStreams
	}

	require.NoError(t, resourceManagerRcv.ViewTransient(func(scope network.ResourceScope) error {
		t.Logf("transient scope; inbound stream count %d; inbound connections; %d", scope.Stat().NumStreamsInbound, scope.Stat().NumConnsInbound)
		return nil
	}))

	require.NoError(t, resourceManagerRcv.ViewProtocol(defaultProtocolID, func(scope network.ProtocolScope) error {
		t.Logf("protocol scope; inbound stream count %d; inbound connections; %d", scope.Stat().NumStreamsInbound, scope.Stat().NumConnsInbound)
		return nil
	}))

	require.NoError(t, resourceManagerRcv.ViewSystem(func(scope network.ResourceScope) error {
		t.Logf("system scope; inbound stream count %d; inbound connections; %d", scope.Stat().NumStreamsInbound, scope.Stat().NumConnsInbound)
		return nil
	}))

	t.Logf("total streams %d", total)
}
