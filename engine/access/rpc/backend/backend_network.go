package backend

import (
	"context"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
)

var SnapshotHistoryLimitErr = fmt.Errorf("reached the snapshot history limit")

type backendNetwork struct {
	state                protocol.State
	chainID              flow.ChainID
	snapshotHistoryLimit int
}

/*
NetworkAPI func

The observer and access nodes need to be able to handle GetNetworkParameters
and GetLatestProtocolStateSnapshot RPCs so this logic was split into
the backendNetwork so that we can ignore the rest of the backend logic
*/
func NewNetworkAPI(state protocol.State, chainID flow.ChainID, snapshotHistoryLimit int) *backendNetwork {
	return &backendNetwork{
		state:                state,
		chainID:              chainID,
		snapshotHistoryLimit: snapshotHistoryLimit,
	}
}

func (b *backendNetwork) GetNetworkParameters(_ context.Context) access.NetworkParameters {
	return access.NetworkParameters{
		ChainID: b.chainID,
	}
}

// GetLatestProtocolStateSnapshot returns the latest finalized snapshot
func (b *backendNetwork) GetLatestProtocolStateSnapshot(_ context.Context) ([]byte, error) {
	snapshot := b.state.Final()

	validSnapshot, err := b.getValidSnapshot(snapshot, 0)
	if err != nil {
		return nil, err
	}

	data, err := convert.SnapshotToBytes(validSnapshot)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to convert snapshot to bytes: %v", err)
	}

	return data, nil
}

func (b *backendNetwork) isEpochOrPhaseDifferent(counter1, counter2 uint64, phase1, phase2 flow.EpochPhase) bool {
	return counter1 != counter2 || phase1 != phase2
}

// getValidSnapshot will return a valid snapshot that has a sealing segment which
// 1. does not contain any blocks that span an epoch transition
// 2. does not contain any blocks that span an epoch phase transition
// If a snapshot does contain an invalid sealing segment query the state
// by height of each block in the segment and return a snapshot at the point
// where the transition happens.
func (b *backendNetwork) getValidSnapshot(snapshot protocol.Snapshot, blocksVisited int) (protocol.Snapshot, error) {
	segment, err := snapshot.SealingSegment()
	if err != nil {
		return nil, fmt.Errorf("failed to get sealing segment: %w", err)
	}

	counterAtHighest, phaseAtHighest, err := b.getCounterAndPhase(segment.Highest().Header.Height)
	if err != nil {
		return nil, fmt.Errorf("failed to get counter and phase at highest block in the segment: %w", err)
	}

	counterAtLowest, phaseAtLowest, err := b.getCounterAndPhase(segment.Sealed().Header.Height)
	if err != nil {
		return nil, fmt.Errorf("failed to get counter and phase at lowest block in the segment: %w", err)
	}

	// Check if the counters and phase are different this indicates that the sealing segment
	// of the snapshot requested spans either an epoch transition or phase transition.
	if b.isEpochOrPhaseDifferent(counterAtHighest, counterAtLowest, phaseAtHighest, phaseAtLowest) {
		// Visit each node in strict order of decreasing height starting at head
		// to find the block that straddles the transition boundary.
		for i := len(segment.Blocks) - 1; i >= 0; i-- {
			blocksVisited++

			// NOTE: Check if we have reached our history limit, in edge cases
			// where the sealing segment is abnormally long we want to short circuit
			// the recursive calls and return an error. The API caller can retry.
			if blocksVisited > b.snapshotHistoryLimit {
				return nil, fmt.Errorf("%w: (%d)", SnapshotHistoryLimitErr, b.snapshotHistoryLimit)
			}

			counterAtBlock, phaseAtBlock, err := b.getCounterAndPhase(segment.Blocks[i].Header.Height)
			if err != nil {
				return nil, fmt.Errorf("failed to get epoch counter and phase for snapshot at block %s: %w", segment.Blocks[i].ID(), err)
			}

			// Check if this block straddles the transition boundary, if it does return the snapshot
			// at that block height.
			if b.isEpochOrPhaseDifferent(counterAtHighest, counterAtBlock, phaseAtHighest, phaseAtBlock) {
				return b.getValidSnapshot(b.state.AtHeight(segment.Blocks[i].Header.Height), blocksVisited)
			}
		}
	}

	return snapshot, nil
}

// getCounterAndPhase will return the epoch counter and phase at the specified height in state
func (b *backendNetwork) getCounterAndPhase(height uint64) (uint64, flow.EpochPhase, error) {
	snapshot := b.state.AtHeight(height)

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
