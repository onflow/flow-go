package engine

import "sync"

type Broadcaster struct {
	subscribers []Notifier
	mu          sync.RWMutex
}

func NewBroadcaster() *Broadcaster {
	return &Broadcaster{}
}

func (b *Broadcaster) Subscribe() *Notifier {
	b.mu.Lock()
	defer b.mu.Unlock()

	n := NewNotifier()
	b.subscribers = append(b.subscribers, n)
	return &n
}

func (b *Broadcaster) Publish() {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for _, n := range b.subscribers {
		n.Notify()
	}
}
