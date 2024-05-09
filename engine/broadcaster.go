package engine

import "sync"

// Notifiable is an interface for objects that can be notified
type Notifiable interface {
	// Notify sends a notification. This method must be concurrency safe and non-blocking.
	// It is expected to be a Notifier object, but does not have to be.
	Notify()
}

// Broadcaster is a distributor for Notifier objects. It implements a simple generic pub/sub pattern.
// Callers can subscribe to single-channel notifications by passing a Notifier object to the Subscribe
// method. When Publish is called, all subscribers are notified.
type Broadcaster struct {
	subscribers []Notifiable
	mu          sync.RWMutex
}

// NewBroadcaster creates a new Broadcaster
func NewBroadcaster() *Broadcaster {
	return &Broadcaster{}
}

// Subscribe adds a Notifier to the list of subscribers to be notified when Publish is called
func (b *Broadcaster) Subscribe(n Notifiable) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.subscribers = append(b.subscribers, n)
}

// Publish sends notifications to all subscribers
func (b *Broadcaster) Publish() {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for _, n := range b.subscribers {
		n.Notify()
	}
}
