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
)

// ErrDialConfigNotFound is a benign error that indicates that the dial config does not exist in the cache. It is not a fatal error.
var ErrDialConfigNotFound = fmt.Errorf("dial config not found")

type DialConfigCache struct {
	peerCache *stdmap.Backend
}

// NewDialConfigCache creates a new DialConfigCache.
// Args:
// - size: the maximum number of records that the cache can hold.
// - logger: the logger used by the cache.
// - collector: the metrics collector used by the cache.
// Returns:
// - *DialConfigCache, the created cache.
// Note that the cache is supposed to keep the dial config for all types of nodes. Since the number of such nodes is
// expected to be small, size must be large enough to hold all the dial configs of the authorized nodes.
// To avoid any crash-failure, the cache is configured to eject the least recently used records when the cache is full.
// Hence, we recommend setting the size to a large value to minimize the ejections.
func NewDialConfigCache(size uint32, logger zerolog.Logger, collector module.HeroCacheMetrics) *DialConfigCache {
	return &DialConfigCache{
		peerCache: stdmap.NewBackend(
			stdmap.WithBackData(
				herocache.NewCache(
					size,
					herocache.DefaultOversizeFactor,
					heropool.LRUEjection,
					logger.With().Str("module", "dial-config-cache").Logger(),
					collector))),
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
func (d *DialConfigCache) Adjust(peerID peer.ID, adjustFunc DialConfigAdjustFunc) error {
	// first we translate the peer id to a flow id (taking
	flowPeerId := PeerIdToFlowId(peerID)
	err := d.adjust(flowPeerId, adjustFunc)
	switch {
	case err == ErrDialConfigNotFound:
		// if the config does not exist, we initialize the config and try to adjust it again.
		// Note: there is an edge case where the config is initialized by another goroutine between the two calls.
		// In this case, the init function is invoked twice, but it is not a problem because the underlying
		// cache is thread-safe. Hence, we do not need to synchronize the two calls. In such cases, one of the
		// two calls returns false, and the other call returns true. We do not care which call returns false, hence,
		// we ignore the return value of the init function.
		_ = d.peerCache.Add(DialConfigEntity{
			PeerId: peerID,
		})
		// as the config is initialized, the adjust function should not return an error, and any returned error
		// is an irrecoverable error and indicates a bug.
		return d.adjust(flowPeerId, adjustFunc)
	case err != nil:
		// if the adjust function returns an unexpected error on the first attempt, we return the error directly.
		return fmt.Errorf("failed to adjust dial config: %w", err)
	default:
		// if the adjust function returns no error, we return nil
		return nil
	}
}

// adjust applies the given adjust function to the dial config of the given origin id.
// It returns an error if the adjustFunc returns an error or if the config does not exist.
// Args:
// - peerIDHash: the hash value of the peer id of the dial config (i.e., the ID of the dial config entity).
// - adjustFunc: the function that adjusts the dial config.
// Returns:
//   - error if the adjustFunc returns an error or if the config does not exist (ErrDialConfigNotFound). Except the ErrDialConfigNotFound,
//     any other error should be treated as an irrecoverable error and indicates a bug.
func (d *DialConfigCache) adjust(peerIdHash flow.Identifier, adjustFunc DialConfigAdjustFunc) error {
	var rErr error
	_, adjusted := d.peerCache.Adjust(peerIdHash, func(entity flow.Entity) flow.Entity {
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
		return fmt.Errorf("failed to adjust config: %w", rErr)
	}

	if !adjusted {
		return ErrDialConfigNotFound
	}

	return nil
}

// Get returns the dial config of the given peer ID.
func (d *DialConfigCache) Get(peerID peer.ID) (*DialConfig, bool) {
	entity, ok := d.peerCache.ByID(PeerIdToFlowId(peerID))
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
	return &DialConfig{
		DialBackoff:        cfg.DialBackoff,
		StreamBackoff:      cfg.StreamBackoff,
		LastSuccessfulDial: cfg.LastSuccessfulDial,
	}, true
}
