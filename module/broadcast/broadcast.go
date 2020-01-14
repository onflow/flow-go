package broadcast

import (
	"sync"

	"github.com/dapperlabs/flow-go/module"
)

type Subscription struct {
	id          int
	ch          <-chan struct{}
	unsubscribe func()
}

func (s *Subscription) Ch() <-chan struct{} {
	return s.ch
}

func (s *Subscription) Unsubscribe() {
	s.unsubscribe()
}

type Broadcaster struct {
	// mapping of subscription IDs to subscription channels
	subscriptions map[int]chan struct{}
	// mutex protecting adding/removing subscribers and broadcasts
	mu sync.Mutex
	// the next subscription ID, IDs increment for each new subscriber
	nextID int
}

func NewBroadcaster() *Broadcaster {
	b := &Broadcaster{
		subscriptions: make(map[int]chan struct{}),
	}

	return b
}

func (b *Broadcaster) Subscribe() module.Subscription {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.nextID++
	id := b.nextID

	ch := make(chan struct{}, 1)
	b.subscriptions[id] = ch

	unsubscribe := func() {
		b.unsubscribe(id)
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

func (b *Broadcaster) unsubscribe(id int) {
	b.mu.Lock()
	defer b.mu.Unlock()

	ch, ok := b.subscriptions[id]
	if !ok {
		return
	}

	close(ch)
	delete(b.subscriptions, id)
}
