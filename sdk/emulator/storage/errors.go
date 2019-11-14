package storage

// ErrNotFound is an error returned when a resource cannot be found.
type ErrNotFound struct{}

func (e ErrNotFound) Error() string {
	return "emulator/store: could not find resource"
}
