package complete

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"go.uber.org/atomic"
	"golang.org/x/sync/semaphore"

	"github.com/onflow/flow-go/ledger/complete/payloadless"
	realWAL "github.com/onflow/flow-go/ledger/complete/wal"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/lifecycle"
	"github.com/onflow/flow-go/module/observable"
)

// PayloadlessCompactor is the payloadless-mode counterpart of [Compactor]. It
// shares the same disk WAL with the full-mtrie compactor (both write the same
// [ledger.TrieUpdate] wire format) and produces V7 checkpoints at the configured
// cadence.
//
// Responsibilities:
//   - drain [WALPayloadlessTrieUpdate] from the ledger's trie-update channel
//   - record each update to the shared WAL via [realWAL.LedgerWAL.RecordUpdate]
//   - track an in-memory queue of recent payloadless tries
//   - periodically snapshot the queue into a V7 checkpoint via
//     [realWAL.StoreCheckpointV7SingleThread]
//   - prune older V7 checkpoints per the [CheckpointsToKeep] policy
//   - honor an external [triggerCheckpointOnNextSegmentFinish] flag for manual
//     checkpointing on the next segment boundary
//
// The implementation deliberately mirrors [Compactor] so reasoning about one
// transfers to the other.
type PayloadlessCompactor struct {
	checkpointer                         *realWAL.Checkpointer
	wal                                  realWAL.LedgerWAL
	trieQueue                            *realWAL.PayloadlessTrieQueue
	logger                               zerolog.Logger
	lm                                   *lifecycle.LifecycleManager
	observers                            map[observable.Observer]struct{}
	checkpointDistance                   uint
	checkpointsToKeep                    uint
	stopCh                               chan chan struct{}
	trieUpdateCh                         <-chan *WALPayloadlessTrieUpdate
	triggerCheckpointOnNextSegmentFinish *atomic.Bool
	metrics                              module.WALMetrics
}

// NewPayloadlessCompactor wires a [PayloadlessLedger] to a shared [LedgerWAL]
// for payloadless checkpoint generation. The ledger must have been constructed
// with a non-nil WAL so that [PayloadlessLedger.TrieUpdateChan] returns a
// non-nil channel — otherwise the compactor has no source of updates.
//
// All returned errors indicate that the compactor can't be created and the
// caller should treat them as unrecoverable.
func NewPayloadlessCompactor(
	l *PayloadlessLedger,
	w realWAL.LedgerWAL,
	logger zerolog.Logger,
	checkpointCapacity uint,
	checkpointDistance uint,
	checkpointsToKeep uint,
	triggerCheckpointOnNextSegmentFinish *atomic.Bool,
	metrics module.WALMetrics,
) (*PayloadlessCompactor, error) {
	if checkpointDistance < 1 {
		checkpointDistance = 1
	}

	checkpointer, err := w.NewCheckpointer()
	if err != nil {
		return nil, err
	}

	trieUpdateCh := l.TrieUpdateChan()
	if trieUpdateCh == nil {
		return nil, errors.New("failed to get valid trie update channel from payloadless ledger; ledger must be constructed with a WAL")
	}

	tries, err := l.Tries()
	if err != nil {
		return nil, fmt.Errorf("failed to read payloadless ledger tries: %w", err)
	}

	trieQueue := realWAL.NewPayloadlessTrieQueueWithValues(checkpointCapacity, tries)

	return &PayloadlessCompactor{
		checkpointer:                         checkpointer,
		wal:                                  w,
		trieQueue:                            trieQueue,
		logger:                               logger.With().Str("ledger_mod", "payloadless-compactor").Logger(),
		stopCh:                               make(chan chan struct{}),
		trieUpdateCh:                         trieUpdateCh,
		observers:                            make(map[observable.Observer]struct{}),
		lm:                                   lifecycle.NewLifecycleManager(),
		checkpointDistance:                   checkpointDistance,
		checkpointsToKeep:                    checkpointsToKeep,
		triggerCheckpointOnNextSegmentFinish: triggerCheckpointOnNextSegmentFinish,
		metrics:                              metrics,
	}, nil
}

// Subscribe registers an observer for checkpoint-completion notifications.
func (c *PayloadlessCompactor) Subscribe(observer observable.Observer) {
	var void struct{}
	c.observers[observer] = void
}

// Unsubscribe removes a previously-registered observer.
func (c *PayloadlessCompactor) Unsubscribe(observer observable.Observer) {
	delete(c.observers, observer)
}

// Ready starts the compactor goroutine.
func (c *PayloadlessCompactor) Ready() <-chan struct{} {
	c.lm.OnStart(func() {
		go c.run()
	})
	return c.lm.Started()
}

// Done stops the compactor goroutine and waits for the WAL to shut down.
func (c *PayloadlessCompactor) Done() <-chan struct{} {
	c.lm.OnStop(func() {
		doneCh := make(chan struct{})
		c.stopCh <- doneCh
		<-doneCh

		// Shut down WAL only after compactor has stopped so no further writes
		// race the WAL close.
		<-c.wal.Done()

		for observer := range c.observers {
			observer.OnComplete()
		}
	})
	return c.lm.Stopped()
}

// run is the main goroutine. It mirrors [Compactor.run]: drain updates,
// write to WAL, drive V7 checkpointing on segment boundaries.
func (c *PayloadlessCompactor) run() {
	checkpointSem := semaphore.NewWeighted(1)
	checkpointResultCh := make(chan checkpointResult, 1)

	_, activeSegmentNum, err := c.wal.Segments()
	if err != nil {
		c.logger.Error().Err(err).Msg("payloadless compactor failed to get active segment number")
		activeSegmentNum = -1
	}

	lastCheckpointNum := latestV7CheckpointNum(c.checkpointer, c.logger)
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

		case res := <-checkpointResultCh:
			if res.err != nil {
				c.logger.Error().Err(res.err).Msg(
					"payloadless compactor failed to create or remove checkpoint",
				)
				var createError *createCheckpointError
				if errors.As(res.err, &createError) {
					nextCheckpointNum = activeSegmentNum
				}
			}

		case update, ok := <-c.trieUpdateCh:
			if !ok {
				continue
			}

			// Manual trigger handling identical to V6.
			if c.triggerCheckpointOnNextSegmentFinish.CompareAndSwap(true, false) {
				if nextCheckpointNum >= activeSegmentNum {
					original := nextCheckpointNum
					nextCheckpointNum = activeSegmentNum
					c.logger.Info().Msgf("payloadless compactor will trigger once finish writing segment %v, originalNextCheckpointNum: %v", nextCheckpointNum, original)
				} else {
					c.logger.Warn().Msgf("could not force triggering checkpoint, nextCheckpointNum %v < activeSegmentNum %v", nextCheckpointNum, activeSegmentNum)
				}
			}

			var checkpointNum int
			var checkpointTries []*payloadless.MTrie
			activeSegmentNum, checkpointNum, checkpointTries =
				c.processTrieUpdate(update, c.trieQueue, activeSegmentNum, nextCheckpointNum)

			if checkpointTries == nil {
				continue
			}

			if checkpointSem.TryAcquire(1) {
				nextCheckpointNum = checkpointNum + int(c.checkpointDistance)
				go func() {
					defer checkpointSem.Release(1)
					err := c.checkpoint(ctx, checkpointTries, checkpointNum)
					checkpointResultCh <- checkpointResult{checkpointNum, err}
				}()
			} else {
				c.logger.Info().Msgf("payloadless compactor delayed checkpoint %d because prior checkpointing is ongoing", nextCheckpointNum)
				nextCheckpointNum = activeSegmentNum
			}
		}
	}

	// Drain remaining trie updates on shutdown so callers don't block on
	// ResultCh forever. We still record updates to the WAL.
	c.logger.Info().Msg("payloadless compactor draining trie update channel on shutdown")
	for update := range c.trieUpdateCh {
		_, _, err := c.wal.RecordUpdate(update.Update)
		select {
		case update.ResultCh <- err:
		default:
		}
	}
	c.logger.Info().Msg("payloadless compactor finished draining trie update channel")

	if !checkpointSem.TryAcquire(1) {
		select {
		case <-checkpointResultCh:
		case <-time.After(10 * time.Millisecond):
		}
	}
}

// processTrieUpdate writes the WAL record, tracks the active segment, hands
// the newly-built trie to the queue, and signals when enough segments have
// rolled over to checkpoint. Mirrors [Compactor.processTrieUpdate].
func (c *PayloadlessCompactor) processTrieUpdate(
	update *WALPayloadlessTrieUpdate,
	trieQueue *realWAL.PayloadlessTrieQueue,
	activeSegmentNum int,
	nextCheckpointNum int,
) (_activeSegmentNum int, checkpointNum int, checkpointTries []*payloadless.MTrie) {

	segmentNum, skipped, updateErr := c.wal.RecordUpdate(update.Update)
	update.ResultCh <- updateErr

	defer func() {
		// Receive the freshly-built trie from the ledger goroutine and stage it.
		trie := <-update.TrieCh
		if trie == nil {
			c.logger.Error().Msg("payloadless compactor failed to get updated trie")
			return
		}
		trieQueue.Push(trie)
	}()

	if activeSegmentNum == -1 {
		return segmentNum, -1, nil
	}

	if updateErr != nil || skipped || segmentNum == activeSegmentNum {
		return activeSegmentNum, -1, nil
	}

	// segmentNum > activeSegmentNum — a segment just rolled over.

	if segmentNum != activeSegmentNum+1 {
		c.logger.Error().Msgf("payloadless compactor got unexpected new segment %d, want %d", segmentNum, activeSegmentNum+1)
	}

	prevSegmentNum := activeSegmentNum
	activeSegmentNum = segmentNum

	c.logger.Info().Msgf("finish writing segment file %v, payloadless trie update writing to segment %v; checkpoint triggers at segment %v",
		prevSegmentNum, activeSegmentNum, nextCheckpointNum)

	if nextCheckpointNum > prevSegmentNum {
		return activeSegmentNum, -1, nil
	}

	// nextCheckpointNum == prevSegmentNum — enough segments accumulated.
	tries := trieQueue.Tries()
	return activeSegmentNum, nextCheckpointNum, tries
}

// checkpoint serializes a V7 checkpoint, then prunes older V7 files per the
// retention policy, and notifies observers.
func (c *PayloadlessCompactor) checkpoint(ctx context.Context, tries []*payloadless.MTrie, checkpointNum int) error {
	if err := createPayloadlessCheckpoint(c.checkpointer, c.logger, tries, checkpointNum, c.metrics); err != nil {
		return &createCheckpointError{num: checkpointNum, err: err}
	}

	select {
	case <-ctx.Done():
		return nil
	default:
	}

	if err := cleanupCheckpointsV7(c.checkpointer, int(c.checkpointsToKeep)); err != nil {
		return &removeCheckpointError{err: err}
	}

	if checkpointNum > 0 {
		for observer := range c.observers {
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

// createPayloadlessCheckpoint writes a V7 checkpoint to the checkpointer's directory.
func createPayloadlessCheckpoint(
	checkpointer *realWAL.Checkpointer,
	logger zerolog.Logger,
	tries []*payloadless.MTrie,
	checkpointNum int,
	metrics module.WALMetrics,
) error {
	logger.Info().Msgf("serializing V7 checkpoint %d with %d tries", checkpointNum, len(tries))

	startTime := time.Now()
	fileName := realWAL.NumberToFilenameV7(checkpointNum)
	if err := realWAL.StoreCheckpointV7SingleThread(tries, checkpointer.Dir(), fileName, logger); err != nil {
		return fmt.Errorf("error serializing V7 checkpoint (%d): %w", checkpointNum, err)
	}

	size, err := realWAL.ReadCheckpointFileSize(checkpointer.Dir(), fileName)
	if err != nil {
		return fmt.Errorf("error reading V7 checkpoint file size (%d): %w", checkpointNum, err)
	}
	metrics.ExecutionCheckpointSize(size)

	logger.Info().
		Float64("total_time_s", time.Since(startTime).Seconds()).
		Msgf("created V7 checkpoint %d", checkpointNum)
	return nil
}

// cleanupCheckpointsV7 removes V7 checkpoints in excess of the
// keep-count, oldest first. V6 files in the same directory are untouched.
func cleanupCheckpointsV7(checkpointer *realWAL.Checkpointer, checkpointsToKeep int) error {
	if checkpointsToKeep == 0 {
		return nil
	}
	checkpoints, err := checkpointer.CheckpointsV7()
	if err != nil {
		return fmt.Errorf("cannot list V7 checkpoints: %w", err)
	}
	if len(checkpoints) > checkpointsToKeep {
		toRemove := checkpoints[:len(checkpoints)-checkpointsToKeep]
		for _, cp := range toRemove {
			if err := checkpointer.RemoveCheckpointV7(cp); err != nil {
				return fmt.Errorf("cannot remove V7 checkpoint %d: %w", cp, err)
			}
		}
	}
	return nil
}

// latestV7CheckpointNum returns the highest V7 checkpoint number on disk,
// or -1 if none exist or listing fails (with the error logged).
func latestV7CheckpointNum(checkpointer *realWAL.Checkpointer, logger zerolog.Logger) int {
	checkpoints, err := checkpointer.CheckpointsV7()
	if err != nil {
		logger.Error().Err(err).Msg("payloadless compactor failed to list V7 checkpoints")
		return -1
	}
	if len(checkpoints) == 0 {
		return -1
	}
	return checkpoints[len(checkpoints)-1]
}
