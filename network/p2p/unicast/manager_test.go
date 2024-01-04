package unicast_test

import (
	"context"
	"fmt"
	"testing"

	libp2pnet "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-libp2p/p2p/net/swarm"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/config"
	"github.com/onflow/flow-go/module/metrics"
	mockp2p "github.com/onflow/flow-go/network/p2p/mock"
	p2ptest "github.com/onflow/flow-go/network/p2p/test"
	"github.com/onflow/flow-go/network/p2p/unicast"
	unicastcache "github.com/onflow/flow-go/network/p2p/unicast/cache"
	"github.com/onflow/flow-go/network/p2p/unicast/stream"
	"github.com/onflow/flow-go/utils/unittest"
)

func unicastManagerFixture(t *testing.T) (*unicast.Manager, *mockp2p.StreamFactory, unicast.ConfigCache) {
	streamFactory := mockp2p.NewStreamFactory(t)
	streamFactory.On("SetStreamHandler", mock.AnythingOfType("protocol.ID"), mock.AnythingOfType("network.StreamHandler")).Return().Once()

	cfg, err := config.DefaultConfig()
	require.NoError(t, err)

	unicastConfigCache := unicastcache.NewUnicastConfigCache(cfg.NetworkConfig.Unicast.UnicastManager.ConfigCacheSize,
		unittest.Logger(),
		metrics.NewNoopCollector(),
		func() unicast.Config {
			return unicast.Config{
				StreamCreationRetryAttemptBudget: cfg.NetworkConfig.Unicast.UnicastManager.MaxStreamCreationRetryAttemptTimes,
			}
		})

	mgr, err := unicast.NewUnicastManager(&unicast.ManagerConfig{
		Logger:        unittest.Logger(),
		StreamFactory: streamFactory,
		SporkId:       unittest.IdentifierFixture(),
		Metrics:       metrics.NewNoopCollector(),
		Parameters:    &cfg.NetworkConfig.Unicast.UnicastManager,
		UnicastConfigCacheFactory: func(func() unicast.Config) unicast.ConfigCache {
			return unicastConfigCache
		},
	})
	require.NoError(t, err)
	mgr.SetDefaultHandler(func(libp2pnet.Stream) {}) // no-op handler, we don't care about the handler for this test

	return mgr, streamFactory, unicastConfigCache
}

// TestManagerConfigValidation tests the validation of the unicast manager config.
// It tests that the config is valid when all the required fields are provided.
func TestManagerConfigValidation(t *testing.T) {
	cfg, err := config.DefaultConfig()
	require.NoError(t, err)

	validConfig := unicast.ManagerConfig{
		Logger:        unittest.Logger(),
		StreamFactory: mockp2p.NewStreamFactory(t),
		SporkId:       unittest.IdentifierFixture(),
		Parameters:    &cfg.NetworkConfig.Unicast.UnicastManager,
		Metrics:       metrics.NewNoopCollector(),
		UnicastConfigCacheFactory: func(func() unicast.Config) unicast.ConfigCache {
			return unicastcache.NewUnicastConfigCache(cfg.NetworkConfig.Unicast.UnicastManager.ConfigCacheSize,
				unittest.Logger(),
				metrics.NewNoopCollector(),
				func() unicast.Config {
					return unicast.Config{
						StreamCreationRetryAttemptBudget: cfg.NetworkConfig.Unicast.UnicastManager.MaxStreamCreationRetryAttemptTimes,
					}
				})
		},
	}

	t.Run("Valid Config", func(t *testing.T) {
		mgr, err := unicast.NewUnicastManager(&validConfig)
		require.NoError(t, err)
		require.NotNil(t, mgr)
	})

	t.Run("Missing Fields", func(t *testing.T) {
		cfg := &unicast.ManagerConfig{}
		mgr, err := unicast.NewUnicastManager(cfg)
		require.Error(t, err)
		require.Nil(t, mgr)
	})

	t.Run("Nil Parameters", func(t *testing.T) {
		cfg := validConfig
		cfg.Parameters = nil
		mgr, err := unicast.NewUnicastManager(&cfg)
		require.Error(t, err)
		require.Nil(t, mgr)
	})

	t.Run("Invalid UnicastConfigCacheFactory", func(t *testing.T) {
		cfg := validConfig
		cfg.UnicastConfigCacheFactory = nil
		mgr, err := unicast.NewUnicastManager(&cfg)
		require.Error(t, err)
		require.Nil(t, mgr)
	})

	t.Run("Missing StreamFactory", func(t *testing.T) {
		cfg := validConfig
		cfg.StreamFactory = nil
		mgr, err := unicast.NewUnicastManager(&cfg)
		require.Error(t, err)
		require.Nil(t, mgr)
	})

	t.Run("Missing Metrics", func(t *testing.T) {
		cfg := validConfig
		cfg.Metrics = nil
		mgr, err := unicast.NewUnicastManager(&cfg)
		require.Error(t, err)
		require.Nil(t, mgr)
	})
}

// TestUnicastManager_SuccessfulStream tests that when CreateStream is successful on the first attempt for stream creation,
// it updates the consecutive successful stream counter.
func TestUnicastManager_SuccessfulStream(t *testing.T) {
	peerID := unittest.PeerIdFixture(t)
	mgr, streamFactory, configCache := unicastManagerFixture(t)

	cfg, err := config.DefaultConfig()
	require.NoError(t, err)

	streamFactory.On("NewStream", mock.Anything, peerID, mock.Anything).Return(&p2ptest.MockStream{}, nil).Once()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s, err := mgr.CreateStream(ctx, peerID)
	require.NoError(t, err)
	require.NotNil(t, s)

	// The unicast config must be updated with the backoff budget decremented.
	unicastCfg, err := configCache.GetOrInit(peerID)
	require.NoError(t, err)
	require.Equal(t, cfg.NetworkConfig.Unicast.UnicastManager.MaxStreamCreationRetryAttemptTimes, unicastCfg.StreamCreationRetryAttemptBudget) // stream backoff budget must remain intact.
	require.Equal(t, uint64(1), unicastCfg.ConsecutiveSuccessfulStream)                                                                        // consecutive successful stream must incremented.
}

// TestUnicastManager_StreamBackoff tests the backoff mechanism of the unicast manager for stream creation.
// It tests the situation that CreateStream is called but the stream creation fails.
// It tests that it tries to create a stream some number of times (unicastmodel.MaxStreamCreationAttemptTimes), before giving up.
// It also checks the consecutive successful stream counter is reset when the stream creation fails.
func TestUnicastManager_StreamBackoff(t *testing.T) {
	peerID := unittest.PeerIdFixture(t)
	mgr, streamFactory, configCache := unicastManagerFixture(t)

	cfg, err := config.DefaultConfig()
	require.NoError(t, err)

	// mocks that it attempts to create a stream some number of times, before giving up.
	streamFactory.On("NewStream", mock.Anything, peerID, mock.Anything).
		Return(nil, fmt.Errorf("some error")).
		Times(int(cfg.NetworkConfig.Unicast.UnicastManager.MaxStreamCreationRetryAttemptTimes + 1))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s, err := mgr.CreateStream(ctx, peerID)
	require.Error(t, err)
	require.Nil(t, s)

	// The unicast config must be updated with the backoff budget decremented.
	unicastCfg, err := configCache.GetOrInit(peerID)
	require.NoError(t, err)
	// stream backoff budget must be decremented by 1 since all budget is used up.
	require.Equal(t, cfg.NetworkConfig.Unicast.UnicastManager.MaxStreamCreationRetryAttemptTimes-1, unicastCfg.StreamCreationRetryAttemptBudget)
	// consecutive successful stream must be reset to zero, since the stream creation failed.
	require.Equal(t, uint64(0), unicastCfg.ConsecutiveSuccessfulStream)
}

// TestUnicastManager_StreamFactory_StreamBackoff tests the backoff mechanism of the unicast manager for stream creation.
// It tests when there is a connection, but no stream, it tries to create a stream some number of times (unicastmodel.MaxStreamCreationAttemptTimes), before
// giving up.
func TestUnicastManager_StreamFactory_StreamBackoff(t *testing.T) {
	mgr, streamFactory, unicastConfigCache := unicastManagerFixture(t)
	peerID := unittest.PeerIdFixture(t)

	cfg, err := config.DefaultConfig()
	require.NoError(t, err)

	// mocks that it attempts to create a stream some number of times, before giving up.
	streamFactory.On("NewStream", mock.Anything, peerID, mock.Anything).
		Return(nil, fmt.Errorf("some error")).
		Times(int(cfg.NetworkConfig.Unicast.UnicastManager.MaxStreamCreationRetryAttemptTimes + 1))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s, err := mgr.CreateStream(ctx, peerID)
	require.Error(t, err)
	require.Nil(t, s)

	// The unicast config must be updated with the stream backoff budget decremented.
	unicastCfg, err := unicastConfigCache.GetOrInit(peerID)
	require.NoError(t, err)
	// stream backoff budget must be decremented by 1.
	require.Equal(t, cfg.NetworkConfig.Unicast.UnicastManager.MaxStreamCreationRetryAttemptTimes-1, unicastCfg.StreamCreationRetryAttemptBudget)
	// consecutive successful stream must be zero as we have not created a successful stream yet.
	require.Equal(t, uint64(0), unicastCfg.ConsecutiveSuccessfulStream)
}

// TestUnicastManager_Stream_ConsecutiveStreamCreation_Increment tests that when stream creation is successful,
// it increments the consecutive successful stream counter in the unicast config.
func TestUnicastManager_Stream_ConsecutiveStreamCreation_Increment(t *testing.T) {
	mgr, streamFactory, unicastConfigCache := unicastManagerFixture(t)
	peerID := unittest.PeerIdFixture(t)

	cfg, err := config.DefaultConfig()
	require.NoError(t, err)

	// total times we successfully create a stream to the peer.
	totalSuccessAttempts := 10

	// mocks that it attempts to create a stream 10 times, and each time it succeeds.
	streamFactory.On("NewStream", mock.Anything, peerID, mock.Anything).Return(&p2ptest.MockStream{}, nil).Times(totalSuccessAttempts)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for i := 0; i < totalSuccessAttempts; i++ {
		s, err := mgr.CreateStream(ctx, peerID)
		require.NoError(t, err)
		require.NotNil(t, s)

		// The unicast config must be updated with the stream backoff budget decremented.
		unicastCfg, err := unicastConfigCache.GetOrInit(peerID)
		require.NoError(t, err)
		// stream backoff budget must be intact (all stream creation attempts are successful).
		require.Equal(t, cfg.NetworkConfig.Unicast.UnicastManager.MaxStreamCreationRetryAttemptTimes, unicastCfg.StreamCreationRetryAttemptBudget)
		// consecutive successful stream must be incremented.
		require.Equal(t, uint64(i+1), unicastCfg.ConsecutiveSuccessfulStream)
	}
}

// TestUnicastManager_Stream_ConsecutiveStreamCreation_Reset tests that when the stream creation fails, it resets
// the consecutive successful stream counter in the unicast config.
func TestUnicastManager_Stream_ConsecutiveStreamCreation_Reset(t *testing.T) {
	mgr, streamFactory, unicastConfigCache := unicastManagerFixture(t)
	peerID := unittest.PeerIdFixture(t)

	// mocks that it attempts to create a stream once and fails.
	streamFactory.On("NewStream", mock.Anything, peerID, mock.Anything).
		Return(nil, fmt.Errorf("some error")).
		Once()

	adjustedUnicastConfig, err := unicastConfigCache.AdjustWithInit(peerID, func(unicastConfig unicast.Config) (unicast.Config, error) {
		// sets the consecutive successful stream to 5 meaning that the last 5 stream creation attempts were successful.
		unicastConfig.ConsecutiveSuccessfulStream = 5
		// sets the stream back budget to 0 meaning that the stream backoff budget is exhausted.
		unicastConfig.StreamCreationRetryAttemptBudget = 0

		return unicastConfig, nil
	})
	require.NoError(t, err)
	require.Equal(t, uint64(5), adjustedUnicastConfig.ConsecutiveSuccessfulStream)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s, err := mgr.CreateStream(ctx, peerID)
	require.Error(t, err)
	require.Nil(t, s)

	// The unicast config must be updated with the stream backoff budget decremented.
	unicastCfg, err := unicastConfigCache.GetOrInit(peerID)
	require.NoError(t, err)

	// stream backoff budget must be intact (we can't decrement it below 0).
	require.Equal(t, uint64(0), unicastCfg.StreamCreationRetryAttemptBudget)
	// consecutive successful stream must be reset to 0.
	require.Equal(t, uint64(0), unicastCfg.ConsecutiveSuccessfulStream)
}

// TestUnicastManager_StreamFactory_ErrProtocolNotSupported tests that when there is a protocol not supported error, it does not retry creating a stream.
func TestUnicastManager_StreamFactory_ErrProtocolNotSupported(t *testing.T) {
	mgr, streamFactory, _ := unicastManagerFixture(t)
	peerID := unittest.PeerIdFixture(t)

	// mocks that upon creating a stream, it returns a protocol not supported error, the mock is set to once, meaning that it won't retry stream creation again.
	streamFactory.On("NewStream", mock.Anything, peerID, mock.Anything).
		Return(nil, stream.NewProtocolNotSupportedErr(peerID, protocol.ID("protocol-1"), fmt.Errorf("some error"))).
		Once()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s, err := mgr.CreateStream(ctx, peerID)
	require.Error(t, err)
	require.Nil(t, s)
}

// TestUnicastManager_StreamFactory_ErrNoAddresses tests that when stream creation returns a no addresses error,
// it does not retry stream creation again and returns an error immediately.
func TestUnicastManager_StreamFactory_ErrNoAddresses(t *testing.T) {
	mgr, streamFactory, unicastConfigCache := unicastManagerFixture(t)

	cfg, err := config.DefaultConfig()
	require.NoError(t, err)

	peerID := unittest.PeerIdFixture(t)

	// mocks that stream creation returns a no addresses error, and the mock is set to once, meaning that it won't retry stream creation again.
	streamFactory.On("NewStream", mock.Anything, peerID, mock.Anything).
		Return(nil, fmt.Errorf("some error to ensure wrapping works fine: %w", swarm.ErrNoAddresses)).
		Once()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s, err := mgr.CreateStream(ctx, peerID)
	require.Error(t, err)
	require.Nil(t, s)

	unicastCfg, err := unicastConfigCache.GetOrInit(peerID)
	require.NoError(t, err)

	// stream backoff budget must be reduced by 1 due to failed stream creation.
	require.Equal(t, cfg.NetworkConfig.Unicast.UnicastManager.MaxStreamCreationRetryAttemptTimes-1, unicastCfg.StreamCreationRetryAttemptBudget)
	// consecutive successful stream must be set to zero.
	require.Equal(t, uint64(0), unicastCfg.ConsecutiveSuccessfulStream)
}

// TestUnicastManager_Stream_ErrSecurityProtocolNegotiationFailed tests that when there is a security protocol negotiation error, it does not retry stream creation.
func TestUnicastManager_Stream_ErrSecurityProtocolNegotiationFailed(t *testing.T) {
	mgr, streamFactory, unicastConfigCache := unicastManagerFixture(t)

	cfg, err := config.DefaultConfig()
	require.NoError(t, err)

	peerID := unittest.PeerIdFixture(t)

	// mocks that stream creation returns a security protocol negotiation error, and the mock is set to once, meaning that it won't retry stream creation.
	streamFactory.On("NewStream", mock.Anything, peerID, mock.Anything).
		Return(nil, stream.NewSecurityProtocolNegotiationErr(peerID, fmt.Errorf("some error"))).
		Once()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s, err := mgr.CreateStream(ctx, peerID)
	require.Error(t, err)
	require.Nil(t, s)

	unicastCfg, err := unicastConfigCache.GetOrInit(peerID)
	require.NoError(t, err)
	// stream retry budget must be decremented by 1 (since we didn't have a successful stream creation, the budget is decremented).
	require.Equal(t, cfg.NetworkConfig.Unicast.UnicastManager.MaxStreamCreationRetryAttemptTimes-1, unicastCfg.StreamCreationRetryAttemptBudget)
	// consecutive successful stream must be set to zero.
	require.Equal(t, uint64(0), unicastCfg.ConsecutiveSuccessfulStream)
}

// TestUnicastManager_StreamFactory_ErrGaterDisallowedConnection tests that when there is a connection-gater disallow listing error, it does not retry stream creation.
func TestUnicastManager_StreamFactory_ErrGaterDisallowedConnection(t *testing.T) {
	mgr, streamFactory, unicastConfigCache := unicastManagerFixture(t)
	peerID := unittest.PeerIdFixture(t)

	cfg, err := config.DefaultConfig()
	require.NoError(t, err)

	// mocks that stream creation to the peer returns a connection gater disallow-listing, and the mock is set to once, meaning that it won't retry stream creation.
	streamFactory.On("NewStream", mock.Anything, peerID, mock.Anything).
		Return(nil, stream.NewGaterDisallowedConnectionErr(fmt.Errorf("some error"))).
		Once()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s, err := mgr.CreateStream(ctx, peerID)
	require.Error(t, err)
	require.Nil(t, s)

	unicastCfg, err := unicastConfigCache.GetOrInit(peerID)
	require.NoError(t, err)
	// stream backoff budget must be reduced by 1 due to failed stream creation.
	require.Equal(t, cfg.NetworkConfig.Unicast.UnicastManager.MaxStreamCreationRetryAttemptTimes-1, unicastCfg.StreamCreationRetryAttemptBudget)
	// consecutive successful stream must be set to zero.
	require.Equal(t, uint64(0), unicastCfg.ConsecutiveSuccessfulStream)
}

// TestUnicastManager_Connection_BackoffBudgetDecremented tests that everytime the unicast manger gives up on creating a stream (after retrials),
// it decrements the backoff budget for the remote peer.
func TestUnicastManager_Stream_BackoffBudgetDecremented(t *testing.T) {
	mgr, streamFactory, unicastConfigCache := unicastManagerFixture(t)
	peerID := unittest.PeerIdFixture(t)

	cfg, err := config.DefaultConfig()
	require.NoError(t, err)

	// totalAttempts is the total number of times that unicast manager calls NewStream on the stream factory to create stream to the peer.
	// Note that it already assumes that the connection is established, so it does not try to connect to the peer.
	// Let's consider x = unicastmodel.MaxStreamCreationRetryAttemptTimes + 1. Then the test tries x times CreateStream. With dynamic backoffs,
	// the first CreateStream call will try to NewStream x times, the second CreateStream call will try to NewStream x-1 times,
	// and so on. So the total number of Connect calls is x + (x-1) + (x-2) + ... + 1 = x(x+1)/2.
	maxStreamRetryBudget := cfg.NetworkConfig.Unicast.UnicastManager.MaxStreamCreationRetryAttemptTimes
	maxStreamAttempt := maxStreamRetryBudget + 1 // 1 attempt + retry times
	totalAttempts := maxStreamAttempt * (maxStreamAttempt + 1) / 2

	streamFactory.On("NewStream", mock.Anything, peerID, mock.Anything).
		Return(nil, fmt.Errorf("some error")).
		Times(int(totalAttempts))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for i := 0; i < int(maxStreamRetryBudget); i++ {
		s, err := mgr.CreateStream(ctx, peerID)
		require.Error(t, err)
		require.Nil(t, s)

		unicastCfg, err := unicastConfigCache.GetOrInit(peerID)
		require.NoError(t, err)

		if i == int(maxStreamRetryBudget)-1 {
			require.Equal(t, uint64(0), unicastCfg.StreamCreationRetryAttemptBudget)
		} else {
			require.Equal(t, maxStreamRetryBudget-uint64(i)-1, unicastCfg.StreamCreationRetryAttemptBudget)
		}
	}
	// At this time the backoff budget for connection must be 0.
	unicastCfg, err := unicastConfigCache.GetOrInit(peerID)
	require.NoError(t, err)
	require.Equal(t, uint64(0), unicastCfg.StreamCreationRetryAttemptBudget)

	// After all the backoff budget is used up, it should stay at 0.
	s, err := mgr.CreateStream(ctx, peerID)
	require.Error(t, err)
	require.Nil(t, s)

	unicastCfg, err = unicastConfigCache.GetOrInit(peerID)
	require.NoError(t, err)
	require.Equal(t, uint64(0), unicastCfg.StreamCreationRetryAttemptBudget)
}

// TestUnicastManager_Stream_BackoffBudgetResetToDefault tests that when the stream retry attempt budget is zero, and the consecutive successful stream counter is above the reset threshold,
// it resets the stream retry attempt budget to the default value and increments the consecutive successful stream counter.
func TestUnicastManager_Stream_BackoffBudgetResetToDefault(t *testing.T) {
	mgr, streamFactory, unicastConfigCache := unicastManagerFixture(t)
	peerID := unittest.PeerIdFixture(t)

	cfg, err := config.DefaultConfig()
	require.NoError(t, err)

	// mocks that it attempts to create a stream once and succeeds.
	streamFactory.On("NewStream", mock.Anything, peerID, mock.Anything).Return(&p2ptest.MockStream{}, nil).Once()

	// update the unicast config of the peer to have a zero stream backoff budget but a consecutive successful stream counter above the reset threshold.
	adjustedCfg, err := unicastConfigCache.AdjustWithInit(peerID, func(unicastConfig unicast.Config) (unicast.Config, error) {
		unicastConfig.StreamCreationRetryAttemptBudget = 0
		unicastConfig.ConsecutiveSuccessfulStream = cfg.NetworkConfig.Unicast.UnicastManager.StreamZeroRetryResetThreshold + 1
		return unicastConfig, nil
	})
	require.NoError(t, err)
	require.Equal(t, uint64(0), adjustedCfg.StreamCreationRetryAttemptBudget)
	require.Equal(t, cfg.NetworkConfig.Unicast.UnicastManager.StreamZeroRetryResetThreshold+1, adjustedCfg.ConsecutiveSuccessfulStream)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s, err := mgr.CreateStream(ctx, peerID)
	require.NoError(t, err)
	require.NotNil(t, s)

	unicastCfg, err := unicastConfigCache.GetOrInit(peerID)
	require.NoError(t, err)
	// stream backoff budget must reset to default.
	require.Equal(t, cfg.NetworkConfig.Unicast.UnicastManager.MaxStreamCreationRetryAttemptTimes, unicastCfg.StreamCreationRetryAttemptBudget)
	// consecutive successful stream must increment by 1 (it was threshold + 1 before).
	require.Equal(t, cfg.NetworkConfig.Unicast.UnicastManager.StreamZeroRetryResetThreshold+1+1, unicastCfg.ConsecutiveSuccessfulStream)
}

// TestUnicastManager_Stream_NoBackoff_When_Budget_Is_Zero tests that when the stream backoff budget is zero and the consecutive successful stream counter is not above the
// zero rest threshold, the unicast manager does not backoff if the stream creation attempt fails.
func TestUnicastManager_Stream_NoBackoff_When_Budget_Is_Zero(t *testing.T) {
	mgr, streamFactory, unicastConfigCache := unicastManagerFixture(t)
	peerID := unittest.PeerIdFixture(t)

	// mocks that it attempts to create a stream once and fails, and does not retry.
	streamFactory.On("NewStream", mock.Anything, peerID, mock.Anything).Return(nil, fmt.Errorf("some error")).Once()

	adjustedCfg, err := unicastConfigCache.AdjustWithInit(peerID, func(unicastConfig unicast.Config) (unicast.Config, error) {
		unicastConfig.ConsecutiveSuccessfulStream = 2      // set the consecutive successful stream to 2, which is below the reset threshold.
		unicastConfig.StreamCreationRetryAttemptBudget = 0 // set the stream backoff budget to 0, meaning that the stream backoff budget is exhausted.
		return unicastConfig, nil
	})
	require.NoError(t, err)
	require.Equal(t, uint64(0), adjustedCfg.StreamCreationRetryAttemptBudget)
	require.Equal(t, uint64(2), adjustedCfg.ConsecutiveSuccessfulStream)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s, err := mgr.CreateStream(ctx, peerID)
	require.Error(t, err)
	require.Nil(t, s)

	unicastCfg, err := unicastConfigCache.GetOrInit(peerID)
	require.NoError(t, err)
	require.Equal(t, uint64(0), unicastCfg.StreamCreationRetryAttemptBudget) // stream backoff budget must remain zero.
	require.Equal(t, uint64(0), unicastCfg.ConsecutiveSuccessfulStream)      // consecutive successful stream must be set to zero.
}
