package queue_test

import (
	"math/rand"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/network/gossip/libp2p/queue"
)

func TestPushAndPop(t *testing.T) {
	// Some items and their priorities.
	items := map[int]string{
		3: "banana", 2: "apple", 4: "pear",
	}

	mq := queue.NewMessageQueue()

	for k, v := range items {
		mq.Insert(v, k)
	}

	assert.Equal(t, len(items), mq.Len())

	keys := make([]int, len(items))
	i := 0
	for k := range items {
		keys[i] = k
		i++
	}
	sort.Sort(sort.Reverse(sort.IntSlice(keys)))

	for _, i := range keys {
		item := mq.Remove()
		assert.Equal(t, items[i], item.(string))
	}
}

var mq = queue.NewMessageQueue()

func  BenchmarkPush(b *testing.B) {
	b.StopTimer()
	for i := 0; i < b.N; i++ {
		mq.Insert("test", rand.Intn(9) + 1)
	}
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		mq.Insert("test", rand.Intn(9) + 1)
	}
}

func  BenchmarkPop(b *testing.B) {
	b.StopTimer()
	for i := 0; i < b.N; i++ {
		mq.Insert("test", rand.Intn(9) + 1)
	}
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		mq.Remove()
	}
}
