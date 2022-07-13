package complete

import (
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
	Update  *ledger.TrieUpdate
	Resultc chan<- error
}

type Compactor struct {
	checkpointer       *realWAL.Checkpointer
	wal                *realWAL.DiskWAL
	ledger             *Ledger
	logger             zerolog.Logger
	stopc              chan struct{}
	trieUpdatec        <-chan *WALTrieUpdate
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

	trieUpdatec := l.TrieUpdateChan()
	if trieUpdatec == nil {
		return nil, errors.New("failed to get valid trie update channel from ledger")
	}

	return &Compactor{
		checkpointer:       checkpointer,
		wal:                w,
		ledger:             l,
		logger:             logger,
		stopc:              make(chan struct{}),
		trieUpdatec:        trieUpdatec,
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
		go c.start()
	})
	return c.lm.Started()
}

func (c *Compactor) Done() <-chan struct{} {
	c.lm.OnStop(func() {
		for observer := range c.observers {
			observer.OnComplete()
		}
		c.stopc <- struct{}{}
	})
	return c.lm.Stopped()
}

func (c *Compactor) start() {

	// Use checkpointSem to limit checkpointing goroutine to one.
	// If checkpointing isn't finished, then wait before starting new checkpointing.
	checkpointSem := semaphore.NewWeighted(1)

	// Get active segment number
	// activeSegmentNum is updated when record is written to a new segment.
	_, activeSegmentNum, err := c.wal.Segments()
	if err != nil {
		// TODO: handle error
		c.logger.Error().Err(err).Msg("error getting active segment number")
	}

	lastCheckpointNum, err := c.checkpointer.LatestCheckpoint()
	if err != nil {
		// TODO: handle error
		c.logger.Error().Err(err).Msg("error getting last checkpoint number")
	}

	// Compute next checkpoint number
	// nextCheckpointNum is updated when checkpointing goroutine starts.
	// NOTE: next checkpoint number must be >= active segment num
	nextCheckpointNum := lastCheckpointNum + int(c.checkpointDistance)
	if activeSegmentNum > nextCheckpointNum {
		nextCheckpointNum = activeSegmentNum
	}

Loop:
	for {
		select {

		case <-c.stopc:
			break Loop

		case update := <-c.trieUpdatec:
			// RecordUpdate returns the segment number the record was written to.
			// Returned segment number can be
			// - the same as previous segment number (same segment), or
			// - incremented by 1 from previous segment number (new segment)
			segmentNum, skipped, err := c.wal.RecordUpdate(update.Update)

			if err != nil || skipped || segmentNum == activeSegmentNum {
				update.Resultc <- err
				continue
			}

			// Check new segment number is incremented by 1
			if segmentNum != activeSegmentNum+1 {
				// TODO: must handle error without panic before merging this code
				panic(fmt.Sprintf("expected new segment numer %d, got %d", activeSegmentNum+1, segmentNum))
			}

			// Update activeSegmentNum
			prevSegmentNum := activeSegmentNum
			activeSegmentNum = segmentNum

			if nextCheckpointNum > prevSegmentNum {
				// Not enough segments for checkpointing
				update.Resultc <- nil
				continue
			}

			// Enough segments are created for checkpointing

			// Get forest from ledger before sending WAL update result
			tries, err := c.ledger.Tries()
			if err != nil {
				// TODO: handle error
				c.logger.Error().Err(err).Msg("error getting ledger tries")
				// Try again after active segment is finalized.
				nextCheckpointNum = activeSegmentNum
				continue
			}

			// Send WAL update result after ledger state snapshot is done.
			update.Resultc <- nil

			// Try to checkpoint
			if checkpointSem.TryAcquire(1) {

				checkpointNum := nextCheckpointNum

				// Compute next checkpoint number
				nextCheckpointNum += int(c.checkpointDistance)

				go func() {
					defer checkpointSem.Release(1)

					err = c.checkpoint(tries, checkpointNum)
					if err != nil {
						c.logger.Error().Err(err).Msg("error checkpointing")
					}
					// TODO: retry if checkpointing fails.
				}()
			} else {
				// Failed to get semaphore because checkpointing is running.
				// Try again when active segment is finalized.
				c.logger.Info().Msgf("checkpoint %d is delayed because prior checkpointing is ongoing", nextCheckpointNum)
				nextCheckpointNum = activeSegmentNum
			}
		}
	}

	// Drain and record remaining trie update in the channel.
	for update := range c.trieUpdatec {
		_, _, err := c.wal.RecordUpdate(update.Update)
		update.Resultc <- err
	}

	// TODO: wait for checkpointing goroutine?
}

func (c *Compactor) checkpoint(tries []*trie.MTrie, checkpointNum int) error {

	err := createCheckpoint(c.checkpointer, c.logger, tries, checkpointNum)
	if err != nil {
		return fmt.Errorf("cannot create checkpoints: %w", err)
	}

	err = cleanupCheckpoints(c.checkpointer, int(c.checkpointsToKeep))
	if err != nil {
		return fmt.Errorf("cannot cleanup checkpoints: %w", err)
	}

	if checkpointNum > 0 {
		for observer := range c.observers {
			observer.OnNext(checkpointNum)
		}
	}

	return nil
}

func createCheckpoint(checkpointer *realWAL.Checkpointer, logger zerolog.Logger, tries []*trie.MTrie, checkpointNum int) error {

	logger.Info().Msgf("serializing checkpoint %d with %d tries", checkpointNum, len(tries))

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
	logger.Info().Float64("total_time_s", duration.Seconds()).Msgf("created checkpoint %d with %d tries", checkpointNum, len(tries))

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
