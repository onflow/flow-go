package queue_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/network/gossip/libp2p/queue"
)

// TestSingleQueueWorkers tests that a single worker can successfully read all elements from the queue
func TestSingleQueueWorker(t *testing.T) {
	testWorkers(t, 10, 100, 1)
}

// TestMultipleQueueWorkers tests that multiple workers can successfully read all elements from the queue
func TestMultipleQueueWorkers(t *testing.T) {
	for i := 2; i < 10; i++ {
		testWorkers(t, 10, 100, i)
	}
}

// testWorkers tests that with the given max priority, message count and worker count, a queue can be successfully read.
// workerCnt should not be more than maxPriority for this test
func testWorkers(t *testing.T, maxPriority int, messageCnt int, workerCnt int) {

	assert.LessOrEqual(t, workerCnt, maxPriority)

	// the priority function just returns the message as the priority itself (message = priority)
	var q queue.MessageQueue = queue.NewMessageQueue(func(m interface{}) queue.Priority {
		i, ok := m.(int)
		assert.True(t, ok)
		return queue.Priority(i)
	})

	msgCntPerPr := messageCnt / maxPriority // messages per priority
	expectedPriority := maxPriority - 1     // when dequeing, the priority can be the current highest priority or one less
	callbackCnt := 0                        //count the number of times the callback gets called
	// callback checks if message is of expected priority
	callback := func(data interface{}) {
		actual := data.(int)
		assert.LessOrEqual(t, expectedPriority, actual)
		callbackCnt++
		if callbackCnt%msgCntPerPr == 0 {
			expectedPriority--
		}
	}

	// the queue is populated with messageCnt number of messages
	// each message is an int which is also its priority
	// messages are inserted in increasing order of priority
	// e.g. 1,2,3...10,1,2,3,..10,....messagecnt
	for i := 0; i < messageCnt; i++ {
		priority := (i + 1) % maxPriority
		if priority == 0 {
			priority = maxPriority
		}
		err := q.Insert(priority)
		assert.NoError(t, err)
	}

	// create all the workers
	queue.CreateQueueWorkers(context.Background(), uint64(workerCnt), q, callback)

	// check that callback was eventually called expected number of times
	assert.Eventually(t, func() bool { return callbackCnt == messageCnt }, time.Second, 5*time.Millisecond)
}
