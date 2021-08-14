package module

// ReadyDoneAware provides easy interface to wait for module startup and shutdown
type ReadyDoneAware interface {
	Ready() <-chan struct{}
	Done() <-chan struct{}
}

type NoopReadDoneAware struct{}

func (n *NoopReadDoneAware) Ready() <-chan struct{} {
	ready := make(chan struct{})
	defer close(ready)
	return ready
}

func (n *NoopReadDoneAware) Done() <-chan struct{} {
	done := make(chan struct{})
	defer close(done)
	return done
}
