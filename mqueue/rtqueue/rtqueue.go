package rtqueue

import (
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/mqueue"

	"github.com/onflow/flow-go/utils/fifoqueue"
)

// Queue is a real-time queue which guarantees that items added to the queue
// will immediately be available to be received over Recv channel.
type Queue interface {
	Add(item mqueue.Message)

	Recv() <-chan mqueue.Message
}

type queue struct {
	items      *fifoqueue.FifoQueue
	in, out    chan mqueue.Message
	shovelling *atomic.Bool
}

func New() Queue {
	fifo, _ := fifoqueue.NewFifoQueue()
	q := &queue{
		items:      fifo,
		in:         make(chan mqueue.Message),
		out:        make(chan mqueue.Message),
		shovelling: atomic.NewBool(false),
	}
	return q
}

// shovel is a goroutine that lives for as long as the queue is non-empty.
func (q *queue) shovel() {

	// shovel is always started when at least one item is being added
	head := <-q.in

	for {
		// if the queue is non-empty wait on both input and output channels
		select {
		case tail := <-q.in:
			q.items.Push(tail)
			continue
		case q.out <- head:
		}

		next, ok := q.items.Pop()
		if !ok {
			q.shovelling.Store(false)
			return
		}
		head = next.(mqueue.Message)
	}
}

func (q *queue) Add(item mqueue.Message) {
	if q.shovelling.CAS(false, true) {
		go q.shovel()
	}
	q.in <- item
}

func (q *queue) Recv() <-chan mqueue.Message {
	return q.out
}
