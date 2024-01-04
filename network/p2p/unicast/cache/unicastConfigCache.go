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
	peerCache  *stdmap.Backend
	cfgFactory func() unicast.Config // factory function that creates a new unicast config.

	// atomicAdjustMutex is a atomicAdjustMutex used to ensure that the init-and-adjust operation is atomic.
	// The init-and-adjust operation is used to initialize a record in the cache and then update it.
	// The init-and-adjust operation is used when the record does not exist in the cache and needs to be initialized.
	// The current implementation is not thread-safe, and the atomicAdjustMutex is used to ensure that the init-and-adjust operation is atomic, otherwise
	// more than one thread may try to initialize records at the same time and cause an LRU eviction, hence trying an adjust on a record that does not exist,
	// which will result in an error.
	// TODO: implement a thread-safe atomic adjust operation and remove the mutex.
	atomicAdjustMutex sync.Mutex
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
	entityId := entityIdOf(peerID)
	// adjustedUnicastCfg, err := d.adjust(entityId, adjustFunc)
	// if err != nil {
	// 	if err == ErrUnicastConfigNotFound {
	// 		// if the config does not exist, we initialize the config and try to adjust it again.
	// 		// Note: there is an edge case where the config is initialized by another goroutine between the two calls.
	// 		// In this case, the init function is invoked twice, but it is not a problem because the underlying
	// 		// cache is thread-safe. Hence, we do not need to synchronize the two calls. In such cases, one of the
	// 		// two calls returns false, and the other call returns true. We do not care which call returns false, hence,
	// 		// we ignore the return value of the init function.
	// 		e := UnicastConfigEntity{
	// 			PeerId:   peerID,
	// 			Config:   d.cfgFactory(),
	// 			EntityId: entityId,
	// 		}
	//
	// 		// ensuring that the init-and-adjust operation is atomic.
	// 		d.atomicAdjustMutex.Lock()
	// 		defer d.atomicAdjustMutex.Unlock()
	//
	// 		add := d.peerCache.Add(e)
	// 		fmt.Println("add: ", add, "peerId: ", peerID.String(), "entityId: ", entityId.String(), "size: ", d.peerCache.Size())
	// 		// if !added {
	// 		// 	return nil, fmt.Errorf("failed to initialize unicast config for peer %s", peerID)
	// 		// }
	//
	// 		// as the config is initialized, the adjust function should not return an error, and any returned error
	// 		// is an irrecoverable error and indicates a bug.
	// 		return d.adjust(entityId, adjustFunc)
	// 	}
	// 	// if the adjust function returns an unexpected error on the first attempt, we return the error directly.
	// 	// any returned error should be considered as an irrecoverable error and indicates a bug.
	// 	return nil, fmt.Errorf("failed to adjust unicast config: %w", err)
	// }
	// // if the adjust function returns no error on the first attempt, we return the adjusted config.
	// return adjustedUnicastCfg, nil
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
		return nil, fmt.Errorf("adjsut operation aborted: %w", rErr)
	}

	if !adjusted {
		return nil, fmt.Errorf("failed to adjust config: %w", ErrUnicastConfigNotFound)
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
