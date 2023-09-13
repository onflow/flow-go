package internal_test

import (
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network/p2p/p2pnode"
	p2ptest "github.com/onflow/flow-go/network/p2p/test"
	"github.com/onflow/flow-go/network/p2p/unicast/internal"
	"github.com/onflow/flow-go/network/p2p/unicast/model"
)

// TestNewDialConfigCache tests the creation of a new DialConfigCache.
// It asserts that the cache is created and its size is 0.
func TestNewDialConfigCache(t *testing.T) {
	sizeLimit := uint32(100)
	logger := zerolog.Nop()
	collector := metrics.NewNoopCollector()
	cache := internal.NewDialConfigCache(sizeLimit, logger, collector, dialConfigFixture)
	require.NotNil(t, cache)
	require.Equalf(t, uint(0), cache.Size(), "cache size must be 0")
}

// dialConfigFixture returns a dial config fixture.
// The dial config is initialized with the default values.
func dialConfigFixture() model.DialConfig {
	return model.DialConfig{
		DialBackoff:        p2pnode.MaxConnectAttempt,
		StreamBackoff:      p2pnode.MaxStreamCreationAttempt,
		LastSuccessfulDial: 0,
	}
}

// TestDialConfigCache_Adjust tests the Adjust method of the DialConfigCache. It asserts that the dial config is initialized, adjusted,
// and stored in the cache.
func TestDialConfigCache_Adjust_Init(t *testing.T) {
	sizeLimit := uint32(100)
	logger := zerolog.Nop()
	collector := metrics.NewNoopCollector()

	dialFactoryCalled := 0
	dialConfigFactory := func() model.DialConfig {
		require.Less(t, dialFactoryCalled, 2, "dial config factory must be called at most twice")
		dialFactoryCalled++
		return dialConfigFixture()
	}
	adjustFuncIncrement := func(cfg model.DialConfig) (model.DialConfig, error) {
		cfg.DialBackoff++
		return cfg, nil
	}

	cache := internal.NewDialConfigCache(sizeLimit, logger, collector, dialConfigFactory)
	require.NotNil(t, cache)
	require.Zerof(t, cache.Size(), "cache size must be 0")

	peerID1 := p2ptest.PeerIdFixture(t)
	peerID2 := p2ptest.PeerIdFixture(t)

	// initially the dial config for peerID1 and peerID2 do not exist in the cache.
	cfg, ok := cache.Get(peerID1)
	require.False(t, ok, "dial config must not exist in the cache")
	require.Nil(t, cfg, "dial config must be nil")
	cfg, ok = cache.Get(peerID2)
	require.False(t, ok, "dial config must not exist in the cache")
	require.Nil(t, cfg, "dial config must be nil")

	// adjust the dial config for peerID1; it does not exist in the cache, so it must be initialized.
	err := cache.Adjust(peerID1, adjustFuncIncrement)
	require.NoError(t, err)
	require.Equal(t, uint(1), cache.Size(), "cache size must be 1")

	// retrieve the dial config for peerID1 and assert that it is initialized with the default values; and the adjust function is applied.
	cfg, ok = cache.Get(peerID1)
	require.True(t, ok, "dial config must exist in the cache")
	require.NotNil(t, cfg, "dial config must not be nil")
	require.Equal(t, cfg.LastSuccessfulDial, dialConfigFixture().LastSuccessfulDial, "last successful dial must be 0")
	require.Equal(t, cfg.DialBackoff, dialConfigFixture().DialBackoff+1, "dial backoff must be adjusted")
	require.Equal(t, cfg.StreamBackoff, dialConfigFixture().StreamBackoff, "stream backoff must be 1")

	// adjusting the dial config for peerID2; it does not exist in the cache, so it must be initialized.
	err = cache.Adjust(peerID2, adjustFuncIncrement)
	require.NoError(t, err)
	require.Equal(t, uint(2), cache.Size(), "cache size must be 2")
	require.Equal(t, 2, dialFactoryCalled, "dial config factory must be called twice")

	// retrieve the dial config for peerID2 and assert that it is initialized with the default values; and the adjust function is applied.
	cfg, ok = cache.Get(peerID2)
	require.True(t, ok, "dial config must exist in the cache")
	require.NotNil(t, cfg, "dial config must not be nil")
	require.Equal(t, cfg.LastSuccessfulDial, dialConfigFixture().LastSuccessfulDial, "last successful dial must be 0")
	require.Equal(t, cfg.DialBackoff, dialConfigFixture().DialBackoff+1, "dial backoff must be adjusted")
	require.Equal(t, cfg.StreamBackoff, dialConfigFixture().StreamBackoff, "stream backoff must be 1")

	// adjust the dial config for peerID1 again; it exists in the cache, so the adjust function must be applied without initializing the config.
	err = cache.Adjust(peerID1, adjustFuncIncrement)
	require.NoError(t, err)
	require.Equal(t, uint(2), cache.Size(), "cache size must be 2")
	require.Equal(t, 2, dialFactoryCalled, "dial config factory must be called twice")

	// retrieve the dial config for peerID1 and assert that it is initialized with the default values; and the adjust function is applied.
	cfg, ok = cache.Get(peerID1)
	require.True(t, ok, "dial config must exist in the cache")
	require.NotNil(t, cfg, "dial config must not be nil")
	require.Equal(t, cfg.LastSuccessfulDial, dialConfigFixture().LastSuccessfulDial, "last successful dial must be 0")
	require.Equal(t, cfg.DialBackoff, dialConfigFixture().DialBackoff+2, "dial backoff must be adjusted")
	require.Equal(t, cfg.StreamBackoff, dialConfigFixture().StreamBackoff, "stream backoff must be 1")
}
