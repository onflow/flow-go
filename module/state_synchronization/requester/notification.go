package requester

import (
	"errors"
	"sort"
	"sync"

	"github.com/onflow/flow-go/model/flow"
)

var (
	ErrDuplicateNotification = errors.New("notification already sent for height")
	ErrUnavailableHeight     = errors.New("notified for a height not yet received")
	ErrNonconsecutiveHeight  = errors.New("notified for a non-consecutive height")
)

type status struct {
	startHeight              uint64
	lastNotified             uint64
	lastSealed               uint64
	lastReceived             uint64
	missingHeights           map[uint64]struct{}
	outstandingNotifications map[uint64]flow.Identifier
	halted                   bool

	firstNotificationSent bool

	mu sync.Mutex
}

func (m *status) Load() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// load the status data from the db

	m.missingHeights = make(map[uint64]struct{})
	m.outstandingNotifications = make(map[uint64]flow.Identifier)

	return nil
}

func (m *status) MissingHeights() []uint64 {
	m.mu.Lock()
	defer m.mu.Unlock()

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
	m.mu.Lock()
	defer m.mu.Unlock()

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
		return 0, flow.ZeroID, false
	}

	return 0, flow.ZeroID, false
}

func (m *status) Sealed(height uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.lastSealed = height
}

func (m *status) LastSealed() uint64 {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.lastSealed
}

func (m *status) Fetched(height uint64, blockID flow.Identifier) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// this is the next height we're expecting
	// this also includes special handling for block 0.
	if height == m.lastReceived+1 || height == 0 {
		m.lastReceived = height
		m.outstandingNotifications[height] = blockID
		return
	}

	// check if it's missing
	if height <= m.lastReceived {
		if _, missing := m.missingHeights[height]; missing {
			delete(m.missingHeights, height)
			m.outstandingNotifications[height] = blockID
			return
		}
		// otherwise we've already received this height
		return
	}

	// if there's a gap, record all missing heights
	for h := m.lastReceived + 1; h < height; h++ {
		m.missingHeights[h] = struct{}{}
	}

	m.lastReceived = height
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
	if height > m.lastReceived {
		return ErrUnavailableHeight
	}

	// there's a problem with the state
	return ErrNonconsecutiveHeight
}
