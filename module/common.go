package module

// ReadyDoneAware provides easy interface to wait for module startup and shutdown.
// Modules that implement this interface only support a single start-stop cycle,
// and if Ready() is called again after shutdown has already commenced, the module
// will not restart.
type ReadyDoneAware interface {
	// Ready commences startup of the module, and returns a ready channel that is closed once
	// startup has completed.
	Ready() <-chan struct{}

	// Done commences shutdown of the module, and returns a done channel that is closed once
	// shutdown has completed.
	Done() <-chan struct{}
}
