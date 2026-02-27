package iterator

// Entry is a single stored entry returned by an index iterator.
type Entry[T any, C any] struct {
	cursor   C
	getValue func(C) (*T, error)
}

func NewEntry[T any, C any](cursor C, getValue func(C) (*T, error)) Entry[T, C] {
	return Entry[T, C]{
		cursor:   cursor,
		getValue: getValue,
	}
}

// Cursor returns the cursor for the entry, which includes all data included in the storage key.
func (i Entry[T, C]) Cursor() C {
	return i.cursor
}

// Value returns the fully reconstructed value for the entry.
//
// Any error indicates the value cannot be reconstructed from the storage value.
func (i Entry[T, C]) Value() (T, error) {
	v, err := i.getValue(i.cursor)
	if err != nil {
		var zero T
		return zero, err
	}
	return *v, nil
}
