package stop

import (
	"context"
	"testing"
	"time"

	"github.com/coreos/go-semver/semver"
	testifyMock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	storageMock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

// If stopping mechanism has caused any changes to execution flow
// (skipping execution of blocks) we disallow setting new values
func TestCannotSetNewValuesAfterStoppingCommenced(t *testing.T) {

	t.Run("when processing block at stop height", func(t *testing.T) {
		sc := NewStopControl(
			unittest.Logger(),
			nil,
			nil,
			&flow.Header{},
			false,
			false,
		)

		require.False(t, sc.GetStopParameters().Set())

		// first update is always successful
		stop := StopParameters{StopBeforeHeight: 21}
		err := sc.SetStopParameters(stop)
		require.NoError(t, err)

		require.Equal(t, stop, sc.GetStopParameters())

		// no stopping has started yet, block below stop height
		header := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(20))
		require.True(t, sc.ShouldExecuteBlock(header))

		stop2 := StopParameters{StopBeforeHeight: 37}
		err = sc.SetStopParameters(stop2)
		require.NoError(t, err)

		// block at stop height, it should be skipped
		header = unittest.BlockHeaderFixture(unittest.WithHeaderHeight(37))
		require.False(t, sc.ShouldExecuteBlock(header))

		// cannot set new stop height after stopping has started
		err = sc.SetStopParameters(StopParameters{StopBeforeHeight: 2137})
		require.ErrorIs(t, err, ErrCannotChangeStop)

		// state did not change
		require.Equal(t, stop2, sc.GetStopParameters())
	})

	t.Run("when processing finalized blocks", func(t *testing.T) {
		sc := NewStopControl(
			unittest.Logger(),
			nil,
			nil,
			&flow.Header{},
			false,
			false,
		)

		require.False(t, sc.GetStopParameters().Set())

		// first update is always successful
		stop := StopParameters{StopBeforeHeight: 22}
		err := sc.SetStopParameters(stop)
		require.NoError(t, err)
		require.Equal(t, stop, sc.GetStopParameters())

		// no stopping has started yet, block below stop height
		header := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(20))
		sc.BlockFinalizedAndExecutedForTesting(header)

		stop2 := StopParameters{StopBeforeHeight: 37}
		err = sc.SetStopParameters(stop2)
		require.NoError(t, err)
		require.Equal(t, stop2, sc.GetStopParameters())

		// block at stop height, it should be triggered stop
		header = unittest.BlockHeaderFixture(unittest.WithHeaderHeight(37))
		sc.BlockFinalizedAndExecutedForTesting(header)

		// since we set shouldCrash to false, execution should be stopped
		require.True(t, sc.IsExecutionStopped())

		err = sc.SetStopParameters(StopParameters{StopBeforeHeight: 2137})
		require.ErrorIs(t, err, ErrCannotChangeStop)
	})
}

func TestAddStopForPastBlocks(t *testing.T) {

	headerA := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(20))
	headerB := unittest.BlockHeaderWithParentFixture(headerA) // 21
	headerC := unittest.BlockHeaderWithParentFixture(headerB) // 22
	headerD := unittest.BlockHeaderWithParentFixture(headerC) // 23

	sc := NewStopControl(
		unittest.Logger(),
		nil,
		nil,
		&flow.Header{},
		false,
		false,
	)

	// finalize blocks first
	sc.BlockFinalizedAndExecutedForTesting(headerA)
	sc.BlockFinalizedAndExecutedForTesting(headerB)
	sc.BlockFinalizedAndExecutedForTesting(headerC)

	// set stop at 22, but finalization and execution is at 23
	// so stop right away
	stop := StopParameters{StopBeforeHeight: 22}
	err := sc.SetStopParameters(stop)
	require.NoError(t, err)
	require.Equal(t, stop, sc.GetStopParameters())

	// finalize one more block after stop is set
	sc.BlockFinalizedAndExecutedForTesting(headerD)

	require.True(t, sc.IsExecutionStopped())
}

func TestStopControlWithVersionControl(t *testing.T) {
	t.Run("normal case", func(t *testing.T) {
		versionBeacons := new(storageMock.VersionBeacons)

		headerA := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(20))
		headerB := unittest.BlockHeaderWithParentFixture(headerA) // 21
		headerC := unittest.BlockHeaderWithParentFixture(headerB) // 22

		sc := NewStopControl(
			unittest.Logger(),
			versionBeacons,
			semver.New("1.0.0"),
			&flow.Header{},
			false,
			false,
		)

		versionBeacons.
			On("Highest", testifyMock.Anything).
			Return(&flow.SealedVersionBeacon{
				VersionBeacon: unittest.VersionBeaconFixture(
					unittest.WithBoundaries(
						// zero boundary is expected if there
						// is no boundary set by the contract yet
						flow.VersionBoundary{
							BlockHeight: 0,
							Version:     "0.0.0",
						}),
				),
				SealHeight: headerA.Height,
			}, nil).Once()

		// finalize first block
		sc.BlockFinalizedAndExecutedForTesting(headerA)
		require.False(t, sc.IsExecutionStopped())
		require.False(t, sc.GetStopParameters().Set())

		// new version beacon
		versionBeacons.
			On("Highest", testifyMock.Anything).
			Return(&flow.SealedVersionBeacon{
				VersionBeacon: unittest.VersionBeaconFixture(
					unittest.WithBoundaries(
						// zero boundary is expected if there
						// is no boundary set by the contract yet
						flow.VersionBoundary{
							BlockHeight: 0,
							Version:     "0.0.0",
						}, flow.VersionBoundary{
							BlockHeight: 21,
							Version:     "1.0.0",
						}),
				),
				SealHeight: headerB.Height,
			}, nil).Once()

		// finalize second block. we are still ok as the node version
		// is the same as the version beacon one
		sc.BlockFinalizedAndExecutedForTesting(headerB)
		require.False(t, sc.IsExecutionStopped())
		require.False(t, sc.GetStopParameters().Set())

		// new version beacon
		versionBeacons.
			On("Highest", testifyMock.Anything).
			Return(&flow.SealedVersionBeacon{
				VersionBeacon: unittest.VersionBeaconFixture(
					unittest.WithBoundaries(
						// The previous version is included in the new version beacon
						flow.VersionBoundary{
							BlockHeight: 21,
							Version:     "1.0.0",
						}, flow.VersionBoundary{
							BlockHeight: 22,
							Version:     "2.0.0",
						}),
				),
				SealHeight: headerC.Height,
			}, nil).Once()
		sc.BlockFinalizedAndExecutedForTesting(headerC)
		// should be stopped as this is height 22 and height 21 is already considered executed
		require.True(t, sc.IsExecutionStopped())
	})

	t.Run("version boundary removed", func(t *testing.T) {

		// future version boundaries can be removed
		// in which case they will be missing from the version beacon
		versionBeacons := storageMock.NewVersionBeacons(t)

		headerA := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(20))
		headerB := unittest.BlockHeaderWithParentFixture(headerA) // 21

		sc := NewStopControl(
			unittest.Logger(),
			versionBeacons,
			semver.New("1.0.0"),
			&flow.Header{},
			false,
			false,
		)

		versionBeacons.
			On("Highest", testifyMock.Anything).
			Return(&flow.SealedVersionBeacon{
				VersionBeacon: unittest.VersionBeaconFixture(
					unittest.WithBoundaries(
						// set to stop at height 21
						flow.VersionBoundary{
							BlockHeight: 0,
							Version:     "0.0.0",
						}, flow.VersionBoundary{
							BlockHeight: 22,
							Version:     "2.0.0",
						}),
				),
				SealHeight: headerA.Height,
			}, nil).Once()

		// finalize first block
		sc.BlockFinalizedAndExecutedForTesting(headerA)
		require.False(t, sc.IsExecutionStopped())
		require.Equal(t, StopParameters{
			StopBeforeHeight: 22,
			ShouldCrash:      false,
		}, sc.GetStopParameters())

		// new version beacon
		versionBeacons.
			On("Highest", testifyMock.Anything).
			Return(&flow.SealedVersionBeacon{
				VersionBeacon: unittest.VersionBeaconFixture(
					unittest.WithBoundaries(
						// stop removed
						flow.VersionBoundary{
							BlockHeight: 0,
							Version:     "0.0.0",
						}),
				),
				SealHeight: headerB.Height,
			}, nil).Once()

		// finalize second block. we are still ok as the node version
		// is the same as the version beacon one
		sc.BlockFinalizedAndExecutedForTesting(headerB)
		require.False(t, sc.IsExecutionStopped())
		require.False(t, sc.GetStopParameters().Set())
	})

	t.Run("manual not cleared by version beacon", func(t *testing.T) {
		// future version boundaries can be removed
		// in which case they will be missing from the version beacon
		versionBeacons := storageMock.NewVersionBeacons(t)

		headerA := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(20))
		headerB := unittest.BlockHeaderWithParentFixture(headerA) // 21

		sc := NewStopControl(
			unittest.Logger(),
			versionBeacons,
			semver.New("1.0.0"),
			&flow.Header{},
			false,
			false,
		)

		versionBeacons.
			On("Highest", testifyMock.Anything).
			Return(&flow.SealedVersionBeacon{
				VersionBeacon: unittest.VersionBeaconFixture(
					unittest.WithBoundaries(
						flow.VersionBoundary{
							BlockHeight: 0,
							Version:     "0.0.0",
						}),
				),
				SealHeight: headerA.Height,
			}, nil).Once()

		// finalize first block
		sc.BlockFinalizedAndExecutedForTesting(headerA)
		require.False(t, sc.IsExecutionStopped())
		require.False(t, sc.GetStopParameters().Set())

		// set manual stop
		stop := StopParameters{
			StopBeforeHeight: 23,
			ShouldCrash:      false,
		}
		err := sc.SetStopParameters(stop)
		require.NoError(t, err)
		require.Equal(t, stop, sc.GetStopParameters())

		// new version beacon
		versionBeacons.
			On("Highest", testifyMock.Anything).
			Return(&flow.SealedVersionBeacon{
				VersionBeacon: unittest.VersionBeaconFixture(
					unittest.WithBoundaries(
						// stop removed
						flow.VersionBoundary{
							BlockHeight: 0,
							Version:     "0.0.0",
						}),
				),
				SealHeight: headerB.Height,
			}, nil).Once()

		sc.BlockFinalizedAndExecutedForTesting(headerB)
		require.False(t, sc.IsExecutionStopped())
		// stop is not cleared due to being set manually
		require.Equal(t, stop, sc.GetStopParameters())
	})

	t.Run("version beacon not cleared by manual", func(t *testing.T) {
		// future version boundaries can be removed
		// in which case they will be missing from the version beacon
		versionBeacons := storageMock.NewVersionBeacons(t)

		headerA := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(20))

		sc := NewStopControl(
			unittest.Logger(),
			versionBeacons,
			semver.New("1.0.0"),
			&flow.Header{},
			false,
			false,
		)

		vbStop := StopParameters{
			StopBeforeHeight: 22,
			ShouldCrash:      false,
		}
		versionBeacons.
			On("Highest", testifyMock.Anything).
			Return(&flow.SealedVersionBeacon{
				VersionBeacon: unittest.VersionBeaconFixture(
					unittest.WithBoundaries(
						// set to stop at height 21
						flow.VersionBoundary{
							BlockHeight: 0,
							Version:     "0.0.0",
						}, flow.VersionBoundary{
							BlockHeight: vbStop.StopBeforeHeight,
							Version:     "2.0.0",
						}),
				),
				SealHeight: headerA.Height,
			}, nil).Once()

		// finalize first block
		sc.BlockFinalizedAndExecutedForTesting(headerA)
		require.False(t, sc.IsExecutionStopped())
		require.Equal(t, vbStop, sc.GetStopParameters())

		// set manual stop
		stop := StopParameters{
			StopBeforeHeight: 23,
			ShouldCrash:      false,
		}
		err := sc.SetStopParameters(stop)
		require.ErrorIs(t, err, ErrCannotChangeStop)
		// stop is not cleared due to being set earlier by a version beacon
		require.Equal(t, vbStop, sc.GetStopParameters())
	})
}

// StopControl created as stopped will keep the state
func TestStartingStopped(t *testing.T) {

	sc := NewStopControl(
		unittest.Logger(),
		nil,
		nil,
		&flow.Header{},
		true,
		false,
	)
	require.True(t, sc.IsExecutionStopped())
}

func TestStoppedStateRejectsAllBlocksAndChanged(t *testing.T) {
	sc := NewStopControl(
		unittest.Logger(),
		nil,
		nil,
		&flow.Header{},
		true,
		false,
	)
	require.True(t, sc.IsExecutionStopped())

	err := sc.SetStopParameters(StopParameters{
		StopBeforeHeight: 2137,
		ShouldCrash:      true,
	})
	require.ErrorIs(t, err, ErrCannotChangeStop)

	header := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(20))

	sc.BlockFinalizedAndExecutedForTesting(header)
	require.True(t, sc.IsExecutionStopped())
}

func Test_StopControlWorkers(t *testing.T) {

	t.Run("start and stop, stopped = true", func(t *testing.T) {

		sc := NewStopControl(
			unittest.Logger(),
			nil,
			nil,
			&flow.Header{},
			true,
			false,
		)

		ctx, cancel := context.WithCancel(context.Background())
		ictx := irrecoverable.NewMockSignalerContext(t, ctx)

		sc.Start(ictx)

		unittest.AssertClosesBefore(t, sc.Ready(), 10*time.Second)

		cancel()

		unittest.AssertClosesBefore(t, sc.Done(), 10*time.Second)
	})

	t.Run("start and stop, stopped = false", func(t *testing.T) {

		sc := NewStopControl(
			unittest.Logger(),
			nil,
			nil,
			&flow.Header{},
			false,
			false,
		)

		ctx, cancel := context.WithCancel(context.Background())
		ictx := irrecoverable.NewMockSignalerContext(t, ctx)

		sc.Start(ictx)

		unittest.AssertClosesBefore(t, sc.Ready(), 10*time.Second)

		cancel()

		unittest.AssertClosesBefore(t, sc.Done(), 10*time.Second)
	})

	t.Run("start as stopped if execution is at version boundary", func(t *testing.T) {

		headerA := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(20))
		headerB := unittest.BlockHeaderWithParentFixture(headerA) // 21

		versionBeacons := storageMock.NewVersionBeacons(t)
		versionBeacons.On("Highest", headerB.Height).
			Return(&flow.SealedVersionBeacon{
				VersionBeacon: unittest.VersionBeaconFixture(
					unittest.WithBoundaries(
						flow.VersionBoundary{
							BlockHeight: headerB.Height,
							Version:     "2.0.0",
						},
					),
				),
				SealHeight: 1, // sealed in the past
			}, nil).
			Once()

		// This is a likely scenario where the node stopped because of a version
		// boundary but was restarted without being upgraded to the new version.
		// In this case, the node should start as stopped.
		sc := NewStopControl(
			unittest.Logger(),
			versionBeacons,
			semver.New("1.0.0"),
			headerB,
			false,
			false,
		)

		ctx, cancel := context.WithCancel(context.Background())
		ictx := irrecoverable.NewMockSignalerContext(t, ctx)

		sc.Start(ictx)

		unittest.AssertClosesBefore(t, sc.Ready(), 10*time.Second)

		// should start as stopped
		require.True(t, sc.IsExecutionStopped())
		require.Equal(t, StopParameters{
			StopBeforeHeight: headerB.Height,
			ShouldCrash:      false,
		}, sc.GetStopParameters())

		cancel()

		unittest.AssertClosesBefore(t, sc.Done(), 10*time.Second)
	})

	t.Run("test stopping with block finalized events", func(t *testing.T) {

		headerA := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(20))
		headerB := unittest.BlockHeaderWithParentFixture(headerA) // 21
		headerC := unittest.BlockHeaderWithParentFixture(headerB) // 22

		vb := &flow.SealedVersionBeacon{
			VersionBeacon: unittest.VersionBeaconFixture(
				unittest.WithBoundaries(
					flow.VersionBoundary{
						BlockHeight: headerC.Height,
						Version:     "2.0.0",
					},
				),
			),
			SealHeight: 1, // sealed in the past
		}

		versionBeacons := storageMock.NewVersionBeacons(t)
		versionBeacons.On("Highest", headerA.Height).
			Return(vb, nil).
			Once()
		versionBeacons.On("Highest", headerB.Height).
			Return(vb, nil).
			Once()

		// The stop is set by a previous version beacon and is in one blocks time.
		sc := NewStopControl(
			unittest.Logger(),
			versionBeacons,
			semver.New("1.0.0"),
			headerA,
			false,
			false,
		)

		ctx, cancel := context.WithCancel(context.Background())
		ictx := irrecoverable.NewMockSignalerContext(t, ctx)

		sc.Start(ictx)

		unittest.AssertClosesBefore(t, sc.Ready(), 10*time.Second)

		require.False(t, sc.IsExecutionStopped())
		require.Equal(t, StopParameters{
			StopBeforeHeight: headerC.Height,
			ShouldCrash:      false,
		}, sc.GetStopParameters())

		sc.FinalizedAndExecuted(headerB)

		done := make(chan struct{})
		go func() {
			for !sc.IsExecutionStopped() {
				<-time.After(100 * time.Millisecond)
			}
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for stop control to stop execution")
		}

		cancel()
		unittest.AssertClosesBefore(t, sc.Done(), 10*time.Second)
	})
}
