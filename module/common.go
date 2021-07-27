package module

// ReadyDoneAware provides easy interface to wait for module startup and shutdown.
// Modules that implement this interface only support a single start-stop cycle, and
// will not restart if Ready() is called again after shutdown has already commenced.
type ReadyDoneAware interface {
	// Ready commences startup of the module, and returns a ready channel that is closed once
	// startup has completed.
	// If shutdown has already commenced before this method is called for the first time,
	// startup will not be performed and the returned channel will never close.
	// This is an idempotent method.
	Ready() <-chan struct{}

	// Done commences shutdown of the module, and returns a done channel that is closed once
	// shutdown has completed.
	// This is an idempotent method.
	Done() <-chan struct{}
}
