package engine

import (
	"github.com/onflow/flow-go/engine/common/fifoqueue"
)

// FifoMessageStore wraps a FiFo Queue to implement the MessageStore interface.
type FifoMessageStore struct {
	*fifoqueue.FifoQueue
}

// NewFifoMessageStore creates a FifoMessageStore backed by a fifoqueue.FifoQueue.
// No errors are expected during normal operations.
func NewFifoMessageStore(maxCapacity int) (*FifoMessageStore, error) {
	queue, err := fifoqueue.NewFifoQueue(maxCapacity)
	if err != nil {
		return nil, err
	}
	return &FifoMessageStore{FifoQueue: queue}, nil
}

func (s *FifoMessageStore) Put(msg *Message) bool {
	return s.Push(msg)
}

func (s *FifoMessageStore) Get() (*Message, bool) {
	msgint, ok := s.Pop()
	if !ok {
		return nil, false
	}
	return msgint.(*Message), true
}
