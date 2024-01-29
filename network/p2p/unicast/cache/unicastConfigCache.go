package unicastcache

import (
	"fmt"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	herocache "github.com/onflow/flow-go/module/mempool/herocache/backdata"
	"github.com/onflow/flow-go/module/mempool/herocache/backdata/heropool"
	"github.com/onflow/flow-go/module/mempool/stdmap"
	"github.com/onflow/flow-go/network/p2p/unicast"
)

type UnicastConfigCache struct {
	peerCache  *stdmap.Backend
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
		peerCache: stdmap.NewBackend(stdmap.WithBackData(herocache.NewCache(size,
			herocache.DefaultOversizeFactor,
			heropool.LRUEjection,
			logger.With().Str("module", "unicast-config-cache").Logger(),
			collector))),
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
	entityId := entityIdOf(peerID)
	var rErr error
	// wraps external adjust function to adjust the unicast config.
	wrapAdjustFunc := func(entity flow.Entity) flow.Entity {
		cfgEntity, ok := entity.(UnicastConfigEntity)
		if !ok {
			// sanity check
			// This should never happen, because the cache only contains UnicastConfigEntity entities.
			panic(fmt.Sprintf("invalid entity type, expected UnicastConfigEntity type, got: %T", entity))
		}

		// adjust the unicast config.
		adjustedCfg, err := adjustFunc(cfgEntity.Config)
		if err != nil {
			rErr = fmt.Errorf("adjust function failed: %w", err)
			return entity // returns the original entity (reverse the adjustment).
		}

		// Return the adjusted config.
		cfgEntity.Config = adjustedCfg
		return cfgEntity
	}

	initFunc := func() flow.Entity {
		return UnicastConfigEntity{
			PeerId:   peerID,
			Config:   d.cfgFactory(),
			EntityId: entityId,
		}
	}

	adjustedEntity, adjusted := d.peerCache.AdjustWithInit(entityId, wrapAdjustFunc, initFunc)
	if rErr != nil {
		return nil, fmt.Errorf("adjust operation aborted with an error: %w", rErr)
	}

	if !adjusted {
		return nil, fmt.Errorf("adjust operation aborted, entity not found")
	}

	return &unicast.Config{
		StreamCreationRetryAttemptBudget: adjustedEntity.(UnicastConfigEntity).StreamCreationRetryAttemptBudget,
		ConsecutiveSuccessfulStream:      adjustedEntity.(UnicastConfigEntity).ConsecutiveSuccessfulStream,
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
	entityId := entityIdOf(peerID)
	initFunc := func() flow.Entity {
		return UnicastConfigEntity{
			PeerId:   peerID,
			Config:   d.cfgFactory(),
			EntityId: entityId,
		}
	}
	entity, ok := d.peerCache.GetWithInit(entityId, initFunc)
	if !ok {
		return nil, fmt.Errorf("get or init for unicast config for peer %s failed", peerID)
	}
	cfg, ok := entity.(UnicastConfigEntity)
	if !ok {
		// sanity check
		// This should never happen, because the cache only contains UnicastConfigEntity entities.
		panic(fmt.Sprintf("invalid entity type, expected UnicastConfigEntity type, got: %T", entity))
	}

	// return a copy of the config (we do not want the caller to modify the config).
	return &unicast.Config{
		StreamCreationRetryAttemptBudget: cfg.StreamCreationRetryAttemptBudget,
		ConsecutiveSuccessfulStream:      cfg.ConsecutiveSuccessfulStream,
	}, nil
}

// Size returns the number of unicast configs in the cache.
func (d *UnicastConfigCache) Size() uint {
	return d.peerCache.Size()
}
