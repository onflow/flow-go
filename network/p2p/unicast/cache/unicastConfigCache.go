package unicastcache

import (
	"fmt"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/mempool"
	herocache "github.com/onflow/flow-go/module/mempool/herocache/backdata"
	"github.com/onflow/flow-go/module/mempool/herocache/backdata/heropool"
	"github.com/onflow/flow-go/module/mempool/stdmap"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/unicast"
)

// UnicastConfigCache cache that stores the unicast configs for all types of nodes.
// Stored configs are keyed by the hash of the peerID.
type UnicastConfigCache struct {
	peerCache  *stdmap.Backend[flow.Identifier, unicast.Config]
	cfgFactory func() unicast.Config // factory function that creates a new unicast config.
}

var _ unicast.ConfigCache = (*UnicastConfigCache)(nil)

// NewUnicastConfigCache creates a new UnicastConfigCache.
// Args:
// - size: the maximum number of unicast configs that the cache can hold.
// - logger: the logger used by the cache.
// - collector: the metrics collector used by the cache.
// - cfgFactory: a factory function that creates a new unicast config.
// Returns:
// - *UnicastConfigCache, the created cache.
// Note that the cache is supposed to keep the unicast config for all types of nodes. Since the number of such nodes is
// expected to be small, size must be large enough to hold all the unicast configs of the authorized nodes.
// To avoid any crash-failure, the cache is configured to eject the least recently used configs when the cache is full.
// Hence, we recommend setting the size to a large value to minimize the ejections.
func NewUnicastConfigCache(
	size uint32,
	logger zerolog.Logger,
	collector module.HeroCacheMetrics,
	cfgFactory func() unicast.Config,
) *UnicastConfigCache {
	return &UnicastConfigCache{
		peerCache: stdmap.NewBackend(
			stdmap.WithMutableBackData[flow.Identifier, unicast.Config](
				herocache.NewCache[unicast.Config](
					size,
					herocache.DefaultOversizeFactor,
					heropool.LRUEjection,
					logger.With().Str("module", "unicast-config-cache").Logger(),
					collector,
				),
			),
		),
		cfgFactory: cfgFactory,
	}
}

// AdjustWithInit applies the given adjust function to the unicast config of the given peer ID, and stores the adjusted config in the cache.
// It returns an error if the adjustFunc returns an error.
// Note that if the Adjust is called when the config does not exist, the config is initialized and the
// adjust function is applied to the initialized config again. In this case, the adjust function should not return an error.
// Args:
// - peerID: the peer id of the unicast config.
// - adjustFunc: the function that adjusts the unicast config.
// Returns:
//   - error any returned error should be considered as an irrecoverable error and indicates a bug.
func (d *UnicastConfigCache) AdjustWithInit(peerID peer.ID, adjustFunc unicast.UnicastConfigAdjustFunc) (*unicast.Config, error) {
	var rErr error
	// wraps external adjust function to adjust the unicast config.
	wrapAdjustFunc := func(config unicast.Config) unicast.Config {
		// adjust the unicast config.
		adjustedCfg, err := adjustFunc(config)
		if err != nil {
			rErr = fmt.Errorf("adjust function failed: %w", err)
			return config // returns the original config (reverse the adjustment).
		}

		// Return the adjusted config.
		return adjustedCfg
	}

	initFunc := func() unicast.Config {
		return d.cfgFactory()
	}

	adjustedConfig, adjusted := d.peerCache.AdjustWithInit(p2p.MakeId(peerID), wrapAdjustFunc, initFunc)
	if rErr != nil {
		return nil, fmt.Errorf("adjust operation aborted with an error: %w", rErr)
	}

	if !adjusted {
		return nil, fmt.Errorf("adjust operation aborted, unicast config was not adjusted")
	}

	return &unicast.Config{
		StreamCreationRetryAttemptBudget: adjustedConfig.StreamCreationRetryAttemptBudget,
		ConsecutiveSuccessfulStream:      adjustedConfig.ConsecutiveSuccessfulStream,
	}, nil
}

// GetWithInit returns the unicast config for the given peer id. If the config does not exist, it creates a new config
// using the factory function and stores it in the cache.
// Args:
// - peerID: the peer id of the unicast config.
// Returns:
//   - *Config, the unicast config for the given peer id.
//   - error if the factory function returns an error. Any error should be treated as an irrecoverable error and indicates a bug.
func (d *UnicastConfigCache) GetWithInit(peerID peer.ID) (*unicast.Config, error) {
	// ensuring that the init-and-get operation is atomic.
	key := p2p.MakeId(peerID)

	var config unicast.Config
	err := d.peerCache.Run(func(backData mempool.BackData[flow.Identifier, unicast.Config]) error {
		val, ok := backData.Get(key)
		if ok {
			config = val
			return nil
		}

		config = d.cfgFactory()
		backData.Add(key, config)

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("run operation aborted with an error: %w", err)
	}

	// return a copy of the config (we do not want the caller to modify the config).
	return &unicast.Config{
		StreamCreationRetryAttemptBudget: config.StreamCreationRetryAttemptBudget,
		ConsecutiveSuccessfulStream:      config.ConsecutiveSuccessfulStream,
	}, nil
}

// Size returns the number of unicast configs in the cache.
func (d *UnicastConfigCache) Size() uint {
	return d.peerCache.Size()
}
