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
	actualNumOfStreams := p2putils.CountStream(sender.Host(), receiver.ID(), defaultProtocolID, network.DirOutbound)
	require.Equal(t,
		expectedNumOfStreams,
		int64(actualNumOfStreams),
		fmt.Sprintf("expected to create %d number of streams got %d", expectedNumOfStreams, actualNumOfStreams))
}

// TestCreateStream_InboundConnResourceLimit ensures that the setting the resource limit config for
// PeerDefaultLimits.ConnsInbound restricts the number of inbound connections created from a peer to the configured value.
// NOTE: If this test becomes flaky, it indicates a violation of the single inbound connection guarantee.
// In such cases the test should not be quarantined but requires immediate resolution.
func TestCreateStream_PeerResourceLimit_NoScale(t *testing.T) {
	idProvider := mockmodule.NewIdentityProvider(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)

	sporkID := unittest.IdentifierFixture()

	// cfg, err := flowconfig.DefaultConfig()
	// require.NoError(t, err)

	// p2pbuilder.NewLimitConfigLogger(unittest.Logger()).LogResourceManagerLimits(l)

	// mem, err := p2pbuilder.AllowedMemory(cfg.NetworkConfig.MemoryLimitRatio)
	// require.NoError(t, err)
	//
	// fd, err := p2pbuilder.AllowedFileDescriptors(cfg.NetworkConfig.FileDescriptorsRatio)
	// require.NoError(t, err)
	// limits.SystemBaseLimit.StreamsInbound = 1
	// limits.SystemLimitIncrease.StreamsInbound = 1
	// limits.ProtocolBaseLimit.StreamsOutbound = 10_0000
	resourceManagerSnd, err := rcmgr.NewResourceManager(rcmgr.NewFixedLimiter(rcmgr.InfiniteLimits))
	require.NoError(t, err)
	sender, id1 := p2ptest.NodeFixture(
		t,
		sporkID,
		t.Name(),
		idProvider,
		p2ptest.WithResourceManager(resourceManagerSnd),
		p2ptest.WithCreateStreamRetryDelay(10*time.Millisecond))

	limits := rcmgr.DefaultLimits
	libp2p.SetDefaultServiceLimits(&limits)
	// mem, err := p2pbuilder.AllowedMemory(cfg.NetworkConfig.MemoryLimitRatio)
	// require.NoError(t, err)
	//
	// fd, err := p2pbuilder.AllowedFileDescriptors(cfg.NetworkConfig.FileDescriptorsRatio)
	// require.NoError(t, err)
	// limits.SystemBaseLimit.StreamsInbound = 1
	// limits.SystemLimitIncrease.StreamsInbound = 1
	// limits.ProtocolBaseLimit.StreamsOutbound = 10_0000
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
	maxAllowedStreams := l.ToPartialLimitConfig().ProtocolPeerDefault.StreamsInbound
	t.Log("maxAllowedStreamsInbound", maxAllowedStreams)
	t.Log("maxAllowedPeerStreamInbound", l.ToPartialLimitConfig().ProtocolPeerDefault.StreamsInbound)
	t.Log("maxAllowedStreamsOutboundPeer (sender)", rcmgr.InfiniteLimits.ToPartialLimitConfig().ProtocolPeerDefault.StreamsOutbound)
	t.Log("maxAllowedStreamSystemInbound", l.ToPartialLimitConfig().System.StreamsInbound)
	surplus := int64(l.ToPartialLimitConfig().System.StreamsInbound)
	errorCount := int64(0)
	for i := int64(0); i < int64(maxAllowedStreams)+surplus; i++ {
		allStreamsCreated.Add(1)
		go func() {
			defer allStreamsCreated.Done()
			_, err := sender.Host().NewStream(ctx, receiver.ID(), defaultProtocolID)
			if err != nil {
				atomic.AddInt64(&errorCount, 1) // count the number of errors
			}
		}()
	}

	unittest.RequireReturnsBefore(t, allStreamsCreated.Wait, 2*time.Second, "could not create streams on time")
	require.Len(t, receiver.Host().Network().ConnsToPeer(sender.ID()), 1)
	actualNumOfStreams := p2putils.CountStream(sender.Host(), receiver.ID(), defaultProtocolID, network.DirOutbound)
	require.Equal(t,
		int64(maxAllowedStreams),
		int64(actualNumOfStreams),
		fmt.Sprintf("expected to create %d number of streams got %d", maxAllowedStreams, actualNumOfStreams))
	require.Equalf(t, int64(surplus), atomic.LoadInt64(&errorCount), "expected to get %d errors got %d", surplus, atomic.LoadInt64(&errorCount))
}
