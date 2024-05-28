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

	sealedHeader := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(10))

	t.Run("happy case", func(t *testing.T) {
		// Set up a mock version beacons storage.
		versionBeacons := storageMock.NewVersionBeacons(t)

		// Create a new VersionControl instance with initial parameters.
		vc := NewVersionControl(
			unittest.Logger(),
			versionBeacons,
			semver.New("0.0.1"),
			sealedHeader,
		)

		// Mock the Highest method to return a version beacon with a specific version.
		versionBeacons.
			On("Highest", testifyMock.AnythingOfType("uint64")).
			Return(&flow.SealedVersionBeacon{
				VersionBeacon: unittest.VersionBeaconFixture(
					unittest.WithBoundaries(
						flow.VersionBoundary{
							BlockHeight: sealedHeader.Height,
							Version:     "0.0.1",
						}),
				),
				SealHeight: sealedHeader.Height,
			}, nil).Once()

		// Create a mock signaler context for testing.
		ictx := irrecoverable.NewMockSignalerContext(t, ctx)

		// Start the VersionControl component.
		vc.Start(ictx)

		// Ensure the component is ready before proceeding.
		unittest.RequireComponentsReadyBefore(t, 2*time.Second, vc)

		// Assert that the VersionControl is compatible at the specified block height.
		assert.True(t, vc.CompatibleAtBlock(sealedHeader.Height))
	})

	t.Run("no version beacon found", func(t *testing.T) {
		// Set up a mock version beacons storage.
		versionBeacons := storageMock.NewVersionBeacons(t)

		// Create a new VersionControl instance with initial parameters.
		vc := NewVersionControl(
			unittest.Logger(),
			versionBeacons,
			semver.New("0.0.1"),
			sealedHeader,
		)

		// Mock the Highest method to return nil, indicating no version beacon found.
		versionBeacons.
			On("Highest", testifyMock.AnythingOfType("uint64")).
			Return(nil, nil).Once()

		// Create a mock signaler context for testing.
		ictx := irrecoverable.NewMockSignalerContext(t, ctx)

		// Start the VersionControl component.
		vc.Start(ictx)

		// Ensure the component is ready before proceeding.
		unittest.RequireComponentsReadyBefore(t, 2*time.Second, vc)

		// Assert that the VersionControl is not compatible at the specified block height.
		assert.False(t, vc.CompatibleAtBlock(sealedHeader.Height))
	})

	t.Run("version beacon highest error", func(t *testing.T) {
		decodeErr := fmt.Errorf("test decode error")

		// Set up a mock version beacons storage.
		versionBeacons := storageMock.NewVersionBeacons(t)

		// Create a new VersionControl instance with initial parameters.
		vc := NewVersionControl(
			unittest.Logger(),
			versionBeacons,
			semver.New("0.0.1"),
			sealedHeader,
		)

		// Mock the Highest method to return an error.
		versionBeacons.
			On("Highest", testifyMock.AnythingOfType("uint64")).
			Return(nil, decodeErr).Once()

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
	vc := NewVersionControl(
		unittest.Logger(),
		versionBeacons,
		semver.New("0.0.1"),
		sealedHeader,
	)

	// Mock the Highest method to return a specific version beacon.
	versionBeacons.
		On("Highest", testifyMock.AnythingOfType("uint64")).
		Return(func(belowOrEqualTo uint64) (*flow.SealedVersionBeacon, error) {
			return sealedVersionBeacon, nil
		}, nil)

	// Create a mock signaler context for testing.
	ictx := irrecoverable.NewMockSignalerContext(t, ctx)

	// Start the VersionControl component.
	vc.Start(ictx)

	// Ensure the component is ready before proceeding.
	unittest.RequireComponentsReadyBefore(t, 2*time.Second, vc)

	// Wait for the asynchronous version controller initialization process.
	time.Sleep(time.Second)

	// Assert that the VersionControl is compatible at the specified block height.
	require.True(t, vc.CompatibleAtBlock(sealedHeader.Height))

	// Create a new block header for testing the block finalized event.
	blockHeaderA := unittest.BlockHeaderWithParentFixture(sealedHeader) // Height 11
	vc.BlockFinalizedForTesting(blockHeaderA)

	// Assert that the VersionControl is compatible at the new block height.
	assert.True(t, vc.CompatibleAtBlock(blockHeaderA.Height))

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
	assert.True(t, vc.CompatibleAtBlock(sealedHeader.Height))
	assert.True(t, vc.CompatibleAtBlock(blockHeaderA.Height))
	assert.False(t, vc.CompatibleAtBlock(newBlockHeaderWithHigherVersion.Height))
}
