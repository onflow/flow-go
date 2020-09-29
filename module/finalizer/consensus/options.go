package consensus

func WithCleanup(cleanup CleanupFunc) func(*Finalizer) {
	return func(f *Finalizer) {
		f.cleanup = cleanup
	}
}
