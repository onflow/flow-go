package complete

import (
	"context"
	"errors"
	"fmt"

	"github.com/rs/zerolog"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/complete/common"
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
	wal                                  realWAL.LedgerWAL
	logger                               zerolog.Logger
	trieQueue                            *common.TrieQueue
	trigger                              *Trigger // determine when to trigger checkpointing
	lm                                   *lifecycle.LifecycleManager
	observers                            map[observable.Observer]struct{}
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
	trieQueue := common.NewTrieQueueWithValues(checkpointCapacity, tries)
	trigger, err := createTrigger(logger, w, checkpointsToKeep, checkpointDistance)
	if err != nil {
		return nil, fmt.Errorf("could not create trigger: %w", err)
	}

	return &Compactor{
		wal:                                  w,
		trieQueue:                            trieQueue,
		trigger:                              trigger,
		logger:                               logger,
		stopCh:                               make(chan chan struct{}),
		trieUpdateCh:                         trieUpdateCh,
		observers:                            make(map[observable.Observer]struct{}),
		lm:                                   lifecycle.NewLifecycleManager(),
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

func createTrigger(logger zerolog.Logger, wal realWAL.LedgerWAL, checkpointsToKeep uint, checkpointDistance uint) (*Trigger, error) {
	checkpointer, err := wal.NewCheckpointer()
	if err != nil {
		return nil, err
	}

	// Get active segment number (opened segment that new records write to).
	// activeSegmentNum is updated when record is written to a new segment.
	_, activeSegmentNum, err := wal.Segments()
	if err != nil {
		return nil, fmt.Errorf("compactor failed to get active segment number: %w", err)
	}

	lastCheckpointNum, err := checkpointer.LatestCheckpoint()
	if err != nil {
		return nil, fmt.Errorf("compactor failed to get last checkpoint number: %w", err)
	}

	runner := NewRunner(logger, checkpointer, checkpointsToKeep)
	trigger := NewTrigger(logger, activeSegmentNum, lastCheckpointNum, checkpointDistance, nil, runner)
	return trigger, nil
}

// notify all observers about a new checkpoint file has been generated
func (c *Compactor) notifyObservors(ctx context.Context, checkpointNum int) {
	for observer := range c.observers {
		// Don't notify observer if context is canceled.
		// observer.OnComplete() is called when Compactor starts shutting down,
		// which may close channel that observer.OnNext() uses to send data.
		select {
		case <-ctx.Done():
			return
		default:
			observer.OnNext(checkpointNum)
		}
	}
}

// run writes WAL records from trie updates and starts checkpointing
// asynchronously when enough segments are finalized.
func (c *Compactor) run() {

	ctx, cancel := context.WithCancel(context.Background())

	// when checkpoint is completed, notify observors
	c.trigger.WithCallback(func(checkpointNum int) {
		c.notifyObservors(ctx, checkpointNum)
	})

Loop:
	for {
		select {

		case doneCh := <-c.stopCh:
			defer close(doneCh)
			cancel()
			break Loop

		case update, ok := <-c.trieUpdateCh:
			if !ok {
				// trieUpdateCh channel is closed.
				// Wait for stop signal from c.stopCh
				continue
			}

			c.processTrieUpdate(ctx, update, c.trieQueue)
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

// processTrieUpdate writes trie update to WAL, updates activeSegmentNum,
// and returns tries for checkpointing if needed.
// It sends WAL update result, receives updated trie, and pushes updated trie to trieQueue.
// When this function returns, WAL update is in sync with trieQueue update.
func (c *Compactor) processTrieUpdate(
	ctx context.Context,
	update *WALTrieUpdate,
	trieQueue *common.TrieQueue,
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

	if skipped {
		return
	}

	// notify the checkpoint trigger about the segment num the trie update is written to,
	// trigger is a stateful module that knows the segment num of the past trie updates were
	// written to, therefore, it has everything needed to when to trigger and how to run checkpoint.
	// in case checkpointing was running into error, it logs the error.
	err := c.trigger.NotifyTrieUpdateWrittenToWAL(ctx, segmentNum, trieQueue)
	if err != nil {
		c.logger.Error().Err(err).Msgf("could not notify checkpoint trigger for trie updates in segment num %v", segmentNum)
	}
}
