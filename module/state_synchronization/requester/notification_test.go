package requester

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestMissingHeights(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		status := &status{
			missingHeights: map[uint64]bool{},
		}

		heights := status.MissingHeights()

		assert.Equal(t, 0, len(heights))
	})

	t.Run("single", func(t *testing.T) {
		status := &status{
			missingHeights: map[uint64]bool{
				1: true,
			},
		}

		heights := status.MissingHeights()

		assert.Equal(t, 1, len(heights))
		assert.Equal(t, uint64(1), heights[0])
	})

	t.Run("multiple returns sorted", func(t *testing.T) {
		status := &status{
			missingHeights: map[uint64]bool{
				4: true,
				1: true,
				2: true,
			},
		}

		heights := status.MissingHeights()

		assert.Equal(t, 3, len(heights))
		assert.Equal(t, uint64(1), heights[0])
		assert.Equal(t, uint64(2), heights[1])
		assert.Equal(t, uint64(4), heights[2])
	})
}

func TestNextNotification(t *testing.T) {
	t.Run("next ready", func(t *testing.T) {
		nextBlockID := unittest.IdentifierFixture()
		status := &status{
			lastNotified: 5,
			lastReceived: 7,
			outstandingNotifications: map[uint64]flow.Identifier{
				6: nextBlockID,
			},
			missingHeights: map[uint64]bool{},
		}

		next, blockID, ok := status.NextNotification()
		assert.Equal(t, uint64(6), next)
		assert.Equal(t, nextBlockID, blockID)
		assert.True(t, ok)
	})

	t.Run("none ready", func(t *testing.T) {
		status := &status{
			lastNotified:   5,
			lastReceived:   5,
			missingHeights: map[uint64]bool{},
		}

		next, blockID, ok := status.NextNotification()
		assert.Equal(t, uint64(0), next)
		assert.Equal(t, flow.ZeroID, blockID)
		assert.False(t, ok)
	})

	t.Run("next missing", func(t *testing.T) {
		status := &status{
			lastNotified: 5,
			lastReceived: 7,
			missingHeights: map[uint64]bool{
				6: true,
			},
		}

		next, blockID, ok := status.NextNotification()
		assert.Equal(t, uint64(0), next)
		assert.Equal(t, flow.ZeroID, blockID)
		assert.False(t, ok)
	})

	t.Run("zero handling - empty state", func(t *testing.T) {
		status := &status{}

		next, blockID, ok := status.NextNotification()
		assert.Equal(t, uint64(0), next)
		assert.Equal(t, flow.ZeroID, blockID)
		assert.False(t, ok)
	})

	t.Run("zero handling - block 0 fetched", func(t *testing.T) {
		nextBlockID := unittest.IdentifierFixture()
		status := &status{
			outstandingNotifications: map[uint64]flow.Identifier{
				0: nextBlockID,
			},
		}

		next, blockID, ok := status.NextNotification()
		assert.Equal(t, uint64(0), next)
		assert.Equal(t, nextBlockID, blockID)
		assert.True(t, ok)
	})

	t.Run("zero handling - block 0 notified", func(t *testing.T) {
		nextBlockID := unittest.IdentifierFixture()
		status := &status{
			firstNotificationSent: true,
			outstandingNotifications: map[uint64]flow.Identifier{
				1: nextBlockID,
			},
		}

		next, blockID, ok := status.NextNotification()
		assert.Equal(t, uint64(1), next)
		assert.Equal(t, nextBlockID, blockID)
		assert.True(t, ok)
	})
}

func TestSeals(t *testing.T) {
	status := &status{}
	assert.Equal(t, uint64(0), status.lastSealed)

	status.Sealed(10)
	assert.Equal(t, uint64(10), status.lastSealed)
}

func TestFetched(t *testing.T) {
	t.Run("fetched next", func(t *testing.T) {
		height := uint64(6)
		blockID := unittest.IdentifierFixture()
		status := &status{
			lastReceived:             5,
			outstandingNotifications: map[uint64]flow.Identifier{},
			missingHeights:           map[uint64]bool{},
		}

		status.Fetched(height, blockID)

		assert.Equal(t, height, status.lastReceived)
		assert.Len(t, status.outstandingNotifications, 1)
		assert.Equal(t, blockID, status.outstandingNotifications[height])
	})

	t.Run("fetched missing", func(t *testing.T) {
		height := uint64(4)
		blockID := unittest.IdentifierFixture()
		status := &status{
			lastReceived:             5,
			outstandingNotifications: map[uint64]flow.Identifier{},
			missingHeights: map[uint64]bool{
				height: true,
			},
		}

		status.Fetched(height, blockID)

		assert.Equal(t, uint64(5), status.lastReceived)
		assert.Len(t, status.outstandingNotifications, 1)
		assert.Equal(t, blockID, status.outstandingNotifications[height])
		assert.Len(t, status.missingHeights, 0)
	})

	t.Run("fetched with gap", func(t *testing.T) {
		height := uint64(8)
		blockID := unittest.IdentifierFixture()
		status := &status{
			lastReceived:             5,
			outstandingNotifications: map[uint64]flow.Identifier{},
			missingHeights:           map[uint64]bool{},
		}

		status.Fetched(height, blockID)

		assert.Equal(t, height, status.lastReceived)
		assert.Len(t, status.outstandingNotifications, 1)
		assert.Equal(t, blockID, status.outstandingNotifications[height])
		assert.Equal(t, []uint64{6, 7}, status.MissingHeights())
	})

	t.Run("fetched duplicate", func(t *testing.T) {
		height := uint64(5)
		blockID := unittest.IdentifierFixture()
		status := &status{
			lastReceived:             5,
			outstandingNotifications: map[uint64]flow.Identifier{},
			missingHeights:           map[uint64]bool{},
		}

		status.Fetched(height, blockID)

		assert.Equal(t, height, status.lastReceived)
		assert.Len(t, status.outstandingNotifications, 0)
	})

	t.Run("fetched already notified", func(t *testing.T) {
		height := uint64(4)
		blockID := unittest.IdentifierFixture()
		status := &status{
			lastNotified:             5,
			lastReceived:             5,
			outstandingNotifications: map[uint64]flow.Identifier{},
			missingHeights:           map[uint64]bool{},
		}

		status.Fetched(height, blockID)

		assert.Equal(t, uint64(5), status.lastReceived)
		assert.Len(t, status.outstandingNotifications, 0)
	})

	t.Run("fetched already received", func(t *testing.T) {
		height := uint64(7)
		blockID := unittest.IdentifierFixture()
		status := &status{
			lastNotified:             5,
			lastReceived:             8,
			outstandingNotifications: map[uint64]flow.Identifier{},
			missingHeights:           map[uint64]bool{},
		}

		status.Fetched(height, blockID)

		assert.Equal(t, uint64(8), status.lastReceived)
		assert.Len(t, status.outstandingNotifications, 0)
	})

	t.Run("zero handling - empty state", func(t *testing.T) {
		height := uint64(0)
		blockID := unittest.IdentifierFixture()
		status := &status{
			outstandingNotifications: map[uint64]flow.Identifier{},
			missingHeights:           map[uint64]bool{},
		}

		status.Fetched(height, blockID)

		assert.Equal(t, uint64(0), status.lastReceived)
		assert.Len(t, status.outstandingNotifications, 1)
		assert.Equal(t, blockID, status.outstandingNotifications[height])
		assert.Len(t, status.missingHeights, 0)
	})

	t.Run("zero handling - block 0 fetched", func(t *testing.T) {
		height := uint64(0)
		blockID := unittest.IdentifierFixture()
		status := &status{
			firstNotificationSent:    true,
			outstandingNotifications: map[uint64]flow.Identifier{},
			missingHeights:           map[uint64]bool{},
		}

		status.Fetched(height, blockID)

		assert.Equal(t, uint64(0), status.lastReceived)
		assert.Len(t, status.outstandingNotifications, 0)
		assert.Len(t, status.missingHeights, 0)
	})
}

func TestNotified(t *testing.T) {
	t.Run("notified next", func(t *testing.T) {
		height := uint64(6)
		status := &status{
			lastNotified: 5,
			outstandingNotifications: map[uint64]flow.Identifier{
				height: unittest.IdentifierFixture(),
			},
		}

		err := status.Notified(height)
		assert.NoError(t, err)

		assert.Equal(t, height, status.lastNotified)
		assert.Len(t, status.outstandingNotifications, 0)
	})

	t.Run("notified duplicate", func(t *testing.T) {
		height := uint64(5)
		status := &status{
			lastNotified: 5,
			outstandingNotifications: map[uint64]flow.Identifier{
				height: unittest.IdentifierFixture(),
			},
		}

		err := status.Notified(height)
		assert.ErrorIs(t, err, ErrDuplicateNotification)

		assert.Equal(t, uint64(5), status.lastNotified)
		assert.Len(t, status.outstandingNotifications, 1)
	})

	t.Run("notified unavailable height", func(t *testing.T) {
		height := uint64(7)
		status := &status{
			lastNotified: 5,
			lastReceived: 6,
			outstandingNotifications: map[uint64]flow.Identifier{
				height: unittest.IdentifierFixture(),
			},
		}

		err := status.Notified(height)
		assert.ErrorIs(t, err, ErrUnavailableHeight)

		assert.Equal(t, uint64(5), status.lastNotified)
		assert.Len(t, status.outstandingNotifications, 1)
	})

	t.Run("notified non-consecutive height", func(t *testing.T) {
		height := uint64(7)
		status := &status{
			lastNotified: 5,
			lastReceived: 8,
			outstandingNotifications: map[uint64]flow.Identifier{
				height: unittest.IdentifierFixture(),
			},
		}

		err := status.Notified(height)
		assert.ErrorIs(t, err, ErrNonconsecutiveHeight)

		assert.Equal(t, uint64(5), status.lastNotified)
		assert.Len(t, status.outstandingNotifications, 1)
	})

	t.Run("zero handling - notified next", func(t *testing.T) {
		height := uint64(0)
		status := &status{
			outstandingNotifications: map[uint64]flow.Identifier{
				height: unittest.IdentifierFixture(),
			},
		}

		err := status.Notified(height)
		assert.NoError(t, err)

		assert.Equal(t, height, status.lastNotified)
		assert.True(t, status.firstNotificationSent)
		assert.Len(t, status.outstandingNotifications, 0)
	})

	t.Run("zero handling - after first notification treated as duplicate", func(t *testing.T) {
		height := uint64(0)
		status := &status{
			firstNotificationSent: true,
			outstandingNotifications: map[uint64]flow.Identifier{
				height: unittest.IdentifierFixture(),
			},
		}

		err := status.Notified(height)
		assert.ErrorIs(t, err, ErrDuplicateNotification)

		assert.Equal(t, height, status.lastNotified)
		assert.Len(t, status.outstandingNotifications, 1)
	})
}

func TestNotificationTransitions(t *testing.T) {
	singleTestRun := func(t *testing.T) {
		t.Parallel()

		heightCount := 1000
		heights := make([]uint64, heightCount)
		for i := range heights {
			heights[i] = uint64(i)
		}

		status := &status{
			// firstNotificationSent:    true,
			outstandingNotifications: map[uint64]flow.Identifier{},
			missingHeights:           map[uint64]bool{},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		notifications := engine.NewNotifier()

		done := make(chan bool)
		go func() {
			defer close(done)
			for {
				select {
				case <-ctx.Done():
					return
				case <-notifications.Channel():
				}

				// process all available notifications
				for {
					height, _, ok := status.NextNotification()
					if !ok {
						break
					}

					err := status.Notified(height)
					assert.NoError(t, err)

					if height == uint64(heightCount)-1 {
						return // last notification
					}
				}
			}
		}()

		// Mark random heights as fetched
		rand.Seed(time.Now().UnixNano())
		for i := 0; i < heightCount; i++ {
			index := rand.Intn(len(heights))
			height := heights[index]

			heights = append(heights[:index], heights[index+1:]...)

			status.Fetched(height, unittest.IdentifierFixture())
			notifications.Notify()
		}

		// Wait until all notifications are processed, or timeout
		select {
		case <-ctx.Done():
			t.Fatalf("timed out waiting to notify for block %d", status.lastNotified+1)
		case <-done:
		}
	}

	for r := 0; r < 100; r++ {
		t.Run(fmt.Sprintf("run %d", r), singleTestRun)
	}
}
