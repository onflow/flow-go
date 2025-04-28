package operation

import "sync"

type Callbacks struct {
	sync.RWMutex // protect callbacks
	callbacks    []func(error)
}

func NewCallbacks() *Callbacks {
	return &Callbacks{}
}

func (b *Callbacks) AddCallback(callback func(error)) {
	b.Lock()
	defer b.Unlock()

	b.callbacks = append(b.callbacks, callback)
}

func (b *Callbacks) NotifyCallbacks(err error) {
	b.RLock()
	defer b.RUnlock()

	for _, callback := range b.callbacks {
		callback(err)
	}
}
