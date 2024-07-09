package consensus

func WithCleanup(cleanup CleanupFunc) func(*Finalizer) {
	return func(f *Finalizer) {
		f.cleanup = cleanup
	}
}

func WithCleanupPebble(cleanup CleanupFunc) func(*FinalizerPebble) {
	return func(f *FinalizerPebble) {
		f.cleanup = cleanup
	}
}
