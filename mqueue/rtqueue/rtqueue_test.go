package rtqueue

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/mqueue"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestQueueNonBlockingAdd(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		q := New()

		unittest.AssertReturnsBefore(t, func() {
			q.Add(mqueue.FakeMessage())
		}, time.Second)
	})

	t.Run("non-empty", func(t *testing.T) {
		q := New()

		unittest.AssertReturnsBefore(t, func() {
			q.Add(mqueue.FakeMessage())
			q.Add(mqueue.FakeMessage())
		}, time.Second)
	})
}

func TestQueueReadWrite(t *testing.T) {
	q := New()

	expected := mqueue.FakeMessage()
	q.Add(expected)

	unittest.AssertReturnsBefore(t, func() {
		actual := <-q.Recv()
		assert.Equal(t, expected.Payload(), actual.Payload())
	}, time.Second)
}

func TestReadEmpty(t *testing.T) {
	q := New()

	select {
	case <-q.Recv():
		t.Fail()
	default:
	}
}
