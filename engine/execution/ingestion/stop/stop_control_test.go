package stop

import (
	"fmt"
	"testing"

	"github.com/coreos/go-semver/semver"
	testifyMock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/execution/state/mock"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
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
			nil,
			nil,
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

		execState := mock.NewExecutionState(t)

		sc := NewStopControl(
			unittest.Logger(),
			execState,
			nil,
			nil,
			nil,
			false,
			false,
		)

		require.False(t, sc.GetStopParameters().Set())

		// first update is always successful
		stop := StopParameters{StopBeforeHeight: 21}
		err := sc.SetStopParameters(stop)
		require.NoError(t, err)
		require.Equal(t, stop, sc.GetStopParameters())

		// make execution check pretends block has been executed
		execState.On("StateCommitmentByBlockID", testifyMock.Anything, testifyMock.Anything).Return(nil, nil)

		// no stopping has started yet, block below stop height
		header := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(20))
		sc.BlockFinalizedForTesting(header)

		stop2 := StopParameters{StopBeforeHeight: 37}
		err = sc.SetStopParameters(stop2)
		require.NoError(t, err)
		require.Equal(t, stop2, sc.GetStopParameters())

		// block at stop height, it should be triggered stop
		header = unittest.BlockHeaderFixture(unittest.WithHeaderHeight(37))
		sc.BlockFinalizedForTesting(header)

		// since we set shouldCrash to false, execution should be stopped
		require.True(t, sc.IsExecutionStopped())

		err = sc.SetStopParameters(StopParameters{StopBeforeHeight: 2137})
		require.ErrorIs(t, err, ErrCannotChangeStop)
	})
}

// TestExecutionFallingBehind check if StopControl behaves properly even if EN runs behind
// and blocks are finalized before they are executed
func TestExecutionFallingBehind(t *testing.T) {

	execState := mock.NewExecutionState(t)

	headerA := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(20))
	headerB := unittest.BlockHeaderWithParentFixture(headerA) // 21
	headerC := unittest.BlockHeaderWithParentFixture(headerB) // 22
	headerD := unittest.BlockHeaderWithParentFixture(headerC) // 23

	sc := NewStopControl(
		unittest.Logger(),
		execState,
		nil,
		nil,
		nil,
		false,
		false,
	)

	// set stop at 22, so 21 is the last height which should be processed
	stop := StopParameters{StopBeforeHeight: 22}
	err := sc.SetStopParameters(stop)
	require.NoError(t, err)
	require.Equal(t, stop, sc.GetStopParameters())

	execState.
		On("StateCommitmentByBlockID", testifyMock.Anything, headerC.ParentID).
		Return(nil, storage.ErrNotFound)

	// finalize blocks first
	sc.BlockFinalizedForTesting(headerA)
	sc.BlockFinalizedForTesting(headerB)
	sc.BlockFinalizedForTesting(headerC)
	sc.BlockFinalizedForTesting(headerD)

	// simulate execution
	sc.OnBlockExecuted(headerA)
	sc.OnBlockExecuted(headerB)
	require.True(t, sc.IsExecutionStopped())
}

type stopControlMockHeaders struct {
	headers map[uint64]*flow.Header
}

func (m *stopControlMockHeaders) ByHeight(height uint64) (*flow.Header, error) {
	h, ok := m.headers[height]
	if !ok {
		return nil, fmt.Errorf("header not found")
	}
	return h, nil
}

func TestAddStopForPastBlocks(t *testing.T) {
	execState := mock.NewExecutionState(t)

	headerA := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(20))
	headerB := unittest.BlockHeaderWithParentFixture(headerA) // 21
	headerC := unittest.BlockHeaderWithParentFixture(headerB) // 22
	headerD := unittest.BlockHeaderWithParentFixture(headerC) // 23

	headers := &stopControlMockHeaders{
		headers: map[uint64]*flow.Header{
			headerA.Height: headerA,
			headerB.Height: headerB,
			headerC.Height: headerC,
			headerD.Height: headerD,
		},
	}

	sc := NewStopControl(
		unittest.Logger(),
		execState,
		headers,
		nil,
		nil,
		false,
		false,
	)

	// finalize blocks first
	sc.BlockFinalizedForTesting(headerA)
	sc.BlockFinalizedForTesting(headerB)
	sc.BlockFinalizedForTesting(headerC)

	// simulate execution
	sc.OnBlockExecuted(headerA)
	sc.OnBlockExecuted(headerB)
	sc.OnBlockExecuted(headerC)

	// block is executed
	execState.
		On("StateCommitmentByBlockID", testifyMock.Anything, headerD.ParentID).
		Return(nil, nil)

	// set stop at 22, but finalization and execution is at 23
	// so stop right away
	stop := StopParameters{StopBeforeHeight: 22}
	err := sc.SetStopParameters(stop)
	require.NoError(t, err)
	require.Equal(t, stop, sc.GetStopParameters())

	// finalize one more block after stop is set
	sc.BlockFinalizedForTesting(headerD)

	require.True(t, sc.IsExecutionStopped())
}

func TestAddStopForPastBlocksExecutionFallingBehind(t *testing.T) {
	execState := mock.NewExecutionState(t)

	headerA := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(20))
	headerB := unittest.BlockHeaderWithParentFixture(headerA) // 21
	headerC := unittest.BlockHeaderWithParentFixture(headerB) // 22
	headerD := unittest.BlockHeaderWithParentFixture(headerC) // 23

	headers := &stopControlMockHeaders{
		headers: map[uint64]*flow.Header{
			headerA.Height: headerA,
			headerB.Height: headerB,
			headerC.Height: headerC,
			headerD.Height: headerD,
		},
	}

	sc := NewStopControl(
		unittest.Logger(),
		execState,
		headers,
		nil,
		nil,
		false,
		false,
	)

	execState.
		On("StateCommitmentByBlockID", testifyMock.Anything, headerD.ParentID).
		Return(nil, storage.ErrNotFound)

	// finalize blocks first
	sc.BlockFinalizedForTesting(headerA)
	sc.BlockFinalizedForTesting(headerB)
	sc.BlockFinalizedForTesting(headerC)

	// set stop at 22, but finalization is at 23 so 21
	// is the last height which wil be executed
	stop := StopParameters{StopBeforeHeight: 22}
	err := sc.SetStopParameters(stop)
	require.NoError(t, err)
	require.Equal(t, stop, sc.GetStopParameters())

	// finalize one more block after stop is set
	sc.BlockFinalizedForTesting(headerD)

	// simulate execution
	sc.OnBlockExecuted(headerA)
	sc.OnBlockExecuted(headerB)
	require.True(t, sc.IsExecutionStopped())
}

func TestStopControlWithVersionControl(t *testing.T) {
	t.Run("normal case", func(t *testing.T) {
		execState := mock.NewExecutionState(t)
		versionBeacons := new(storageMock.VersionBeacons)

		headerA := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(20))
		headerB := unittest.BlockHeaderWithParentFixture(headerA) // 21
		headerC := unittest.BlockHeaderWithParentFixture(headerB) // 22

		headers := &stopControlMockHeaders{
			headers: map[uint64]*flow.Header{
				headerA.Height: headerA,
				headerB.Height: headerB,
				headerC.Height: headerC,
			},
		}

		sc := NewStopControl(
			unittest.Logger(),
			execState,
			headers,
			versionBeacons,
			semver.New("1.0.0"),
			false,
			false,
		)

		// setting this means all finalized blocks are considered already executed
		execState.
			On("StateCommitmentByBlockID", testifyMock.Anything, headerC.ParentID).
			Return(nil, nil)

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
		sc.BlockFinalizedForTesting(headerA)
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
		sc.BlockFinalizedForTesting(headerB)
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
		sc.BlockFinalizedForTesting(headerC)
		// should be stopped as this is height 22 and height 21 is already considered executed
		require.True(t, sc.IsExecutionStopped())
	})

	t.Run("version boundary removed", func(t *testing.T) {

		// future version boundaries can be removed
		// in which case they will be missing from the version beacon
		execState := mock.NewExecutionState(t)
		versionBeacons := storageMock.NewVersionBeacons(t)

		headerA := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(20))
		headerB := unittest.BlockHeaderWithParentFixture(headerA) // 21
		headerC := unittest.BlockHeaderWithParentFixture(headerB) // 22

		headers := &stopControlMockHeaders{
			headers: map[uint64]*flow.Header{
				headerA.Height: headerA,
				headerB.Height: headerB,
				headerC.Height: headerC,
			},
		}

		sc := NewStopControl(
			unittest.Logger(),
			execState,
			headers,
			versionBeacons,
			semver.New("1.0.0"),
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
							BlockHeight: 21,
							Version:     "2.0.0",
						}),
				),
				SealHeight: headerA.Height,
			}, nil).Once()

		// finalize first block
		sc.BlockFinalizedForTesting(headerA)
		require.False(t, sc.IsExecutionStopped())
		require.Equal(t, StopParameters{
			StopBeforeHeight: 21,
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
		sc.BlockFinalizedForTesting(headerB)
		require.False(t, sc.IsExecutionStopped())
		require.False(t, sc.GetStopParameters().Set())
	})

	t.Run("manual not cleared by version beacon", func(t *testing.T) {
		// future version boundaries can be removed
		// in which case they will be missing from the version beacon
		execState := mock.NewExecutionState(t)
		versionBeacons := storageMock.NewVersionBeacons(t)

		headerA := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(20))
		headerB := unittest.BlockHeaderWithParentFixture(headerA) // 21
		headerC := unittest.BlockHeaderWithParentFixture(headerB) // 22

		headers := &stopControlMockHeaders{
			headers: map[uint64]*flow.Header{
				headerA.Height: headerA,
				headerB.Height: headerB,
				headerC.Height: headerC,
			},
		}

		sc := NewStopControl(
			unittest.Logger(),
			execState,
			headers,
			versionBeacons,
			semver.New("1.0.0"),
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
						}),
				),
				SealHeight: headerA.Height,
			}, nil).Once()

		// finalize first block
		sc.BlockFinalizedForTesting(headerA)
		require.False(t, sc.IsExecutionStopped())
		require.False(t, sc.GetStopParameters().Set())

		// set manual stop
		stop := StopParameters{
			StopBeforeHeight: 22,
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

		sc.BlockFinalizedForTesting(headerB)
		require.False(t, sc.IsExecutionStopped())
		// stop is not cleared due to being set manually
		require.Equal(t, stop, sc.GetStopParameters())
	})

	t.Run("version beacon not cleared by manual", func(t *testing.T) {
		// future version boundaries can be removed
		// in which case they will be missing from the version beacon
		execState := mock.NewExecutionState(t)
		versionBeacons := storageMock.NewVersionBeacons(t)

		headerA := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(20))
		headerB := unittest.BlockHeaderWithParentFixture(headerA) // 21

		headers := &stopControlMockHeaders{
			headers: map[uint64]*flow.Header{
				headerA.Height: headerA,
				headerB.Height: headerB,
			},
		}

		sc := NewStopControl(
			unittest.Logger(),
			execState,
			headers,
			versionBeacons,
			semver.New("1.0.0"),
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
		sc.BlockFinalizedForTesting(headerA)
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
		nil,
		nil,
		true,
		false,
	)
	require.True(t, sc.IsExecutionStopped())
}

func TestStoppedStateRejectsAllBlocksAndChanged(t *testing.T) {

	// make sure we don't even query executed status if stopped
	// mock should fail test on any method call
	execState := mock.NewExecutionState(t)

	sc := NewStopControl(
		unittest.Logger(),
		execState,
		nil,
		nil,
		nil,
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

	sc.BlockFinalizedForTesting(header)
	require.True(t, sc.IsExecutionStopped())
}
