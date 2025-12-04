package execution

import (
	"errors"
	"fmt"

	"github.com/rs/zerolog"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/engine/common/version"
)

// ErrIncompatibleNodeVersion indicates that node version is incompatible with the block version.
var ErrIncompatibleNodeVersion = errors.New("node version is incompatible with data for block")

// CompatibleHeights tracks and enforces block height compatibility for execution.
//
// It defines a safe range of block heights—between minCompatibleHeight and maxCompatibleHeight—
// that the node can process using its current version.
type CompatibleHeights struct {
	log zerolog.Logger
	// versionControl provides information about the current version beacon for each block
	versionControl *version.VersionControl

	// minCompatibleHeight and maxCompatibleHeight are used to limit the block range that can be queried using local execution
	// to ensure only blocks that are compatible with the node's current software version are allowed.
	// Note: this is a temporary solution for cadence/fvm upgrades while version beacon support is added
	minCompatibleHeight *atomic.Uint64
	maxCompatibleHeight *atomic.Uint64
}

// NewCompatibleHeights creates a new [CompatibleHeights] instance that manages
// the range of block heights compatible with the node's current version.
func NewCompatibleHeights(
	log zerolog.Logger,
	versionControl *version.VersionControl,
	minHeight uint64,
	maxHeight uint64,
) *CompatibleHeights {
	return &CompatibleHeights{
		log:                 zerolog.New(log).With().Str("component", "compatible_heights").Logger(),
		versionControl:      versionControl,
		minCompatibleHeight: atomic.NewUint64(minHeight),
		maxCompatibleHeight: atomic.NewUint64(maxHeight),
	}
}

// SetMinCompatibleHeight sets the lowest block height (inclusive).
func (c *CompatibleHeights) SetMinCompatibleHeight(height uint64) {
	c.minCompatibleHeight.Store(height)
	c.log.Info().Uint64("height", height).Msg("minimum compatible height set")
}

// SetMaxCompatibleHeight sets the highest block height (inclusive).
func (c *CompatibleHeights) SetMaxCompatibleHeight(height uint64) {
	c.maxCompatibleHeight.Store(height)
	c.log.Info().Uint64("height", height).Msg("maximum compatible height set")
}

// Check checks whether the given block height is compatible with the node's version.
// It performs checks:
// 1. Ensures the height is within the configured minCompatibleHeight and maxCompatibleHeight.
// 2. If version control is enabled, ensures the height is compatible with the node's version beacon.
//
// Expected error returns during normal operation:
//   - [version.ErrOutOfRange]: If incoming block height is higher that last handled block height.
//   - [execution.ErrIncompatibleNodeVersion]: If the block height is not compatible with the node version.
func (c *CompatibleHeights) Check(height uint64) error {
	if height > c.maxCompatibleHeight.Load() || height < c.minCompatibleHeight.Load() {
		return ErrIncompatibleNodeVersion
	}

	// Version control feature could be disabled. In such a case, ignore related functionality.
	if c.versionControl != nil {
		compatible, err := c.versionControl.CompatibleAtBlock(height)
		if err != nil {
			return fmt.Errorf("failed to check compatibility with block height %d: %w", height, err)
		}

		if !compatible {
			return ErrIncompatibleNodeVersion
		}
	}

	return nil
}
