package internal

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
	"github.com/onflow/flow-go/network/p2p/unicast/model"
)

// ErrDialConfigNotFound is a benign error that indicates that the dial config does not exist in the cache. It is not a fatal error.
var ErrDialConfigNotFound = fmt.Errorf("dial config not found")

type DialConfigCache struct {
	peerCache  *stdmap.Backend
	cfgFactory func() model.DialConfig // factory function that creates a new dial config.
}

var _ unicast.DialConfigCache = (*DialConfigCache)(nil)

// NewDialConfigCache creates a new DialConfigCache.
// Args:
// - size: the maximum number of dial configs that the cache can hold.
// - logger: the logger used by the cache.
// - collector: the metrics collector used by the cache.
// - cfgFactory: a factory function that creates a new dial config.
// Returns:
// - *DialConfigCache, the created cache.
// Note that the cache is supposed to keep the dial config for all types of nodes. Since the number of such nodes is
// expected to be small, size must be large enough to hold all the dial configs of the authorized nodes.
// To avoid any crash-failure, the cache is configured to eject the least recently used configs when the cache is full.
// Hence, we recommend setting the size to a large value to minimize the ejections.
func NewDialConfigCache(size uint32, logger zerolog.Logger, collector module.HeroCacheMetrics, cfgFactory func() model.DialConfig) *DialConfigCache {
	return &DialConfigCache{
		peerCache: stdmap.NewBackend(
			stdmap.WithBackData(
				herocache.NewCache(
					size,
					herocache.DefaultOversizeFactor,
					heropool.LRUEjection,
					logger.With().Str("module", "dial-config-cache").Logger(),
					collector))),
		cfgFactory: cfgFactory,
	}
}

// Adjust applies the given adjust function to the dial config of the given peer ID, and stores the adjusted config in the cache.
// It returns an error if the adjustFunc returns an error.
// Note that if the Adjust is called when the config does not exist, the config is initialized and the
// adjust function is applied to the initialized config again. In this case, the adjust function should not return an error.
// Args:
// - peerID: the peer id of the dial config.
// - adjustFunc: the function that adjusts the dial config.
// Returns:
//   - error any returned error should be considered as an irrecoverable error and indicates a bug.
func (d *DialConfigCache) Adjust(peerID peer.ID, adjustFunc model.DialConfigAdjustFunc) (*model.DialConfig, error) {
	// first we translate the peer id to a flow id (taking
	flowPeerId := PeerIdToFlowId(peerID)
	adjustedDialCfg, err := d.adjust(flowPeerId, adjustFunc)
	if err != nil {
		if err == ErrDialConfigNotFound {
			// if the config does not exist, we initialize the config and try to adjust it again.
			// Note: there is an edge case where the config is initialized by another goroutine between the two calls.
			// In this case, the init function is invoked twice, but it is not a problem because the underlying
			// cache is thread-safe. Hence, we do not need to synchronize the two calls. In such cases, one of the
			// two calls returns false, and the other call returns true. We do not care which call returns false, hence,
			// we ignore the return value of the init function.
			_ = d.peerCache.Add(DialConfigEntity{
				PeerId:     peerID,
				DialConfig: d.cfgFactory(),
			})
			// as the config is initialized, the adjust function should not return an error, and any returned error
			// is an irrecoverable error and indicates a bug.
			return d.adjust(flowPeerId, adjustFunc)
		}
		// if the adjust function returns an unexpected error on the first attempt, we return the error directly.
		// any returned error should be considered as an irrecoverable error and indicates a bug.
		return nil, fmt.Errorf("failed to adjust dial config: %w", err)
	}
	// if the adjust function returns no error on the first attempt, we return the adjusted config.
	return adjustedDialCfg, nil
}

// adjust applies the given adjust function to the dial config of the given origin id.
// It returns an error if the adjustFunc returns an error or if the config does not exist.
// Args:
// - peerIDHash: the hash value of the peer id of the dial config (i.e., the ID of the dial config entity).
// - adjustFunc: the function that adjusts the dial config.
// Returns:
//   - error if the adjustFunc returns an error or if the config does not exist (ErrDialConfigNotFound). Except the ErrDialConfigNotFound,
//     any other error should be treated as an irrecoverable error and indicates a bug.
func (d *DialConfigCache) adjust(peerIdHash flow.Identifier, adjustFunc model.DialConfigAdjustFunc) (*model.DialConfig, error) {
	var rErr error
	adjustedEntity, adjusted := d.peerCache.Adjust(peerIdHash, func(entity flow.Entity) flow.Entity {
		cfgEntity, ok := entity.(DialConfigEntity)
		if !ok {
			// sanity check
			// This should never happen, because the cache only contains DialConfigEntity entities.
			panic(fmt.Sprintf("invalid entity type, expected DialConfigEntity type, got: %T", entity))
		}

		// adjust the dial config.
		adjustedCfg, err := adjustFunc(cfgEntity.DialConfig)
		if err != nil {
			rErr = fmt.Errorf("adjust function failed: %w", err)
			return entity // returns the original entity (reverse the adjustment).
		}

		// Return the adjusted config.
		cfgEntity.DialConfig = adjustedCfg
		return cfgEntity
	})

	if rErr != nil {
		return nil, fmt.Errorf("failed to adjust config: %w", rErr)
	}

	if !adjusted {
		return nil, ErrDialConfigNotFound
	}

	return &model.DialConfig{
		DialBackoff:        adjustedEntity.(DialConfigEntity).DialBackoff,
		StreamBackoff:      adjustedEntity.(DialConfigEntity).StreamBackoff,
		LastSuccessfulDial: adjustedEntity.(DialConfigEntity).LastSuccessfulDial,
	}, nil
}

// GetOrInit returns the dial config for the given peer id. If the config does not exist, it creates a new config
// using the factory function and stores it in the cache.
// Args:
// - peerID: the peer id of the dial config.
// Returns:
//   - *DialConfig, the dial config for the given peer id.
//   - error if the factory function returns an error. Any error should be treated as an irrecoverable error and indicates a bug.
func (d *DialConfigCache) GetOrInit(peerID peer.ID) (*model.DialConfig, error) {
	// first we translate the peer id to a flow id (taking
	flowPeerId := PeerIdToFlowId(peerID)
	cfg, ok := d.get(flowPeerId)
	if !ok {
		_ = d.peerCache.Add(DialConfigEntity{
			PeerId:     peerID,
			DialConfig: d.cfgFactory(),
		})
		cfg, ok = d.get(flowPeerId)
		if !ok {
			return nil, fmt.Errorf("failed to initialize dial config for peer %s", peerID)
		}
	}
	return cfg, nil
}

// Get returns the dial config of the given peer ID.
func (d *DialConfigCache) get(peerIDHash flow.Identifier) (*model.DialConfig, bool) {
	entity, ok := d.peerCache.ByID(peerIDHash)
	if !ok {
		return nil, false
	}

	cfg, ok := entity.(DialConfigEntity)
	if !ok {
		// sanity check
		// This should never happen, because the cache only contains DialConfigEntity entities.
		panic(fmt.Sprintf("invalid entity type, expected DialConfigEntity type, got: %T", entity))
	}

	// return a copy of the config (we do not want the caller to modify the config).
	return &model.DialConfig{
		DialBackoff:        cfg.DialBackoff,
		StreamBackoff:      cfg.StreamBackoff,
		LastSuccessfulDial: cfg.LastSuccessfulDial,
	}, true
}

// Size returns the number of dial configs in the cache.
func (d *DialConfigCache) Size() uint {
	return d.peerCache.Size()
}
