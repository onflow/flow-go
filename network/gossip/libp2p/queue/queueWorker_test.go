package queue

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSingleQueueWorker(t *testing.T) {
	var q MessageQueue = NewMessageQueue()
	messageCnt := 10
	workerCnt := 5

	index := messageCnt
	callback := func(data interface{}) {
		actual := data.(int)
		expected := messageCnt % workerCnt
		assert.GreaterOrEqual(t, expected, actual)
		index = index - 1
	}

	for i:=1;i<messageCnt;i++ {
		q.Insert(i, i)
	}

	CreateQueueWorkers(context.Background(), 1, q, callback)

	assert.Eventually(t, func() bool { return index == 0}, time.Second, 5 * time.Millisecond)
}
