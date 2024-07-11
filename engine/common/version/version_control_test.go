package version

import (
	"context"
	"math"
	"sort"
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
)

// testCaseConfig contains custom tweaks for test cases
type testCaseConfig struct {
	name        string
	nodeVersion string

	versionEvents []*flow.SealedVersionBeacon
	expectedStart uint64
	expectedEnd   uint64
}

// TestVersionControlInitialization tests the initialization process of the VersionControl component
func TestVersionControlInitialization(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	finalizedRootBlockHeight := uint64(1000)
	latestBlockHeight := finalizedRootBlockHeight + 100

	testCases := []testCaseConfig{
		{
			name:        "no version beacon found",
			nodeVersion: "0.0.1",
			versionEvents: []*flow.SealedVersionBeacon{
				VersionBeaconEvent(finalizedRootBlockHeight-100,
					flow.VersionBoundary{BlockHeight: finalizedRootBlockHeight - 50, Version: "0.0.1"}),
			},
			expectedStart: finalizedRootBlockHeight,
			expectedEnd:   latestBlockHeight,
		},
		{
			name:        "height is bigger than latest block height",
			nodeVersion: "0.0.1",
			versionEvents: []*flow.SealedVersionBeacon{
				VersionBeaconEvent(finalizedRootBlockHeight+10,
					flow.VersionBoundary{BlockHeight: finalizedRootBlockHeight + 12, Version: "0.0.1"}),
			},
			expectedStart: latestBlockHeight + 1,
			expectedEnd:   latestBlockHeight + 1,
		},
		{
			name:        "height is smaller than finalized root block height",
			nodeVersion: "0.0.1",
			versionEvents: []*flow.SealedVersionBeacon{
				VersionBeaconEvent(finalizedRootBlockHeight+10,
					flow.VersionBoundary{BlockHeight: finalizedRootBlockHeight + 12, Version: "0.0.1"}),
			},
			expectedStart: finalizedRootBlockHeight - 1,
			expectedEnd:   finalizedRootBlockHeight - 1,
		},
		{
			name:        "start version set",
			nodeVersion: "0.0.1",
			versionEvents: []*flow.SealedVersionBeacon{
				VersionBeaconEvent(finalizedRootBlockHeight+10,
					flow.VersionBoundary{BlockHeight: finalizedRootBlockHeight + 12, Version: "0.0.1"}),
			},
			expectedStart: finalizedRootBlockHeight + 12,
			expectedEnd:   latestBlockHeight,
		},
		{
			name:        "correct start version found",
			nodeVersion: "0.0.3",
			versionEvents: []*flow.SealedVersionBeacon{
				VersionBeaconEvent(finalizedRootBlockHeight+2,
					flow.VersionBoundary{BlockHeight: finalizedRootBlockHeight + 4, Version: "0.0.1"}),
				VersionBeaconEvent(finalizedRootBlockHeight+5,
					flow.VersionBoundary{BlockHeight: finalizedRootBlockHeight + 7, Version: "0.0.2"}),
			},
			expectedStart: finalizedRootBlockHeight + 7,
			expectedEnd:   latestBlockHeight,
		},
		{
			name:        "end version set",
			nodeVersion: "0.0.1",
			versionEvents: []*flow.SealedVersionBeacon{
				VersionBeaconEvent(finalizedRootBlockHeight-100,
					flow.VersionBoundary{BlockHeight: finalizedRootBlockHeight - 50, Version: "0.0.1"}),
				VersionBeaconEvent(latestBlockHeight-10,
					flow.VersionBoundary{BlockHeight: latestBlockHeight - 8, Version: "0.0.3"}),
			},
			expectedStart: finalizedRootBlockHeight,
			expectedEnd:   latestBlockHeight - 9,
		},
		{
			name:        "correct end version found",
			nodeVersion: "0.0.1",
			versionEvents: []*flow.SealedVersionBeacon{
				VersionBeaconEvent(finalizedRootBlockHeight-100,
					flow.VersionBoundary{BlockHeight: finalizedRootBlockHeight - 50, Version: "0.0.1"}),
				VersionBeaconEvent(latestBlockHeight-10,
					flow.VersionBoundary{BlockHeight: latestBlockHeight - 8, Version: "0.0.3"}),
				VersionBeaconEvent(latestBlockHeight-3,
					flow.VersionBoundary{BlockHeight: latestBlockHeight - 1, Version: "0.0.4"}),
			},
			expectedStart: finalizedRootBlockHeight,
			expectedEnd:   latestBlockHeight - 9,
		},
		{
			name:        "start and end version set",
			nodeVersion: "0.0.2",
			versionEvents: []*flow.SealedVersionBeacon{
				VersionBeaconEvent(finalizedRootBlockHeight+10,
					flow.VersionBoundary{BlockHeight: finalizedRootBlockHeight + 12, Version: "0.0.1"}),
				VersionBeaconEvent(latestBlockHeight-10,
					flow.VersionBoundary{BlockHeight: latestBlockHeight - 8, Version: "0.0.3"}),
			},
			expectedStart: finalizedRootBlockHeight + 12,
			expectedEnd:   latestBlockHeight - 9,
		},
		{
			name:        "correct start and end version found",
			nodeVersion: "0.0.2",
			versionEvents: []*flow.SealedVersionBeacon{
				VersionBeaconEvent(finalizedRootBlockHeight+2,
					flow.VersionBoundary{BlockHeight: finalizedRootBlockHeight + 4, Version: "0.0.1"}),
				VersionBeaconEvent(finalizedRootBlockHeight+10,
					flow.VersionBoundary{BlockHeight: finalizedRootBlockHeight + 12, Version: "0.0.2"}),
				VersionBeaconEvent(latestBlockHeight-10,
					flow.VersionBoundary{BlockHeight: latestBlockHeight - 8, Version: "0.0.3"}),
				VersionBeaconEvent(latestBlockHeight-3,
					flow.VersionBoundary{BlockHeight: latestBlockHeight - 1, Version: "0.0.4"}),
			},
			expectedStart: finalizedRootBlockHeight + 12,
			expectedEnd:   latestBlockHeight - 9,
		},
		{
			name:        "node's version is too old for current latest",
			nodeVersion: "0.0.1",
			versionEvents: []*flow.SealedVersionBeacon{
				// the node's version is too old for the earliest version boundary for the network
				VersionBeaconEvent(finalizedRootBlockHeight-100,
					flow.VersionBoundary{BlockHeight: finalizedRootBlockHeight - 50, Version: "0.0.2"}),
			},
			expectedStart: math.MaxUint64,
			expectedEnd:   math.MaxUint64,
		},
		{
			name:        "node's version is too new for current latest",
			nodeVersion: "0.0.3",
			versionEvents: []*flow.SealedVersionBeacon{
				VersionBeaconEvent(finalizedRootBlockHeight-100,
					flow.VersionBoundary{BlockHeight: finalizedRootBlockHeight - 50, Version: "0.0.2"}),

				// the version boundary that transitions to the node's version applies after the
				// latest finalized block, so the node's version is not compatible with any block
				VersionBeaconEvent(latestBlockHeight-3,
					flow.VersionBoundary{BlockHeight: latestBlockHeight + 1, Version: "0.0.3"}),
				VersionBeaconEvent(latestBlockHeight-2,
					flow.VersionBoundary{BlockHeight: latestBlockHeight + 2, Version: "0.0.4"}),
			},
			expectedStart: math.MaxUint64,
			expectedEnd:   math.MaxUint64,
		},
		{
			name:        "pre-release versions handled as expected",
			nodeVersion: "0.0.1-pre-release.1",
			versionEvents: []*flow.SealedVersionBeacon{
				// 0.0.1-pre-release.1 > 0.0.1-pre-release.0
				VersionBeaconEvent(finalizedRootBlockHeight+10,
					flow.VersionBoundary{BlockHeight: finalizedRootBlockHeight + 12, Version: "0.0.1-pre-release.0"}),
				// 0.0.1-pre-release.1 < 0.0.1
				VersionBeaconEvent(finalizedRootBlockHeight+12,
					flow.VersionBoundary{BlockHeight: finalizedRootBlockHeight + 14, Version: "0.0.1"}),
			},
			expectedStart: finalizedRootBlockHeight + 12,
			expectedEnd:   finalizedRootBlockHeight + 13,
		},
		{
			name:        "0.0.0 handled as expected",
			nodeVersion: "0.0.0-20230101000000-c0c9f774e40c",
			versionEvents: []*flow.SealedVersionBeacon{
				// 0.0.0-20230101000000-c0c9f774e40c > 0.0.0-20220101000000-7b4eea64cf58
				VersionBeaconEvent(finalizedRootBlockHeight+10,
					flow.VersionBoundary{BlockHeight: finalizedRootBlockHeight + 12, Version: "0.0.0-20220101000000-7b4eea64cf58"}),
				// 0.0.0-20230101000000-c0c9f774e40c < 0.0.0-20240101000000-6ceb2ff114de
				VersionBeaconEvent(finalizedRootBlockHeight+12,
					flow.VersionBoundary{BlockHeight: finalizedRootBlockHeight + 14, Version: "0.0.0-20240101000000-6ceb2ff114de"}),
			},
			expectedStart: finalizedRootBlockHeight + 12,
			expectedEnd:   finalizedRootBlockHeight + 13,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			eventMap := make(map[uint64]*flow.SealedVersionBeacon, len(testCase.versionEvents))
			for _, event := range testCase.versionEvents {
				eventMap[event.SealHeight] = event
			}

			// make sure events are sorted descending by seal height
			sort.Slice(testCase.versionEvents, func(i, j int) bool {
				return testCase.versionEvents[i].SealHeight > testCase.versionEvents[j].SealHeight
			})

			versionBeacons := storageMock.NewVersionBeacons(t)
			versionBeacons.
				On("Highest", testifyMock.AnythingOfType("uint64")).
				Return(func(height uint64) (*flow.SealedVersionBeacon, error) {
					// iterating through events sorted descending by seal height
					// return the first event that was sealed in a height less than or equal to height
					for _, event := range testCase.versionEvents {
						if event.SealHeight <= height {
							return event, nil
						}
					}
					return nil, storage.ErrNotFound
				})

			vc := createVersionControlComponent(t, versionComponentTestConfigs{
				nodeVersion:                testCase.nodeVersion,
				versionBeacons:             versionBeacons,
				finalizedRootBlockHeight:   finalizedRootBlockHeight,
				latestFinalizedBlockHeight: latestBlockHeight,
				signalerContext:            irrecoverable.NewMockSignalerContext(t, ctx),
			})

			checks := generateChecks(testCase, finalizedRootBlockHeight, latestBlockHeight)
			shouldExpectOutOfRangeError := finalizedRootBlockHeight > testCase.expectedStart || testCase.expectedEnd > latestBlockHeight

			for height, expected := range checks {
				compatible, err := vc.CompatibleAtBlock(height)

				if shouldExpectOutOfRangeError {
					assert.Error(t, ErrOutOfRange)
				} else {
					require.NoError(t, err)
					assert.Equal(t, expected, compatible, "unexpected compatibility at height %d. want: %t, got %t", height, expected, compatible)
				}
			}
		})
	}
}

func generateChecks(testCase testCaseConfig, finalizedRootBlockHeight, latestBlockHeight uint64) map[uint64]bool {
	checks := map[uint64]bool{}
	if testCase.expectedStart == math.MaxUint64 && testCase.expectedEnd == math.MaxUint64 {
		for height := finalizedRootBlockHeight; height <= latestBlockHeight; height++ {
			checks[height] = false
		}
		return checks
	}

	checks[testCase.expectedStart] = true
	checks[testCase.expectedEnd] = true

	if testCase.expectedStart > finalizedRootBlockHeight {
		checks[finalizedRootBlockHeight] = false
		checks[testCase.expectedStart-1] = false
	}

	if testCase.expectedEnd < latestBlockHeight {
		checks[latestBlockHeight] = false
		checks[testCase.expectedEnd+1] = false
	}

	return checks
}

// TestVersionBoundaryUpdated tests the behavior of the VersionControl component when the version is updated.
func TestVersionBoundaryUpdated(t *testing.T) {
	signalCtx := irrecoverable.NewMockSignalerContext(t, context.Background())

	contract := &versionBeaconContract{}

	// Create version event for initial height
	latestHeight := uint64(10)
	boundaryHeight := uint64(13)

	vc := createVersionControlComponent(t, versionComponentTestConfigs{
		nodeVersion:                "0.0.1",
		versionBeacons:             contract,
		finalizedRootBlockHeight:   0,
		latestFinalizedBlockHeight: latestHeight,
		signalerContext:            signalCtx,
	})

	var assertUpdate func(height uint64, semver string)
	var assertCallbackCalled func()

	// Add a consumer to verify version updates
	vc.AddVersionUpdatesConsumer(func(height uint64, semver string) {
		assertUpdate(height, semver)
	})
	assert.Len(t, vc.consumers, 1)

	// At this point, both start and end heights are unset

	// Add a new boundary, and finalize the block
	latestHeight++ // 11
	contract.AddBoundary(latestHeight, flow.VersionBoundary{BlockHeight: boundaryHeight, Version: "0.0.2"})

	assertUpdate, assertCallbackCalled = generateConsumerAssertions(t, boundaryHeight, "0.0.2")
	vc.blockFinalized(signalCtx, latestHeight)
	assertCallbackCalled()

	// Next, update the boundary and finalize the block
	latestHeight++ // 12
	contract.UpdateBoundary(latestHeight, boundaryHeight, "0.0.3")

	assertUpdate, assertCallbackCalled = generateConsumerAssertions(t, boundaryHeight, "0.0.3")
	vc.blockFinalized(signalCtx, latestHeight)
	assertCallbackCalled()

	// Finally, finalize one more block to get past the boundary
	latestHeight++ // 13
	vc.blockFinalized(signalCtx, latestHeight)

	// Check compatibility at various heights
	compatible, err := vc.CompatibleAtBlock(10)
	require.NoError(t, err)
	assert.True(t, compatible)

	compatible, err = vc.CompatibleAtBlock(12)
	require.NoError(t, err)
	assert.True(t, compatible)

	compatible, err = vc.CompatibleAtBlock(13)
	require.NoError(t, err)
	assert.False(t, compatible)
}

// TestVersionBoundaryDeleted tests the behavior of the VersionControl component when the version is deleted.
func TestVersionBoundaryDeleted(t *testing.T) {
	signalCtx := irrecoverable.NewMockSignalerContext(t, context.Background())

	contract := &versionBeaconContract{}

	// Create version event for initial height
	latestHeight := uint64(10)
	boundaryHeight := uint64(13)

	vc := createVersionControlComponent(t, versionComponentTestConfigs{
		nodeVersion:                "0.0.1",
		versionBeacons:             contract,
		finalizedRootBlockHeight:   0,
		latestFinalizedBlockHeight: latestHeight,
		signalerContext:            signalCtx,
	})

	var assertUpdate func(height uint64, semver string)
	var assertCallbackCalled func()

	// Add a consumer to verify version updates
	vc.AddVersionUpdatesConsumer(func(height uint64, semver string) {
		assertUpdate(height, semver)
	})
	assert.Len(t, vc.consumers, 1)

	// Add a new boundary, and finalize the block
	latestHeight++ // 11
	contract.AddBoundary(latestHeight, flow.VersionBoundary{BlockHeight: boundaryHeight, Version: "0.0.2"})

	assertUpdate, assertCallbackCalled = generateConsumerAssertions(t, boundaryHeight, "0.0.2")
	vc.blockFinalized(signalCtx, latestHeight)
	assertCallbackCalled()

	// Next, delete the boundary and finalize the block
	latestHeight++ // 12
	contract.DeleteBoundary(latestHeight, boundaryHeight)

	assertUpdate, assertCallbackCalled = generateConsumerAssertions(t, boundaryHeight, "") // called with empty string signalling deleted
	vc.blockFinalized(signalCtx, latestHeight)
	assertCallbackCalled()

	// Finally, finalize one more block to get past the boundary
	latestHeight++ // 13
	vc.blockFinalized(signalCtx, latestHeight)

	// Check compatibility at various heights
	compatible, err := vc.CompatibleAtBlock(10)
	require.NoError(t, err)
	assert.True(t, compatible)

	compatible, err = vc.CompatibleAtBlock(12)
	require.NoError(t, err)
	assert.True(t, compatible)

	compatible, err = vc.CompatibleAtBlock(13)
	require.NoError(t, err)
	assert.True(t, compatible)
}

// TestNotificationSkippedForCompatibleVersions tests that the VersionControl component does not
// send notifications to consumers VersionBeacon events with compatible versions.
func TestNotificationSkippedForCompatibleVersions(t *testing.T) {
	signalCtx := irrecoverable.NewMockSignalerContext(t, context.Background())

	contract := &versionBeaconContract{}

	// Create version event for initial height
	latestHeight := uint64(10)
	boundaryHeight := uint64(13)

	vc := createVersionControlComponent(t, versionComponentTestConfigs{
		nodeVersion:                "0.0.1",
		versionBeacons:             contract,
		finalizedRootBlockHeight:   0,
		latestFinalizedBlockHeight: latestHeight,
		signalerContext:            signalCtx,
	})

	// Add a consumer to verify notification is never sent
	vc.AddVersionUpdatesConsumer(func(height uint64, semver string) {
		t.Errorf("unexpected callback called at height %d with version %s", height, semver)
	})
	assert.Len(t, vc.consumers, 1)

	// Add a new boundary, and finalize the block
	latestHeight++ // 11
	contract.AddBoundary(latestHeight, flow.VersionBoundary{BlockHeight: boundaryHeight, Version: "0.0.1-pre-release"})

	vc.blockFinalized(signalCtx, latestHeight)

	// Check compatibility at various heights
	compatible, err := vc.CompatibleAtBlock(10)
	require.NoError(t, err)
	assert.True(t, compatible)

	compatible, err = vc.CompatibleAtBlock(11)
	require.NoError(t, err)
	assert.True(t, compatible)
}

func generateConsumerAssertions(
	t *testing.T,
	boundaryHeight uint64,
	version string,
) (func(height uint64, semver string), func()) {
	called := false

	assertUpdate := func(height uint64, semver string) {
		assert.Equal(t, boundaryHeight, height)
		assert.Equal(t, version, semver)
		called = true
	}

	assertCalled := func() {
		assert.True(t, called)
	}

	return assertUpdate, assertCalled
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
	unittest.RequireComponentsReadyBefore(t, 2*time.Second, vc)

	return vc
}

// VersionBeaconEvent creates a SealedVersionBeacon for the given heights and versions.
func VersionBeaconEvent(sealHeight uint64, vb ...flow.VersionBoundary) *flow.SealedVersionBeacon {
	return &flow.SealedVersionBeacon{
		VersionBeacon: unittest.VersionBeaconFixture(
			unittest.WithBoundaries(vb...),
		),
		SealHeight: sealHeight,
	}
}

type versionBeaconContract struct {
	boundaries []flow.VersionBoundary
	events     []*flow.SealedVersionBeacon
}

func (c *versionBeaconContract) Highest(belowOrEqualTo uint64) (*flow.SealedVersionBeacon, error) {
	for _, event := range c.events {
		if event.SealHeight <= belowOrEqualTo {
			return event, nil
		}
	}
	return nil, storage.ErrNotFound
}

func (c *versionBeaconContract) AddBoundary(sealedHeight uint64, boundary flow.VersionBoundary) {
	c.boundaries = append(c.boundaries, boundary)
	c.emitEvent(sealedHeight)
}

func (c *versionBeaconContract) DeleteBoundary(sealedHeight, boundaryHeight uint64) {
	for i, boundary := range c.boundaries {
		if boundary.BlockHeight == boundaryHeight {
			c.boundaries = append(c.boundaries[:i], c.boundaries[i+1:]...)
			break
		}
	}
	c.emitEvent(sealedHeight)
}

func (c *versionBeaconContract) UpdateBoundary(sealedHeight, boundaryHeight uint64, version string) {
	for i, boundary := range c.boundaries {
		if boundary.BlockHeight == boundaryHeight {
			c.boundaries[i].Version = version
			break
		}
	}
	c.emitEvent(sealedHeight)
}

func (c *versionBeaconContract) emitEvent(sealedHeight uint64) {
	// sort boundaries ascending by height
	sort.Slice(c.boundaries, func(i, j int) bool {
		return c.boundaries[i].BlockHeight < c.boundaries[j].BlockHeight
	})

	// include only future boundaries
	boundaries := make([]flow.VersionBoundary, 0)
	for _, boundary := range c.boundaries {
		if boundary.BlockHeight >= sealedHeight {
			boundaries = append(boundaries, boundary)
		}
	}
	c.events = append(c.events, VersionBeaconEvent(sealedHeight, boundaries...))

	// sort boundaries descending by height
	sort.Slice(c.events, func(i, j int) bool {
		return c.events[i].SealHeight > c.events[j].SealHeight
	})
}
