package manager_test

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
	"github.com/onflow/flow-go/network/p2p/unicast/manager"
	"github.com/onflow/flow-go/network/p2p/unicast/stream"
	"github.com/onflow/flow-go/utils/unittest"
)

func unicastManagerFixture(t *testing.T) (*manager.Manager, *mockp2p.StreamFactory, *mockp2p.PeerConnections, unicast.DialConfigCache) {
	streamFactory := mockp2p.NewStreamFactory(t)
	streamFactory.On("SetStreamHandler", mock.Anything, mock.Anything).Return().Once()
	connStatus := mockp2p.NewPeerConnections(t)

	cfg, err := config.DefaultConfig()
	require.NoError(t, err)

	dialConfigCache := unicastcache.NewDialConfigCache(cfg.NetworkConfig.UnicastDialConfigCacheSize, unittest.Logger(), metrics.NewNoopCollector(), func() unicast.DialConfig {
		return unicast.DialConfig{
			DialRetryAttemptBudget:           cfg.NetworkConfig.UnicastMaxDialRetryAttemptTimes,
			StreamCreationRetryAttemptBudget: cfg.NetworkConfig.UnicastMaxStreamCreationRetryAttemptTimes,
		}
	})
	mgr, err := manager.NewUnicastManager(&manager.ManagerConfig{
		Logger:                             unittest.Logger(),
		StreamFactory:                      streamFactory,
		SporkId:                            unittest.IdentifierFixture(),
		ConnStatus:                         connStatus,
		CreateStreamRetryDelay:             cfg.NetworkConfig.UnicastCreateStreamRetryDelay,
		Metrics:                            metrics.NewNoopCollector(),
		StreamZeroRetryResetThreshold:      cfg.NetworkConfig.UnicastStreamZeroRetryResetThreshold,
		DialZeroRetryResetThreshold:        cfg.NetworkConfig.UnicastDialZeroRetryResetThreshold,
		MaxStreamCreationRetryAttemptTimes: cfg.NetworkConfig.UnicastMaxStreamCreationRetryAttemptTimes,
		MaxDialRetryAttemptTimes:           cfg.NetworkConfig.UnicastMaxDialRetryAttemptTimes,
		DialConfigCacheFactory: func(func() unicast.DialConfig) unicast.DialConfigCache {
			return dialConfigCache
		},
	})
	require.NoError(t, err)
	mgr.SetDefaultHandler(func(libp2pnet.Stream) {}) // no-op handler, we don't care about the handler for this test

	return mgr, streamFactory, connStatus, dialConfigCache
}

// TestUnicastManager_StreamFactory_ConnectionBackoff tests the backoff mechanism of the unicast manager for connection creation.
// It tests that when there is no connection, it tries to connect to the peer some number of times (unicastmodel.MaxDialAttemptTimes), before
// giving up.
func TestUnicastManager_Connection_ConnectionBackoff(t *testing.T) {
	peerID := p2ptest.PeerIdFixture(t)
	mgr, streamFactory, connStatus, dialConfigCache := unicastManagerFixture(t)

	cfg, err := config.DefaultConfig()
	require.NoError(t, err)

	connStatus.On("IsConnected", peerID).Return(false, nil) // not connected
	streamFactory.On("Connect", mock.Anything, peer.AddrInfo{ID: peerID}).
		Return(fmt.Errorf("some error")).Times(int(cfg.NetworkConfig.UnicastMaxDialRetryAttemptTimes + 1)) // connect

	_, err = dialConfigCache.Adjust(peerID, func(dialConfig unicast.DialConfig) (unicast.DialConfig, error) {
		// assumes that there was a successful connection to the peer before (2 minutes ago), and now the connection is lost.
		dialConfig.LastSuccessfulDial = time.Now().Add(2 * time.Minute)
		return dialConfig, nil
	})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s, err := mgr.CreateStream(ctx, peerID)
	require.Error(t, err)
	require.Nil(t, s)

	// The dial config must be updated with the backoff budget decremented.
	dialCfg, err := dialConfigCache.GetOrInit(peerID)
	require.NoError(t, err)
	require.Equal(t, cfg.NetworkConfig.UnicastMaxDialRetryAttemptTimes-1, dialCfg.DialRetryAttemptBudget) // dial backoff budget must be decremented by 1.
	require.Equal(t,
		cfg.NetworkConfig.UnicastMaxStreamCreationRetryAttemptTimes,
		dialCfg.StreamCreationRetryAttemptBudget) // stream backoff budget must remain intact (no stream creation attempt yet).
	// last successful dial is set back to zero, since although we have a successful dial in the past, the most recent dial failed.
	require.True(t, dialCfg.LastSuccessfulDial.IsZero())
	require.Equal(t, uint64(0), dialCfg.ConsecutiveSuccessfulStream) // consecutive successful stream must be intact.
}

// TestUnicastManager_StreamFactory_Connection_SuccessfulConnection_And_Stream tests that when there is no connection, and CreateStream is successful on the first attempt for connection and stream creation,
// it updates the last successful dial time and the consecutive successful stream counter.
func TestUnicastManager_Connection_SuccessfulConnection_And_Stream(t *testing.T) {
	peerID := p2ptest.PeerIdFixture(t)
	mgr, streamFactory, connStatus, dialConfigCache := unicastManagerFixture(t)

	cfg, err := config.DefaultConfig()
	require.NoError(t, err)

	connStatus.On("IsConnected", peerID).Return(false, nil)                                  // not connected
	streamFactory.On("Connect", mock.Anything, peer.AddrInfo{ID: peerID}).Return(nil).Once() // connect on the first attempt.
	// mocks that it attempts to create a stream once and succeeds.
	streamFactory.On("NewStream", mock.Anything, peerID, mock.Anything).Return(&p2ptest.MockStream{}, nil).Once()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dialTime := time.Now()
	s, err := mgr.CreateStream(ctx, peerID)
	require.NoError(t, err)
	require.NotNil(t, s)

	// The dial config must be updated with the backoff budget decremented.
	dialCfg, err := dialConfigCache.GetOrInit(peerID)
	require.NoError(t, err)
	require.Equal(t, cfg.NetworkConfig.UnicastMaxDialRetryAttemptTimes, dialCfg.DialRetryAttemptBudget)                     // dial backoff budget must be intact.
	require.Equal(t, cfg.NetworkConfig.UnicastMaxStreamCreationRetryAttemptTimes, dialCfg.StreamCreationRetryAttemptBudget) // stream backoff budget must remain intact.
	// last successful dial must be set AFTER the successful dial.
	require.True(t, dialCfg.LastSuccessfulDial.After(dialTime))
	require.Equal(t, uint64(1), dialCfg.ConsecutiveSuccessfulStream) // consecutive successful stream must incremented.
}

// TestUnicastManager_StreamFactory_Connection_SuccessfulConnection_StreamBackoff tests the backoff mechanism of the unicast manager for stream creation.
// It tests the situation that there is no connection when CreateStream is called. The connection is created successfully, but the stream creation fails.
// It tests that when there is a connection, but no stream, it tries to create a stream some number of times (unicastmodel.MaxStreamCreationAttemptTimes), before
// giving up.
// It also checks the consecutive successful stream counter is reset when the stream creation fails, and the last successful dial time is updated.
func TestUnicastManager_Connection_SuccessfulConnection_StreamBackoff(t *testing.T) {
	peerID := p2ptest.PeerIdFixture(t)
	mgr, streamFactory, connStatus, dialConfigCache := unicastManagerFixture(t)

	cfg, err := config.DefaultConfig()
	require.NoError(t, err)

	isConnectedCalled := 0
	connStatus.On("IsConnected", peerID).Return(func(id peer.ID) bool {
		if isConnectedCalled == 0 {
			// we mock that the connection is not established on the first call, and is established on the second call and onwards.
			isConnectedCalled++
			return false
		}
		return true
	}, nil)
	streamFactory.On("Connect", mock.Anything, peer.AddrInfo{ID: peerID}).Return(nil).Once() // connect on the first attempt.
	streamFactory.On("NewStream", mock.Anything, peerID, mock.Anything).Return(nil, fmt.Errorf("some error")).
		Times(int(cfg.NetworkConfig.UnicastMaxStreamCreationRetryAttemptTimes + 1)) // mocks that it attempts to create a stream some number of times, before giving up.

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dialTime := time.Now()
	s, err := mgr.CreateStream(ctx, peerID)
	require.Error(t, err)
	require.Nil(t, s)

	// The dial config must be updated with the backoff budget decremented.
	dialCfg, err := dialConfigCache.GetOrInit(peerID)
	require.NoError(t, err)
	require.Equal(t, cfg.NetworkConfig.UnicastMaxDialRetryAttemptTimes, dialCfg.DialRetryAttemptBudget) // dial backoff budget must be intact, since the connection is successful.
	require.Equal(t,
		cfg.NetworkConfig.UnicastMaxStreamCreationRetryAttemptTimes-1,
		dialCfg.StreamCreationRetryAttemptBudget) // stream backoff budget must be decremented by 1 since all budget is used up.
	// last successful dial must be set AFTER the successful dial.
	require.True(t, dialCfg.LastSuccessfulDial.After(dialTime))
	require.Equal(t, uint64(0), dialCfg.ConsecutiveSuccessfulStream) // consecutive successful stream must be reset to zero, since the stream creation failed.
}

// TestUnicastManager_StreamFactory_StreamBackoff tests the backoff mechanism of the unicast manager for stream creation.
// It tests when there is a connection, but no stream, it tries to create a stream some number of times (unicastmodel.MaxStreamCreationAttemptTimes), before
// giving up.
func TestUnicastManager_StreamFactory_StreamBackoff(t *testing.T) {
	mgr, streamFactory, connStatus, dialConfigCache := unicastManagerFixture(t)
	peerID := p2ptest.PeerIdFixture(t)

	cfg, err := config.DefaultConfig()
	require.NoError(t, err)

	connStatus.On("IsConnected", peerID).Return(true, nil) // connected.
	streamFactory.On("NewStream", mock.Anything, peerID, mock.Anything).
		Return(nil, fmt.Errorf("some error")).
		Times(int(cfg.NetworkConfig.UnicastMaxStreamCreationRetryAttemptTimes + 1)) // mocks that it attempts to create a stream some number of times, before giving up.

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s, err := mgr.CreateStream(ctx, peerID)
	require.Error(t, err)
	require.Nil(t, s)

	// The dial config must be updated with the stream backoff budget decremented.
	dialCfg, err := dialConfigCache.GetOrInit(peerID)
	require.NoError(t, err)
	require.Equal(t, cfg.NetworkConfig.UnicastMaxDialRetryAttemptTimes, dialCfg.DialRetryAttemptBudget)                       // dial backoff budget must be intact.
	require.Equal(t, cfg.NetworkConfig.UnicastMaxStreamCreationRetryAttemptTimes-1, dialCfg.StreamCreationRetryAttemptBudget) // stream backoff budget must be decremented by 1.
	require.Equal(t,
		uint64(0),
		dialCfg.ConsecutiveSuccessfulStream) // consecutive successful stream must be zero as we have not created a successful stream yet.
}

// TestUnicastManager_Stream_ConsecutiveStreamCreation_Increment tests that when there is a connection, and the stream creation is successful,
// it increments the consecutive successful stream counter in the dial config.
func TestUnicastManager_Stream_ConsecutiveStreamCreation_Increment(t *testing.T) {
	mgr, streamFactory, connStatus, dialConfigCache := unicastManagerFixture(t)
	peerID := p2ptest.PeerIdFixture(t)

	cfg, err := config.DefaultConfig()
	require.NoError(t, err)

	// total times we successfully create a stream to the peer.
	totalSuccessAttempts := 10

	connStatus.On("IsConnected", peerID).Return(true, nil) // connected.
	// mocks that it attempts to create a stream 10 times, and each time it succeeds.
	streamFactory.On("NewStream", mock.Anything, peerID, mock.Anything).Return(&p2ptest.MockStream{}, nil).Times(totalSuccessAttempts)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for i := 0; i < totalSuccessAttempts; i++ {
		s, err := mgr.CreateStream(ctx, peerID)
		require.NoError(t, err)
		require.NotNil(t, s)

		// The dial config must be updated with the stream backoff budget decremented.
		dialCfg, err := dialConfigCache.GetOrInit(peerID)
		require.NoError(t, err)
		require.Equal(t, cfg.NetworkConfig.UnicastMaxDialRetryAttemptTimes, dialCfg.DialRetryAttemptBudget) // dial backoff budget must be intact.
		// stream backoff budget must be intact (all stream creation attempts are successful).
		require.Equal(t, cfg.NetworkConfig.UnicastMaxStreamCreationRetryAttemptTimes, dialCfg.StreamCreationRetryAttemptBudget)
		require.Equal(t, uint64(i+1), dialCfg.ConsecutiveSuccessfulStream) // consecutive successful stream must be incremented.
	}
}

// TestUnicastManager_Stream_ConsecutiveStreamCreation_Reset tests that when there is a connection, and the stream creation fails, it resets
// the consecutive successful stream counter in the dial config.
func TestUnicastManager_Stream_ConsecutiveStreamCreation_Reset(t *testing.T) {
	mgr, streamFactory, connStatus, dialConfigCache := unicastManagerFixture(t)
	peerID := p2ptest.PeerIdFixture(t)

	cfg, err := config.DefaultConfig()
	require.NoError(t, err)

	streamFactory.On("NewStream", mock.Anything, peerID, mock.Anything).
		Return(nil, fmt.Errorf("some error")).
		Once() // mocks that it attempts to create a stream once and fails.
	connStatus.On("IsConnected", peerID).Return(true, nil) // connected.

	adjustedDialConfig, err := dialConfigCache.Adjust(peerID, func(dialConfig unicast.DialConfig) (unicast.DialConfig, error) {
		dialConfig.ConsecutiveSuccessfulStream = 5      // sets the consecutive successful stream to 5 meaning that the last 5 stream creation attempts were successful.
		dialConfig.StreamCreationRetryAttemptBudget = 0 // sets the stream back budget to 0 meaning that the stream backoff budget is exhausted.

		return dialConfig, nil
	})
	require.NoError(t, err)
	require.Equal(t, uint64(5), adjustedDialConfig.ConsecutiveSuccessfulStream)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s, err := mgr.CreateStream(ctx, peerID)
	require.Error(t, err)
	require.Nil(t, s)

	// The dial config must be updated with the stream backoff budget decremented.
	dialCfg, err := dialConfigCache.GetOrInit(peerID)
	require.NoError(t, err)
	require.Equal(t, cfg.NetworkConfig.UnicastMaxDialRetryAttemptTimes, dialCfg.DialRetryAttemptBudget) // dial backoff budget must be intact.
	require.Equal(t, uint64(0), dialCfg.StreamCreationRetryAttemptBudget)                               // stream backoff budget must be intact (we can't decrement it below 0).
	require.Equal(t, uint64(0), dialCfg.ConsecutiveSuccessfulStream)                                    // consecutive successful stream must be reset to 0.
}

// TestUnicastManager_StreamFactory_ErrProtocolNotSupported tests that when there is a protocol not supported error, it does not retry creating a stream.
func TestUnicastManager_StreamFactory_ErrProtocolNotSupported(t *testing.T) {
	mgr, streamFactory, connStatus, _ := unicastManagerFixture(t)
	peerID := p2ptest.PeerIdFixture(t)

	connStatus.On("IsConnected", peerID).Return(true, nil) // connected
	streamFactory.On("NewStream", mock.Anything, peerID, mock.Anything).
		Return(nil, stream.NewProtocolNotSupportedErr(peerID, []protocol.ID{"protocol-1"}, fmt.Errorf("some error"))).
		Once() // mocks that upon creating a stream, it returns a protocol not supported error, the mock is set to once, meaning that it won't retry stream creation again.

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s, err := mgr.CreateStream(ctx, peerID)
	require.Error(t, err)
	require.Nil(t, s)
}

// TestUnicastManager_Dial_ErrSecurityProtocolNegotiationFailed tests that when there is a security protocol negotiation error, it does not retry dialing.
func TestUnicastManager_Dial_ErrSecurityProtocolNegotiationFailed(t *testing.T) {
	mgr, streamFactory, connStatus, dialConfigCache := unicastManagerFixture(t)

	cfg, err := config.DefaultConfig()
	require.NoError(t, err)

	peerID := p2ptest.PeerIdFixture(t)
	// mocks that the connection is not established.
	connStatus.On("IsConnected", peerID).Return(false, nil)

	// mocks that dialing the peer returns a security protocol negotiation error, and the mock is set to once, meaning that it won't retry dialing again.
	streamFactory.On("Connect", mock.Anything, peer.AddrInfo{ID: peerID}).
		Return(stream.NewSecurityProtocolNegotiationErr(peerID, fmt.Errorf("some error"))).
		Once()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s, err := mgr.CreateStream(ctx, peerID)
	require.Error(t, err)
	require.Nil(t, s)

	dialCfg, err := dialConfigCache.GetOrInit(peerID)
	require.NoError(t, err)
	// dial backoff budget must be decremented by 1 (although we didn't have a backoff attempt, the connection was unsuccessful).
	require.Equal(t, cfg.NetworkConfig.UnicastMaxDialRetryAttemptTimes-1, dialCfg.DialRetryAttemptBudget)
	// stream backoff budget must remain intact, as we have not tried to create a stream yet.
	require.Equal(t, cfg.NetworkConfig.UnicastMaxStreamCreationRetryAttemptTimes, dialCfg.StreamCreationRetryAttemptBudget)
	// last successful dial must be set to zero.
	require.True(t, dialCfg.LastSuccessfulDial.IsZero())
	// consecutive successful stream must be set to zero.
	require.Equal(t, uint64(0), dialCfg.ConsecutiveSuccessfulStream)
}

// TestUnicastManager_Dial_ErrGaterDisallowedConnection tests that when there is a connection gater disallow listing error, it does not retry dialing.
func TestUnicastManager_Dial_ErrGaterDisallowedConnection(t *testing.T) {
	mgr, streamFactory, connStatus, dialConfigCache := unicastManagerFixture(t)
	peerID := p2ptest.PeerIdFixture(t)
	// mocks that the connection is not established.
	connStatus.On("IsConnected", peerID).Return(false, nil)

	cfg, err := config.DefaultConfig()
	require.NoError(t, err)

	// mocks that dialing the peer returns a security protocol negotiation error, and the mock is set to once, meaning that it won't retry dialing again.
	streamFactory.On("Connect", mock.Anything, peer.AddrInfo{ID: peerID}).
		Return(stream.NewGaterDisallowedConnectionErr(fmt.Errorf("some error"))).
		Once()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s, err := mgr.CreateStream(ctx, peerID)
	require.Error(t, err)
	require.Nil(t, s)

	dialCfg, err := dialConfigCache.GetOrInit(peerID)
	require.NoError(t, err)
	// dial backoff budget must be decremented by 1 (although we didn't have a backoff attempt, the connection was unsuccessful).
	require.Equal(t, cfg.NetworkConfig.UnicastMaxDialRetryAttemptTimes-1, dialCfg.DialRetryAttemptBudget)
	// stream backoff budget must remain intact, as we have not tried to create a stream yet.
	require.Equal(t, cfg.NetworkConfig.UnicastMaxStreamCreationRetryAttemptTimes, dialCfg.StreamCreationRetryAttemptBudget)
	// last successful dial must be set to zero.
	require.True(t, dialCfg.LastSuccessfulDial.IsZero())
	// consecutive successful stream must be set to zero.
	require.Equal(t, uint64(0), dialCfg.ConsecutiveSuccessfulStream)
}

// TestUnicastManager_Connection_BackoffBudgetDecremented tests that everytime the unicast manger gives up on creating a connection (after retrials),
// it decrements the backoff budget for the remote peer.
func TestUnicastManager_Connection_BackoffBudgetDecremented(t *testing.T) {
	mgr, streamFactory, connStatus, dialConfigCache := unicastManagerFixture(t)
	peerID := p2ptest.PeerIdFixture(t)

	cfg, err := config.DefaultConfig()
	require.NoError(t, err)

	// totalAttempts is the total number of times that unicast manager calls Connect on the stream factory to dial the peer.
	// Let's consider x = unicastmodel.UnicastMaxDialRetryAttemptTimes + 1. Then the test tries x times CreateStream. With dynamic backoffs,
	// the first CreateStream call will try to Connect x times, the second CreateStream call will try to Connect x-1 times,
	// and so on. So the total number of Connect calls is x + (x-1) + (x-2) + ... + 1 = x(x+1)/2.
	// However, we also attempt one more time at the end of the test to CreateStream, when the backoff budget is 0.
	maxDialRetryAttemptBudget := int(cfg.NetworkConfig.UnicastMaxDialRetryAttemptTimes)
	attemptTimes := maxDialRetryAttemptBudget + 1 // 1 attempt + retry times
	totalAttempts := attemptTimes * (attemptTimes + 1) / 2

	connStatus.On("IsConnected", peerID).Return(false, nil) // not connected
	streamFactory.On("Connect", mock.Anything, peer.AddrInfo{ID: peerID}).
		Return(fmt.Errorf("some error")).
		Times(int(totalAttempts))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for i := 0; i < maxDialRetryAttemptBudget; i++ {
		s, err := mgr.CreateStream(ctx, peerID)
		require.Error(t, err)
		require.Nil(t, s)

		dialCfg, err := dialConfigCache.GetOrInit(peerID)
		require.NoError(t, err)

		if i == maxDialRetryAttemptBudget-1 {
			require.Equal(t, uint64(0), dialCfg.DialRetryAttemptBudget)
		} else {
			require.Equal(t, uint64(maxDialRetryAttemptBudget-i-1), dialCfg.DialRetryAttemptBudget)
		}

		// The stream backoff budget must remain intact, as we have not tried to create a stream yet.
		require.Equal(t, cfg.NetworkConfig.UnicastMaxStreamCreationRetryAttemptTimes, dialCfg.StreamCreationRetryAttemptBudget)
	}
	// At this time the backoff budget for connection must be 0.
	dialCfg, err := dialConfigCache.GetOrInit(peerID)
	require.NoError(t, err)

	require.Equal(t, uint64(0), dialCfg.DialRetryAttemptBudget)
	// The stream backoff budget must remain intact, as we have not tried to create a stream yet.
	require.Equal(t, cfg.NetworkConfig.UnicastMaxStreamCreationRetryAttemptTimes, dialCfg.StreamCreationRetryAttemptBudget)

	// After all the backoff budget is used up, it should stay at 0.
	s, err := mgr.CreateStream(ctx, peerID)
	require.Error(t, err)
	require.Nil(t, s)

	dialCfg, err = dialConfigCache.GetOrInit(peerID)
	require.NoError(t, err)
	require.Equal(t, uint64(0), dialCfg.DialRetryAttemptBudget)

	// The stream backoff budget must remain intact, as we have not tried to create a stream yet.
	require.Equal(t, cfg.NetworkConfig.UnicastMaxStreamCreationRetryAttemptTimes, dialCfg.StreamCreationRetryAttemptBudget)
}

// TestUnicastManager_Connection_BackoffBudgetDecremented tests that everytime the unicast manger gives up on creating a connection (after retrials),
// it decrements the backoff budget for the remote peer.
func TestUnicastManager_Stream_BackoffBudgetDecremented(t *testing.T) {
	mgr, streamFactory, connStatus, dialConfigCache := unicastManagerFixture(t)
	peerID := p2ptest.PeerIdFixture(t)

	cfg, err := config.DefaultConfig()
	require.NoError(t, err)

	// totalAttempts is the total number of times that unicast manager calls NewStream on the stream factory to create stream to the peer.
	// Note that it already assumes that the connection is established, so it does not try to connect to the peer.
	// Let's consider x = unicastmodel.UnicastMaxStreamCreationRetryAttemptTimes + 1. Then the test tries x times CreateStream. With dynamic backoffs,
	// the first CreateStream call will try to NewStream x times, the second CreateStream call will try to NewStream x-1 times,
	// and so on. So the total number of Connect calls is x + (x-1) + (x-2) + ... + 1 = x(x+1)/2.
	maxStreamRetryBudget := cfg.NetworkConfig.UnicastMaxStreamCreationRetryAttemptTimes
	maxStreamAttempt := maxStreamRetryBudget + 1 // 1 attempt + retry times
	totalAttempts := maxStreamAttempt * (maxStreamAttempt + 1) / 2

	connStatus.On("IsConnected", peerID).Return(true, nil) // not connected
	streamFactory.On("NewStream", mock.Anything, peerID, mock.Anything).
		Return(nil, fmt.Errorf("some error")).
		Times(int(totalAttempts))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for i := 0; i < int(maxStreamRetryBudget); i++ {
		s, err := mgr.CreateStream(ctx, peerID)
		require.Error(t, err)
		require.Nil(t, s)

		dialCfg, err := dialConfigCache.GetOrInit(peerID)
		require.NoError(t, err)

		if i == int(maxStreamRetryBudget)-1 {
			require.Equal(t, uint64(0), dialCfg.StreamCreationRetryAttemptBudget)
		} else {
			require.Equal(t, maxStreamRetryBudget-uint64(i)-1, dialCfg.StreamCreationRetryAttemptBudget)
		}

		// The dial backoff budget must remain intact, as we have not tried to create a stream yet.
		require.Equal(t, cfg.NetworkConfig.UnicastMaxDialRetryAttemptTimes, dialCfg.DialRetryAttemptBudget)
	}
	// At this time the backoff budget for connection must be 0.
	dialCfg, err := dialConfigCache.GetOrInit(peerID)
	require.NoError(t, err)

	require.Equal(t, uint64(0), dialCfg.StreamCreationRetryAttemptBudget)
	// The dial backoff budget must remain intact, as we have not tried to create a stream yet.
	require.Equal(t, cfg.NetworkConfig.UnicastMaxDialRetryAttemptTimes, dialCfg.DialRetryAttemptBudget)

	// After all the backoff budget is used up, it should stay at 0.
	s, err := mgr.CreateStream(ctx, peerID)
	require.Error(t, err)
	require.Nil(t, s)

	dialCfg, err = dialConfigCache.GetOrInit(peerID)
	require.NoError(t, err)
	require.Equal(t, uint64(0), dialCfg.StreamCreationRetryAttemptBudget)

	// The dial backoff budget must remain intact, as we have not tried to create a stream yet.
	require.Equal(t, cfg.NetworkConfig.UnicastMaxDialRetryAttemptTimes, dialCfg.DialRetryAttemptBudget)
}

// TestUnicastManager_StreamFactory_Connection_SuccessfulConnection_And_Stream tests that when there is no connection, and CreateStream is successful on the first attempt for connection and stream creation,
// it updates the last successful dial time and the consecutive successful stream counter.
func TestUnicastManager_Stream_BackoffBudgetResetToDefault(t *testing.T) {
	mgr, streamFactory, connStatus, dialConfigCache := unicastManagerFixture(t)
	peerID := p2ptest.PeerIdFixture(t)

	cfg, err := config.DefaultConfig()
	require.NoError(t, err)

	connStatus.On("IsConnected", peerID).Return(true, nil) // there is a connection.
	// mocks that it attempts to create a stream once and succeeds.
	streamFactory.On("NewStream", mock.Anything, peerID, mock.Anything).Return(&p2ptest.MockStream{}, nil).Once()

	// update the dial config of the peer to have a zero stream backoff budget but a consecutive successful stream counter above the reset threshold.
	adjustedCfg, err := dialConfigCache.Adjust(peerID, func(dialConfig unicast.DialConfig) (unicast.DialConfig, error) {
		dialConfig.StreamCreationRetryAttemptBudget = 0
		dialConfig.ConsecutiveSuccessfulStream = cfg.NetworkConfig.UnicastStreamZeroRetryResetThreshold + 1
		return dialConfig, nil
	})
	require.NoError(t, err)
	require.Equal(t, uint64(0), adjustedCfg.StreamCreationRetryAttemptBudget)
	require.Equal(t, cfg.NetworkConfig.UnicastStreamZeroRetryResetThreshold+1, adjustedCfg.ConsecutiveSuccessfulStream)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s, err := mgr.CreateStream(ctx, peerID)
	require.NoError(t, err)
	require.NotNil(t, s)

	// The dial config must be updated with the backoff budget decremented.
	dialCfg, err := dialConfigCache.GetOrInit(peerID)
	require.NoError(t, err)
	require.Equal(t, cfg.NetworkConfig.UnicastMaxDialRetryAttemptTimes, dialCfg.DialRetryAttemptBudget)                     // dial backoff budget must be intact.
	require.Equal(t, cfg.NetworkConfig.UnicastMaxStreamCreationRetryAttemptTimes, dialCfg.StreamCreationRetryAttemptBudget) // stream backoff budget must reset to default.
	require.True(t, dialCfg.LastSuccessfulDial.IsZero())                                                                    // last successful dial must be intact.
	// consecutive successful stream must increment by 1 (it was threshold + 1 before).
	require.Equal(t, cfg.NetworkConfig.UnicastStreamZeroRetryResetThreshold+1+1, dialCfg.ConsecutiveSuccessfulStream)
}

// TestUnicastManager_StreamFactory_Connection_SuccessfulConnection_And_Stream tests that when there is no connection, and CreateStream is successful on the first attempt for connection and stream creation,
// it updates the last successful dial time and the consecutive successful stream counter.
func TestUnicastManager_Stream_BackoffConnectionBudgetResetToDefault(t *testing.T) {
	mgr, streamFactory, connStatus, dialConfigCache := unicastManagerFixture(t)
	peerID := p2ptest.PeerIdFixture(t)

	cfg, err := config.DefaultConfig()
	require.NoError(t, err)

	connStatus.On("IsConnected", peerID).Return(false, nil)                                  // there is no connection.
	streamFactory.On("Connect", mock.Anything, peer.AddrInfo{ID: peerID}).Return(nil).Once() // connect on the first attempt.
	// mocks that it attempts to create a stream once and succeeds.
	streamFactory.On("NewStream", mock.Anything, peerID, mock.Anything).Return(&p2ptest.MockStream{}, nil).Once()

	// update the dial config of the peer to have a zero dial backoff budget but it has not been long enough since the last successful dial.
	adjustedCfg, err := dialConfigCache.Adjust(peerID, func(dialConfig unicast.DialConfig) (unicast.DialConfig, error) {
		dialConfig.DialRetryAttemptBudget = 0
		dialConfig.LastSuccessfulDial = time.Now().Add(-cfg.NetworkConfig.UnicastDialZeroRetryResetThreshold)
		return dialConfig, nil
	})
	require.NoError(t, err)
	require.Equal(t, uint64(0), adjustedCfg.DialRetryAttemptBudget)
	require.True(t, adjustedCfg.LastSuccessfulDial.Before(time.Now().Add(-cfg.NetworkConfig.UnicastDialZeroRetryResetThreshold))) // last successful dial must be within the threshold.

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dialTime := time.Now()
	s, err := mgr.CreateStream(ctx, peerID)
	require.NoError(t, err)
	require.NotNil(t, s)

	// The dial config must be updated with the backoff budget decremented.
	dialCfg, err := dialConfigCache.GetOrInit(peerID)
	require.NoError(t, err)
	require.Equal(t, cfg.NetworkConfig.UnicastMaxDialRetryAttemptTimes, dialCfg.DialRetryAttemptBudget)                     // dial backoff budget must be reset to default.
	require.Equal(t, cfg.NetworkConfig.UnicastMaxStreamCreationRetryAttemptTimes, dialCfg.StreamCreationRetryAttemptBudget) // stream backoff budget must be intact.
	require.True(t, dialCfg.LastSuccessfulDial.After(dialTime))                                                             // last successful dial must be updated when the dial was successful.
	require.Equal(t, uint64(1), dialCfg.ConsecutiveSuccessfulStream)                                                        // consecutive successful stream must be incremented by 1 (0 -> 1).
}

// TestUnicastManager_Connection_NoBackoff_When_Budget_Is_Zero tests that when there is no connection, and the dial backoff budget is zero and last successful dial is not within the zero reset threshold
// the unicast manager does not backoff if the dial attempt fails.
func TestUnicastManager_Connection_NoBackoff_When_Budget_Is_Zero(t *testing.T) {
	mgr, streamFactory, connStatus, dialConfigCache := unicastManagerFixture(t)
	peerID := p2ptest.PeerIdFixture(t)

	cfg, err := config.DefaultConfig()
	require.NoError(t, err)

	connStatus.On("IsConnected", peerID).Return(false, nil)                                                       // there is no connection.
	streamFactory.On("Connect", mock.Anything, peer.AddrInfo{ID: peerID}).Return(fmt.Errorf("some error")).Once() // connection is tried only once and fails.

	// update the dial config of the peer to have a zero dial backoff, and the last successful dial is not within the threshold.
	adjustedCfg, err := dialConfigCache.Adjust(peerID, func(dialConfig unicast.DialConfig) (unicast.DialConfig, error) {
		dialConfig.DialRetryAttemptBudget = 0                             // set the dial backoff budget to 0, meaning that the dial backoff budget is exhausted.
		dialConfig.LastSuccessfulDial = time.Now().Add(-10 * time.Minute) // last successful dial is not within the threshold.
		dialConfig.ConsecutiveSuccessfulStream = 2                        // set the consecutive successful stream to 2, meaning that the last 2 stream creation attempts were successful.
		return dialConfig, nil
	})
	require.NoError(t, err)
	require.Equal(t, uint64(0), adjustedCfg.DialRetryAttemptBudget)
	require.False(t, adjustedCfg.LastSuccessfulDial.Before(time.Now().Add(-cfg.NetworkConfig.UnicastDialZeroRetryResetThreshold))) // last successful dial must not be within the threshold.
	require.Equal(t, uint64(2), adjustedCfg.ConsecutiveSuccessfulStream)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s, err := mgr.CreateStream(ctx, peerID)
	require.Error(t, err)
	require.Nil(t, s)

	dialCfg, err := dialConfigCache.GetOrInit(peerID)
	require.NoError(t, err)
	require.Equal(t, uint64(0), dialCfg.DialRetryAttemptBudget)                                                             // dial backoff budget must remain at 0.
	require.Equal(t, cfg.NetworkConfig.UnicastMaxStreamCreationRetryAttemptTimes, dialCfg.StreamCreationRetryAttemptBudget) // stream backoff budget must be intact.
	require.True(t, dialCfg.LastSuccessfulDial.IsZero())                                                                    // last successful dial must be set to zero.
	require.Equal(t, uint64(0), dialCfg.ConsecutiveSuccessfulStream)                                                        // consecutive successful stream must be set to zero.
}

// TestUnicastManager_Stream_NoBackoff_When_Budget_Is_Zero tests that when there is a connection, and the stream backoff budget is zero and the consecutive successful stream counter is not above the
// zero rest threshold, the unicast manager does not backoff if the dial attempt fails.
func TestUnicastManager_Stream_NoBackoff_When_Budget_Is_Zero(t *testing.T) {
	mgr, streamFactory, connStatus, dialConfigCache := unicastManagerFixture(t)
	peerID := p2ptest.PeerIdFixture(t)

	cfg, err := config.DefaultConfig()
	require.NoError(t, err)

	connStatus.On("IsConnected", peerID).Return(true, nil) // there is a connection.
	// mocks that it attempts to create a stream once and fails, and does not retry.
	streamFactory.On("NewStream", mock.Anything, peerID, mock.Anything).Return(nil, fmt.Errorf("some error")).Once()

	// update the dial config of the peer to have a zero dial backoff, and the last successful dial is not within the threshold.
	lastSuccessfulDial := time.Now().Add(-10 * time.Minute)
	adjustedCfg, err := dialConfigCache.Adjust(peerID, func(dialConfig unicast.DialConfig) (unicast.DialConfig, error) {
		dialConfig.LastSuccessfulDial = lastSuccessfulDial // last successful dial is not within the threshold.
		dialConfig.ConsecutiveSuccessfulStream = 2         // set the consecutive successful stream to 2, which is below the reset threshold.
		dialConfig.StreamCreationRetryAttemptBudget = 0    // set the stream backoff budget to 0, meaning that the stream backoff budget is exhausted.
		return dialConfig, nil
	})
	require.NoError(t, err)
	require.Equal(t, uint64(0), adjustedCfg.StreamCreationRetryAttemptBudget)
	require.False(t, adjustedCfg.LastSuccessfulDial.Before(time.Now().Add(-cfg.NetworkConfig.UnicastDialZeroRetryResetThreshold))) // last successful dial must not be within the threshold.
	require.Equal(t, uint64(2), adjustedCfg.ConsecutiveSuccessfulStream)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s, err := mgr.CreateStream(ctx, peerID)
	require.Error(t, err)
	require.Nil(t, s)

	dialCfg, err := dialConfigCache.GetOrInit(peerID)
	require.NoError(t, err)
	require.Equal(t, cfg.NetworkConfig.UnicastMaxDialRetryAttemptTimes, dialCfg.DialRetryAttemptBudget) // dial backoff budget must remain intact.
	require.Equal(t, uint64(0), dialCfg.StreamCreationRetryAttemptBudget)                               // stream backoff budget must remain zero.
	require.Equal(t, lastSuccessfulDial, dialCfg.LastSuccessfulDial)                                    // last successful dial must be intact.
	require.Equal(t, uint64(0), dialCfg.ConsecutiveSuccessfulStream)                                    // consecutive successful stream must be set to zero.
}
