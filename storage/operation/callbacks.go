package operation

import "sync"

type Callbacks struct {
	sync.Mutex // protect callbacks
	callbacks  []func(error)
}

func (b *Callbacks) AddCallback(callback func(error)) {
	b.Lock()
	defer b.Unlock()

	b.callbacks = append(b.callbacks, callback)
}

func (b *Callbacks) NotifyCallbacks(err error) {
	b.Lock()
	defer b.Unlock()

	for _, callback := range b.callbacks {
		callback(err)
	}
}
