package execution

import (
	"testing"

	"github.com/coreos/go-semver/semver"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/common/version"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestCompatibleHeights covers behavior of [CompatibleHeights] and its compatibility checks.
//
// Test cases:
//  1. Succeeds when height is within min and max bounds and versionControl is nil.
//  2. Errors when height is below the minimum bound.
//  3. Errors when height is above the maximum bound.
//  4. Succeeds when versionControl reports a compatible version.
//  5. Errors when versionControl reports an incompatible version.
func TestVerifyHeight(t *testing.T) {
	logger := unittest.LoggerForTest(t, zerolog.InfoLevel)

	t.Run("height within range without version control", func(t *testing.T) {
		ch := NewCompatibleHeights(logger, nil, 10, 20)
		err := ch.Check(15)
		require.NoError(t, err)
	})

	t.Run("error when below minimum height", func(t *testing.T) {
		ch := NewCompatibleHeights(logger, nil, 10, 20)
		err := ch.Check(9)
		require.ErrorIs(t, err, ErrIncompatibleNodeVersion)
	})

	t.Run("error when above maximum height", func(t *testing.T) {
		ch := NewCompatibleHeights(logger, nil, 10, 20)
		err := ch.Check(21)
		require.ErrorIs(t, err, ErrIncompatibleNodeVersion)
	})

	t.Run("versionControl reports compatible", func(t *testing.T) {
		beacons := storagemock.NewVersionBeacons(t)
		vc, err := version.NewVersionControl(
			logger,
			beacons,
			semver.New("0.0.1"),
			10,
			20,
		)
		require.NoError(t, err)

		ch := NewCompatibleHeights(logger, vc, 10, 20)
		err = ch.Check(15)
		require.NoError(t, err)
	})

	t.Run("versionControl reports incompatible", func(t *testing.T) {
		// Create a version control with a small indexed range
		beacons := storagemock.NewVersionBeacons(t)
		vc, err := version.NewVersionControl(
			logger,
			beacons,
			semver.New("0.0.1"),
			10,
			12, // only compatible up to height 12
		)
		require.NoError(t, err)

		ch := NewCompatibleHeights(logger, vc, 10, 20)
		err = ch.Check(15)
		require.ErrorIs(t, err, version.ErrOutOfRange)
	})
}
