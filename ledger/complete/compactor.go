package complete

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"go.uber.org/atomic"
	"golang.org/x/sync/semaphore"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
	realWAL "github.com/onflow/flow-go/ledger/complete/wal"
	"github.com/onflow/flow-go/module/lifecycle"
	"github.com/onflow/flow-go/module/observable"
)

// WALTrieUpdate is a message communicated through channel between Ledger and Compactor.
type WALTrieUpdate struct {
	Update   *ledger.TrieUpdate // Update data needs to be encoded and saved in WAL.
	ResultCh chan<- error       // ResultCh channel is used to send WAL update result from Compactor to Ledger.
	TrieCh   <-chan *trie.MTrie // TrieCh channel is used to send new trie from Ledger to Compactor.
}

// checkpointResult is a message to communicate checkpointing number and error if any.
type checkpointResult struct {
	num int
	err error
}

// Compactor is a long-running goroutine responsible for:
// - writing WAL record from trie update,
// - starting checkpointing async when enough segments are finalized.
//
// Compactor communicates with Ledger through channels
// to ensure that by the end of any trie update processing,
// update is written to WAL and new trie is pushed to trie queue.
//
// Compactor stores pointers to tries in ledger state in a fix-sized
// checkpointing queue (FIFO).  Checkpointing queue is decoupled from
// main ledger state to allow separate optimization and looser coupling, etc.
// CAUTION: If the forest LRU Cache is used for main state,
// then ledger state and checkpointing queue may contain different tries.
// This will be resolved automaticaly after the forest LRU Cache
// (code outside checkpointing) is replaced by something like a FIFO queue.
type Compactor struct {
	checkpointer                         *realWAL.Checkpointer
	wal                                  realWAL.LedgerWAL
	trieQueue                            *realWAL.TrieQueue
	logger                               zerolog.Logger
	lm                                   *lifecycle.LifecycleManager
	observers                            map[observable.Observer]struct{}
	checkpointDistance                   uint
	checkpointsToKeep                    uint
	stopCh                               chan chan struct{}
	trieUpdateCh                         <-chan *WALTrieUpdate
	triggerCheckpointOnNextSegmentFinish *atomic.Bool // to trigger checkpoint manually
}

// NewCompactor creates new Compactor which writes WAL record and triggers
// checkpointing asynchronously when enough segments are finalized.
// The checkpointDistance is a flag that specifies how many segments need to
// be finalized to trigger checkpointing.  However, if a prior checkpointing
// is already running and not finished, then more segments than specified
// could be accumulated for the new checkpointing (to reduce memory).
// All returned errors indicate that Compactor can't be created.
// Since failure to create Compactor will end up blocking ledger updates,
// the caller should handle all returned errors as unrecoverable.
func NewCompactor(
	l *Ledger,
	w realWAL.LedgerWAL,
	logger zerolog.Logger,
	checkpointCapacity uint,
	checkpointDistance uint,
	checkpointsToKeep uint,
	triggerCheckpointOnNextSegmentFinish *atomic.Bool,
) (*Compactor, error) {
	if checkpointDistance < 1 {
		checkpointDistance = 1
	}

	checkpointer, err := w.NewCheckpointer()
	if err != nil {
		return nil, err
	}

	// Get trieUpdateCh channel to communicate trieUpdate, WAL result, and new trie
	// created from the update.
	trieUpdateCh := l.TrieUpdateChan()
	if trieUpdateCh == nil {
		return nil, errors.New("failed to get valid trie update channel from ledger")
	}

	// Get all tries from ledger state.
	tries, err := l.Tries()
	if err != nil {
		return nil, err
	}

	// Create trieQueue with initial values from ledger state.
	trieQueue := realWAL.NewTrieQueueWithValues(checkpointCapacity, tries)

	return &Compactor{
		checkpointer:                         checkpointer,
		wal:                                  w,
		trieQueue:                            trieQueue,
		logger:                               logger.With().Str("ledger_mod", "compactor").Logger(),
		stopCh:                               make(chan chan struct{}),
		trieUpdateCh:                         trieUpdateCh,
		observers:                            make(map[observable.Observer]struct{}),
		lm:                                   lifecycle.NewLifecycleManager(),
		checkpointDistance:                   checkpointDistance,
		checkpointsToKeep:                    checkpointsToKeep,
		triggerCheckpointOnNextSegmentFinish: triggerCheckpointOnNextSegmentFinish,
	}, nil
}

// Subscribe subscribes observer to Compactor.
func (c *Compactor) Subscribe(observer observable.Observer) {
	var void struct{}
	c.observers[observer] = void
}

// Unsubscribe unsubscribes observer to Compactor.
func (c *Compactor) Unsubscribe(observer observable.Observer) {
	delete(c.observers, observer)
}

// Ready returns channel which would be closed when Compactor goroutine starts.
func (c *Compactor) Ready() <-chan struct{} {
	c.lm.OnStart(func() {
		go c.run()
	})
	return c.lm.Started()
}

// Done returns channel which would be closed when Compactor goroutine exits.
func (c *Compactor) Done() <-chan struct{} {
	c.lm.OnStop(func() {
		// Signal Compactor goroutine to stop
		doneCh := make(chan struct{})
		c.stopCh <- doneCh

		// Wait for Compactor goroutine to stop
		<-doneCh

		// Shut down WAL component.
		// only shut down wal after compactor has been shut down, in case there
		// is still writing to WAL files.
		<-c.wal.Done()

		// Notify observers
		for observer := range c.observers {
			observer.OnComplete()
		}
	})
	return c.lm.Stopped()
}

// run writes WAL records from trie updates and starts checkpointing
// asynchronously when enough segments are finalized.
func (c *Compactor) run() {

	// checkpointSem is used to limit checkpointing to one.
	// If previous checkpointing isn't finished when enough segments
	// are finalized for next checkpointing, retry checkpointing
	// again when next segment is finalized.
	// This avoids having more tries in memory than needed.
	checkpointSem := semaphore.NewWeighted(1)

	checkpointResultCh := make(chan checkpointResult, 1)

	// Get active segment number (opened segment that new records write to).
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
	// nextCheckpointNum is updated when checkpointing starts, fails to start, or fails.
	// NOTE: next checkpoint number must >= active segment num.
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
				c.logger.Error().Err(checkpointResult.err).Msg(
					"compactor failed to create or remove checkpoint",
				)
				var createError *createCheckpointError
				if errors.As(checkpointResult.err, &createError) {
					// Retry checkpointing when active segment is finalized.
					nextCheckpointNum = activeSegmentNum
				}
			}

		case update, ok := <-c.trieUpdateCh:
			if !ok {
				// trieUpdateCh channel is closed.
				// Wait for stop signal from c.stopCh
				continue
			}

			// listen to signals from admin tool in order to trigger a checkpoint when the current segment file is finished
			if c.triggerCheckpointOnNextSegmentFinish.CompareAndSwap(true, false) {
				// sanity checking, usually the nextCheckpointNum is a segment number in the future that when the activeSegmentNum
				// finishes and reaches the nextCheckpointNum, then checkpoint will be triggered.
				if nextCheckpointNum >= activeSegmentNum {
					originalNextCheckpointNum := nextCheckpointNum
					nextCheckpointNum = activeSegmentNum
					c.logger.Info().Msgf("compactor will trigger once finish writing segment %v, originalNextCheckpointNum: %v", nextCheckpointNum, originalNextCheckpointNum)
				} else {
					c.logger.Warn().Msgf("could not force triggering checkpoint, nextCheckpointNum %v is smaller than activeSegmentNum %v", nextCheckpointNum, activeSegmentNum)
				}
			}

			var checkpointNum int
			var checkpointTries []*trie.MTrie
			activeSegmentNum, checkpointNum, checkpointTries =
				c.processTrieUpdate(update, c.trieQueue, activeSegmentNum, nextCheckpointNum)

			if checkpointTries == nil {
				// Not enough segments for checkpointing (nextCheckpointNum >= activeSegmentNum)
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
	c.logger.Info().Msg("Starting draining trie update channel in compactor on shutdown")
	for update := range c.trieUpdateCh {
		_, _, err := c.wal.RecordUpdate(update.Update)
		select {
		case update.ResultCh <- err:
		default:
		}
	}
	c.logger.Info().Msg("Finished draining trie update channel in compactor on shutdown")

	// Don't wait for checkpointing to finish because it might take too long.
}

// checkpoint creates checkpoint of tries snapshot,
// deletes prior checkpoint files (if needed), and notifies observers.
// Errors indicate that checkpoint file can't be created or prior checkpoints can't be removed.
// Caller should handle returned errors by retrying checkpointing when appropriate.
// Since this function is only for checkpointing, Compactor isn't affected by returned error.
func (c *Compactor) checkpoint(ctx context.Context, tries []*trie.MTrie, checkpointNum int) error {

	err := createCheckpoint(c.checkpointer, c.logger, tries, checkpointNum)
	if err != nil {
		return &createCheckpointError{num: checkpointNum, err: err}
	}

	// Return if context is canceled.
	select {
	case <-ctx.Done():
		return nil
	default:
	}

	err = cleanupCheckpoints(c.checkpointer, int(c.checkpointsToKeep))
	if err != nil {
		return &removeCheckpointError{err: err}
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

// createCheckpoint creates checkpoint with given checkpointNum and tries.
// Errors indicate that checkpoint file can't be created.
// Caller should handle returned errors by retrying checkpointing when appropriate.
func createCheckpoint(checkpointer *realWAL.Checkpointer, logger zerolog.Logger, tries []*trie.MTrie, checkpointNum int) error {

	logger.Info().Msgf("serializing checkpoint %d with %v tries", checkpointNum, len(tries))

	startTime := time.Now()

	fileName := realWAL.NumberToFilename(checkpointNum)
	err := realWAL.StoreCheckpointV6SingleThread(tries, checkpointer.Dir(), fileName, logger)
	if err != nil {
		return fmt.Errorf("error serializing checkpoint (%d): %w", checkpointNum, err)
	}

	duration := time.Since(startTime)
	logger.Info().Float64("total_time_s", duration.Seconds()).Msgf("created checkpoint %d", checkpointNum)

	return nil
}

// cleanupCheckpoints deletes prior checkpoint files if needed.
// Since the function is side-effect free, all failures are simply a no-op.
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
// and returns tries for checkpointing if needed.
// It sends WAL update result, receives updated trie, and pushes updated trie to trieQueue.
// When this function returns, WAL update is in sync with trieQueue update.
func (c *Compactor) processTrieUpdate(
	update *WALTrieUpdate,
	trieQueue *realWAL.TrieQueue,
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

	// Send result of WAL update
	update.ResultCh <- updateErr

	// This ensures that updated trie matches WAL update.
	defer func() {
		// Wait for updated trie
		trie := <-update.TrieCh
		if trie == nil {
			c.logger.Error().Msg("compactor failed to get updated trie")
			return
		}

		trieQueue.Push(trie)
	}()

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

	c.logger.Info().Msgf("finish writing segment file %v, trie update is writing to segment file %v, checkpoint will trigger when segment %v is finished",
		prevSegmentNum, activeSegmentNum, nextCheckpointNum)

	if nextCheckpointNum > prevSegmentNum {
		// Not enough segments for checkpointing
		return activeSegmentNum, -1, nil
	}

	// In the remaining code: nextCheckpointNum == prevSegmentNum

	// Enough segments are created for checkpointing

	// Get tries from checkpoint queue.
	// At this point, checkpoint queue contains tries up to
	// last update (last record in finalized segment)
	// It doesn't include trie for this update
	// until updated trie is received and added to trieQueue.
	tries := trieQueue.Tries()

	checkpointNum = nextCheckpointNum

	return activeSegmentNum, checkpointNum, tries
}

// createCheckpointError creates a checkpoint creation error.
type createCheckpointError struct {
	num int
	err error
}

func (e *createCheckpointError) Error() string {
	return fmt.Sprintf("cannot create checkpoint %d: %s", e.num, e.err)
}

func (e *createCheckpointError) Unwrap() error { return e.err }

// removeCheckpointError creates a checkpoint removal error.
type removeCheckpointError struct {
	err error
}

func (e *removeCheckpointError) Error() string {
	return fmt.Sprintf("cannot cleanup checkpoints: %s", e.err)
}

func (e *removeCheckpointError) Unwrap() error { return e.err }
