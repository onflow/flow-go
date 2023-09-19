package unicastmgr_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	libp2pnet "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
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

// TODO test: When the connection is created, it is added to the stream history.
// TODO test: 5. When connection is successful, it resets the backoff times only when it passes a grace period.
// TODO test: 6. When stream time is 0, it does not back it off.
// TODO test: 7. When stream is successful, it resets the backoff time only when it passes a grace period.
// TODO test: 8. Manager exactly acts based on the dial config of the node.

// TestUnicastManager_StreamFactory_ConnectionBackoff tests the backoff mechanism of the unicast manager for connection creation.
// It tests that when there is no connection, it tries to connect to the peer some number of times (unicastmodel.MaxConnectAttempt), before
// giving up.
func TestUnicastManager_Connection_ConnectionBackoff(t *testing.T) {
	connStatus := mockp2p.NewPeerConnections(t)
	streamFactory := mockp2p.NewStreamFactory(t)
	peerID := p2ptest.PeerIdFixture(t)

	connStatus.On("IsConnected", peerID).Return(false, nil) // not connected
	streamFactory.On("ClearBackoff", peerID).Return().Times(unicastmodel.MaxConnectAttempt)
	streamFactory.On("Connect", mock.Anything, peer.AddrInfo{ID: peerID}).
		Return(fmt.Errorf("some error")).
		Times(unicastmodel.MaxConnectAttempt) // connect
	streamFactory.On("SetStreamHandler", mock.Anything, mock.Anything).Return().Once()

	cfg, err := config.DefaultConfig()
	require.NoError(t, err)
	dialConfigCache := unicastcache.NewDialConfigCache(unicast.DefaultDailConfigCacheSize, unittest.Logger(), metrics.NewNoopCollector(), unicastmodel.DefaultDialConfigFactory)
	_, err = dialConfigCache.Adjust(
		peerID, func(dialConfig unicastmodel.DialConfig) (unicastmodel.DialConfig, error) {
			// assumes that there was a successful connection to the peer before (2 minutes ago), and now the connection is lost.
			dialConfig.LastSuccessfulDial = time.Now().Add(2 * time.Minute)
			return dialConfig, nil
		},
	)
	require.NoError(t, err)

	mgr, err := unicastmgr.NewUnicastManager(
		&unicastmgr.ManagerConfig{
			Logger:                     unittest.Logger(),
			StreamFactory:              streamFactory,
			SporkId:                    unittest.IdentifierFixture(),
			ConnStatus:                 connStatus,
			CreateStreamRetryDelay:     cfg.NetworkConfig.UnicastCreateStreamRetryDelay,
			Metrics:                    metrics.NewNoopCollector(),
			StreamHistoryResetInterval: unicastmodel.StreamHistoryResetInterval,
			DialConfigCacheFactory: func() unicast.DialConfigCache {
				return dialConfigCache
			},
		},
	)
	require.NoError(t, err)
	mgr.SetDefaultHandler(func(libp2pnet.Stream) {}) // no-op handler, we don't care about the handler for this test

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s, err := mgr.CreateStream(ctx, peerID)
	require.Error(t, err)
	require.Nil(t, s)

	// The dial config must be updated with the backoff budget decremented.
	dialCfg, err := dialConfigCache.GetOrInit(peerID)
	require.NoError(t, err)
	require.Equal(t, uint64(unicastmodel.MaxConnectAttempt-1), dialCfg.DialBackoffBudget)     // dial backoff budget must be decremented by 1.
	require.Equal(t, uint64(unicastmodel.MaxStreamCreationAttempt), dialCfg.StreamBackBudget) // stream backoff budget must remain intact (no stream creation attempt yet).
	// last successful dial is set back to zero, since although we have a successful dial in the past, the most recent dial failed.
	require.True(t, dialCfg.LastSuccessfulDial.IsZero())
	require.Equal(t, uint64(0), dialCfg.ConsecutiveSuccessfulStream) // consecutive successful stream must be intact.
}

// TestUnicastManager_StreamFactory_StreamBackoff tests the backoff mechanism of the unicast manager for stream creation.
// It tests when there is a connection, but no stream, it tries to create a stream some number of times (unicastmodel.MaxStreamCreationAttempt), before
// giving up.
func TestUnicastManager_StreamFactory_StreamBackoff(t *testing.T) {
	connStatus := mockp2p.NewPeerConnections(t)
	streamFactory := mockp2p.NewStreamFactory(t)
	peerID := p2ptest.PeerIdFixture(t)

	connStatus.On("IsConnected", peerID).Return(true, nil) // connected.
	streamFactory.On("NewStream", mock.Anything, peerID, mock.Anything).
		Return(nil, fmt.Errorf("some error")).
		Times(unicastmodel.MaxStreamCreationAttempt) // mocks that it attempts to create a stream some number of times, before giving up.
	streamFactory.On("SetStreamHandler", mock.Anything, mock.Anything).Return().Once()

	cfg, err := config.DefaultConfig()
	require.NoError(t, err)

	dialConfigCache := unicastcache.NewDialConfigCache(unicast.DefaultDailConfigCacheSize, unittest.Logger(), metrics.NewNoopCollector(), unicastmodel.DefaultDialConfigFactory)
	mgr, err := unicastmgr.NewUnicastManager(
		&unicastmgr.ManagerConfig{
			Logger:                     unittest.Logger(),
			StreamFactory:              streamFactory,
			SporkId:                    unittest.IdentifierFixture(),
			ConnStatus:                 connStatus,
			CreateStreamRetryDelay:     cfg.NetworkConfig.UnicastCreateStreamRetryDelay,
			Metrics:                    metrics.NewNoopCollector(),
			StreamHistoryResetInterval: unicastmodel.StreamHistoryResetInterval,
			DialConfigCacheFactory: func() unicast.DialConfigCache {
				return dialConfigCache
			},
		},
	)
	require.NoError(t, err)
	mgr.SetDefaultHandler(func(libp2pnet.Stream) {}) // no-op handler, we don't care about the handler for this test

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s, err := mgr.CreateStream(ctx, peerID)
	require.Error(t, err)
	require.Nil(t, s)

	// The dial config must be updated with the stream backoff budget decremented.
	dialCfg, err := dialConfigCache.GetOrInit(peerID)
	require.NoError(t, err)
	require.Equal(t, uint64(unicastmodel.MaxConnectAttempt), dialCfg.DialBackoffBudget)         // dial backoff budget must be intact.
	require.Equal(t, uint64(unicastmodel.MaxStreamCreationAttempt-1), dialCfg.StreamBackBudget) // stream backoff budget must be decremented by 1.
	require.Equal(t, uint64(0), dialCfg.ConsecutiveSuccessfulStream)                            // consecutive successful stream must be zero as we have not created a successful stream yet.
}

// TestUnicastManager_Stream_ConsecutiveStreamCreation_Increment tests that when there is a connection, and the stream creation is successful,
// it increments the consecutive successful stream counter in the dial config.
func TestUnicastManager_Stream_ConsecutiveStreamCreation_Increment(t *testing.T) {
	connStatus := mockp2p.NewPeerConnections(t)
	streamFactory := mockp2p.NewStreamFactory(t)
	peerID := p2ptest.PeerIdFixture(t)

	// total times we successfully create a stream to the peer.
	totalSuccessAttempts := 10

	connStatus.On("IsConnected", peerID).Return(true, nil) // connected.
	// mocks that it attempts to create a stream 10 times, and each time it succeeds.
	streamFactory.On("NewStream", mock.Anything, peerID, mock.Anything).Return(&p2ptest.MockStream{}, nil).Times(totalSuccessAttempts)
	streamFactory.On("SetStreamHandler", mock.Anything, mock.Anything).Return().Once()

	cfg, err := config.DefaultConfig()
	require.NoError(t, err)

	dialConfigCache := unicastcache.NewDialConfigCache(unicast.DefaultDailConfigCacheSize, unittest.Logger(), metrics.NewNoopCollector(), unicastmodel.DefaultDialConfigFactory)
	mgr, err := unicastmgr.NewUnicastManager(
		&unicastmgr.ManagerConfig{
			Logger:                     unittest.Logger(),
			StreamFactory:              streamFactory,
			SporkId:                    unittest.IdentifierFixture(),
			ConnStatus:                 connStatus,
			CreateStreamRetryDelay:     cfg.NetworkConfig.UnicastCreateStreamRetryDelay,
			Metrics:                    metrics.NewNoopCollector(),
			StreamHistoryResetInterval: unicastmodel.StreamHistoryResetInterval,
			DialConfigCacheFactory: func() unicast.DialConfigCache {
				return dialConfigCache
			},
		},
	)
	require.NoError(t, err)
	mgr.SetDefaultHandler(func(libp2pnet.Stream) {}) // no-op handler, we don't care about the handler for this test

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for i := 0; i < totalSuccessAttempts; i++ {
		s, err := mgr.CreateStream(ctx, peerID)
		require.NoError(t, err)
		require.NotNil(t, s)

		// The dial config must be updated with the stream backoff budget decremented.
		dialCfg, err := dialConfigCache.GetOrInit(peerID)
		require.NoError(t, err)
		require.Equal(t, uint64(unicastmodel.MaxConnectAttempt), dialCfg.DialBackoffBudget)       // dial backoff budget must be intact.
		require.Equal(t, uint64(unicastmodel.MaxStreamCreationAttempt), dialCfg.StreamBackBudget) // stream backoff budget must be intact (all stream creation attempts are successful).
		require.Equal(t, uint64(i+1), dialCfg.ConsecutiveSuccessfulStream)                        // consecutive successful stream must be incremented.
	}
}

// TestUnicastManager_Stream_ConsecutiveStreamCreation_Reset tests that when there is a connection, and the stream creation fails, it resets
// the consecutive successful stream counter in the dial config.
func TestUnicastManager_Stream_ConsecutiveStreamCreation_Reset(t *testing.T) {
	connStatus := mockp2p.NewPeerConnections(t)
	streamFactory := mockp2p.NewStreamFactory(t)
	peerID := p2ptest.PeerIdFixture(t)

	streamFactory.On("NewStream", mock.Anything, peerID, mock.Anything).
		Return(nil, fmt.Errorf("some error")).
		Once() // mocks that it attempts to create a stream only once.
	connStatus.On("IsConnected", peerID).Return(true, nil) // connected.
	streamFactory.On("SetStreamHandler", mock.Anything, mock.Anything).Return().Once()

	cfg, err := config.DefaultConfig()
	require.NoError(t, err)

	dialConfigCache := unicastcache.NewDialConfigCache(unicast.DefaultDailConfigCacheSize, unittest.Logger(), metrics.NewNoopCollector(), unicastmodel.DefaultDialConfigFactory)
	adjustedDialConfig, err := dialConfigCache.Adjust(
		peerID, func(dialConfig unicastmodel.DialConfig) (unicastmodel.DialConfig, error) {
			dialConfig.ConsecutiveSuccessfulStream = 5 // sets the consecutive successful stream to 10 meaning that the last 10 stream creation attempts were successful.
			dialConfig.StreamBackBudget = 0            // sets the stream back budget to 0 meaning that the stream backoff budget is exhausted.

			return dialConfig, nil
		},
	)
	require.NoError(t, err)
	require.Equal(t, uint64(5), adjustedDialConfig.ConsecutiveSuccessfulStream)

	mgr, err := unicastmgr.NewUnicastManager(
		&unicastmgr.ManagerConfig{
			Logger:                     unittest.Logger(),
			StreamFactory:              streamFactory,
			SporkId:                    unittest.IdentifierFixture(),
			ConnStatus:                 connStatus,
			CreateStreamRetryDelay:     cfg.NetworkConfig.UnicastCreateStreamRetryDelay,
			Metrics:                    metrics.NewNoopCollector(),
			StreamHistoryResetInterval: unicastmodel.StreamHistoryResetInterval,
			DialConfigCacheFactory: func() unicast.DialConfigCache {
				return dialConfigCache
			},
		},
	)
	require.NoError(t, err)
	mgr.SetDefaultHandler(func(libp2pnet.Stream) {}) // no-op handler, we don't care about the handler for this test

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s, err := mgr.CreateStream(ctx, peerID)
	require.Error(t, err)
	require.Nil(t, s)

	// The dial config must be updated with the stream backoff budget decremented.
	dialCfg, err := dialConfigCache.GetOrInit(peerID)
	require.NoError(t, err)
	require.Equal(t, uint64(unicastmodel.MaxConnectAttempt), dialCfg.DialBackoffBudget) // dial backoff budget must be intact.
	require.Equal(t, uint64(0), dialCfg.StreamBackBudget)                               // stream backoff budget must be intact (we can't decrement it below 0).
	require.Equal(t, uint64(0), dialCfg.ConsecutiveSuccessfulStream)                    // consecutive successful stream must be reset to 0.
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

	mgr, err := unicastmgr.NewUnicastManager(
		&unicastmgr.ManagerConfig{
			Logger:                     unittest.Logger(),
			StreamFactory:              streamFactory,
			SporkId:                    unittest.IdentifierFixture(),
			ConnStatus:                 connStatus,
			CreateStreamRetryDelay:     cfg.NetworkConfig.UnicastCreateStreamRetryDelay,
			Metrics:                    metrics.NewNoopCollector(),
			StreamHistoryResetInterval: unicastmodel.StreamHistoryResetInterval,
			DialConfigCacheFactory: func() unicast.DialConfigCache {
				return unicastcache.NewDialConfigCache(
					unicast.DefaultDailConfigCacheSize, unittest.Logger(), metrics.NewNoopCollector(), unicastmodel.DefaultDialConfigFactory,
				)
			},
		},
	)
	require.NoError(t, err)
	mgr.SetDefaultHandler(func(libp2pnet.Stream) {}) // no-op handler, we don't care about the handler for this test

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s, err := mgr.CreateStream(ctx, peerID)
	require.Error(t, err)
	require.Nil(t, s)
}

// TestUnicastManager_Connection_BackoffBudgetDecremented tests that everytime the unicast manger gives up on creating a connection (after retrials),
// it decrements the backoff budget for the remote peer.
func TestUnicastManager_Connection_BackoffBudgetDecremented(t *testing.T) {
	connStatus := mockp2p.NewPeerConnections(t)
	streamFactory := mockp2p.NewStreamFactory(t)
	peerID := p2ptest.PeerIdFixture(t)

	// totalAttempts is the total number of times that unicast manager calls Connect on the stream factory to dial the peer.
	// Let's consider x = unicastmodel.MaxConnectAttempt. Then the test tries x times CreateStream. With dynamic backoffs,
	// the first CreateStream call will try to Connect x times, the second CreateStream call will try to Connect x-1 times,
	// and so on. So the total number of Connect calls is x + (x-1) + (x-2) + ... + 1 = x(x+1)/2.
	// However, we also attempt one more time at the end of the test to CreateStream, when the backoff budget is 0.
	// When the backoff budget is 0, the unicast manager does not backoff, and tries to Connect once. So the total number
	// of Connect calls is x(x+1)/2 + 1.
	totalAttempts := unicastmodel.MaxConnectAttempt*(unicastmodel.MaxConnectAttempt+1)/2 + 1

	connStatus.On("IsConnected", peerID).Return(false, nil) // not connected
	streamFactory.On("ClearBackoff", peerID).Return().Times(totalAttempts)
	streamFactory.On("Connect", mock.Anything, peer.AddrInfo{ID: peerID}).
		Return(fmt.Errorf("some error")).
		Times(totalAttempts)
	streamFactory.On("SetStreamHandler", mock.Anything, mock.Anything).Return().Once()

	cfg, err := config.DefaultConfig()
	require.NoError(t, err)

	dialCfgCache := unicastcache.NewDialConfigCache(
		unicast.DefaultDailConfigCacheSize, unittest.Logger(), metrics.NewNoopCollector(), unicastmodel.DefaultDialConfigFactory,
	)
	mgr, err := unicastmgr.NewUnicastManager(
		&unicastmgr.ManagerConfig{
			Logger:                     unittest.Logger(),
			StreamFactory:              streamFactory,
			SporkId:                    unittest.IdentifierFixture(),
			ConnStatus:                 connStatus,
			CreateStreamRetryDelay:     cfg.NetworkConfig.UnicastCreateStreamRetryDelay,
			Metrics:                    metrics.NewNoopCollector(),
			StreamHistoryResetInterval: unicastmodel.StreamHistoryResetInterval,
			DialConfigCacheFactory: func() unicast.DialConfigCache {
				return dialCfgCache
			},
		},
	)
	require.NoError(t, err)
	mgr.SetDefaultHandler(func(libp2pnet.Stream) {}) // no-op handler, we don't care about the handler for this test

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for i := 0; i < unicastmodel.MaxConnectAttempt; i++ {
		s, err := mgr.CreateStream(ctx, peerID)
		require.Error(t, err)
		require.Nil(t, s)

		dialCfg, err := dialCfgCache.GetOrInit(peerID)
		require.NoError(t, err)

		if i == unicastmodel.MaxConnectAttempt-1 {
			require.Equal(t, uint64(0), dialCfg.DialBackoffBudget)
		} else {
			require.Equal(t, uint64(unicastmodel.MaxConnectAttempt-i-1), dialCfg.DialBackoffBudget)
		}

		// The stream backoff budget must remain intact, as we have not tried to create a stream yet.
		require.Equal(t, uint64(unicastmodel.MaxStreamCreationAttempt), dialCfg.StreamBackBudget)
	}
	// At this time the backoff budget for connection must be 0.
	dialCfg, err := dialCfgCache.GetOrInit(peerID)
	require.NoError(t, err)

	require.Equal(t, uint64(0), dialCfg.DialBackoffBudget)
	// The stream backoff budget must remain intact, as we have not tried to create a stream yet.
	require.Equal(t, uint64(unicastmodel.MaxStreamCreationAttempt), dialCfg.StreamBackBudget)

	// After all the backoff budget is used up, it should stay at 0.
	s, err := mgr.CreateStream(ctx, peerID)
	require.Error(t, err)
	require.Nil(t, s)

	dialCfg, err = dialCfgCache.GetOrInit(peerID)
	require.NoError(t, err)
	require.Equal(t, uint64(0), dialCfg.DialBackoffBudget)

	// The stream backoff budget must remain intact, as we have not tried to create a stream yet.
	require.Equal(t, uint64(unicastmodel.MaxStreamCreationAttempt), dialCfg.StreamBackBudget)
}

// TestUnicastManager_Connection_BackoffBudgetDecremented tests that everytime the unicast manger gives up on creating a connection (after retrials),
// it decrements the backoff budget for the remote peer.
func TestUnicastManager_Stream_BackoffBudgetDecremented(t *testing.T) {
	connStatus := mockp2p.NewPeerConnections(t)
	streamFactory := mockp2p.NewStreamFactory(t)
	peerID := p2ptest.PeerIdFixture(t)

	// totalAttempts is the total number of times that unicast manager calls NewStream on the stream factory to create stream to the peer.
	// Note that it already assumes that the connection is established, so it does not try to connect to the peer.
	// Let's consider x = unicastmodel.MaxStreamCreationAttempt. Then the test tries x times CreateStream. With dynamic backoffs,
	// the first CreateStream call will try to NewStream x times, the second CreateStream call will try to NewStream x-1 times,
	// and so on. So the total number of Connect calls is x + (x-1) + (x-2) + ... + 1 = x(x+1)/2.
	// However, we also attempt one more time at the end of the test to CreateStream, when the backoff budget is 0.
	// When the backoff budget is 0, the unicast manager does not backoff, and tries to CreateStream once. So the total number
	// of Connect calls is x(x+1)/2 + 1.
	totalAttempts := unicastmodel.MaxConnectAttempt*(unicastmodel.MaxConnectAttempt+1)/2 + 1

	connStatus.On("IsConnected", peerID).Return(true, nil) // not connected
	streamFactory.On("NewStream", mock.Anything, peerID, mock.Anything).
		Return(nil, fmt.Errorf("some error")).
		Times(totalAttempts)
	streamFactory.On("SetStreamHandler", mock.Anything, mock.Anything).Return().Once()

	cfg, err := config.DefaultConfig()
	require.NoError(t, err)

	dialCfgCache := unicastcache.NewDialConfigCache(
		unicast.DefaultDailConfigCacheSize, unittest.Logger(), metrics.NewNoopCollector(), unicastmodel.DefaultDialConfigFactory,
	)
	mgr, err := unicastmgr.NewUnicastManager(
		&unicastmgr.ManagerConfig{
			Logger:                     unittest.Logger(),
			StreamFactory:              streamFactory,
			SporkId:                    unittest.IdentifierFixture(),
			ConnStatus:                 connStatus,
			CreateStreamRetryDelay:     cfg.NetworkConfig.UnicastCreateStreamRetryDelay,
			Metrics:                    metrics.NewNoopCollector(),
			StreamHistoryResetInterval: unicastmodel.StreamHistoryResetInterval,
			DialConfigCacheFactory: func() unicast.DialConfigCache {
				return dialCfgCache
			},
		},
	)
	require.NoError(t, err)
	mgr.SetDefaultHandler(func(libp2pnet.Stream) {}) // no-op handler, we don't care about the handler for this test

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for i := 0; i < unicastmodel.MaxStreamCreationAttempt; i++ {
		s, err := mgr.CreateStream(ctx, peerID)
		require.Error(t, err)
		require.Nil(t, s)

		dialCfg, err := dialCfgCache.GetOrInit(peerID)
		require.NoError(t, err)

		if i == unicastmodel.MaxStreamCreationAttempt-1 {
			require.Equal(t, uint64(0), dialCfg.StreamBackBudget)
		} else {
			require.Equal(t, uint64(unicastmodel.MaxStreamCreationAttempt-i-1), dialCfg.StreamBackBudget)
		}

		// The dial backoff budget must remain intact, as we have not tried to create a stream yet.
		require.Equal(t, uint64(unicastmodel.MaxConnectAttempt), dialCfg.DialBackoffBudget)
	}
	// At this time the backoff budget for connection must be 0.
	dialCfg, err := dialCfgCache.GetOrInit(peerID)
	require.NoError(t, err)

	require.Equal(t, uint64(0), dialCfg.StreamBackBudget)
	// The dial backoff budget must remain intact, as we have not tried to create a stream yet.
	require.Equal(t, uint64(unicastmodel.MaxConnectAttempt), dialCfg.DialBackoffBudget)

	// After all the backoff budget is used up, it should stay at 0.
	s, err := mgr.CreateStream(ctx, peerID)
	require.Error(t, err)
	require.Nil(t, s)

	dialCfg, err = dialCfgCache.GetOrInit(peerID)
	require.NoError(t, err)
	require.Equal(t, uint64(0), dialCfg.StreamBackBudget)

	// The dial backoff budget must remain intact, as we have not tried to create a stream yet.
	require.Equal(t, uint64(unicastmodel.MaxConnectAttempt), dialCfg.DialBackoffBudget)
}
