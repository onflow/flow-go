package state_synchronization

import (
	"github.com/ipfs/go-cid"
	"github.com/onflow/flow-go/model/flow"
)

type StatusTracker interface {
	// StartTransfer tracks the start of the Execution Data transfer.
	StartTransfer() error

	// TrackBlobs tracks the given cids as part of the Execution Data.
	TrackBlobs(cids []cid.Cid) error

	// FinishTransfer marks the transfer of the Execution Data as complete, triggers incorporation
	// if possible starting from the current height, and returns the resulting latest incorporated height.
	FinishTransfer() (latestIncorporatedHeight uint64, err error)
}

type StatusTrackerFactory interface {
	// GetStatusTracker returns a status tracker which can be used to track the Execution Data download progress for the given block.
	GetStatusTracker(blockID flow.Identifier, blockHeight uint64, executionDataID flow.Identifier) StatusTracker
}

type NoopStatusTracker struct{}

func (*NoopStatusTracker) StartTransfer() error {
	return nil
}

func (*NoopStatusTracker) TrackBlobs(cids []cid.Cid) error {
	return nil
}

func (*NoopStatusTracker) FinishTransfer() (uint64, error) {
	return 0, nil
}

type NoopStatusTrackerFactory struct{}

func (*NoopStatusTrackerFactory) GetStatusTracker(blockID flow.Identifier, blockHeight uint64, executionDataID flow.Identifier) StatusTracker {
	return &NoopStatusTracker{}
}
