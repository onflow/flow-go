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
	"github.com/onflow/flow-go/utils/unittest/mocks"
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

		vbEvent := versionBeaconEvent(t, sealedHeader.Height, []uint64{sealedHeader.Height}, []string{"0.0.1"})

		// Mock the Highest method to return a version beacon with a specific version.
		versionBeacons.
			On("Highest", testifyMock.AnythingOfType("uint64")).
			Return(func(height uint64) (*flow.SealedVersionBeacon, error) {
				if height == sealedHeader.Height {
					return vbEvent, nil
				} else {
					return nil, nil
				}
			}, nil)

		// Create a new VersionControl instance with initial parameters.
		vc, err := NewVersionControl(unittest.Logger(), versionBeacons, semver.New("0.0.1"), finalizedRootBlockHeight, sealedHeader.Height)
		require.NoError(t, err)

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

		compatible, err = vc.CompatibleAtBlock(sealedHeader.Height - 1)
		require.NoError(t, err)

		// Assert that the VersionControl is incompatible at the lower block height.
		assert.False(t, compatible)
	})

	t.Run("no version beacon found", func(t *testing.T) {
		// Set up a mock version beacons storage.
		versionBeacons := storageMock.NewVersionBeacons(t)
		// Mock the Highest method to return nil, indicating no version beacon found.
		versionBeacons.
			On("Highest", testifyMock.AnythingOfType("uint64")).
			Return(nil, nil)

		// Create a new VersionControl instance with initial parameters.
		vc, err := NewVersionControl(unittest.Logger(), versionBeacons, semver.New("0.0.1"), finalizedRootBlockHeight, sealedHeader.Height)
		require.NoError(t, err)

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

		compatible, err = vc.CompatibleAtBlock(sealedHeader.Height - 1)
		require.NoError(t, err)

		// Assert that the VersionControl is incompatible at the lower block height.
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
		vc, err := NewVersionControl(unittest.Logger(), versionBeacons, semver.New("0.0.1"), finalizedRootBlockHeight, sealedHeader.Height)
		require.NoError(t, err)

		// Create a mock signaler context that expects an error.
		ictx := irrecoverable.NewMockSignalerContextExpectError(t, ctx, fmt.Errorf(
			"failed to get highest version beacon for version control: %w",
			decodeErr))

		// Start the VersionControl component.
		vc.Start(ictx)

		unittest.AssertClosesBefore(t, vc.Ready(), 2*time.Second)

		//TODO: Should be discussed
		time.Sleep(time.Second)
	})

	t.Run("start height available", func(t *testing.T) {
		startHeight := sealedHeader.Height - 5
		// Set up a mock version beacons storage.
		versionBeacons := storageMock.NewVersionBeacons(t)

		vbEvent := versionBeaconEvent(t, sealedHeader.Height, []uint64{startHeight, sealedHeader.Height}, []string{"0.0.1", "0.0.2"})

		// Mock the Highest method to return a version beacon with a specific version.
		versionBeacons.
			On("Highest", testifyMock.AnythingOfType("uint64")).
			Return(func(height uint64) (*flow.SealedVersionBeacon, error) {
				if height == sealedHeader.Height {
					return vbEvent, nil
				} else {
					return nil, nil
				}
			}, nil)

		// Create a new VersionControl instance with initial parameters.
		vc, err := NewVersionControl(unittest.Logger(), versionBeacons, semver.New("0.0.2"), finalizedRootBlockHeight, sealedHeader.Height)
		require.NoError(t, err)

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

		// Should be incompatible, as lower than start height
		assert.False(t, compatible)
	})

	t.Run("start and end heights available", func(t *testing.T) {
		startHeight := sealedHeader.Height - 5

		endHeight := sealedHeader.Height - 1

		// Set up a mock version beacons storage.
		versionBeacons := storageMock.NewVersionBeacons(t)

		vbEvent := versionBeaconEvent(t, sealedHeader.Height, []uint64{startHeight, endHeight, sealedHeader.Height}, []string{"0.0.1", "0.0.2", "0.0.3"})

		// Mock the Highest method to return a version beacon with a specific version.
		versionBeacons.
			On("Highest", testifyMock.AnythingOfType("uint64")).
			Return(func(height uint64) (*flow.SealedVersionBeacon, error) {
				if height == sealedHeader.Height {
					return vbEvent, nil
				} else {
					return nil, nil
				}
			}, nil)

		// Create a new VersionControl instance with initial parameters.
		vc, err := NewVersionControl(unittest.Logger(), versionBeacons, semver.New("0.0.2"), finalizedRootBlockHeight, sealedHeader.Height)
		require.NoError(t, err)

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
	versionEvents := map[uint64]*flow.SealedVersionBeacon{
		sealedHeader.Height: versionBeaconEvent(
			t,
			sealedHeader.Height,
			[]uint64{sealedHeader.Height},
			[]string{"0.0.1"},
		),
	}

	// Set up a mock version beacons storage.
	versionBeacons := storageMock.NewVersionBeacons(t)

	// Create a new VersionControl instance with initial parameters.
	vc, err := NewVersionControl(unittest.Logger(), versionBeacons, semver.New("0.0.1"), 0, sealedHeader.Height)
	require.NoError(t, err)

	// Mock the Highest method to return a specific version beacon.
	versionBeacons.
		On("Highest", testifyMock.AnythingOfType("uint64")).
		Return(mocks.StorageMapGetter(versionEvents))

	// Create a mock signaler context for testing.
	ictx := irrecoverable.NewMockSignalerContext(t, ctx)

	// Start the VersionControl component.
	vc.Start(ictx)

	// Ensure the component is ready before proceeding.
	unittest.RequireComponentsReadyBefore(t, 2*time.Second, vc)

	compatible, err := vc.CompatibleAtBlock(sealedHeader.Height)
	require.NoError(t, err)

	// Assert that the VersionControl is compatible at the specified block height.
	require.True(t, compatible)

	// Create a new block header for testing the block finalized event.
	blockHeaderA := unittest.BlockHeaderWithParentFixture(sealedHeader) // Height 11
	versionEvents[blockHeaderA.Height] = versionEvents[sealedHeader.Height]

	vc.blockFinalized(irrecoverable.MockSignalerContext{}, blockHeaderA.Height)

	compatible, err = vc.CompatibleAtBlock(blockHeaderA.Height)
	require.NoError(t, err)

	// Assert that the VersionControl is compatible at the new block height.
	assert.True(t, compatible)

	// Create a new block header with a higher version.
	newBlockHeaderWithHigherVersion := unittest.BlockHeaderWithParentFixture(blockHeaderA) // Height 12

	versionEvents[newBlockHeaderWithHigherVersion.Height] = versionBeaconEvent(
		t,
		newBlockHeaderWithHigherVersion.Height,
		[]uint64{sealedHeader.Height, newBlockHeaderWithHigherVersion.Height},
		[]string{"0.0.1", "0.0.2"},
	)

	// Add a consumer to verify version updates.
	vc.AddVersionUpdatesConsumer(func(height uint64, semver string) {
		assert.Equal(t, newBlockHeaderWithHigherVersion.Height, height)
		assert.Equal(t, "0.0.2", semver)
	})
	assert.Len(t, vc.consumers, 1)

	// Simulate a block finalized event for the new block header.
	vc.blockFinalized(irrecoverable.MockSignalerContext{}, newBlockHeaderWithHigherVersion.Height)

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

func versionBeaconEvent(t *testing.T, sealHeight uint64, heights []uint64, versions []string) *flow.SealedVersionBeacon {
	require.Equal(t, len(heights), len(versions), "the heights array should be the same length as the versions array")
	var vb []flow.VersionBoundary
	for i := 0; i < len(heights); i++ {
		vb = append(vb, flow.VersionBoundary{
			BlockHeight: heights[i],
			Version:     versions[i],
		})
	}

	return &flow.SealedVersionBeacon{
		VersionBeacon: unittest.VersionBeaconFixture(
			unittest.WithBoundaries(vb...),
		),
		SealHeight: sealHeight,
	}
}
