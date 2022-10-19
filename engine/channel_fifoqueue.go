package engine

import (
	"fmt"
	"math"
	"sync"

	"github.com/ef-ds/deque"
)

// ChannelFifoQueue implements a FIFO queue with max capacity and length observer.
// Elements that exceeds the queue's max capacity are silently dropped.
// By default, the theoretical capacity equals to the largest `int` value
// (platform dependent). Capacity can be set at construction time via the
// option `WithCapacity`.
// Each time the queue's length changes, the QueueLengthObserver is called
// with the new length. By default, the QueueLengthObserver is a NoOp.
// A single QueueLengthObserver can be set at construction time via the
// option `WithLengthObserver`.
//
// Caution:
// * The queue is NOT concurrency safe.
// * the QueueLengthObserver must be non-blocking
type ChannelFifoQueue struct {
	maxCapacity    uint32
	lengthObserver QueueLengthObserver

	closeOnce   sync.Once
	tailChannel chan interface{}
	headChannel chan interface{}
}

// ConstructorOptions can are optional arguments for the `NewChannelFifoQueue`
// constructor to specify properties of the ChannelFifoQueue.
type ConstructorOption func(*ChannelFifoQueue) error

// QueueLengthObserver is a callback that can optionally provided
// to the `NewChannelFifoQueue` constructor (via `WithLengthObserver` option).
// Caution: implementation must be non-blocking and concurrency safe.
type QueueLengthObserver func(uint32)

// WithCapacity is a constructor option for NewChannelFifoQueue. It specifies the
// max number of elements the queue can hold. By default, the theoretical
// capacity equals to the largest `int` value (platform dependent).
// The WithCapacity option overrides the previous value (default value or
// value specified by previous option).
func WithCapacity(capacity uint32) ConstructorOption {
	return func(queue *ChannelFifoQueue) error {
		if capacity < 1 {
			return fmt.Errorf("capacity for Fifo queue must be positive")
		}
		queue.maxCapacity = capacity
		return nil
	}
}

// WithLengthObserver is a constructor option for NewChannelFifoQueue. Each time the
// queue's length changes, the queue calls the provided callback with the new
// length. By default, the QueueLengthObserver is a NoOp.
// Caution: the QueueLengthObserver callback must be non-blocking and concurrency safe.
func WithLengthObserver(callback QueueLengthObserver) ConstructorOption {
	return func(queue *ChannelFifoQueue) error {
		if callback == nil {
			return fmt.Errorf("nil is not a valid QueueLengthObserver")
		}
		queue.lengthObserver = callback
		return nil
	}
}

// Constructor for ChannelFifoQueue
func NewChannelFifoQueue(options ...ConstructorOption) (*ChannelFifoQueue, error) {
	queue := &ChannelFifoQueue{
		maxCapacity:    math.MaxUint32,
		lengthObserver: func(uint32) { /* noop */ },
		tailChannel:    make(chan interface{}),
		headChannel:    make(chan interface{}),
	}
	for _, opt := range options {
		err := opt(queue)
		if err != nil {
			return nil, fmt.Errorf("failed to apply constructor option to fifoqueue queue: %w", err)
		}
	}

	go queue.shovel()
	return queue, nil
}

// shovel() moves elements from the tail channel into an internal queue and then into the head channel.
// Implementation is inspired by:
// https://medium.com/capital-one-tech/building-an-unbounded-channel-in-go-789e175cd2cd
func (q *ChannelFifoQueue) shovel() {
	var queue deque.Deque
	in := q.tailChannel
	out := q.headChannel
	maxCapacity := q.maxCapacity
	lengthObserver := q.lengthObserver
	var size uint32 = 0

	push2Queue := func(element interface{}) {
		if size >= maxCapacity {
			return // drops element
		}
		size++
		queue.PushBack(element)
		lengthObserver(size)
	}

	for queue.Len() > 0 || in != nil {
		if queue.Len() == 0 {
			element, ok := <-in
			if !ok { // inbound channel was closed: terminate
				break
			}
			push2Queue(element)
		} else {
			item, _ := queue.Front()
			select {
			case element, ok := <-in:
				if !ok { // inbound channel was closed: terminate
					break
				}
				push2Queue(element)
			case out <- item:
				size--
				queue.PopFront()
			}
		}
	}

	close(out)
}

func (q *ChannelFifoQueue) TailChannel() chan<- interface{} {
	return q.tailChannel
}

func (q *ChannelFifoQueue) HeadChannel() <-chan interface{} {
	return q.headChannel
}
