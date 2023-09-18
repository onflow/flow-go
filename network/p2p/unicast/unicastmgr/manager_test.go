package unicastmgr_test

import (
	"context"
	"fmt"
	"testing"

	libp2pnet "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/multiformats/go-multiaddr"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/config"
	"github.com/onflow/flow-go/module/metrics"
	mockp2p "github.com/onflow/flow-go/network/p2p/mock"
	p2ptest "github.com/onflow/flow-go/network/p2p/test"
	"github.com/onflow/flow-go/network/p2p/unicast"
	unicastcache "github.com/onflow/flow-go/network/p2p/unicast/cache"
	"github.com/onflow/flow-go/network/p2p/unicast/stream"
	"github.com/onflow/flow-go/network/p2p/unicast/unicastmgr"
	"github.com/onflow/flow-go/network/p2p/unicast/unicastmodel"
	"github.com/onflow/flow-go/utils/unittest"
)

//func unicastManagerConfigFixture(t *testing.T) *unicastmgr.Manager {
//
//}

// TODO tests: 1. Manager tries streamFactory x times for connection and y times for stream.
// TODO test: 2. when there is a no protocol issue, it does not retry it.
// TODO test: 3. After each unsuccessful attempt (failing all x times), it reduces the backoff time by one.
// TODO test: 4. When backoff time is 0, it does not back it off.
// TODO test: 5. When connection is successful, it resets the backoff times only when it passes a grace period.
// TODO test: 6. When stream time is 0, it does not back it off.
// TODO test: 7. When stream is successful, it resets the backoff time only when it passes a grace period.
// TODO test: 8. Manager exactly acts based on the dial config of the node.

// TestUnicastManager_StreamFactory_ConnectionBackoff tests the backoff mechanism of the unicast manager for connection creation.
// It tests that when there is no connection, it tries to connect to the peer some number of times (unicastmodel.MaxConnectAttempt), before
// giving up.
func TestUnicastManager_StreamFactory_ConnectionBackoff(t *testing.T) {
	connStatus := mockp2p.NewPeerConnections(t)
	streamFactory := mockp2p.NewStreamFactory(t)
	peerID := p2ptest.PeerIdFixture(t)
	peerAddr, err := multiaddr.NewMultiaddr("/ip4/127.0.0.1")
	require.NoError(t, err)

	connStatus.On("IsConnected", peerID).Return(false, nil) // not connected
	streamFactory.On("ClearBackoff", peerID).Return().Times(unicastmodel.MaxConnectAttempt)
	streamFactory.On("DialAddress", peerID).Return([]multiaddr.Multiaddr{peerAddr}) // dial address
	streamFactory.On("Connect", mock.Anything, peer.AddrInfo{ID: peerID}).
		Return(fmt.Errorf("some error")).
		Times(unicastmodel.MaxConnectAttempt) // connect
	streamFactory.On("SetStreamHandler", mock.Anything, mock.Anything).Return().Once()

	cfg, err := config.DefaultConfig()
	require.NoError(t, err)

	mgr, err := unicastmgr.NewUnicastManager(&unicastmgr.ManagerConfig{
		Logger:                    unittest.Logger(),
		StreamFactory:             streamFactory,
		SporkId:                   unittest.IdentifierFixture(),
		ConnStatus:                connStatus,
		CreateStreamRetryDelay:    cfg.NetworkConfig.UnicastCreateStreamRetryDelay,
		Metrics:                   metrics.NewNoopCollector(),
		MaxConnectionBackoffTimes: unicastmodel.MaxConnectAttempt,
		MaxStreamBackoffTimes:     unicastmodel.MaxStreamCreationAttempt,
		PeerReliabilityThreshold:  unicastmodel.PeerReliabilityThreshold,
		DialConfigCacheFactory: func() unicast.DialConfigCache {
			return unicastcache.NewDialConfigCache(unicast.DefaultDailConfigCacheSize, unittest.Logger(), metrics.NewNoopCollector(), unicastmodel.DefaultDialConfigFactory)
		},
	})
	require.NoError(t, err)
	mgr.SetDefaultHandler(func(libp2pnet.Stream) {}) // no-op handler, we don't care about the handler for this test

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s, addrs, err := mgr.CreateStream(ctx, peerID)
	require.Error(t, err)
	require.Nil(t, s)
	require.Nil(t, addrs)
}

// TestUnicastManager_StreamFactory_StreamBackoff tests the backoff mechanism of the unicast manager for stream creation.
// It tests when there is a connection, but no stream, it tries to create a stream some number of times (unicastmodel.MaxStreamCreationAttempt), before
// giving up.
func TestUnicastManager_StreamFactory_StreamBackoff(t *testing.T) {
	connStatus := mockp2p.NewPeerConnections(t)
	streamFactory := mockp2p.NewStreamFactory(t)
	peerID := p2ptest.PeerIdFixture(t)

	connStatus.On("IsConnected", peerID).Return(true, nil) // not connected
	streamFactory.On("NewStream", mock.Anything, peerID, mock.Anything).
		Return(nil, fmt.Errorf("some error")).
		Times(unicastmodel.MaxStreamCreationAttempt) // mocks that it attempts to create a stream some number of times, before giving up.
	streamFactory.On("SetStreamHandler", mock.Anything, mock.Anything).Return().Once()

	cfg, err := config.DefaultConfig()
	require.NoError(t, err)

	mgr, err := unicastmgr.NewUnicastManager(&unicastmgr.ManagerConfig{
		Logger:                    unittest.Logger(),
		StreamFactory:             streamFactory,
		SporkId:                   unittest.IdentifierFixture(),
		ConnStatus:                connStatus,
		CreateStreamRetryDelay:    cfg.NetworkConfig.UnicastCreateStreamRetryDelay,
		Metrics:                   metrics.NewNoopCollector(),
		MaxConnectionBackoffTimes: unicastmodel.MaxConnectAttempt,
		MaxStreamBackoffTimes:     unicastmodel.MaxStreamCreationAttempt,
		PeerReliabilityThreshold:  unicastmodel.PeerReliabilityThreshold,
		DialConfigCacheFactory: func() unicast.DialConfigCache {
			return unicastcache.NewDialConfigCache(unicast.DefaultDailConfigCacheSize, unittest.Logger(), metrics.NewNoopCollector(), unicastmodel.DefaultDialConfigFactory)
		},
	})
	require.NoError(t, err)
	mgr.SetDefaultHandler(func(libp2pnet.Stream) {}) // no-op handler, we don't care about the handler for this test

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s, addrs, err := mgr.CreateStream(ctx, peerID)
	require.Error(t, err)
	require.Nil(t, s)
	require.Nil(t, addrs)
}

// TestUnicastManager_StreamFactory_ErrProtocolNotSupported tests that when there is a protocol not supported error, it does not retry creating a stream.
func TestUnicastManager_StreamFactory_ErrProtocolNotSupported(t *testing.T) {
	connStatus := mockp2p.NewPeerConnections(t)
	streamFactory := mockp2p.NewStreamFactory(t)
	peerID := p2ptest.PeerIdFixture(t)

	connStatus.On("IsConnected", peerID).Return(true, nil) // connected
	streamFactory.On("NewStream", mock.Anything, peerID, mock.Anything).
		Return(nil, stream.NewProtocolNotSupportedErr(peerID, []protocol.ID{"protocol-1"}, fmt.Errorf("some error"))).
		Once() // mocks that upon creating a stream, it returns a protocol not supported error, the mock is set to once, meaning that it won't retry stream creation again.
	streamFactory.On("SetStreamHandler", mock.Anything, mock.Anything).Return().Once()

	cfg, err := config.DefaultConfig()
	require.NoError(t, err)

	mgr, err := unicastmgr.NewUnicastManager(&unicastmgr.ManagerConfig{
		Logger:                    unittest.Logger(),
		StreamFactory:             streamFactory,
		SporkId:                   unittest.IdentifierFixture(),
		ConnStatus:                connStatus,
		CreateStreamRetryDelay:    cfg.NetworkConfig.UnicastCreateStreamRetryDelay,
		Metrics:                   metrics.NewNoopCollector(),
		MaxConnectionBackoffTimes: unicastmodel.MaxConnectAttempt,
		MaxStreamBackoffTimes:     unicastmodel.MaxStreamCreationAttempt,
		PeerReliabilityThreshold:  unicastmodel.PeerReliabilityThreshold,
		DialConfigCacheFactory: func() unicast.DialConfigCache {
			return unicastcache.NewDialConfigCache(unicast.DefaultDailConfigCacheSize, unittest.Logger(), metrics.NewNoopCollector(), unicastmodel.DefaultDialConfigFactory)
		},
	})
	require.NoError(t, err)
	mgr.SetDefaultHandler(func(libp2pnet.Stream) {}) // no-op handler, we don't care about the handler for this test

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s, addrs, err := mgr.CreateStream(ctx, peerID)
	require.Error(t, err)
	require.Nil(t, s)
	require.Nil(t, addrs)
}
