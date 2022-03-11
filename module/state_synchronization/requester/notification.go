package requester

import (
	"container/heap"
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/ipfs/go-datastore"
)

const statusDBKey = "execution_requester_status"

type status struct {
	// The highest block height whose ExecutionData has been fetched. Included in stored state.
	LastReceived uint64

	// The highest block height that's been inspected for newly sealed results
	lastProcessed uint64

	// The highest block height for which notifications have been sent
	lastNotified uint64

	// Whether or not the first notification has been sent since startup. This is used to handle
	// the case where the block height is 0 since the's no way to distinguish between an unset
	// uint64 and 0
	firstNotificationSent bool

	// Whether or not the requester has been halted. Included in stored state.
	// Persisted to the db so the condition can be detected without inspecting
	// the entire datastore.
	Halted bool

	// Maximum number of blocks within the notification heap that can have ExecutionData
	maxCacheSize uint64

	heap *NotificationHeap

	mu sync.RWMutex
	db datastore.Batching
}

func (s *status) Load(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.heap = &NotificationHeap{}
	heap.Init(s.heap)

	data, err := s.db.Get(ctx, datastore.NewKey(statusDBKey))
	if err == datastore.ErrNotFound {
		return nil
	}

	if err != nil {
		return fmt.Errorf("failed to load status: %w", err)
	}

	err = json.Unmarshal(data, s)
	if err != nil {
		return fmt.Errorf("failed to unmarshal status: %w", err)
	}

	return nil
}

func (s *status) save(ctx context.Context) error {
	data, err := json.Marshal(s)
	if err != nil {
		return fmt.Errorf("failed to marshal status: %w", err)
	}

	err = s.db.Put(ctx, datastore.NewKey(statusDBKey), data)
	if err != nil {
		return fmt.Errorf("failed to save status: %w", err)
	}

	return nil
}

func (s *status) NextNotification() (*BlockEntry, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	next := s.lastNotified + 1

	if s.heap.Len() == 0 {
		return &BlockEntry{Height: next}, false
	}

	// Special case for block height 0 to distinguish between an unset uint64 and 0
	if !s.firstNotificationSent && s.lastNotified == 0 {
		next = 0
	}

	entry := s.heap.PeekMin()

	if entry == nil || entry.Height != next {
		return &BlockEntry{Height: next}, false
	}

	entry = heap.Pop(s.heap).(*BlockEntry)

	s.firstNotificationSent = true
	s.lastNotified = entry.Height

	return entry, true
}

func (s *status) Fetched(ctx context.Context, entry *BlockEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Enforce a maximum possible cache size. The cache will be effective under normal operation.
	// However, if the requester gets stuck on a block for more than maxCacheSize blocks, the cache
	// will be empty.
	if entry.Height > (s.lastNotified + s.maxCacheSize) {
		entry.ExecutionData = nil
	}

	heap.Push(s.heap, entry)

	s.LastReceived = entry.Height
	s.save(ctx)
}

func (s *status) Halt(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Halted = true
	s.save(ctx)
}
