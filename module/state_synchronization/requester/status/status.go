package status

import (
	"container/heap"
	"errors"
	"fmt"
	"sync"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/storage"
	"github.com/rs/zerolog"
)

// Status encapsulate the ExecutionDataRequester's state and provides methods to consistently
// manage the requester's responsibilities.
type Status struct {
	// The highest block height for which notifications have been sent
	lastNotified uint64

	// The highest block height whose ExecutionData has been fetched
	lastReceived uint64

	// The error that caused the requester to halt, or nil if it is not halted
	halted error

	// Maximum number of blocks within the notification heap that can have ExecutionData
	maxCachedEntries uint64

	// ConsumerProgress datastore where the requester's state is persisted
	progress storage.ConsumerProgress

	heap *NotificationHeap
	mu   sync.RWMutex
	log  zerolog.Logger
}

var _ module.Jobs = (*Status)(nil)

// New returns a new Status struct
func New(log zerolog.Logger, maxCachedEntries uint64, progress storage.ConsumerProgress) *Status {
	return &Status{
		log:      log,
		heap:     &NotificationHeap{},
		progress: progress,

		maxCachedEntries: maxCachedEntries,
	}
}

// Load loads status from the db
func (s *Status) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.log.Debug().Msg("loading requester status")

	lastNotified, err := s.progress.ProcessedIndex()
	if err != nil {
		return fmt.Errorf("failed to load last processed index: %w", err)
	}

	halted, err := s.progress.Halted()
	if errors.Is(err, storage.ErrNotFound) {
		// initialize the halted state if this is an empty db
		s.log.Info().Msg("halted state not found. initializing")
		err = s.progress.InitHalted()
	}
	if err != nil {
		return fmt.Errorf("failed to load halted state: %w", err)
	}

	// processing restarts after a fresh boot.
	s.lastNotified = lastNotified
	s.lastReceived = lastNotified

	// Note: only the error message is preserved between loads, not the type.
	s.halted = halted

	return nil
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

// Halt marks the requester as halted. This is persisted between restarts
func (s *Status) Halt(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.halted = err

	if err := s.progress.SetHalted(err); err != nil {
		s.log.Error().Err(err).Msg("failed to persist halted state")
	}
}

// LastNotified returns the last block height that was returned from nextNotification
func (s *Status) LastNotified() uint64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.lastNotified
}

// Halted returns true if the requester has been halted
func (s *Status) Halted() error {
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

// nextNotification returns the BlockEntry for the next notification to send and marks it as notified.
// If the next notification is available, it returns the BlockEntry and true. Otherwise, it returns
// nil and false.
func (s *Status) nextNotification() (*BlockEntry, bool) {
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

	return entry, true
}

func (s *Status) nextHeight() uint64 {
	return s.lastNotified + 1
}

// OutstandingNotifications returns the number of blocks that have been Fetched but not notified
func (s *Status) OutstandingNotifications() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.heap.Len()
}

// AtIndex returns the notification job at the given index.
// The notification job at an index is the BlockEntry for that height
// Since notifications are sent sequentially, there is only ever a single job available, which is
// the next height to notify, iff it's ready.
func (s *Status) AtIndex(index uint64) (module.Job, error) {
	// This needs a write lock since it may modify the heap
	s.mu.Lock()
	defer s.mu.Unlock()

	// make sure the requested index is for the next height
	next := s.nextHeight()
	if next != index {
		return nil, storage.ErrNotFound
	}

	// check if the next height is ready
	entry, ok := s.nextNotification()
	if !ok {
		return nil, storage.ErrNotFound
	}

	return BlockEntryToJob(entry), nil
}

// Head returns the highest available index
// For simplicity, head is either the next height to notify, or not found
func (s *Status) Head() (uint64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	min := s.heap.PeekMin()
	if min == nil {
		return 0, storage.ErrNotFound
	}

	next := s.nextHeight()
	if next != min.Height {
		return 0, storage.ErrNotFound
	}

	return next, nil
}
