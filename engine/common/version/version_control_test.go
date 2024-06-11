package version

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/coreos/go-semver/semver"
	"github.com/stretchr/testify/assert"
	testifyMock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	storageMock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestInitialization tests the initialization process of the VersionControl component.
func TestInitialization(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	finalizedRootBlockHeight := uint64(0)
	sealedHeader := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(10))

	t.Run("happy case", func(t *testing.T) {
		// Set up a mock version beacons storage.
		versionBeacons := storageMock.NewVersionBeacons(t)
		// Mock the Highest method to return a version beacon with a specific version.
		versionBeacons.
			On("Highest", testifyMock.AnythingOfType("uint64")).
			Return(func(height uint64) (*flow.SealedVersionBeacon, error) {
				if height == sealedHeader.Height {
					return &flow.SealedVersionBeacon{
						VersionBeacon: unittest.VersionBeaconFixture(
							unittest.WithBoundaries(
								flow.VersionBoundary{
									BlockHeight: sealedHeader.Height,
									Version:     "0.0.1",
								}),
						),
						SealHeight: sealedHeader.Height,
					}, nil
				} else {
					return nil, nil
				}
			}, nil)

		// Create a new VersionControl instance with initial parameters.
		vc := NewVersionControl(unittest.Logger(), versionBeacons, semver.New("0.0.1"), finalizedRootBlockHeight, sealedHeader.Height)

		// Create a mock signaler context for testing.
		ictx := irrecoverable.NewMockSignalerContext(t, ctx)

		// Start the VersionControl component.
		vc.Start(ictx)

		// Ensure the component is ready before proceeding.
		unittest.RequireComponentsReadyBefore(t, 2*time.Second, vc)

		compatible, err := vc.CompatibleAtBlock(sealedHeader.Height)
		require.NoError(t, err)

		// Assert that the VersionControl is compatible at the specified block height.
		assert.True(t, compatible)
	})

	t.Run("no version beacon found", func(t *testing.T) {
		// Set up a mock version beacons storage.
		versionBeacons := storageMock.NewVersionBeacons(t)
		// Mock the Highest method to return nil, indicating no version beacon found.
		versionBeacons.
			On("Highest", testifyMock.AnythingOfType("uint64")).
			Return(nil, nil)

		// Create a new VersionControl instance with initial parameters.
		vc := NewVersionControl(unittest.Logger(), versionBeacons, semver.New("0.0.1"), finalizedRootBlockHeight, sealedHeader.Height)

		// Create a mock signaler context for testing.
		ictx := irrecoverable.NewMockSignalerContext(t, ctx)

		// Start the VersionControl component.
		vc.Start(ictx)

		// Ensure the component is ready before proceeding.
		unittest.RequireComponentsReadyBefore(t, 2*time.Second, vc)

		compatible, err := vc.CompatibleAtBlock(sealedHeader.Height)
		require.NoError(t, err)

		// Should be compatible with all blocks in the network when there is no version beacon available
		assert.True(t, compatible)
	})

	t.Run("version beacon highest error", func(t *testing.T) {
		decodeErr := fmt.Errorf("test decode error")

		// Set up a mock version beacons storage.
		versionBeacons := storageMock.NewVersionBeacons(t)
		// Mock the Highest method to return an error.
		versionBeacons.
			On("Highest", testifyMock.AnythingOfType("uint64")).
			Return(nil, decodeErr)

		// Create a new VersionControl instance with initial parameters.
		vc := NewVersionControl(unittest.Logger(), versionBeacons, semver.New("0.0.1"), finalizedRootBlockHeight, sealedHeader.Height)

		// Create a mock signaler context that expects an error.
		ictx := irrecoverable.NewMockSignalerContextExpectError(t, ctx, fmt.Errorf(
			"failed to get highest version beacon for version control: %w",
			decodeErr))

		// Start the VersionControl component.
		vc.Start(ictx)

		// Ensure the component is ready before proceeding.
		unittest.RequireComponentsReadyBefore(t, 2*time.Second, vc)

		// Wait for the asynchronous version controller initialization process.
		time.Sleep(time.Second)
	})

	t.Run("start height available", func(t *testing.T) {
		startHeight := sealedHeader.Height - 5
		// Set up a mock version beacons storage.
		versionBeacons := storageMock.NewVersionBeacons(t)
		// Mock the Highest method to return a version beacon with a specific version.
		versionBeacons.
			On("Highest", testifyMock.AnythingOfType("uint64")).
			Return(func(height uint64) (*flow.SealedVersionBeacon, error) {
				if height == sealedHeader.Height {
					return &flow.SealedVersionBeacon{
						VersionBeacon: unittest.VersionBeaconFixture(
							unittest.WithBoundaries(
								flow.VersionBoundary{
									BlockHeight: startHeight,
									Version:     "0.0.1",
								}),
							unittest.WithBoundaries(
								flow.VersionBoundary{
									BlockHeight: sealedHeader.Height,
									Version:     "0.0.2",
								}),
						),
						SealHeight: sealedHeader.Height,
					}, nil
				} else {
					return nil, nil
				}
			}, nil)

		// Create a new VersionControl instance with initial parameters.
		vc := NewVersionControl(unittest.Logger(), versionBeacons, semver.New("0.0.2"), finalizedRootBlockHeight, sealedHeader.Height)

		// Create a mock signaler context for testing.
		ictx := irrecoverable.NewMockSignalerContext(t, ctx)

		// Start the VersionControl component.
		vc.Start(ictx)

		// Ensure the component is ready before proceeding.
		unittest.RequireComponentsReadyBefore(t, 2*time.Second, vc)

		compatible, err := vc.CompatibleAtBlock(sealedHeader.Height)
		require.NoError(t, err)

		// Assert that the VersionControl is compatible at the specified block height.
		assert.True(t, compatible)

		compatible, err = vc.CompatibleAtBlock(startHeight - 1)
		require.NoError(t, err)

		// Should be incompatible, as lover than start height
		assert.False(t, compatible)
	})

	t.Run("start and end heights available", func(t *testing.T) {
		startHeight := sealedHeader.Height - 5

		endHeight := sealedHeader.Height - 1

		// Set up a mock version beacons storage.
		versionBeacons := storageMock.NewVersionBeacons(t)
		// Mock the Highest method to return a version beacon with a specific version.
		versionBeacons.
			On("Highest", testifyMock.AnythingOfType("uint64")).
			Return(func(height uint64) (*flow.SealedVersionBeacon, error) {
				if height == sealedHeader.Height {
					return &flow.SealedVersionBeacon{
						VersionBeacon: unittest.VersionBeaconFixture(
							unittest.WithBoundaries(
								flow.VersionBoundary{
									BlockHeight: startHeight,
									Version:     "0.0.1",
								}),
							unittest.WithBoundaries(
								flow.VersionBoundary{
									BlockHeight: endHeight,
									Version:     "0.0.2",
								}),
							unittest.WithBoundaries(
								flow.VersionBoundary{
									BlockHeight: sealedHeader.Height,
									Version:     "0.0.3",
								}),
						),
						SealHeight: sealedHeader.Height,
					}, nil
				} else {
					return nil, nil
				}
			}, nil)

		// Create a new VersionControl instance with initial parameters.
		vc := NewVersionControl(unittest.Logger(), versionBeacons, semver.New("0.0.2"), finalizedRootBlockHeight, sealedHeader.Height)

		// Create a mock signaler context for testing.
		ictx := irrecoverable.NewMockSignalerContext(t, ctx)

		// Start the VersionControl component.
		vc.Start(ictx)

		// Ensure the component is ready before proceeding.
		unittest.RequireComponentsReadyBefore(t, 2*time.Second, vc)

		compatible, err := vc.CompatibleAtBlock(sealedHeader.Height)
		require.NoError(t, err)

		// Should be incompatible, as end height is lover than sealedHeader height
		assert.False(t, compatible)
	})
}

// TestVersionChanged tests the behavior of the VersionControl component when the version changes.
func TestVersionChanged(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sealedHeader := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(10))
	sealedVersionBeacon := &flow.SealedVersionBeacon{
		VersionBeacon: unittest.VersionBeaconFixture(
			unittest.WithBoundaries(
				flow.VersionBoundary{
					BlockHeight: sealedHeader.Height,
					Version:     "0.0.1",
				}),
		),
		SealHeight: sealedHeader.Height,
	}

	// Set up a mock version beacons storage.
	versionBeacons := storageMock.NewVersionBeacons(t)

	// Create a new VersionControl instance with initial parameters.
	vc := NewVersionControl(unittest.Logger(), versionBeacons, semver.New("0.0.1"), 0, sealedHeader.Height)

	// Mock the Highest method to return a specific version beacon.
	versionBeacons.
		On("Highest", testifyMock.AnythingOfType("uint64")).
		Return(func(height uint64) (*flow.SealedVersionBeacon, error) {
			if height == sealedVersionBeacon.SealHeight {
				return sealedVersionBeacon, nil
			} else {
				return nil, nil
			}

		}, nil)

	// Create a mock signaler context for testing.
	ictx := irrecoverable.NewMockSignalerContext(t, ctx)

	// Start the VersionControl component.
	vc.Start(ictx)

	// Ensure the component is ready before proceeding.
	unittest.RequireComponentsReadyBefore(t, 2*time.Second, vc)

	// Wait for the asynchronous version controller initialization process.
	time.Sleep(time.Second)

	compatible, err := vc.CompatibleAtBlock(sealedHeader.Height)
	require.NoError(t, err)

	// Assert that the VersionControl is compatible at the specified block height.
	require.True(t, compatible)

	// Create a new block header for testing the block finalized event.
	blockHeaderA := unittest.BlockHeaderWithParentFixture(sealedHeader) // Height 11
	sealedVersionBeacon.SealHeight = blockHeaderA.Height
	vc.BlockFinalizedForTesting(blockHeaderA)

	compatible, err = vc.CompatibleAtBlock(blockHeaderA.Height)
	require.NoError(t, err)

	// Assert that the VersionControl is compatible at the new block height.
	assert.True(t, compatible)

	// Create a new block header with a higher version.
	newBlockHeaderWithHigherVersion := unittest.BlockHeaderWithParentFixture(blockHeaderA) // Height 12
	sealedVersionBeacon = &flow.SealedVersionBeacon{
		VersionBeacon: unittest.VersionBeaconFixture(
			unittest.WithBoundaries(
				flow.VersionBoundary{
					BlockHeight: sealedHeader.Height,
					Version:     "0.0.1",
				},
				flow.VersionBoundary{
					BlockHeight: newBlockHeaderWithHigherVersion.Height,
					Version:     "0.0.2",
				}),
		),
		SealHeight: newBlockHeaderWithHigherVersion.Height,
	}

	// Add a consumer to verify version updates.
	vc.AddVersionUpdatesConsumer(func(height uint64, semver string) {
		assert.Equal(t, newBlockHeaderWithHigherVersion.Height, height)
		assert.Equal(t, "0.0.2", semver)
	})
	assert.Len(t, vc.consumers, 1)

	// Simulate a block finalized event for the new block header.
	vc.BlockFinalizedForTesting(newBlockHeaderWithHigherVersion)

	// Assert that the VersionControl is not compatible at the new block height.
	compatible, err = vc.CompatibleAtBlock(sealedHeader.Height)
	require.NoError(t, err)
	assert.True(t, compatible)

	compatible, err = vc.CompatibleAtBlock(blockHeaderA.Height)
	require.NoError(t, err)
	assert.True(t, compatible)

	compatible, err = vc.CompatibleAtBlock(newBlockHeaderWithHigherVersion.Height)
	require.NoError(t, err)
	assert.False(t, compatible)
}
