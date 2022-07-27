package checkpoint

import (
	"context"
	"fmt"

	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
	"github.com/rs/zerolog"
	"go.uber.org/atomic"
)

type checkpointRunner interface {
	runCheckpoint(ctx context.Context, checkpointTries []*trie.MTrie, checkpointNum int) error
}

type TrieQueue interface {
	Tries() []*trie.MTrie
}

type Trigger struct {
	logger                  zerolog.Logger
	activeSegmentNum        int                     // the num of the segment that the previous trie update was written to, is used to detect whether trie update is written to a new segment.
	checkpointedSegumentNum *atomic.Int32           // the highest segment num that has been checkpointed
	checkpointDistance      uint                    // the total number of segment (write-ahead log) to trigger checkpointing
	isRunning               *atomic.Bool            // an atomic.Bool for whether checkpointing is currently running.
	runner                  checkpointRunner        // for running checkpointing
	checkpointed            func(checkpointNum int) // to report checkpointing completion
}

// NotifyTrieUpdateWrittenToWAL takes segmentNum that a trieUpdate is written to, and a trieQueue to
// retrieve the latest tries WITHOUT the trie from this trieUpdate, and it checks whether to trigger
// checkpointing. Run checkpointing if should trigger.
// Assumptions:
// 1. this method is not concurrent-safe, must be called sequentially
// 2. the latestTries must contain all the trie nodes up to the trie update without the trie from this trie update.
func (t *Trigger) NotifyTrieUpdateWrittenToWAL(ctx context.Context, segmentNum int, latestTries TrieQueue) error {
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
		return nil
	}

	// ensure checkpointing only run once at any time
	if !t.isRunning.CAS(false, true) {
		return nil
	}

	// only one proccess would enter here at any time

	checkpointNum := prevSegmentNum // create a checkpoint upto the previous segment num on disk
	tries := latestTries.Tries()    // tries should not include the trie

	// running checkpointing in background
	go func() {
		err := t.runner.runCheckpoint(ctx, tries, checkpointNum)
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
	segmentNumToTriggerCheckpoint := int(t.checkpointedSegumentNum.Load()) + int(t.checkpointDistance)
	return segmentNum >= segmentNumToTriggerCheckpoint
}
