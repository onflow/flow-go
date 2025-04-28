package operation

// Callbacks represents a collection of callbacks to be executed.
// Callbacks are not concurrent safe.
// Since Callbacks is only used in ReaderBatchWriter, which
// isn't concurrent safe, there isn't a need to add locking
// overhead to Callbacks.
type Callbacks struct {
	callbacks []func(error)
}

func (b *Callbacks) AddCallback(callback func(error)) {
	b.callbacks = append(b.callbacks, callback)
}

func (b *Callbacks) NotifyCallbacks(err error) {
	for _, callback := range b.callbacks {
		callback(err)
	}
}
