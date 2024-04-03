package snapshots

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
)

var ErrSnapshotPhaseMismatch = errors.New("snapshot does not contain a valid sealing segment")
var ErrSnapshotHistoryLimit = errors.New("reached the snapshot history limit")

// GetDynamicBootstrapSnapshot returns `refSnapshot` if it is valid for use in dynamic bootstrapping.
// Otherwise returns an error. (Effectively this validates that the input snapshot can be used in dynamic bootstrapping.)
// Expected error returns during normal operations:
// * ErrSnapshotPhaseMismatch - snapshot does not contain a valid sealing segment
// All other errors should be treated as exceptions.
func GetDynamicBootstrapSnapshot(state protocol.State, refSnapshot protocol.Snapshot) (protocol.Snapshot, error) {
	return getValidSnapshot(state, refSnapshot, 0, false, 0)
}

// GetClosestDynamicBootstrapSnapshot will return a valid snapshot for dynamic bootstrapping
// Expected error returns during normal operations:
// If a snapshot does contain an invalid sealing segment query the state
// by height of each block in the segment and return a snapshot at the point
// where the transition happens.
// * ErrSnapshotPhaseMismatch - snapshot does not contain a valid sealing segment
// * ErrSnapshotHistoryLimit - reached the snapshot history limit
// All other errors should be treated as exceptions.
func GetClosestDynamicBootstrapSnapshot(state protocol.State, refSnapshot protocol.Snapshot, snapshotHistoryLimit int) (protocol.Snapshot, error) {
	return getValidSnapshot(state, refSnapshot, 0, true, snapshotHistoryLimit)
}

// GetCounterAndPhase returns the current epoch counter and phase, at `height`.
// No errors are expected during normal operation. 
func GetCounterAndPhase(state protocol.State, height uint64) (uint64, flow.EpochPhase, error) {
	snapshot := state.AtHeight(height)

	counter, err := snapshot.Epochs().Current().Counter()
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get counter for block (height=%d): %w", height, err)
	}

	phase, err := snapshot.Phase()
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get phase for block (height=%d): %w", height, err)
	}

	return counter, phase, nil
}

func IsEpochOrPhaseDifferent(counter1, counter2 uint64, phase1, phase2 flow.EpochPhase) bool {
	return counter1 != counter2 || phase1 != phase2
}

// getValidSnapshot will return a valid snapshot that has a sealing segment which
// 1. does not contain any blocks that span an epoch transition
// 2. does not contain any blocks that span an epoch phase transition
// If a snapshot does contain an invalid sealing segment query the state
// by height of each block in the segment and return a snapshot at the point
// where the transition happens.
// Expected error returns during normal operations:
// * ErrSnapshotPhaseMismatch - snapshot does not contain a valid sealing segment
// * ErrSnapshotHistoryLimit - failed to find a valid snapshot after checking `snapshotHistoryLimit` blocks
// All other errors should be treated as exceptions.
func getValidSnapshot(
	state protocol.State,
	snapshot protocol.Snapshot,
	blocksVisited int,
	findNextValidSnapshot bool,
	snapshotHistoryLimit int,
) (protocol.Snapshot, error) {
	segment, err := snapshot.SealingSegment()
	if err != nil {
		return nil, fmt.Errorf("failed to get sealing segment: %w", err)
	}

	counterAtHighest, phaseAtHighest, err := GetCounterAndPhase(state, segment.Highest().Header.Height)
	if err != nil {
		return nil, fmt.Errorf("failed to get counter and phase at highest block in the segment: %w", err)
	}

	counterAtLowest, phaseAtLowest, err := GetCounterAndPhase(state, segment.Sealed().Header.Height)
	if err != nil {
		return nil, fmt.Errorf("failed to get counter and phase at lowest block in the segment: %w", err)
	}

	// Check if the counters and phase are different this indicates that the sealing segment
	// of the snapshot requested spans either an epoch transition or phase transition.
	if IsEpochOrPhaseDifferent(counterAtHighest, counterAtLowest, phaseAtHighest, phaseAtLowest) {
		if !findNextValidSnapshot {
			return nil, ErrSnapshotPhaseMismatch
		}

		// Visit each node in strict order of decreasing height starting at head
		// to find the block that straddles the transition boundary.
		for i := len(segment.Blocks) - 1; i >= 0; i-- {
			blocksVisited++

			// NOTE: Check if we have reached our history limit, in edge cases
			// where the sealing segment is abnormally long we want to short circuit
			// the recursive calls and return an error. The API caller can retry.
			if blocksVisited > snapshotHistoryLimit {
				return nil, fmt.Errorf("%w: (%d)", ErrSnapshotHistoryLimit, snapshotHistoryLimit)
			}

			counterAtBlock, phaseAtBlock, err := GetCounterAndPhase(state, segment.Blocks[i].Header.Height)
			if err != nil {
				return nil, fmt.Errorf("failed to get epoch counter and phase for snapshot at block %s: %w", segment.Blocks[i].ID(), err)
			}

			// Check if this block straddles the transition boundary, if it does return the snapshot
			// at that block height.
			if IsEpochOrPhaseDifferent(counterAtHighest, counterAtBlock, phaseAtHighest, phaseAtBlock) {
				return getValidSnapshot(state, state.AtHeight(segment.Blocks[i].Header.Height), blocksVisited, true, snapshotHistoryLimit)
			}
		}
	}

	return snapshot, nil
}
