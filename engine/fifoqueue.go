package engine

import (
	"github.com/onflow/flow-go/engine/common/fifoqueue"
)

// FifoMessageStore wraps a FiFo Queue to implement the MessageStore interface.
type FifoMessageStore struct {
	*fifoqueue.FifoQueue
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
