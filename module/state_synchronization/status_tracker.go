package state_synchronization

import (
	"github.com/ipfs/go-cid"
	"github.com/onflow/flow-go/model/flow"
)

type StatusTracker interface {
	StartTransfer() error
	TrackBlobs(cids []cid.Cid) error
	FinishTransfer() (latestIncorporatedHeight uint64, err error)
}

type StatusTrackerFactory interface {
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
