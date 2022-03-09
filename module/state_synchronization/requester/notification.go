package requester

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/ipfs/go-datastore"
	"github.com/onflow/flow-go/model/flow"
)

const statusDBKey = "execution_requester_status"

var (
	ErrDuplicateNotification = errors.New("notification already sent for height")
	ErrUnavailableHeight     = errors.New("notified for a height not yet received")
	ErrNonconsecutiveHeight  = errors.New("notified for a non-consecutive height")
)

type status struct {
	// The highest block height whose ExecutionData has been fetched. Included in stored state.
	LastReceived uint64

	// The highest block height that's been inspected for newly sealed results
	lastProcessed uint64

	// The highest block height for which notifications have been sent
	lastNotified uint64

	// Whether or not the requester has been halted. Included in stored state.
	// Persisted to the db so the condition can be detected without inspecting
	// the entire datastore.
	Halted bool

	// List of heights below lastProcessed that have not been received
	missingHeights map[uint64]bool

	// List of heights that have been received, but notifications have not been sent
	outstandingNotifications map[uint64]flow.Identifier

	// Whether or not the first notification has been sent since startup. This is used to handle
	// the case where the block height is 0 since the's no way to distinguish between an unset
	// uint64 and 0
	firstNotificationSent bool

	mu sync.RWMutex
	db datastore.Batching
}

func (m *status) Load(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.missingHeights = make(map[uint64]bool)
	m.outstandingNotifications = make(map[uint64]flow.Identifier)

	data, err := m.db.Get(ctx, datastore.NewKey(statusDBKey))
	if err == datastore.ErrNotFound {
		return nil
	}

	if err != nil {
		return fmt.Errorf("failed to load status: %w", err)
	}

	err = json.Unmarshal(data, m)
	if err != nil {
		return fmt.Errorf("failed to unmarshal status: %w", err)
	}

	return nil
}

func (m *status) save(ctx context.Context) error {
	data, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("failed to marshal status: %w", err)
	}

	err = m.db.Put(ctx, datastore.NewKey(statusDBKey), data)
	if err != nil {
		return fmt.Errorf("failed to save status: %w", err)
	}

	return nil
}

func (m *status) MissingHeights() []uint64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	heights := make([]uint64, len(m.missingHeights))

	i := 0
	for h := range m.missingHeights {
		heights[i] = h
		i++
	}

	// sort ascending
	sort.Slice(heights, func(i, j int) bool {
		return heights[i] < heights[j]
	})

	return heights
}

func (m *status) NextNotification() (uint64, flow.Identifier, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// special case for block height 0
	if m.lastNotified == 0 && !m.firstNotificationSent {
		// the next notification is for block 0
		if blockID, ok := m.outstandingNotifications[0]; ok {
			return 0, blockID, true
		}
		return 0, flow.ZeroID, false
	}

	next := m.lastNotified + 1
	blockID, received := m.outstandingNotifications[next]
	if received {
		return next, blockID, true
	}

	// check if next height is missing
	if _, missing := m.missingHeights[next]; missing {
		return next, flow.ZeroID, false
	}

	return next, flow.ZeroID, false
}

func (m *status) Fetched(ctx context.Context, height uint64, blockID flow.Identifier) {
	m.mu.Lock()
	defer m.mu.Unlock()
	defer m.save(ctx)

	// this is the next height we're expecting
	// this also includes special handling for block 0.

	if m.LastReceived == 0 && !m.firstNotificationSent {
		if height == 0 {
			m.LastReceived = height
			m.outstandingNotifications[height] = blockID
			return
		}
	} else if height == m.LastReceived+1 {
		m.LastReceived = height
		m.outstandingNotifications[height] = blockID
		return
	}

	// check if it's missing
	if height <= m.LastReceived {
		if m.missingHeights[height] {
			delete(m.missingHeights, height)
			m.outstandingNotifications[height] = blockID
			return
		}
		// otherwise we've already received this height
		return
	}

	// if there's a gap, record all missing heights
	start := m.LastReceived + 1
	if m.LastReceived == 0 && !m.firstNotificationSent {
		start = 0
	}
	for h := start; h < height; h++ {
		m.missingHeights[h] = true
	}

	m.LastReceived = height
	m.outstandingNotifications[height] = blockID
}

func (m *status) Notified(height uint64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// this is the next height we're expecting
	if height == m.lastNotified+1 {
		m.lastNotified++
		delete(m.outstandingNotifications, height)
		return nil
	}

	// special case for block height 0. lastNotified remains 0 if no notification have been sent
	// since 0 means both uninitialized and block 0
	if height == 0 && !m.firstNotificationSent {
		m.lastNotified = 0
		m.firstNotificationSent = true
		delete(m.outstandingNotifications, height)
		return nil
	}

	// we've already notified for this height
	if height <= m.lastNotified {
		return ErrDuplicateNotification
	}

	// we haven't seen this height yet
	if height > m.LastReceived {
		return ErrUnavailableHeight
	}

	// there's a problem with the state
	return ErrNonconsecutiveHeight
}

func (m *status) Halt(ctx context.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Halted = true

	m.save(ctx)
}
