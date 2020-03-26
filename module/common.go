package module

// ReadyDoneAware provides easy interface to wait for module startup and shutdown
type ReadyDoneAware interface {
	Ready() <-chan struct{}
	Done() <-chan struct{}
}
