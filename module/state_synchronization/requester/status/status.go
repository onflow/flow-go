package status

import (
	"container/heap"
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/ipfs/go-datastore"
	"github.com/rs/zerolog"
)

const statusDBKey = "execution_requester_status"

type Status struct {
	startHeight uint64

	// The highest block height whose ExecutionData has been fetched
	lastReceived uint64

	// The highest block height that's been inspected for newly sealed results
	lastProcessed uint64

	// The highest block height for which notifications have been sent
	lastNotified uint64

	// Whether or not the first notification has been sent since startup. This is used to handle
	// the case where the block height is 0 since the's no way to distinguish between an unset
	// uint64 and 0
	firstNotificationSent bool

	// Whether or not the requester has been halted
	halted bool

	// Maximum number of blocks within the notification heap that can have ExecutionData
	maxCachedEntries uint64

	// Maximum number of blocks heights to process beyond the lastNotified before pausing the requester.
	// This puts a limit on the resources used by the requestor when it gets stuck on fetching a height.
	maxSearchAhead uint64

	heap *notificationHeap

	mu  sync.RWMutex
	db  datastore.Batching
	log zerolog.Logger
}

// persistedStatus is the status data that is persisted to the db, so it can be reused across reloads
type persistedStatus struct {
	// LastNotified is the highest consecutive execution data received. When loaded, this is used
	// as the starting point requesting ExecutionData for new blocks.
	// Note: LastNotified is used instead of LastReceived since there could be a large gap of
	// un-downloaded blocks between LastReceived and LastNotified. Using LastNotified allows the
	// requester to backfill all of missing blocks even when it's not configured to recheck the
	// datastore.
	LastNotified uint64

	// Halted is persisted to the db so the condition can be detected without inspecting the entire
	// datastore
	Halted bool
}

func New(db datastore.Batching, log zerolog.Logger, startHeight uint64, maxCachedEntries, maxSearchAhead uint64) *Status {
	h := &notificationHeap{}
	heap.Init(h)

	return &Status{
		db:   db,
		log:  log,
		heap: h,

		startHeight:      startHeight,
		maxCachedEntries: maxCachedEntries,
		maxSearchAhead:   maxSearchAhead,
	}
}

// Load loads status from the db
func (s *Status) Load(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	defer func() {
		// defaults to the start block when booting with a fresh db
		if s.lastNotified == 0 {
			s.lastNotified = s.startHeight
		}

		// notifications restart after a fresh boot
		s.lastReceived = s.lastNotified
		s.lastProcessed = s.lastNotified
	}()

	data, err := s.db.Get(ctx, datastore.NewKey(statusDBKey))
	if err == datastore.ErrNotFound {
		return nil
	}

	if err != nil {
		return fmt.Errorf("failed to load status: %w", err)
	}

	savedStatus := persistedStatus{}

	err = json.Unmarshal(data, &savedStatus)
	if err != nil {
		return fmt.Errorf("failed to unmarshal status: %w", err)
	}

	s.lastNotified = savedStatus.LastNotified
	s.halted = savedStatus.Halted

	return nil
}

// save writes status to the db
func (s *Status) save(ctx context.Context) error {
	savedStatus := persistedStatus{
		LastNotified: s.lastNotified,
		Halted:       s.halted,
	}

	data, err := json.Marshal(&savedStatus)
	if err != nil {
		return fmt.Errorf("failed to marshal status: %w", err)
	}

	err = s.db.Put(ctx, datastore.NewKey(statusDBKey), data)
	if err != nil {
		return fmt.Errorf("failed to save status: %w", err)
	}

	return nil
}

// NextNotification returns the BlockEntry for the next notification to send and marks it as notified.
// If the next notification is available, it returns the BlockEntry and true. Otherwise, it returns
// nil and false.
func (s *Status) NextNotification(ctx context.Context) (*BlockEntry, bool) {
	// This needs a write lock since it may modify the heap
	s.mu.Lock()
	defer s.mu.Unlock()

	entry := s.heap.PeekMin()
	next := s.nextHeight()

	// Next notification is not available yet
	if entry == nil || entry.Height > next {
		return nil, false
	}

	// Remove any entires below the next height. This clears out any duplicates that have already
	// been notified.
	for entry.Height < next && s.heap.Len() > 0 {
		entry = heap.Pop(s.heap).(*BlockEntry)
	}

	// All entries were duplicates. The heap now is empty
	if entry == nil || entry.Height != next {
		return nil, false
	}

	// Now we have the next height to notify

	s.firstNotificationSent = true
	s.lastNotified = entry.Height

	if err := s.save(ctx); err != nil {
		s.log.Error().Err(err).Msg("failed to persist fetched state")
	}

	return entry, true
}

// Fetched submits the BlockEntry for notification
func (s *Status) Fetched(entry *BlockEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// don't accept entries we've already notified for
	if entry.Height < s.nextHeight() {
		return
	}

	// Enforce a maximum possible cache size. The cache will be effective under normal operation.
	// However, if the requester gets stuck on a block for more than maxCachedEntries blocks, the cache
	// will be empty.
	if entry.Height > (s.lastNotified + s.maxCachedEntries) {
		entry.ExecutionData = nil
	}

	heap.Push(s.heap, entry)

	// Only track the highest height
	if entry.Height <= s.lastReceived {
		return
	}

	s.lastReceived = entry.Height
}

// Processed marks a height as processed
func (s *Status) Processed(height uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.lastProcessed = height
}

// Halt marks the requester as halted. This is persisted between restarts
func (s *Status) Halt(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.halted = true

	if err := s.save(ctx); err != nil {
		s.log.Error().Err(err).Msg("failed to persist halted state")
	}
}

// IsPaused returns true if the requester should pause fetching new heights
// This puts a limit on the number of blocks that are fetched when the requester gets stuck on
// fetching a past height.
func (s *Status) IsPaused() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.lastProcessed > (s.lastNotified + s.maxSearchAhead)
}

// LastProcessed returns the last block height that was processed by the requester
func (s *Status) LastProcessed() uint64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.lastProcessed
}

// LastNotified returns the last block height that was notified by the requester
func (s *Status) LastNotified() uint64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.lastNotified
}

// LastReceived returns the last block height that was received by the requester
func (s *Status) LastReceived() uint64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.lastReceived
}

// Halted returns true if the requester has been halted
func (s *Status) Halted() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.halted
}

// NextNotificationHeight returns the next block height waiting to be notified.
// This height may not be available yet.
func (s *Status) NextNotificationHeight() uint64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.nextHeight()
}

func (s *Status) nextHeight() uint64 {
	next := s.lastNotified + 1

	// Special case for the first block
	if !s.firstNotificationSent && s.lastNotified == s.startHeight {
		next = s.startHeight
	}

	return next
}

func (s *Status) OutstandingNotifications() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.heap.Len()
}
