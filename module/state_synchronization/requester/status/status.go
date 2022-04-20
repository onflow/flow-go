package status

import (
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

	// The error that caused the requester to halt, or nil if it is not halted
	halted error

	// Maximum number of blocks within the notification heap that can have ExecutionData
	maxCachedEntries uint64

	// Blocks that have been downloaded, but not yet notified
	fetched map[uint64]*BlockEntry

	// ConsumerProgress datastore where the requester's state is persisted
	progress storage.ConsumerProgress

	mu  sync.RWMutex
	log zerolog.Logger
}

var _ module.Jobs = (*Status)(nil)

// New returns a new Status struct
func New(log zerolog.Logger, maxCachedEntries uint64, progress storage.ConsumerProgress) *Status {
	return &Status{
		log:      log,
		fetched:  make(map[uint64]*BlockEntry),
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

	s.fetched[entry.Height] = entry
}

// Notified marks a block height as notified
func (s *Status) Notified(index uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.fetched[index]; ok {
		delete(s.fetched, index)
		s.lastNotified = index
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

func (s *Status) nextHeight() uint64 {
	return s.lastNotified + 1
}

// OutstandingNotifications returns the number of blocks that have been Fetched but not notified
func (s *Status) OutstandingNotifications() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.fetched)
}

// AtIndex returns the notification job at the given index.
// The notification job at an index is the BlockEntry for that height
// Since notifications are sent sequentially, there is only ever a single job available, which is
// the next height to notify, iff it's ready.
func (s *Status) AtIndex(index uint64) (module.Job, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// only return new jobs for the next height
	if index != s.nextHeight() {
		return nil, storage.ErrNotFound
	}

	// make sure the next height is available
	entry, ok := s.fetched[index]
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

	next := s.nextHeight()
	if _, ok := s.fetched[next]; !ok {
		return 0, storage.ErrNotFound
	}

	return next, nil
}
