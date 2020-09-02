package queue_test

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/network/gossip/libp2p/queue"
)

func TestSingleQueueWorkers(t *testing.T) {
	testWorkers(t, 10, 100, 1)
}

func TestMultipleQueueWorkers(t *testing.T) {
	for i:=uint64(1);i<10;i++ {
		testWorkers(t, 10, 100, i)
	}
}

func testWorkers(t *testing.T, maxPriority int, messageCnt int, workerCnt uint64) {
	var q queue.MessageQueue = queue.NewMessageQueue(func (m interface{}) (queue.Priority, error) {
		i, err := strconv.Atoi(m.(string))
		return queue.Priority(i), err
	})
	msgCntPerPr := messageCnt / maxPriority

	expectedPriority := maxPriority -1
	callbackCnt := 0
	callback := func(data interface{}) {
		actual := data.(int)
		fmt.Println(actual)
		assert.LessOrEqual(t, expectedPriority, actual)
		callbackCnt++
		if callbackCnt % msgCntPerPr == 0{
			expectedPriority--
		}
	}

	for i:=0;i<messageCnt;i++ {
		priority := (i + 1) % maxPriority
		if priority == 0 {
			priority = maxPriority
		}
		q.Insert(priority)
	}

	queue.CreateQueueWorkers(context.Background(), workerCnt, q, callback)

	assert.Eventually(t, func() bool { return callbackCnt == messageCnt}, time.Second, 5 * time.Millisecond)
}
