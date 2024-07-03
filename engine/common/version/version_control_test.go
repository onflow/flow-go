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
	"github.com/onflow/flow-go/storage"
	storageMock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/onflow/flow-go/utils/unittest/mocks"
)

// testCaseConfig contains custom tweaks for test cases
type testCaseConfig struct {
	name                           string
	nodeVersion                    string
	setupVersionBeaconMock         func(t *testing.T, versionBeacons *storageMock.VersionBeacons)
	checkResult                    func(t *testing.T, vc *VersionControl)
	signalerMockCtx                *irrecoverable.MockSignalerContext
	versionControlAlternativeCheck func(*VersionControl)
}

// TestVersionControlInitialization tests the initialization process of the VersionControl component
func TestVersionControlInitialization(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	finalizedRootBlockHeight := uint64(0)
	latestBlockHeight := uint64(10)
	decodeErr := fmt.Errorf("test decode error")

	testCases := []testCaseConfig{
		{
			name:        "happy case",
			nodeVersion: "0.0.1",
			setupVersionBeaconMock: func(t *testing.T, versionBeacons *storageMock.VersionBeacons) {
				versionEvents := map[uint64]*flow.SealedVersionBeacon{
					latestBlockHeight: VersionBeaconEvent(t, latestBlockHeight, []uint64{latestBlockHeight}, []string{"0.0.1"}),
				}
				versionBeacons.
					On("Highest", testifyMock.AnythingOfType("uint64")).
					Return(mocks.StorageMapGetter(versionEvents))
			},
			checkResult: func(t *testing.T, vc *VersionControl) {
				unittest.RequireComponentsReadyBefore(t, 2*time.Second, vc)

				compatible, err := vc.CompatibleAtBlock(latestBlockHeight)
				require.NoError(t, err)
				assert.True(t, compatible)

				compatible, err = vc.CompatibleAtBlock(latestBlockHeight - 1)
				require.NoError(t, err)
				assert.False(t, compatible)
			},
		},
		{
			name:        "no version beacon found",
			nodeVersion: "0.0.1",
			setupVersionBeaconMock: func(t *testing.T, versionBeacons *storageMock.VersionBeacons) {
				versionBeacons.
					On("Highest", testifyMock.AnythingOfType("uint64")).
					Return(nil, nil)
			},
			checkResult: func(t *testing.T, vc *VersionControl) {
				unittest.RequireComponentsReadyBefore(t, 2*time.Second, vc)

				compatible, err := vc.CompatibleAtBlock(latestBlockHeight)
				require.NoError(t, err)
				assert.True(t, compatible)

				compatible, err = vc.CompatibleAtBlock(latestBlockHeight - 1)
				require.NoError(t, err)
				assert.True(t, compatible)
			},
		},
		{
			name:        "version beacon highest error",
			nodeVersion: "0.0.1",
			setupVersionBeaconMock: func(t *testing.T, versionBeacons *storageMock.VersionBeacons) {
				versionBeacons.
					On("Highest", testifyMock.AnythingOfType("uint64")).
					Return(nil, decodeErr)
			},
			signalerMockCtx: irrecoverable.NewMockSignalerContextExpectError(t, ctx, fmt.Errorf(
				"failed to get highest version beacon for version control: %w",
				decodeErr)),
			checkResult: func(t *testing.T, vc *VersionControl) {
				// Assert that the component closes before a timeout, indicating error handling.
				unittest.AssertNotClosesBefore(t, vc.Ready(), 2*time.Second)
			},
		},
		{
			name:        "start height available",
			nodeVersion: "0.0.2",
			setupVersionBeaconMock: func(t *testing.T, versionBeacons *storageMock.VersionBeacons) {
				startHeight := latestBlockHeight - 5
				versionEvents := map[uint64]*flow.SealedVersionBeacon{
					latestBlockHeight: VersionBeaconEvent(t, latestBlockHeight, []uint64{startHeight, latestBlockHeight}, []string{"0.0.1", "0.0.2"}),
				}
				versionBeacons.
					On("Highest", testifyMock.AnythingOfType("uint64")).
					Return(mocks.StorageMapGetter(versionEvents))
			},
			checkResult: func(t *testing.T, vc *VersionControl) {
				unittest.RequireComponentsReadyBefore(t, 2*time.Second, vc)

				compatible, err := vc.CompatibleAtBlock(latestBlockHeight)
				require.NoError(t, err)
				assert.True(t, compatible)

				compatible, err = vc.CompatibleAtBlock(latestBlockHeight - 5 - 1)
				require.NoError(t, err)
				assert.False(t, compatible)
			},
		},
		{
			name:        "start and end heights available",
			nodeVersion: "0.0.2",
			setupVersionBeaconMock: func(t *testing.T, versionBeacons *storageMock.VersionBeacons) {
				startHeight := latestBlockHeight - 5
				endHeight := latestBlockHeight - 1
				versionEvents := map[uint64]*flow.SealedVersionBeacon{
					latestBlockHeight: VersionBeaconEvent(t, latestBlockHeight, []uint64{startHeight, endHeight, latestBlockHeight}, []string{"0.0.1", "0.0.2", "0.0.3"}),
				}
				versionBeacons.
					On("Highest", testifyMock.AnythingOfType("uint64")).
					Return(mocks.StorageMapGetter(versionEvents))
			},
			checkResult: func(t *testing.T, vc *VersionControl) {
				unittest.RequireComponentsReadyBefore(t, 2*time.Second, vc)

				compatible, err := vc.CompatibleAtBlock(latestBlockHeight)
				require.NoError(t, err)
				assert.False(t, compatible)
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			versionBeacons := storageMock.NewVersionBeacons(t)

			testCase.setupVersionBeaconMock(t, versionBeacons)

			if testCase.signalerMockCtx == nil {
				testCase.signalerMockCtx = irrecoverable.NewMockSignalerContext(t, ctx)
			}

			vc := createVersionControlComponent(t, versionComponentTestConfigs{
				nodeVersion:                testCase.nodeVersion,
				versionBeacons:             versionBeacons,
				finalizedRootBlockHeight:   finalizedRootBlockHeight,
				latestFinalizedBlockHeight: latestBlockHeight,
				signalerContext:            testCase.signalerMockCtx,
			})

			testCase.checkResult(t, vc)
		})
	}
}

// TestVersionBoundaryUpdated tests the behavior of the VersionControl component when the version is updated.
func TestVersionBoundaryUpdated(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create version event for initial height
	blockHeight10 := uint64(10)
	versionEvents := map[uint64]*flow.SealedVersionBeacon{
		blockHeight10: VersionBeaconEvent(
			t,
			blockHeight10,
			[]uint64{blockHeight10},
			[]string{"0.0.1"},
		),
	}

	// Set up a mock version beacons storage.
	versionBeacons := storageMock.NewVersionBeacons(t)

	// Mock the Highest method to return a specific version beacon.
	versionBeacons.
		On("Highest", testifyMock.AnythingOfType("uint64")).
		Return(mocks.StorageMapGetter(versionEvents))

	vc := createVersionControlComponent(t, versionComponentTestConfigs{
		nodeVersion:                "0.0.1",
		versionBeacons:             versionBeacons,
		finalizedRootBlockHeight:   0,
		latestFinalizedBlockHeight: blockHeight10,
		signalerContext:            irrecoverable.NewMockSignalerContext(t, ctx),
	})

	// Check compatibility at the initial height
	compatible, err := vc.CompatibleAtBlock(blockHeight10)
	require.NoError(t, err)
	require.True(t, compatible)

	// Create a new height and version event for testing the block finalized event
	blockHeight11 := uint64(11)
	// The version should be the same as previous one
	versionEvents[blockHeight11] = versionEvents[blockHeight10]

	// Simulate a block finalized event
	vc.blockFinalized(irrecoverable.MockSignalerContext{}, blockHeight11)

	// Check compatibility at the new height
	compatible, err = vc.CompatibleAtBlock(blockHeight11)
	require.NoError(t, err)
	assert.True(t, compatible)

	// Create a new height and version event with a higher version.
	blockHeight12 := uint64(12)
	versionEvents[blockHeight12] = VersionBeaconEvent(
		t,
		blockHeight12,
		[]uint64{blockHeight10, blockHeight12},
		[]string{"0.0.1", "0.0.2"},
	)

	// Add a consumer to verify version updates
	vc.AddVersionUpdatesConsumer(func(height uint64, semver string) {
		assert.Equal(t, blockHeight12, height)
		assert.Equal(t, "0.0.2", semver)
	})
	assert.Len(t, vc.consumers, 1)

	// Simulate a block finalized event for the new height
	vc.blockFinalized(irrecoverable.MockSignalerContext{}, blockHeight12)

	// Check compatibility at various heights
	compatible, err = vc.CompatibleAtBlock(blockHeight10)
	require.NoError(t, err)
	assert.True(t, compatible)

	compatible, err = vc.CompatibleAtBlock(blockHeight11)
	require.NoError(t, err)
	assert.True(t, compatible)

	compatible, err = vc.CompatibleAtBlock(blockHeight12)
	require.NoError(t, err)
	assert.False(t, compatible)
}

// TestVersionBoundaryDeleted tests the behavior of the VersionControl component when the version is deleted.
func TestVersionBoundaryDeleted(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create version event for initial height
	blockHeight10 := uint64(10)
	versionEvents := map[uint64]*flow.SealedVersionBeacon{
		blockHeight10: VersionBeaconEvent(
			t,
			blockHeight10,
			[]uint64{blockHeight10},
			[]string{"0.0.1"},
		),
	}

	// Set up a mock version beacons storage.
	versionBeacons := storageMock.NewVersionBeacons(t)

	// Mock the Highest method to return a specific version beacon.
	versionBeacons.
		On("Highest", testifyMock.AnythingOfType("uint64")).
		Return(mocks.StorageMapGetter(versionEvents))

	vc := createVersionControlComponent(t, versionComponentTestConfigs{
		nodeVersion:                "0.0.1",
		versionBeacons:             versionBeacons,
		finalizedRootBlockHeight:   0,
		latestFinalizedBlockHeight: blockHeight10,
		signalerContext:            irrecoverable.NewMockSignalerContext(t, ctx),
	})

	// Check compatibility at the initial height
	compatible, err := vc.CompatibleAtBlock(blockHeight10)
	require.NoError(t, err)
	require.True(t, compatible)

	// Create a new height with the same version event
	blockHeight11 := uint64(11)
	versionEvents[blockHeight11] = versionEvents[blockHeight10]

	// Simulate a block finalized event
	vc.blockFinalized(irrecoverable.MockSignalerContext{}, blockHeight11)

	// Check compatibility at the new height
	compatible, err = vc.CompatibleAtBlock(blockHeight11)
	require.NoError(t, err)
	assert.True(t, compatible)

	// Create a new height
	blockHeight12 := uint64(12)
	// Create height, where version should change
	blockHeight15 := uint64(15)
	// Create a version beacon event with new version
	versionEvents[blockHeight12] = VersionBeaconEvent(
		t,
		blockHeight12,
		[]uint64{blockHeight10, blockHeight15},
		[]string{"0.0.1", "0.0.2"},
	)

	// Add a consumer to verify version updates
	consumer := func(height uint64, semver string) {
		assert.Equal(t, blockHeight15, height)
		assert.Equal(t, "0.0.2", semver)
	}
	vc.AddVersionUpdatesConsumer(consumer)
	assert.Len(t, vc.consumers, 1)

	// Simulate a block finalized event
	vc.blockFinalized(irrecoverable.MockSignalerContext{}, blockHeight12)

	// Check compatibility for new height
	compatible, err = vc.CompatibleAtBlock(blockHeight12)
	require.NoError(t, err)
	assert.True(t, compatible)

	// Create block height where updated version will be deleted
	blockHeight13 := uint64(13)
	delete(versionEvents, blockHeight12)
	versionEvents[blockHeight13] = versionEvents[blockHeight10]

	// Delete old consumer checks, and add new one to test version delete notification
	vc.consumers = nil
	consumer = func(height uint64, semver string) {
		assert.Equal(t, blockHeight13, height)
		assert.Equal(t, "", semver)
	}
	vc.AddVersionUpdatesConsumer(consumer)
	assert.Len(t, vc.consumers, 1)

	// Simulate a block finalized event.
	vc.blockFinalized(irrecoverable.MockSignalerContext{}, blockHeight13)

	compatible, err = vc.CompatibleAtBlock(blockHeight13)
	require.NoError(t, err)
	assert.True(t, compatible)
}

// versionComponentTestConfigs contains custom tweaks for version control creation
type versionComponentTestConfigs struct {
	nodeVersion                string
	versionBeacons             storage.VersionBeacons
	finalizedRootBlockHeight   uint64
	latestFinalizedBlockHeight uint64
	signalerContext            *irrecoverable.MockSignalerContext
}

func createVersionControlComponent(
	t *testing.T,
	config versionComponentTestConfigs,
) *VersionControl {
	// Create a new VersionControl instance with initial parameters.
	vc, err := NewVersionControl(
		unittest.Logger(),
		config.versionBeacons,
		semver.New(config.nodeVersion),
		config.finalizedRootBlockHeight,
		config.latestFinalizedBlockHeight,
	)
	require.NoError(t, err)

	// Start the VersionControl component.
	vc.Start(config.signalerContext)

	return vc
}

// VersionBeaconEvent creates a SealedVersionBeacon for the given heights and versions.
func VersionBeaconEvent(t *testing.T, sealHeight uint64, heights []uint64, versions []string) *flow.SealedVersionBeacon {
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
