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
	// The first block height for which notifications should be sent
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
	// as the starting point for requesting ExecutionData for new blocks.
	// Note: LastNotified is used instead of LastReceived since there could be a large gap of
	// un-downloaded blocks between LastReceived and LastNotified. Using LastNotified allows the
	// requester to backfill all of missing blocks even when it's not configured to recheck the
	// datastore.
	LastNotified uint64

	// Halted is persisted so the condition can be detected without inspecting the entire datastore
	Halted bool
}

func New(db datastore.Batching, log zerolog.Logger, startHeight uint64, maxCachedEntries, maxSearchAhead uint64) *Status {
	return &Status{
		db:   db,
		log:  log,
		heap: &notificationHeap{},

		startHeight:   startHeight,
		lastNotified:  startHeight,
		lastReceived:  startHeight,
		lastProcessed: startHeight,

		maxCachedEntries: maxCachedEntries,
		maxSearchAhead:   maxSearchAhead,
	}
}

// Load loads status from the db
func (s *Status) Load(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := s.db.Get(ctx, datastore.NewKey(statusDBKey))
	if err == datastore.ErrNotFound {
		return nil
	}

	if err != nil {
		return fmt.Errorf("failed to load status: %w", err)
	}

	saved := persistedStatus{}

	err = json.Unmarshal(data, &saved)
	if err != nil {
		return fmt.Errorf("failed to unmarshal status: %w", err)
	}

	s.lastNotified = saved.LastNotified
	s.halted = saved.Halted

	// processing restarts after a fresh boot.
	s.lastReceived = s.lastNotified
	s.lastProcessed = s.lastNotified

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

	var entry *BlockEntry
	next := s.nextHeight()

	// Remove any entries for the next height or below. This ensures duplicate or unexpected heights
	// below the tracked heights are removed. If there are duplicate entries for the same height, the
	// last entry popped from the heap will be returned.
	for s.heap.Len() > 0 && s.heap.PeekMin().Height <= next {
		entry = heap.Pop(s.heap).(*BlockEntry)
	}

	if entry == nil || entry.Height != next {
		return nil, false
	}

	// Now we have the next height to notify

	s.lastNotified = entry.Height
	s.firstNotificationSent = true

	if err := s.save(ctx); err != nil {
		s.log.Error().Err(err).Msg("failed to persist fetched state")
	}

	return entry, true
}

// Fetched submits the BlockEntry for notification
func (s *Status) Fetched(entry *BlockEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// don't accept entries which have already been notified
	if entry.Height < s.nextHeight() {
		return
	}

	// Enforce a maximum possible cache size. The cache will be effective under normal operation.
	// However, the cache will be under utilized if there is a large gap of unfetched heights.
	if entry.Height >= (s.lastNotified + s.maxCachedEntries) {
		entry.ExecutionData = nil
	}

	heap.Push(s.heap, entry)

	// Only track the highest height
	if entry.Height > s.lastReceived {
		s.lastReceived = entry.Height
	}
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

// IsPaused returns true if the requester should pause downloading new heights
// This puts a limit on the number of blocks that are downloaded when the requester is temporarily
// unable to download a past height.
func (s *Status) IsPaused() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.lastProcessed > (s.lastNotified + s.maxSearchAhead)
}

// LastProcessed returns the last block height that was marked Processed
func (s *Status) LastProcessed() uint64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.lastProcessed
}

// LastNotified returns the last block height that was returned from NextNotification
func (s *Status) LastNotified() uint64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.lastNotified
}

// LastReceived returns the last block height that was marked Fetched
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

// OutstandingNotifications returns the number of blocks that have been Fetched but not notified
func (s *Status) OutstandingNotifications() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.heap.Len()
}
