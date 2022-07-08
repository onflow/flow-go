package wal

import (
	"fmt"
	"sync"
	"time"

	"github.com/onflow/flow-go/ledger/complete/mtrie"
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
	"github.com/onflow/flow-go/module/lifecycle"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/observable"
	"github.com/rs/zerolog"
	"golang.org/x/sync/semaphore"
)

const (
	defaultSegmentTrieSize             = 500
	defaultTrieUpdateChannelBufSize    = 500
	defaultSegmentUpdateChannelBufSize = 500
)

// SegmentTrie contains trie and WAL segment number it was recorded in.
type SegmentTrie struct {
	Trie       *trie.MTrie
	SegmentNum int
}

// segmentTries contains tries that were recorded in given WAL segment.
type segmentTries struct {
	tries      []*trie.MTrie
	segmentNum int
}

// newSegmentTries creates a segment with given segment number and tries recorded in given segment.
func newSegmentTries(segmentNum int, tries ...*trie.MTrie) *segmentTries {
	segmentTries := &segmentTries{
		tries:      make([]*trie.MTrie, 0, defaultSegmentTrieSize),
		segmentNum: segmentNum,
	}
	segmentTries.tries = append(segmentTries.tries, tries...)
	return segmentTries
}

func (s *segmentTries) add(trie *trie.MTrie) {
	s.tries = append(s.tries, trie)
}

// activeSegmentTrieCompactor contains cached mtries in active segment.
// It receives and gathers updated trie and segment number for ledger update through trieUpdateCh channel.
// It sends batched trie updates for finalized segment through segmentUpdateCh channel.
type activeSegmentTrieCompactor struct {
	sync.Mutex
	logger          zerolog.Logger
	tries           *segmentTries
	trieUpdateCh    <-chan *SegmentTrie
	segmentUpdateCh chan<- *segmentTries
	stopCh          chan struct{}
}

func newActiveSegmentTrieCompactor(
	logger zerolog.Logger,
	trieUpdateCh <-chan *SegmentTrie,
	segmentUpdateCh chan<- *segmentTries,
) *activeSegmentTrieCompactor {
	return &activeSegmentTrieCompactor{
		logger:          logger,
		stopCh:          make(chan struct{}),
		trieUpdateCh:    trieUpdateCh,
		segmentUpdateCh: segmentUpdateCh,
	}
}

func (c *activeSegmentTrieCompactor) stop() {
	c.stopCh <- struct{}{}
}

func (c *activeSegmentTrieCompactor) start() {
Loop:
	for {
		select {
		case <-c.stopCh:
			break Loop
		case update := <-c.trieUpdateCh:
			prevSegmentTries, err := c.update(update.Trie, update.SegmentNum)
			if err != nil {
				c.logger.Error().Err(err).Msg("error updating active segment trie")
				continue
			}
			if prevSegmentTries != nil {
				c.segmentUpdateCh <- prevSegmentTries
			}
		}
	}

	// Drain remaining trie updates to remove trie references.
	for range c.trieUpdateCh {
	}
}

func (c *activeSegmentTrieCompactor) update(trie *trie.MTrie, segmentNum int) (*segmentTries, error) {
	c.Lock()
	defer c.Unlock()

	if c.tries == nil {
		c.tries = newSegmentTries(segmentNum, trie)
		return nil, nil
	}

	// Add to active segment tries cache
	if segmentNum == c.tries.segmentNum {
		c.tries.add(trie)
		return nil, nil
	}

	// New segment is created
	if segmentNum != c.tries.segmentNum+1 {
		return nil, fmt.Errorf("got segment number %d, want %d", segmentNum, c.tries.segmentNum+1)
	}

	// Save previous segment tries
	prevSegmentTries := c.tries

	// Create new segment tries
	c.tries = newSegmentTries(segmentNum, trie)

	return prevSegmentTries, nil
}

// segmentsTrieCompactor contains mtrie forest in finalized segments.
// When enough segments are accumulated, a new goroutine is created to create checkpoints async.
// At most one checkpoint goroutine is run at any given time.
// It receives and gathers updated tries in segment through segmentUpdateCh channel.
// It sends created checkpoint number through checkpointCh channel.
type segmentsTrieCompactor struct {
	sync.Mutex
	checkpointer       *Checkpointer
	logger             zerolog.Logger
	checkpointDistance uint
	lastCheckpointNum  int

	// forest contains mtries up to and including tries in forestSegmentNum.
	forest           *mtrie.Forest
	forestSegmentNum int

	segmentUpdateCh <-chan *segmentTries
	stopCh          chan struct{}
	checkpointCh    chan<- int
}

func newSegmentsTrieCompactor(
	logger zerolog.Logger,
	checkpointer *Checkpointer,
	segmentUpdateCh <-chan *segmentTries,
	checkpointCh chan<- int,
	tries []*trie.MTrie,
	segmentNum int,
	checkpointForestCapacity int,
	checkpointDistance uint,
) (*segmentsTrieCompactor, error) {

	forest, err := mtrie.NewForest(checkpointForestCapacity, &metrics.NoopCollector{}, nil)
	if err != nil {
		return nil, fmt.Errorf("cannot create Forest: %w", err)
	}
	err = forest.AddTries(tries)
	if err != nil {
		return nil, fmt.Errorf("cannot add tries to forest: %w", err)
	}

	lastCheckpointNum, err := checkpointer.LatestCheckpoint()
	if err != nil {
		return nil, fmt.Errorf("cannot get last checkpointed number: %w", err)
	}

	return &segmentsTrieCompactor{
		checkpointer:       checkpointer,
		logger:             logger,
		checkpointDistance: checkpointDistance,
		lastCheckpointNum:  lastCheckpointNum,
		forest:             forest,
		forestSegmentNum:   segmentNum,
		segmentUpdateCh:    segmentUpdateCh,
		checkpointCh:       checkpointCh,
		stopCh:             make(chan struct{}),
	}, nil
}

type checkpointResult struct {
	checkpointNum int
	err           error
}

func (c *segmentsTrieCompactor) stop() {
	c.stopCh <- struct{}{}
}

func (c *segmentsTrieCompactor) start() {

	checkpointSem := semaphore.NewWeighted(1) // limit to 1 checkpointing goroutine
	checkpointResultChan := make(chan checkpointResult)

Loop:
	for {
		select {

		case <-c.stopCh:
			break Loop

		case tries := <-c.segmentUpdateCh:
			triesToBeCheckpointed, checkpointNum, err := c.update(tries.tries, tries.segmentNum)
			if err != nil {
				c.logger.Error().Err(err).Msg("error updating cache")
				continue
			}

			if len(triesToBeCheckpointed) > 0 {
				if checkpointSem.TryAcquire(1) {
					go func() {
						defer checkpointSem.Release(1)
						err := createCheckpoint(c.checkpointer, c.logger, triesToBeCheckpointed, checkpointNum)
						checkpointResultChan <- checkpointResult{checkpointNum, err}
					}()
				}
			}

		case cpResult := <-checkpointResultChan:
			if cpResult.err != nil {
				c.logger.Error().Err(cpResult.err).Msg("error checkpointing")
				continue
			}

			c.Lock()
			c.lastCheckpointNum = cpResult.checkpointNum
			c.Unlock()

			c.checkpointCh <- cpResult.checkpointNum
		}
	}

	// Drain remaining segment updates to remove trie references.
	for range c.segmentUpdateCh {
	}
}

// update adds tries in given segment to forest, and returns tries to be checkpointed
// with checkpoint number if enough segments are finalized.
func (c *segmentsTrieCompactor) update(tries []*trie.MTrie, segmentNum int) ([]*trie.MTrie, int, error) {
	c.Lock()
	defer c.Unlock()

	err := c.forest.AddTries(tries)
	if err != nil {
		return nil, 0, fmt.Errorf("error adding trie %w", err)
	}
	c.forestSegmentNum = segmentNum

	uncheckpointedSegmentCount := segmentNum - c.lastCheckpointNum
	if uncheckpointedSegmentCount < int(c.checkpointDistance) {
		return nil, 0, nil
	}

	triesToBeCheckpointed, err := c.forest.GetTries()
	if err != nil {
		return nil, 0, err
	}
	return triesToBeCheckpointed, c.forestSegmentNum, nil
}

// CachedCompactor creates and manages segmentsTrieCompactor and activeSegmentTrieCompactor.
type CachedCompactor struct {
	checkpointer      *Checkpointer
	logger            zerolog.Logger
	lm                *lifecycle.LifecycleManager
	observers         map[observable.Observer]struct{}
	checkpointsToKeep uint

	activeSegmentTrieCompactor *activeSegmentTrieCompactor
	segmentsTrieCompactor      *segmentsTrieCompactor

	stopCh       chan struct{}
	checkpointCh <-chan int
}

func NewCachedCompactor(
	checkpointer *Checkpointer,
	trieUpdateCh <-chan *SegmentTrie,
	tries []*trie.MTrie,
	segmentNum int,
	checkpointForestCapacity int,
	checkpointDistance uint,
	checkpointsToKeep uint,
	logger zerolog.Logger,
) (*CachedCompactor, error) {
	if checkpointDistance < 1 {
		checkpointDistance = 1
	}

	segmentUpdateCh := make(chan *segmentTries, defaultSegmentUpdateChannelBufSize)
	checkpointCh := make(chan int)

	activeSegmentTrieCompactor := newActiveSegmentTrieCompactor(
		logger,
		trieUpdateCh,
		segmentUpdateCh,
	)

	segmentsTrieCompactor, err := newSegmentsTrieCompactor(
		logger,
		checkpointer,
		segmentUpdateCh,
		checkpointCh,
		tries,
		segmentNum,
		checkpointForestCapacity,
		checkpointDistance,
	)
	if err != nil {
		return nil, err
	}

	return &CachedCompactor{
		checkpointer:               checkpointer,
		logger:                     logger,
		observers:                  make(map[observable.Observer]struct{}),
		lm:                         lifecycle.NewLifecycleManager(),
		checkpointsToKeep:          checkpointsToKeep,
		activeSegmentTrieCompactor: activeSegmentTrieCompactor,
		segmentsTrieCompactor:      segmentsTrieCompactor,
		checkpointCh:               checkpointCh,
		stopCh:                     make(chan struct{}),
	}, nil
}

func (c *CachedCompactor) Subscribe(observer observable.Observer) {
	var void struct{}
	c.observers[observer] = void
}

func (c *CachedCompactor) Unsubscribe(observer observable.Observer) {
	delete(c.observers, observer)
}

func (c *CachedCompactor) Ready() <-chan struct{} {
	c.lm.OnStart(func() {
		go c.activeSegmentTrieCompactor.start()
		go c.segmentsTrieCompactor.start()
		go c.start()
	})
	return c.lm.Started()
}

func (c *CachedCompactor) Done() <-chan struct{} {
	c.lm.OnStop(func() {
		for observer := range c.observers {
			observer.OnComplete()
		}
		c.activeSegmentTrieCompactor.stop()
		c.segmentsTrieCompactor.stop()
		c.stopCh <- struct{}{}
	})
	return c.lm.Stopped()
}

func (c *CachedCompactor) start() {
	for {
		select {
		case <-c.stopCh:
			return
		case checkpointNum := <-c.checkpointCh:

			for observer := range c.observers {
				observer.OnNext(checkpointNum)
			}

			err := cleanupCheckpoints(c.checkpointer, int(c.checkpointsToKeep))
			if err != nil {
				c.logger.Error().Err(err).Msg("cannot cleanup checkpoints")
			}
		}
	}
}

func createCheckpoint(checkpointer *Checkpointer, logger zerolog.Logger, tries []*trie.MTrie, checkpointNum int) error {

	logger.Info().Msgf("serializing checkpoint %d with %d tries", checkpointNum, len(tries))

	startTime := time.Now()

	writer, err := checkpointer.CheckpointWriter(checkpointNum)
	if err != nil {
		return fmt.Errorf("cannot generate writer: %w", err)
	}
	defer func() {
		closeErr := writer.Close()
		// Return close error if there isn't any prior error to return.
		if err == nil {
			err = closeErr
		}
	}()

	err = StoreCheckpoint(writer, tries...)
	if err != nil {
		return fmt.Errorf("error serializing checkpoint (%d): %w", checkpointNum, err)
	}

	logger.Info().Msgf("created checkpoint %d with %d tries", checkpointNum, len(tries))

	duration := time.Since(startTime)
	logger.Info().Float64("total_time_s", duration.Seconds()).Msgf("created checkpoint %d with %d tries", checkpointNum, len(tries))

	return nil
}

func cleanupCheckpoints(checkpointer *Checkpointer, checkpointsToKeep int) error {
	// don't bother listing checkpoints if we keep them all
	if checkpointsToKeep == 0 {
		return nil
	}
	checkpoints, err := checkpointer.Checkpoints()
	if err != nil {
		return fmt.Errorf("cannot list checkpoints: %w", err)
	}
	if len(checkpoints) > int(checkpointsToKeep) {
		checkpointsToRemove := checkpoints[:len(checkpoints)-int(checkpointsToKeep)] // if condition guarantees this never fails

		for _, checkpoint := range checkpointsToRemove {
			err := checkpointer.RemoveCheckpoint(checkpoint)
			if err != nil {
				return fmt.Errorf("cannot remove checkpoint %d: %w", checkpoint, err)
			}
		}
	}
	return nil
}
