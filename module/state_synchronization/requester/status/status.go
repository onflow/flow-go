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

type persistedStatus struct {
	LastReceived uint64

	// Persisted to the db so the condition can be detected without inspecting the entire datastore
	Halted bool
}

func New(db datastore.Batching, log zerolog.Logger, maxCachedEntries, maxSearchAhead uint64) *Status {
	return &Status{
		db:  db,
		log: log,

		maxCachedEntries: maxCachedEntries,
		maxSearchAhead:   maxSearchAhead,
	}
}

// Load loads status from the db
func (s *Status) Load(ctx context.Context, startHeight uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	defer func() {
		// defaults to the start block when booting with a fresh db
		if s.lastReceived == 0 {
			s.lastReceived = startHeight
		}

		// notifications restart after a fresh boot
		s.lastNotified = s.lastReceived
		s.lastProcessed = s.lastReceived
	}()

	s.heap = &notificationHeap{}
	heap.Init(s.heap)

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

	s.lastReceived = savedStatus.LastReceived
	s.halted = savedStatus.Halted

	return nil
}

// save writes status to the db
func (s *Status) save(ctx context.Context) error {
	savedStatus := persistedStatus{
		LastReceived: s.lastReceived,
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
func (s *Status) NextNotification() (*BlockEntry, bool) {
	// This needs a write lock since it may modify the heap
	s.mu.Lock()
	defer s.mu.Unlock()

	entry := s.heap.PeekMin()
	next := s.nextHeight()

	if entry == nil || entry.Height != next {
		return nil, false
	}

	entry = heap.Pop(s.heap).(*BlockEntry)

	s.firstNotificationSent = true
	s.lastNotified = entry.Height

	return entry, true
}

// Fetched submits the BlockEntry for notification
func (s *Status) Fetched(ctx context.Context, entry *BlockEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Enforce a maximum possible cache size. The cache will be effective under normal operation.
	// However, if the requester gets stuck on a block for more than maxCachedEntries blocks, the cache
	// will be empty.
	if entry.Height > (s.lastNotified + s.maxCachedEntries) {
		entry.ExecutionData = nil
	}

	heap.Push(s.heap, entry)

	s.lastReceived = entry.Height

	if err := s.save(ctx); err != nil {
		s.log.Error().Err(err).Msg("failed to persist halted state")
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
		s.log.Error().Err(err).Msg("failed to persist fetched state")
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

	// Special case for block height 0 to distinguish between an unset uint64 and 0
	if !s.firstNotificationSent && s.lastNotified == 0 {
		next = 0
	}

	return next
}
