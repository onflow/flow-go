package complete

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/ledger/complete/common"
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
)

// CheckpointRunner abstraction for the runner, so that easy to test Trigger
type CheckpointRunner interface {
	RunCheckpoint(ctx context.Context, checkpointTries []*trie.MTrie, checkpointNum int) error
}

// Trigger takes checkpointing related configs and decides when to trigger checkpointing,
// if triggered, it will use the checkpointRunner to create checkpoint.
// It ensures only one checkpoint is being created at any time.
type Trigger struct {
	logger                               zerolog.Logger
	activeSegmentNum                     int                     // the num of the segment that the previous trie update was written to, is used to detect whether trie update is written to a new segment.
	checkpointedSegumentNum              *atomic.Int32           // the highest segment num that has been checkpointed
	checkpointDistance                   uint                    // the total number of segment (write-ahead log) to trigger checkpointing
	isRunning                            *atomic.Bool            // an atomic.Bool for whether checkpointing is currently running.
	runner                               CheckpointRunner        // for running checkpointing
	checkpointed                         func(checkpointNum int) // to report checkpointing completion
	triggerCheckpointOnNextSegmentFinish *atomic.Bool            // to trigger checkpoint manually
}

func NewTrigger(
	logger zerolog.Logger,
	activeSegmentNum int,
	checkpointedSegumentNum int,
	checkpointDistance uint,
	runner CheckpointRunner,
	checkpointed func(checkpointNum int),
	triggerCheckpointOnNextSegmentFinish *atomic.Bool,
) *Trigger {
	notNil := checkpointed
	if notNil == nil {
		notNil = func(int) {} // noop
	}
	return &Trigger{
		logger:                               logger,
		activeSegmentNum:                     activeSegmentNum,
		checkpointedSegumentNum:              atomic.NewInt32(int32(checkpointedSegumentNum)),
		checkpointDistance:                   checkpointDistance,
		isRunning:                            atomic.NewBool(false),
		runner:                               runner,
		checkpointed:                         notNil,
		triggerCheckpointOnNextSegmentFinish: triggerCheckpointOnNextSegmentFinish,
	}
}

// WithCallback takes callback to notify when checkpointing is completed.
func (t *Trigger) WithCallback(checkpointed func(checkpointNum int)) {
	t.checkpointed = checkpointed
}

// NotifyTrieUpdateWrittenToWAL takes segmentNum that a trieUpdate is written to, and a trieQueue to
// retrieve the latest tries WITHOUT the trie from this trieUpdate, and it checks whether to trigger
// checkpointing. Run checkpointing if should trigger.
// Assumptions:
// 1. this method is not concurrent-safe, must be called sequentially
// 2. the latestTries must contain all the trie nodes up to the trie update without the trie from this trie update.
func (t *Trigger) NotifyTrieUpdateWrittenToWAL(ctx context.Context, segmentNum int, latestTries *common.TrieQueue) error {
	isWritingToNewSegmentFile, err := t.isWritingToNewSegmentFile(segmentNum)
	if err != nil {
		return fmt.Errorf("could not determine whether the trie update was written to a new segment file: %w", err)
	}

	// only trigger checkpointing when a segment file is completed, meaning no more trie update will be written to it
	// if the current trie update is still written to the current segment file, then bail
	if !isWritingToNewSegmentFile {
		return nil
	}

	// the previous segment file is no longer being written to, save the file num before updating
	// the activeSegmentNum
	prevSegmentNum := t.activeSegmentNum

	// update activeSegmentNum when we've learned trie updates are writing to a
	// new WAL with different segment num
	t.activeSegmentNum = segmentNum

	toTrigger := t.hasEnoughSegmentsToTriggerCheckpoint(prevSegmentNum)
	if !toTrigger {
		// if the admin tool asks to trigger checkpointing when next segment finish,
		// then trigger it, otherwise don't
		shouldTriggerOnSegmentFinish := t.triggerCheckpointOnNextSegmentFinish.CompareAndSwap(true, false)
		if !shouldTriggerOnSegmentFinish {
			return nil
		}
	}

	// ensure checkpointing only run once at any time
	if !t.isRunning.CompareAndSwap(false, true) {
		return nil
	}

	// only one proccess would enter here at any time

	checkpointNum := prevSegmentNum // create a checkpoint upto the previous segment num on disk
	tries := latestTries.Tries()    // tries should not include the trie

	// running checkpointing in background
	go func() {
		err := t.runner.RunCheckpoint(ctx, tries, checkpointNum)
		if err != nil {
			t.logger.Error().Err(err).Msgf("critical: fail to run checkpointing")
		}

		t.isRunning.Store(false)
		t.checkpointedSegumentNum.Store(int32(checkpointNum))

		// notify the caller that checkpointing has completed
		if checkpointNum > 0 {
			t.checkpointed(checkpointNum)
		}
	}()

	return nil
}

func (t *Trigger) isWritingToNewSegmentFile(segmentNum int) (bool, error) {
	// t.activeSegmentNum stores the segment num of the last trie update was written to.
	// if the segment num of the current trie update is the same one as t.activeSegmentNum,
	// it means it's still writing to the same segment file, and we don't want to trigger checkpoint
	if segmentNum == t.activeSegmentNum {
		return false, nil
	}

	// sanity check
	if segmentNum != t.activeSegmentNum+1 {
		return false, fmt.Errorf("compactor got unexpected new segment numer %d, want %d",
			segmentNum, t.activeSegmentNum+1)
	}

	return true, nil
}

// checkpointDistance defines how many segment files to trigger a checkpoint
func (t *Trigger) hasEnoughSegmentsToTriggerCheckpoint(segmentNum int) bool {
	last := t.checkpointedSegumentNum.Load()
	segmentNumToTriggerCheckpoint := int(last) + int(t.checkpointDistance)
	return segmentNum >= segmentNumToTriggerCheckpoint
}
