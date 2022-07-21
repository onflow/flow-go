package complete

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"golang.org/x/sync/semaphore"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
	realWAL "github.com/onflow/flow-go/ledger/complete/wal"
	"github.com/onflow/flow-go/module/lifecycle"
	"github.com/onflow/flow-go/module/observable"
)

type WALTrieUpdate struct {
	Update   *ledger.TrieUpdate
	ResultCh chan<- error
	DoneCh   <-chan struct{}
}

type checkpointResult struct {
	num int
	err error
}

type Compactor struct {
	checkpointer       *realWAL.Checkpointer
	wal                *realWAL.DiskWAL
	ledger             *Ledger
	logger             zerolog.Logger
	stopCh             chan chan struct{}
	trieUpdateCh       <-chan *WALTrieUpdate
	trieUpdateDoneCh   chan<- struct{}
	lm                 *lifecycle.LifecycleManager
	observers          map[observable.Observer]struct{}
	checkpointDistance uint
	checkpointsToKeep  uint
}

func NewCompactor(l *Ledger, w *realWAL.DiskWAL, checkpointDistance uint, checkpointsToKeep uint, logger zerolog.Logger) (*Compactor, error) {
	if checkpointDistance < 1 {
		checkpointDistance = 1
	}

	checkpointer, err := w.NewCheckpointer()
	if err != nil {
		return nil, err
	}

	trieUpdateCh := l.TrieUpdateChan()
	if trieUpdateCh == nil {
		return nil, errors.New("failed to get valid trie update channel from ledger")
	}

	trieUpdateDoneCh := l.TrieUpdateDoneChan()
	if trieUpdateDoneCh == nil {
		return nil, errors.New("failed to get valid trie update done channel from ledger")
	}

	return &Compactor{
		checkpointer:       checkpointer,
		wal:                w,
		ledger:             l,
		logger:             logger,
		stopCh:             make(chan chan struct{}),
		trieUpdateCh:       trieUpdateCh,
		trieUpdateDoneCh:   l.trieUpdateDoneCh,
		observers:          make(map[observable.Observer]struct{}),
		lm:                 lifecycle.NewLifecycleManager(),
		checkpointDistance: checkpointDistance,
		checkpointsToKeep:  checkpointsToKeep,
	}, nil
}

func (c *Compactor) Subscribe(observer observable.Observer) {
	var void struct{}
	c.observers[observer] = void
}

func (c *Compactor) Unsubscribe(observer observable.Observer) {
	delete(c.observers, observer)
}

// Ready periodically fires Run function, every `interval`
func (c *Compactor) Ready() <-chan struct{} {
	c.lm.OnStart(func() {
		go c.run()
	})
	return c.lm.Started()
}

func (c *Compactor) Done() <-chan struct{} {
	c.lm.OnStop(func() {
		// Notify observers
		for observer := range c.observers {
			observer.OnComplete()
		}

		// Signal Compactor goroutine to stop
		doneCh := make(chan struct{})
		c.stopCh <- doneCh

		// Wait for Compactor goroutine to stop
		<-doneCh

		// Close trieUpdateDoneCh to signal trie updates are finished
		close(c.trieUpdateDoneCh)
	})
	return c.lm.Stopped()
}

func (c *Compactor) run() {

	// checkpointSem is used to limit checkpointing to one.
	// If previous checkpointing isn't finished when enough segments
	// are finalized for next checkpointing, retry checkpointing
	// again when next segment is finalized.
	// This avoids having more tries in memory than needed.
	checkpointSem := semaphore.NewWeighted(1)

	checkpointResultCh := make(chan checkpointResult, 1)

	// Get active segment number.
	// activeSegmentNum is updated when record is written to a new segment.
	_, activeSegmentNum, err := c.wal.Segments()
	if err != nil {
		c.logger.Error().Err(err).Msg("compactor failed to get active segment number")
		activeSegmentNum = -1
	}

	lastCheckpointNum, err := c.checkpointer.LatestCheckpoint()
	if err != nil {
		c.logger.Error().Err(err).Msg("compactor failed to get last checkpoint number")
		lastCheckpointNum = -1
	}

	// Compute next checkpoint number.
	// nextCheckpointNum is updated when
	// - checkpointing starts, fails to start, or fails.
	// - tries snapshot fails.
	// NOTE: next checkpoint number must >= active segment num.
	// We can't reuse mtrie state to checkpoint tries in older segments.
	nextCheckpointNum := lastCheckpointNum + int(c.checkpointDistance)
	if activeSegmentNum > nextCheckpointNum {
		nextCheckpointNum = activeSegmentNum
	}

	ctx, cancel := context.WithCancel(context.Background())

Loop:
	for {
		select {

		case doneCh := <-c.stopCh:
			defer close(doneCh)
			cancel()
			break Loop

		case checkpointResult := <-checkpointResultCh:
			if checkpointResult.err != nil {
				c.logger.Error().Err(checkpointResult.err).Msgf(
					"compactor failed to checkpoint %d", checkpointResult.num,
				)

				// Retry checkpointing after active segment is finalized.
				nextCheckpointNum = activeSegmentNum
			}

		case update, ok := <-c.trieUpdateCh:
			if !ok {
				// trieUpdateCh channel is closed.
				// Wait for stop signal from c.stopCh
				continue
			}

			var checkpointNum int
			var checkpointTries []*trie.MTrie
			activeSegmentNum, checkpointNum, checkpointTries =
				c.processTrieUpdate(update, activeSegmentNum, nextCheckpointNum)

			if checkpointTries == nil {
				// Don't checkpoint yet because
				// - not enough segments for checkpointing (nextCheckpointNum >= activeSegmentNum), or
				// - failed to get ledger state snapshop (nextCheckpointNum < activeSegmentNum)
				if nextCheckpointNum < activeSegmentNum {
					nextCheckpointNum = activeSegmentNum
				}
				continue
			}

			// Try to checkpoint
			if checkpointSem.TryAcquire(1) {

				// Compute next checkpoint number
				nextCheckpointNum = checkpointNum + int(c.checkpointDistance)

				go func() {
					defer checkpointSem.Release(1)
					err := c.checkpoint(ctx, checkpointTries, checkpointNum)
					checkpointResultCh <- checkpointResult{checkpointNum, err}
				}()
			} else {
				// Failed to get semaphore because checkpointing is running.
				// Try again when active segment is finalized.
				c.logger.Info().Msgf("compactor delayed checkpoint %d because prior checkpointing is ongoing", nextCheckpointNum)
				nextCheckpointNum = activeSegmentNum
			}
		}
	}

	// Drain and process remaining trie updates in channel.
	for update := range c.trieUpdateCh {
		_, _, err := c.wal.RecordUpdate(update.Update)
		select {
		case update.ResultCh <- err:
		default:
		}
	}

	// Don't wait for checkpointing to finish because it might take too long.
}

// processUpdateResult sends WAL update result using ResultCh channel
// and waits for signal from DoneCh channel.
// This ensures that WAL update and ledger state update are in sync.
func processUpdateResult(update *WALTrieUpdate, updateResult error) {
	// Send result of WAL update
	update.ResultCh <- updateResult

	// Wait for trie update to complete
	<-update.DoneCh
}

func (c *Compactor) checkpoint(ctx context.Context, tries []*trie.MTrie, checkpointNum int) error {

	err := createCheckpoint(c.checkpointer, c.logger, tries, checkpointNum)
	if err != nil {
		return fmt.Errorf("cannot create checkpoints: %w", err)
	}

	// Return if context is canceled.
	select {
	case <-ctx.Done():
		return nil
	default:
	}

	err = cleanupCheckpoints(c.checkpointer, int(c.checkpointsToKeep))
	if err != nil {
		return fmt.Errorf("cannot cleanup checkpoints: %w", err)
	}

	if checkpointNum > 0 {
		for observer := range c.observers {
			// Don't notify observer if context is canceled.
			// observer.OnComplete() is called when Compactor starts shutting down,
			// which may close channel that observer.OnNext() uses to send data.
			select {
			case <-ctx.Done():
				return nil
			default:
				observer.OnNext(checkpointNum)
			}
		}
	}

	return nil
}

func createCheckpoint(checkpointer *realWAL.Checkpointer, logger zerolog.Logger, tries []*trie.MTrie, checkpointNum int) error {

	logger.Info().Msgf("serializing checkpoint %d", checkpointNum)

	startTime := time.Now()

	writer, err := checkpointer.CheckpointWriter(checkpointNum)
	if err != nil {
		return fmt.Errorf("cannot generate checkpoint writer: %w", err)
	}
	defer func() {
		closeErr := writer.Close()
		// Return close error if there isn't any prior error to return.
		if err == nil {
			err = closeErr
		}
	}()

	err = realWAL.StoreCheckpoint(writer, tries...)
	if err != nil {
		return fmt.Errorf("error serializing checkpoint (%d): %w", checkpointNum, err)
	}

	duration := time.Since(startTime)
	logger.Info().Float64("total_time_s", duration.Seconds()).Msgf("created checkpoint %d", checkpointNum)

	return nil
}

func cleanupCheckpoints(checkpointer *realWAL.Checkpointer, checkpointsToKeep int) error {
	// Don't list checkpoints if we keep them all
	if checkpointsToKeep == 0 {
		return nil
	}
	checkpoints, err := checkpointer.Checkpoints()
	if err != nil {
		return fmt.Errorf("cannot list checkpoints: %w", err)
	}
	if len(checkpoints) > int(checkpointsToKeep) {
		// if condition guarantees this never fails
		checkpointsToRemove := checkpoints[:len(checkpoints)-int(checkpointsToKeep)]

		for _, checkpoint := range checkpointsToRemove {
			err := checkpointer.RemoveCheckpoint(checkpoint)
			if err != nil {
				return fmt.Errorf("cannot remove checkpoint %d: %w", checkpoint, err)
			}
		}
	}
	return nil
}

// processTrieUpdate writes trie update to WAL, updates activeSegmentNum,
// and takes snapshot of ledger state for checkpointing if needed.
// It also sends WAL update result and waits for trie update completion.
func (c *Compactor) processTrieUpdate(
	update *WALTrieUpdate,
	activeSegmentNum int,
	nextCheckpointNum int,
) (
	_activeSegmentNum int,
	checkpointNum int,
	checkpointTries []*trie.MTrie,
) {

	// RecordUpdate returns the segment number the record was written to.
	// Returned segment number (>= 0) can be
	// - the same as previous segment number (same segment), or
	// - incremented by 1 from previous segment number (new segment)
	segmentNum, skipped, updateErr := c.wal.RecordUpdate(update.Update)

	// processUpdateResult must be called to ensure that ledger state update isn't blocked.
	defer processUpdateResult(update, updateErr)

	if activeSegmentNum == -1 {
		// Recover from failure to get active segment number at initialization.
		return segmentNum, -1, nil
	}

	if updateErr != nil || skipped || segmentNum == activeSegmentNum {
		return activeSegmentNum, -1, nil
	}

	// In the remaining code: segmentNum > activeSegmentNum

	// active segment is finalized.

	// Check new segment number is incremented by 1
	if segmentNum != activeSegmentNum+1 {
		c.logger.Error().Msg(fmt.Sprintf("compactor got unexpected new segment numer %d, want %d", segmentNum, activeSegmentNum+1))
	}

	// Update activeSegmentNum
	prevSegmentNum := activeSegmentNum
	activeSegmentNum = segmentNum

	if nextCheckpointNum > prevSegmentNum {
		// Not enough segments for checkpointing
		return activeSegmentNum, -1, nil
	}

	// In the remaining code: nextCheckpointNum == prevSegmentNum

	// Enough segments are created for checkpointing

	// Get ledger snapshot before sending WAL update result.
	// At this point, ledger snapshot contains tries up to
	// last update (logged as last record in finalized segment)
	// Ledger doesn't include new trie for this update
	// until WAL result is sent back.
	tries, err := c.ledger.Tries()
	if err != nil {
		c.logger.Error().Err(err).Msg("compactor failed to get ledger tries")
		return activeSegmentNum, -1, nil
	}

	checkpointNum = nextCheckpointNum

	return activeSegmentNum, checkpointNum, tries
}
