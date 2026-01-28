package engine

import (
	"github.com/onflow/flow-go/engine/common/fifoqueue"
)

// FifoMessageStore wraps a FiFo Queue to implement the MessageStore interface.
type FifoMessageStore struct {
	*fifoqueue.FifoQueue
}

var _ MessageStore = (*FifoMessageStore)(nil)

// NewFifoMessageStore creates a FifoMessageStore backed by a [fifoqueue.FifoQueue].
// No errors are expected during normal operations.
func NewFifoMessageStore(maxCapacity int) (*FifoMessageStore, error) {
	queue, err := fifoqueue.NewFifoQueue(maxCapacity)
	if err != nil {
		return nil, err
	}
	return &FifoMessageStore{FifoQueue: queue}, nil
}

// Put appends the given value to the tail of the queue.
// Returns true if and only if the element was added, or false if the
// element was dropped due the queue being full.
// Elements successfully added to the queue stay in the queue until they
// are popped by a Get() call. In other words, a return value of `true`
// implies that the message will eventually be processed. This provides
// stronger guarantees than minimally required by the [MessageStore] interface.
func (s *FifoMessageStore) Put(msg *Message) bool {
	return s.Push(msg)
}

// Get retrieves the next message from the head of the queue. It returns
// true if a message is retrieved, and false if the message store is empty.
func (s *FifoMessageStore) Get() (*Message, bool) {
	msgint, ok := s.Pop()
	if !ok {
		return nil, false
	}
	return msgint.(*Message), true
}
