package broadcast

import (
	"fmt"
	"sync"

	"github.com/dapperlabs/flow-go/module"
)

const defaultSubscriptionBufferSize = 10

type Subscription struct {
	id          int
	ch          <-chan struct{}
	unsubscribe func() error
}

func (s *Subscription) Ch() <-chan struct{} {
	return s.ch
}

func (s *Subscription) Unsubscribe() error {
	return s.unsubscribe()
}

type Broadcaster struct {
	// mapping of subscription IDs to subscription channels
	subscriptions map[int]chan struct{}
	// mutex protecting adding/removing subscribers and broadcasts
	mu sync.Mutex
	// the next subscription ID, IDs increment for each new subscriber
	nextID int
	// the size of subscribers' buffer channels
	subBufSize uint
}

type Opt func(*Broadcaster)

func WithBufferSize(size uint) Opt {
	return func(broadcaster *Broadcaster) {
		broadcaster.subBufSize = size
	}
}

func NewBroadcaster(opts ...Opt) *Broadcaster {
	b := &Broadcaster{
		subscriptions: make(map[int]chan struct{}),
		subBufSize:    defaultSubscriptionBufferSize,
	}

	for _, apply := range opts {
		apply(b)
	}

	return b
}

func (b *Broadcaster) Subscribe() module.Subscription {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.nextID++
	id := b.nextID

	ch := make(chan struct{}, b.subBufSize)
	b.subscriptions[id] = ch

	unsubscribe := func() error {
		return b.unsubscribe(id)
	}

	sub := Subscription{
		id:          id,
		ch:          ch,
		unsubscribe: unsubscribe,
	}

	return &sub
}

func (b *Broadcaster) Broadcast() {
	b.mu.Lock()
	defer b.mu.Unlock()

	for _, ch := range b.subscriptions {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

func (b *Broadcaster) unsubscribe(id int) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	ch, ok := b.subscriptions[id]
	if !ok {
		return fmt.Errorf("cannot unsubscribe non-existent subscription")
	}

	close(ch)
	delete(b.subscriptions, id)
	return nil
}
