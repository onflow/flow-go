package unicastcache

import (
	"fmt"
	"sync"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	herocache "github.com/onflow/flow-go/module/mempool/herocache/backdata"
	"github.com/onflow/flow-go/module/mempool/herocache/backdata/heropool"
	"github.com/onflow/flow-go/module/mempool/stdmap"
	"github.com/onflow/flow-go/network/p2p/unicast"
)

// ErrUnicastConfigNotFound is a benign error that indicates that the unicast config does not exist in the cache. It is not a fatal error.
var ErrUnicastConfigNotFound = fmt.Errorf("unicast config not found")

type UnicastConfigCache struct {
	// mutex is temporarily protect the edge case in HeroCache that optimistic adjustment causes the cache to be full.
	// TODO: remove this mutex after the HeroCache is fixed.
	mutex      sync.RWMutex
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

// Adjust applies the given adjust function to the unicast config of the given peer ID, and stores the adjusted config in the cache.
// It returns an error if the adjustFunc returns an error.
// Note that if the Adjust is called when the config does not exist, the config is initialized and the
// adjust function is applied to the initialized config again. In this case, the adjust function should not return an error.
// Args:
// - peerID: the peer id of the unicast config.
// - adjustFunc: the function that adjusts the unicast config.
// Returns:
//   - error any returned error should be considered as an irrecoverable error and indicates a bug.
func (d *UnicastConfigCache) Adjust(peerID peer.ID, adjustFunc unicast.UnicastConfigAdjustFunc) (*unicast.Config, error) {
	d.mutex.Lock() // making optimistic adjustment atomic.
	defer d.mutex.Unlock()

	// first we translate the peer id to a flow id (taking
	peerIdHash := PeerIdToFlowId(peerID)
	adjustedUnicastCfg, err := d.adjust(peerIdHash, adjustFunc)
	if err != nil {
		if err == ErrUnicastConfigNotFound {
			// if the config does not exist, we initialize the config and try to adjust it again.
			// Note: there is an edge case where the config is initialized by another goroutine between the two calls.
			// In this case, the init function is invoked twice, but it is not a problem because the underlying
			// cache is thread-safe. Hence, we do not need to synchronize the two calls. In such cases, one of the
			// two calls returns false, and the other call returns true. We do not care which call returns false, hence,
			// we ignore the return value of the init function.
			e := UnicastConfigEntity{
				PeerId: peerID,
				Config: d.cfgFactory(),
			}

			_ = d.peerCache.Add(e)

			// as the config is initialized, the adjust function should not return an error, and any returned error
			// is an irrecoverable error and indicates a bug.
			return d.adjust(peerIdHash, adjustFunc)
		}
		// if the adjust function returns an unexpected error on the first attempt, we return the error directly.
		// any returned error should be considered as an irrecoverable error and indicates a bug.
		return nil, fmt.Errorf("failed to adjust unicast config: %w", err)
	}
	// if the adjust function returns no error on the first attempt, we return the adjusted config.
	return adjustedUnicastCfg, nil
}

// adjust applies the given adjust function to the unicast config of the given origin id.
// It returns an error if the adjustFunc returns an error or if the config does not exist.
// Args:
// - peerIDHash: the hash value of the peer id of the unicast config (i.e., the ID of the unicast config entity).
// - adjustFunc: the function that adjusts the unicast config.
// Returns:
//   - error if the adjustFunc returns an error or if the config does not exist (ErrUnicastConfigNotFound). Except the ErrUnicastConfigNotFound,
//     any other error should be treated as an irrecoverable error and indicates a bug.
func (d *UnicastConfigCache) adjust(peerIdHash flow.Identifier, adjustFunc unicast.UnicastConfigAdjustFunc) (*unicast.Config, error) {
	var rErr error
	adjustedEntity, adjusted := d.peerCache.Adjust(peerIdHash, func(entity flow.Entity) flow.Entity {
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
	})

	if rErr != nil {
		return nil, fmt.Errorf("failed to adjust config: %w", rErr)
	}

	if !adjusted {
		return nil, ErrUnicastConfigNotFound
	}

	return &unicast.Config{
		StreamCreationRetryAttemptBudget: adjustedEntity.(UnicastConfigEntity).StreamCreationRetryAttemptBudget,
		ConsecutiveSuccessfulStream:      adjustedEntity.(UnicastConfigEntity).ConsecutiveSuccessfulStream,
	}, nil
}

// GetOrInit returns the unicast config for the given peer id. If the config does not exist, it creates a new config
// using the factory function and stores it in the cache.
// Args:
// - peerID: the peer id of the unicast config.
// Returns:
//   - *Config, the unicast config for the given peer id.
//   - error if the factory function returns an error. Any error should be treated as an irrecoverable error and indicates a bug.
func (d *UnicastConfigCache) GetOrInit(peerID peer.ID) (*unicast.Config, error) {
	// first we translate the peer id to a flow id (taking
	flowPeerId := PeerIdToFlowId(peerID)
	cfg, ok := d.get(flowPeerId)
	if !ok {
		_ = d.peerCache.Add(UnicastConfigEntity{
			PeerId: peerID,
			Config: d.cfgFactory(),
		})
		cfg, ok = d.get(flowPeerId)
		if !ok {
			return nil, fmt.Errorf("failed to initialize unicast config for peer %s", peerID)
		}
	}
	return cfg, nil
}

// Get returns the unicast config of the given peer ID.
func (d *UnicastConfigCache) get(peerIDHash flow.Identifier) (*unicast.Config, bool) {
	entity, ok := d.peerCache.ByID(peerIDHash)
	if !ok {
		return nil, false
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
	}, true
}

// Size returns the number of unicast configs in the cache.
func (d *UnicastConfigCache) Size() uint {
	return d.peerCache.Size()
}
